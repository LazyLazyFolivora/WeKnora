package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Legacy entity type used for entities imported via the old /graph/import endpoint.
const legacyEntityType = "CustomNode"

// Legacy namespace kept for backward compatibility — the old endpoint wrote
// with Knowledge = "manual_import" in the namespace.
const manualGraphImportKnowledgeID = "manual_import"

type graphImportService struct {
	entityRepo   interfaces.GraphEntityRepository
	relationRepo interfaces.GraphRelationRepository
	kbService    interfaces.KnowledgeBaseService
}

// NewGraphImportService creates a new graph import service (legacy compatibility wrapper).
// It writes to the database with sync_status = pending instead of writing Neo4j directly.
func NewGraphImportService(
	entityRepo interfaces.GraphEntityRepository,
	relationRepo interfaces.GraphRelationRepository,
	kbService interfaces.KnowledgeBaseService,
) interfaces.GraphImportService {
	return &graphImportService{
		entityRepo:   entityRepo,
		relationRepo: relationRepo,
		kbService:    kbService,
	}
}

// ImportGraph converts legacy nodes/relations into graph_entities/graph_relations rows
// and persists them to the database. It does NOT write Neo4j directly.
func (s *graphImportService) ImportGraph(
	ctx context.Context,
	kbID string,
	req *types.GraphImportRequest,
) (*types.GraphImportResult, error) {
	if kbID == "" {
		return nil, apperrors.NewBadRequestError("知识库 ID 不能为空")
	}
	if req == nil || len(req.Nodes) == 0 && len(req.Relations) == 0 {
		return nil, apperrors.NewBadRequestError("nodes 和 relations 不能同时为空")
	}

	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil || kb == nil {
		logger.Warnf(ctx, "[graph_import] kb not found: %s err=%v", kbID, err)
		return nil, apperrors.NewNotFoundError("知识库不存在")
	}

	// Validate basics
	if err := validateGraphImportRequest(req); err != nil {
		return nil, err
	}

	tenantID := kb.TenantID
	now := time.Now()

	// Convert and persist entities.
	nodeCount := len(req.Nodes)
	entities := make([]*types.GraphEntity, 0, nodeCount)
	for _, node := range req.Nodes {
		e := legacyNodeToEntity(node, tenantID, kbID, now)
		entities = append(entities, e)
	}
	if len(entities) > 0 {
		// Preserve existing IDs for upsert by looking up source IDs.
		sourceIDs := make([]string, len(entities))
		for i, e := range entities {
			sourceIDs[i] = e.SourceEntityID
		}
		existing, err := s.entityRepo.FindBySourceIDs(ctx, tenantID, kbID, sourceIDs)
		if err != nil {
			logger.Errorf(ctx, "[graph_import] find existing entities failed: %v", err)
			return nil, apperrors.NewInternalServerError("查询已有实体失败").WithDetails(err.Error())
		}
		existingBySource := make(map[string]*types.GraphEntity, len(existing))
		for _, e := range existing {
			existingBySource[e.SourceEntityID] = e
		}
		for _, e := range entities {
			if prev, ok := existingBySource[e.SourceEntityID]; ok {
				e.ID = prev.ID
				e.CreatedAt = prev.CreatedAt
			}
		}
		if err := s.entityRepo.BatchUpsert(ctx, entities); err != nil {
			logger.Errorf(ctx, "[graph_import] batch upsert entities failed: %v", err)
			return nil, apperrors.NewInternalServerError("写入实体失败").WithDetails(err.Error())
		}
	}

	// Convert and persist relations.
	relationCount := len(req.Relations)
	relations := make([]*types.GraphRelationRecord, 0, relationCount)
	for _, rel := range req.Relations {
		r := legacyRelationToRecord(rel, tenantID, kbID, now)
		relations = append(relations, r)
	}
	if len(relations) > 0 {
		if err := s.relationRepo.BatchUpsert(ctx, relations); err != nil {
			logger.Errorf(ctx, "[graph_import] batch upsert relations failed: %v", err)
			return nil, apperrors.NewInternalServerError("写入关系失败").WithDetails(err.Error())
		}
	}

	result := &types.GraphImportResult{
		ImportedNodes:     nodeCount,
		ImportedRelations: relationCount,
	}
	logger.Infof(ctx, "[graph_import] imported kb=%s nodes=%d relations=%d",
		kbID, result.ImportedNodes, result.ImportedRelations)
	return result, nil
}

// ── conversion helpers ──

func legacyNodeToEntity(node *types.GraphNode, tenantID uint64, kbID string, now time.Time) *types.GraphEntity {
	entityData := map[string]interface{}{}
	if len(node.Attributes) > 0 {
		entityData["attributes"] = node.Attributes
	}
	if len(node.Chunks) > 0 {
		entityData["chunks"] = node.Chunks
	}
	return &types.GraphEntity{
		ID:              uuid.New().String(),
		TenantID:        tenantID,
		KnowledgeBaseID: kbID,
		SourceEntityID:  node.Name,
		EntityType:      legacyEntityType,
		EntityName:      node.Name,
		EntityData:      jsonFromMap(entityData),
		ReviewStatus:    types.GraphReviewStatusPending,
		SyncStatus:      types.GraphSyncStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func legacyRelationToRecord(
	rel *types.GraphRelation, tenantID uint64, kbID string, now time.Time,
) *types.GraphRelationRecord {
	sourceRelationID := legacyRelationSourceID(kbID, rel.Node1, rel.Node2, rel.Type)
	return &types.GraphRelationRecord{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		KnowledgeBaseID:  kbID,
		SourceRelationID: sourceRelationID,
		FromEntityID:     rel.Node1,
		ToEntityID:       rel.Node2,
		RelationType:     rel.Type,
		RelationProps:    types.JSON("{}"),
		ReviewStatus:     types.GraphReviewStatusPending,
		SyncStatus:       types.GraphSyncStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// legacyRelationSourceID deterministically derives a source relation ID
// from kbID + node1 + type + node2 when the legacy request has no explicit one.
func legacyRelationSourceID(kbID, node1, node2, relType string) string {
	parts := []string{kbID, node1, relType, node2}
	sort.Strings(parts)
	joined := strings.Join(parts, "|")
	sum := sha256.Sum224([]byte(joined))
	return fmt.Sprintf("legacy:%x", sum)
}

// validateGraphImportRequest performs basic validation (name / endpoint non-empty).
// Schema-type validation is NOT applied — legacy imports may contain arbitrary types.
func validateGraphImportRequest(req *types.GraphImportRequest) error {
	for _, node := range req.Nodes {
		if node == nil || strings.TrimSpace(node.Name) == "" {
			return apperrors.NewBadRequestError("nodes 中的 name 不能为空")
		}
	}
	for _, rel := range req.Relations {
		if rel == nil || strings.TrimSpace(rel.Node1) == "" || strings.TrimSpace(rel.Node2) == "" || strings.TrimSpace(rel.Type) == "" {
			return apperrors.NewBadRequestError("relations 中的 node1、node2 和 type 不能为空")
		}
	}
	return nil
}
