package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

// Source → external_id key mapping (consistent with ZK import_primekg_to_db.py).
var sourceToIDKey = map[string]string{
	"DrugBank": "drugbank",
	"MONDO":    "mondo",
	"NCBI":     "ncbi_gene",
	"Reactome": "reactome",
}

type graphProjectionService struct {
	entityRepo   interfaces.GraphEntityRepository
	relationRepo interfaces.GraphRelationRepository
	neo4j        neo4j.Driver
}

// NewGraphProjectionService creates a new DB→Neo4j projection service.
func NewGraphProjectionService(
	entityRepo interfaces.GraphEntityRepository,
	relationRepo interfaces.GraphRelationRepository,
	neo4j neo4j.Driver,
) interfaces.GraphProjectionService {
	return &graphProjectionService{
		entityRepo:   entityRepo,
		relationRepo: relationRepo,
		neo4j:        neo4j,
	}
}

// ProjectEntities reads pending entities, writes them to Neo4j, and updates sync_status.
// For primekg entities (source_site=primekg), also creates a REFERENCES edge to the
// corresponding PrimeKG original node in Neo4j.
func (s *graphProjectionService) ProjectEntities(
	ctx context.Context, tenantID uint64, kbID string, limit int,
) (int, error) {
	if s.neo4j == nil {
		return 0, fmt.Errorf("Neo4j 未配置")
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.entityRepo.ListForProjection(ctx, tenantID, kbID, limit)
	if err != nil {
		return 0, fmt.Errorf("查询待同步实体: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	var synced int
	for _, entity := range rows {
		if entity.IsDeleted {
			if err := s.deleteEntityInNeo4j(ctx, session, entity); err != nil {
				logger.Warnf(ctx, "[GraphProjection] 删除实体失败 %s: %v", entity.ID, err)
				s.entityRepo.MarkFailed(ctx, entity.ID, fmt.Sprintf("Neo4j 删除失败: %v", err))
				continue
			}
			s.entityRepo.MarkDeleted(ctx, entity.ID, time.Now())
			synced++
			continue
		}

		if err := s.mergeEntityInNeo4j(ctx, session, entity); err != nil {
			logger.Warnf(ctx, "[GraphProjection] 同步实体失败 %s (%s): %v", entity.ID, entity.EntityName, err)
			s.entityRepo.MarkFailed(ctx, entity.ID, fmt.Sprintf("Neo4j MERGE 失败: %v", err))
			continue
		}
		s.entityRepo.MarkSynced(ctx, entity.ID, "", time.Now())
		synced++

		// For primekg entities, create REFERENCES edge to PrimeKG original node.
		if entity.SourceSite == "primekg" {
			if err := s.createReferencesEdge(ctx, session, entity); err != nil {
				logger.Warnf(ctx, "[GraphProjection] REFERENCES 边创建失败 %s: %v", entity.ID, err)
			}
		}
	}

	logger.Infof(ctx, "[GraphProjection] 实体同步完成: synced=%d total=%d kb=%s", synced, len(rows), kbID)
	return synced, nil
}

// mergeEntityInNeo4j creates or updates a GraphEntity node in Neo4j.
func (s *graphProjectionService) mergeEntityInNeo4j(
	ctx context.Context, session neo4j.Session, entity *types.GraphEntity,
) error {
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MERGE (n:GraphEntity {source_entity_id: $source_entity_id})
			SET n.entity_type = $entity_type,
			    n.entity_name = $entity_name,
			    n.tenant_id = $tenant_id,
			    n.knowledge_base_id = $knowledge_base_id
		`
		params := map[string]interface{}{
			"source_entity_id":  entity.SourceEntityID,
			"entity_type":       entity.EntityType,
			"entity_name":       entity.EntityName,
			"tenant_id":         entity.TenantID,
			"knowledge_base_id": entity.KnowledgeBaseID,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// deleteEntityInNeo4j removes a GraphEntity node and its relationships from Neo4j.
func (s *graphProjectionService) deleteEntityInNeo4j(
	ctx context.Context, session neo4j.Session, entity *types.GraphEntity,
) error {
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `MATCH (n:GraphEntity {source_entity_id: $source_entity_id}) DETACH DELETE n`
		_, err := tx.Run(ctx, query, map[string]interface{}{
			"source_entity_id": entity.SourceEntityID,
		})
		return nil, err
	})
	return err
}

// createReferencesEdge connects a ZK entity copy to its PrimeKG original node in Neo4j.
// It tries all known source→ID key mappings against the entity's external_ids.
func (s *graphProjectionService) createReferencesEdge(
	ctx context.Context, session neo4j.Session, entity *types.GraphEntity,
) error {
	primekgNodeID := resolvePrimeKGNodeID(entity.EntityData)
	if primekgNodeID == "" {
		return nil // No external IDs to match — nothing to link.
	}

	parts := split2(primekgNodeID, ':')
	if parts[0] == "" || parts[1] == "" {
		return nil
	}
	nodeSource, primekgID := parts[0], parts[1]

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (zk:GraphEntity {source_entity_id: $zk_seid})
			MATCH (p {primekg_id: $primekg_id, node_source: $node_source})
			MERGE (zk)-[:REFERENCES]->(p)
		`
		_, err := tx.Run(ctx, query, map[string]interface{}{
			"zk_seid":     entity.SourceEntityID,
			"primekg_id":  primekgID,
			"node_source": nodeSource,
		})
		return nil, err
	})
	return err
}

// resolvePrimeKGNodeID extracts a PrimeKG Neo4j node identifier from entity_data.
// Format: {Source}:{ID}, e.g. "NCBI:9796" or "DrugBank:DB0001".
func resolvePrimeKGNodeID(entityData types.JSON) string {
	var data map[string]interface{}
	if err := json.Unmarshal(json.RawMessage(entityData), &data); err != nil {
		return ""
	}

	rawIDs, ok := data["external_ids"]
	if !ok {
		return ""
	}
	extIDs, ok := rawIDs.(map[string]interface{})
	if !ok {
		return ""
	}

	// Try known source→key mappings first.
	for src, key := range sourceToIDKey {
		if v, ok := extIDs[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return fmt.Sprintf("%s:%s", src, s)
			}
		}
	}

	// Fallback: use any external_id entry.
	for k, v := range extIDs {
		if s, ok := v.(string); ok && s != "" {
			return fmt.Sprintf("%s:%s", k, s)
		}
	}
	return ""
}

