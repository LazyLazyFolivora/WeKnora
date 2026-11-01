package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"gorm.io/gorm"
)

// Source → external_id key mapping (consistent with ZK import_primekg_to_db.py).
var sourceToIDKey = map[string]string{
	"DrugBank": "drugbank",
	"MONDO":    "mondo",
	"NCBI":     "ncbi_gene",
	"Reactome": "reactome",
}

type graphProjectionService struct {
	db           *gorm.DB
	entityRepo   interfaces.GraphEntityRepository
	relationRepo interfaces.GraphRelationRepository
	neo4j        neo4j.Driver
}

// NewGraphProjectionService creates a new DB→Neo4j projection service.
func NewGraphProjectionService(
	db *gorm.DB,
	entityRepo interfaces.GraphEntityRepository,
	relationRepo interfaces.GraphRelationRepository,
	neo4j neo4j.Driver,
) interfaces.GraphProjectionService {
	return &graphProjectionService{
		db:           db,
		entityRepo:   entityRepo,
		relationRepo: relationRepo,
		neo4j:        neo4j,
	}
}

// ProjectEntities reads pending entities, writes them to Neo4j in batches, and updates sync_status.
func (s *graphProjectionService) ProjectEntities(
	ctx context.Context, tenantID uint64, limit int,
) (int, error) {
	if s.neo4j == nil {
		return 0, fmt.Errorf("Neo4j 未配置")
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.entityRepo.ListForProjection(ctx, tenantID, limit)
	if err != nil {
		return 0, fmt.Errorf("查询待同步实体: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	var (
		toDelete    []*types.GraphEntity
		toUpsert    []*types.GraphEntity
		primekg     []*types.GraphEntity
		toDeleteIDs []string
		toUpsertIDs []string
	)

	for _, e := range rows {
		if e.IsDeleted {
			toDelete = append(toDelete, e)
			toDeleteIDs = append(toDeleteIDs, e.ID)
		} else {
			toUpsert = append(toUpsert, e)
			toUpsertIDs = append(toUpsertIDs, e.ID)
			if e.SourceSite == "primekg" {
				primekg = append(primekg, e)
			}
		}
	}

	var synced int

	// Batch delete
	if len(toDelete) > 0 {
		count, errs := s.batchDeleteEntities(ctx, session, toDelete)
		synced += count
		for id, e := range errs {
			s.entityRepo.MarkFailed(ctx, id, e)
		}
	}

	// Batch upsert
	if len(toUpsert) > 0 {
		count, errs := s.batchMergeEntities(ctx, session, toUpsert)
		synced += count
		for id, e := range errs {
			s.entityRepo.MarkFailed(ctx, id, e)
		}
	}

	// Batch REFERENCES edges for primekg entities
	if len(primekg) > 0 {
		for _, entity := range primekg {
			if err := s.createReferencesEdge(ctx, session, entity); err != nil {
				logger.Warnf(ctx, "[GraphProjection] REFERENCES 边创建失败 %s: %v", entity.ID, err)
			}
		}
	}

	logger.Infof(ctx, "[GraphProjection] 实体同步完成: synced=%d total=%d tenant=%d", synced, len(rows), tenantID)
	return synced, nil
}

// batchMergeEntities uses UNWIND to MERGE multiple entities in one Cypher call.
func (s *graphProjectionService) batchMergeEntities(
	ctx context.Context, session neo4j.Session, entities []*types.GraphEntity,
) (synced int, errs map[string]string) {
	rows := make([]map[string]interface{}, 0, len(entities))
	for _, e := range entities {
		rows = append(rows, map[string]interface{}{
			"source_entity_id": e.SourceEntityID,
			"entity_type":      e.EntityType,
			"entity_name":      e.EntityName,
			"tenant_id":        e.TenantID,
			"entity_data":      jsonToString(e.EntityData),
			"source_site":      e.SourceSite,
			"confidence_score": e.ConfidenceScore,
		})
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			UNWIND $rows AS row
			MERGE (n:GraphEntity {source_entity_id: row.source_entity_id})
			SET n.entity_type = row.entity_type,
			    n.entity_name = row.entity_name,
			    n.tenant_id = row.tenant_id,
			    n.entity_data = row.entity_data,
			    n.source_site = row.source_site,
			    n.confidence_score = row.confidence_score
		`
		_, err := tx.Run(ctx, query, map[string]interface{}{"rows": rows})
		return nil, err
	})
	if err != nil {
		errs = make(map[string]string)
		for _, e := range entities {
			errs[e.ID] = fmt.Sprintf("Neo4j 批量MERGE失败: %v", err)
		}
		return 0, errs
	}

	now := time.Now()
	for _, e := range entities {
		s.entityRepo.MarkSynced(ctx, e.ID, "", now)
	}
	return len(entities), nil
}

// batchDeleteEntities uses UNWIND to DETACH DELETE multiple entities in one Cypher call.
func (s *graphProjectionService) batchDeleteEntities(
	ctx context.Context, session neo4j.Session, entities []*types.GraphEntity,
) (deleted int, errs map[string]string) {
	sourceIDs := make([]string, len(entities))
	for i, e := range entities {
		sourceIDs[i] = e.SourceEntityID
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			UNWIND $ids AS id
			MATCH (n:GraphEntity {source_entity_id: id})
			DETACH DELETE n
		`
		_, err := tx.Run(ctx, query, map[string]interface{}{"ids": sourceIDs})
		return nil, err
	})
	if err != nil {
		errs = make(map[string]string)
		for _, e := range entities {
			errs[e.ID] = fmt.Sprintf("Neo4j 批量删除失败: %v", err)
		}
		return 0, errs
	}

	now := time.Now()
	for _, e := range entities {
		s.entityRepo.MarkDeleted(ctx, e.ID, now)
	}
	return len(entities), nil
}

// createReferencesEdge connects a ZK entity copy to its PrimeKG original node in Neo4j.
func (s *graphProjectionService) createReferencesEdge(
	ctx context.Context, session neo4j.Session, entity *types.GraphEntity,
) error {
	sourceNode := resolvePrimeKGNodeID(s.db, entity.SourceEntityID)
	if sourceNode == "" {
		return nil
	}

	parts := split2(sourceNode, ':')
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

// resolvePrimeKGNodeID reads entity_dict.external_ids for a given source_entity_id.
func resolvePrimeKGNodeID(db *gorm.DB, sourceEntityID string) string {
	if !strings.HasPrefix(sourceEntityID, "dict:") {
		return ""
	}
	dictID, err := strconv.ParseInt(sourceEntityID[5:], 10, 64)
	if err != nil {
		return ""
	}

	var row types.EntityDictRecord
	if err := db.Where("id = ?", dictID).First(&row).Error; err != nil {
		return ""
	}

	var extIDs map[string]interface{}
	if err := json.Unmarshal(json.RawMessage(row.ExternalIDs), &extIDs); err != nil {
		return ""
	}

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

// ProjectRelations reads pending relations, writes them to Neo4j in batches, and updates sync_status.
func (s *graphProjectionService) ProjectRelations(
	ctx context.Context, tenantID uint64, limit int,
) (int, error) {
	if s.neo4j == nil {
		return 0, fmt.Errorf("Neo4j 未配置")
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.relationRepo.ListForProjection(ctx, tenantID, limit)
	if err != nil {
		return 0, fmt.Errorf("查询待同步关系: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	// Validate relation types first
	valid := make([]*types.GraphRelationRecord, 0, len(rows))
	for _, rel := range rows {
		if err := SanitizeRelationType(rel.RelationType); err != nil {
			s.relationRepo.MarkFailed(ctx, rel.ID, fmt.Sprintf("关系类型校验失败: %v", err))
			continue
		}
		valid = append(valid, rel)
	}
	if len(valid) == 0 {
		return 0, nil
	}

	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// Group by relation_type (Cypher requires literal type, can't be parameterized)
	byType := make(map[string][]*types.GraphRelationRecord)
	for _, rel := range valid {
		byType[rel.RelationType] = append(byType[rel.RelationType], rel)
	}

	var synced int
	for relType, group := range byType {
		var toUpsert, toDelete []*types.GraphRelationRecord
		for _, rel := range group {
			if rel.IsDeleted {
				toDelete = append(toDelete, rel)
			} else {
				toUpsert = append(toUpsert, rel)
			}
		}

		if len(toDelete) > 0 {
			n, errs := s.batchDeleteRelations(ctx, session, relType, toDelete)
			synced += n
			for id, e := range errs {
				s.relationRepo.MarkFailed(ctx, id, e)
			}
		}
		if len(toUpsert) > 0 {
			n, errs := s.batchMergeRelations(ctx, session, relType, toUpsert)
			synced += n
			for id, e := range errs {
				s.relationRepo.MarkFailed(ctx, id, e)
			}
		}
	}

	logger.Infof(ctx, "[GraphProjection] 关系同步完成: synced=%d total=%d tenant=%d", synced, len(rows), tenantID)
	return synced, nil
}

// batchMergeRelations uses UNWIND to MERGE multiple relations of the same type in one Cypher call.
func (s *graphProjectionService) batchMergeRelations(
	ctx context.Context, session neo4j.Session, relType string, relations []*types.GraphRelationRecord,
) (synced int, errs map[string]string) {
	rows := make([]map[string]interface{}, 0, len(relations))
	for _, r := range relations {
		rows = append(rows, map[string]interface{}{
			"source_relation_id": r.SourceRelationID,
			"from_entity_id":     r.FromEntityID,
			"to_entity_id":       r.ToEntityID,
			"relation_props":     jsonToString(r.RelationProps),
			"source_site":        r.SourceSite,
			"confidence_score":   r.ConfidenceScore,
		})
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// Relation type is safe to interpolate — validated by SanitizeRelationType.
		query := fmt.Sprintf(`
			UNWIND $rows AS row
			MATCH (from:GraphEntity {source_entity_id: row.from_entity_id})
			MATCH (to:GraphEntity {source_entity_id: row.to_entity_id})
			MERGE (from)-[r:%s {source_relation_id: row.source_relation_id}]->(to)
			SET r.relation_props = row.relation_props,
			    r.source_site = row.source_site,
			    r.confidence_score = row.confidence_score
		`, relType)
		_, err := tx.Run(ctx, query, map[string]interface{}{"rows": rows})
		return nil, err
	})
	if err != nil {
		errs = make(map[string]string)
		for _, r := range relations {
			errs[r.ID] = fmt.Sprintf("Neo4j 批量MERGE失败: %v", err)
		}
		return 0, errs
	}

	now := time.Now()
	for _, r := range relations {
		s.relationRepo.MarkSynced(ctx, r.ID, "", now)
	}
	return len(relations), nil
}

// batchDeleteRelations uses UNWIND to DELETE multiple relations of the same type in one Cypher call.
func (s *graphProjectionService) batchDeleteRelations(
	ctx context.Context, session neo4j.Session, relType string, relations []*types.GraphRelationRecord,
) (deleted int, errs map[string]string) {
	rows := make([]map[string]interface{}, 0, len(relations))
	for _, r := range relations {
		rows = append(rows, map[string]interface{}{
			"source_relation_id": r.SourceRelationID,
			"from_entity_id":     r.FromEntityID,
			"to_entity_id":       r.ToEntityID,
		})
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := fmt.Sprintf(`
			UNWIND $rows AS row
			MATCH (from:GraphEntity {source_entity_id: row.from_entity_id})-[r:%s {source_relation_id: row.source_relation_id}]->(to:GraphEntity {source_entity_id: row.to_entity_id})
			DELETE r
		`, relType)
		_, err := tx.Run(ctx, query, map[string]interface{}{"rows": rows})
		return nil, err
	})
	if err != nil {
		errs = make(map[string]string)
		for _, r := range relations {
			errs[r.ID] = fmt.Sprintf("Neo4j 批量删除失败: %v", err)
		}
		return 0, errs
	}

	now := time.Now()
	for _, r := range relations {
		s.relationRepo.MarkSynced(ctx, r.ID, "", now)
	}
	return len(relations), nil
}

// jsonToString converts types.JSON to a JSON string for Neo4j storage.
func jsonToString(raw types.JSON) string {
	if len(raw) == 0 {
		return "{}"
	}
	return string(raw)
}

func split2(s string, sep byte) [2]string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}