// ── Relation projection ──

// ProjectRelations reads pending relations, writes them to Neo4j, and updates sync_status.
// It validates relation types through the whitelist before constructing Cypher.
func (s *graphProjectionService) ProjectRelations(
	ctx context.Context, tenantID uint64, kbID string, limit int,
) (int, error) {
	if s.neo4j == nil {
		return 0, fmt.Errorf("Neo4j 未配置")
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.relationRepo.ListForProjection(ctx, tenantID, kbID, limit)
	if err != nil {
		return 0, fmt.Errorf("查询待同步关系: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	var synced int
	for _, rel := range rows {
		// Validate relation type before any Cypher that interpolates it.
		if err := SanitizeRelationType(rel.RelationType); err != nil {
			s.relationRepo.MarkFailed(ctx, rel.ID, fmt.Sprintf("关系类型校验失败: %v", err))
			continue
		}

		if rel.IsDeleted {
			if err := s.deleteRelationInNeo4j(ctx, session, rel); err != nil {
				logger.Warnf(ctx, "[GraphProjection] 删除关系失败 %s: %v", rel.ID, err)
				s.relationRepo.MarkFailed(ctx, rel.ID, fmt.Sprintf("Neo4j 删除失败: %v", err))
				continue
			}
			if err := s.relationRepo.MarkSynced(ctx, rel.ID, "", time.Now()); err != nil {
				logger.Warnf(ctx, "[GraphProjection] 标记关系已删除失败 %s: %v", rel.ID, err)
			}
			synced++
			continue
		}

		if err := s.mergeRelationInNeo4j(ctx, session, rel); err != nil {
			logger.Warnf(ctx, "[GraphProjection] 同步关系失败 %s: %v", rel.ID, err)
			s.relationRepo.MarkFailed(ctx, rel.ID, fmt.Sprintf("Neo4j MERGE 失败: %v", err))
			continue
		}
		s.relationRepo.MarkSynced(ctx, rel.ID, "", time.Now())
		synced++
	}

	logger.Infof(ctx, "[GraphProjection] 关系同步完成: synced=%d total=%d kb=%s", synced, len(rows), kbID)
	return synced, nil
}

// mergeRelationInNeo4j creates or updates a relationship in Neo4j.
// The relation type has already been validated by the whitelist.
func (s *graphProjectionService) mergeRelationInNeo4j(
	ctx context.Context, session neo4j.Session, rel *types.GraphRelationRecord,
) error {
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// Relation type is safe to interpolate — validated by SanitizeRelationType.
		query := fmt.Sprintf(`
			MATCH (from:GraphEntity {source_entity_id: $from_seid})
			MATCH (to:GraphEntity {source_entity_id: $to_seid})
			MERGE (from)-[r:%s {source_relation_id: $srid}]->(to)
		`, rel.RelationType)
		_, err := tx.Run(ctx, query, map[string]interface{}{
			"from_seid": rel.FromEntityID,
			"to_seid":   rel.ToEntityID,
			"srid":      rel.SourceRelationID,
		})
		return nil, err
	})
	return err
}

// deleteRelationInNeo4j removes a relationship from Neo4j.
func (s *graphProjectionService) deleteRelationInNeo4j(
	ctx context.Context, session neo4j.Session, rel *types.GraphRelationRecord,
) error {
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// SanitizeRelationType already validated the type is safe to interpolate.
		query := fmt.Sprintf(`
			MATCH (:GraphEntity {source_entity_id: $from_seid})-[r:%s {source_relation_id: $srid}]->(:GraphEntity {source_entity_id: $to_seid})
			DELETE r
		`, rel.RelationType)
		_, err := tx.Run(ctx, query, map[string]interface{}{
			"from_seid": rel.FromEntityID,
			"to_seid":   rel.ToEntityID,
			"srid":      rel.SourceRelationID,
		})
		return nil, err
	})
	return err
}

func split2(s string, sep byte) [2]string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}
