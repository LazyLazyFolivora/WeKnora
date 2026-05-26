package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type graphSyncService struct {
	entityRepo   interfaces.GraphEntityRepository
	relationRepo interfaces.GraphRelationRepository
	kbService    interfaces.KnowledgeBaseService
}

// NewGraphSyncService creates a new graph sync service.
func NewGraphSyncService(
	entityRepo interfaces.GraphEntityRepository,
	relationRepo interfaces.GraphRelationRepository,
	kbService interfaces.KnowledgeBaseService,
) interfaces.GraphSyncService {
	return &graphSyncService{
		entityRepo:   entityRepo,
		relationRepo: relationRepo,
		kbService:    kbService,
	}
}

func (s *graphSyncService) BatchUpsertEntities(
	ctx context.Context,
	kbID string,
	req *types.GraphEntityBatchUpsertRequest,
) (*types.GraphBatchUpsertResult, error) {
	if kbID == "" {
		return nil, apperrors.NewBadRequestError("知识库 ID 不能为空")
	}
	if req == nil || len(req.Entities) == 0 {
		return nil, apperrors.NewBadRequestError("entities 不能为空")
	}

	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil || kb == nil {
		logger.Warnf(ctx, "[GraphSync] kb not found: %s err=%v", kbID, err)
		return nil, apperrors.NewNotFoundError("知识库不存在")
	}
	tenantID := kb.TenantID

	now := time.Now()
	var upserted, deleted int

	for _, in := range req.Entities {
		if in == nil {
			continue
		}
		if err := validateEntityInput(in); err != nil {
			return nil, err
		}
	}

	// Look up existing rows for source IDs to preserve their DB id.
	sourceIDs := make([]string, len(req.Entities))
	for i, in := range req.Entities {
		sourceIDs[i] = in.SourceEntityID
	}
	existing, err := s.entityRepo.FindBySourceIDs(ctx, tenantID, kbID, sourceIDs)
	if err != nil {
		logger.Errorf(ctx, "[GraphSync] find existing entities failed: %v", err)
		return nil, apperrors.NewInternalServerError("查询已有实体失败").WithDetails(err.Error())
	}
	existingBySource := make(map[string]*types.GraphEntity, len(existing))
	for _, e := range existing {
		existingBySource[e.SourceEntityID] = e
	}

	rows := make([]*types.GraphEntity, 0, len(req.Entities))
	for _, in := range req.Entities {
		row := inputToGraphEntity(in, tenantID, kbID, now)
		if prev, ok := existingBySource[in.SourceEntityID]; ok {
			row.ID = prev.ID
			row.CreatedAt = prev.CreatedAt
		}
		if in.IsDeleted {
			row.IsDeleted = true
			row.SyncStatus = types.GraphSyncStatusPending
			deleted++
		} else {
			upserted++
		}
		rows = append(rows, row)
	}

	if err := s.entityRepo.BatchUpsert(ctx, rows); err != nil {
		logger.Errorf(ctx, "[GraphSync] batch upsert entities failed: %v", err)
		return nil, apperrors.NewInternalServerError("写入实体失败").WithDetails(err.Error())
	}

	result := &types.GraphBatchUpsertResult{Upserted: upserted, Deleted: deleted}
	logger.Infof(ctx, "[GraphSync] entities upserted=%d deleted=%d kb=%s", result.Upserted, result.Deleted, kbID)
	return result, nil
}

func (s *graphSyncService) BatchUpsertRelations(
	ctx context.Context,
	kbID string,
	req *types.GraphRelationBatchUpsertRequest,
) (*types.GraphBatchUpsertResult, error) {
	if kbID == "" {
		return nil, apperrors.NewBadRequestError("知识库 ID 不能为空")
	}
	if req == nil || len(req.Relations) == 0 {
		return nil, apperrors.NewBadRequestError("relations 不能为空")
	}

	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil || kb == nil {
		logger.Warnf(ctx, "[GraphSync] kb not found: %s err=%v", kbID, err)
		return nil, apperrors.NewNotFoundError("知识库不存在")
	}
	tenantID := kb.TenantID

	// Validate each relation input first.
	for _, in := range req.Relations {
		if in == nil {
			continue
		}
		if err := validateRelationInput(in); err != nil {
			return nil, err
		}
	}

	// Collect all referenced entity source IDs and check they exist.
	entityIDSet := make(map[string]struct{})
	for _, in := range req.Relations {
		entityIDSet[in.FromEntityID] = struct{}{}
		entityIDSet[in.ToEntityID] = struct{}{}
	}
	entityIDs := make([]string, 0, len(entityIDSet))
	for id := range entityIDSet {
		entityIDs = append(entityIDs, id)
	}
	existingEntities, err := s.entityRepo.FindBySourceIDs(ctx, tenantID, kbID, entityIDs)
	if err != nil {
		logger.Errorf(ctx, "[GraphSync] find entities for relation validation failed: %v", err)
		return nil, apperrors.NewInternalServerError("查询关系端点实体失败").WithDetails(err.Error())
	}
	entityTypeBySource := make(map[string]string, len(existingEntities))
	for _, e := range existingEntities {
		entityTypeBySource[e.SourceEntityID] = e.EntityType
	}

	// Validate direction constraints for non-deleted relations.
	for _, in := range req.Relations {
		if in.IsDeleted {
			continue
		}
		fromType, fromOk := entityTypeBySource[in.FromEntityID]
		toType, toOk := entityTypeBySource[in.ToEntityID]
		if !fromOk {
			return nil, apperrors.NewBadRequestError(
				fmt.Sprintf("起点实体 %q 不存在于知识库中", in.FromEntityID))
		}
		if !toOk {
			return nil, apperrors.NewBadRequestError(
				fmt.Sprintf("终点实体 %q 不存在于知识库中", in.ToEntityID))
		}
		if err := validateRelationDirection(fromType, in.RelationType, toType); err != nil {
			return nil, apperrors.NewBadRequestError(err.Error())
		}
	}

	now := time.Now()
	var upserted, deleted int
	rows := make([]*types.GraphRelationRecord, 0, len(req.Relations))
	for _, in := range req.Relations {
		row := inputToGraphRelationRecord(in, tenantID, kbID, now)
		if in.IsDeleted {
			row.IsDeleted = true
			row.SyncStatus = types.GraphSyncStatusPending
			deleted++
		} else {
			upserted++
		}
		rows = append(rows, row)
	}

	if err := s.relationRepo.BatchUpsert(ctx, rows); err != nil {
		logger.Errorf(ctx, "[GraphSync] batch upsert relations failed: %v", err)
		return nil, apperrors.NewInternalServerError("写入关系失败").WithDetails(err.Error())
	}

	result := &types.GraphBatchUpsertResult{Upserted: upserted, Deleted: deleted}
	logger.Infof(ctx, "[GraphSync] relations upserted=%d deleted=%d kb=%s", result.Upserted, result.Deleted, kbID)
	return result, nil
}

// ── helpers ──

func validateEntityInput(in *types.GraphEntityInput) error {
	if strings.TrimSpace(in.SourceEntityID) == "" {
		return apperrors.NewBadRequestError("source_entity_id 不能为空")
	}
	if strings.TrimSpace(in.EntityType) == "" {
		return apperrors.NewBadRequestError("entity_type 不能为空")
	}
	if strings.TrimSpace(in.EntityName) == "" {
		return apperrors.NewBadRequestError("entity_name 不能为空")
	}
	if err := validateEntityType(in.EntityType); err != nil {
		return apperrors.NewBadRequestError(err.Error())
	}
	if in.ConfidenceScore != nil && (*in.ConfidenceScore < 0 || *in.ConfidenceScore > 1) {
		return apperrors.NewBadRequestError("confidence_score 必须在 0 到 1 之间")
	}
	return nil
}

func validateRelationInput(in *types.GraphRelationInput) error {
	if strings.TrimSpace(in.SourceRelationID) == "" {
		return apperrors.NewBadRequestError("source_relation_id 不能为空")
	}
	if strings.TrimSpace(in.FromEntityID) == "" {
		return apperrors.NewBadRequestError("from_entity_id 不能为空")
	}
	if strings.TrimSpace(in.ToEntityID) == "" {
		return apperrors.NewBadRequestError("to_entity_id 不能为空")
	}
	if strings.TrimSpace(in.RelationType) == "" {
		return apperrors.NewBadRequestError("relation_type 不能为空")
	}
	if err := validateRelationType(in.RelationType); err != nil {
		return apperrors.NewBadRequestError(err.Error())
	}
	if in.ConfidenceScore != nil && (*in.ConfidenceScore < 0 || *in.ConfidenceScore > 1) {
		return apperrors.NewBadRequestError("confidence_score 必须在 0 到 1 之间")
	}
	return nil
}

func jsonFromMap(m map[string]interface{}) types.JSON {
	if len(m) == 0 {
		return types.JSON(json.RawMessage("{}"))
	}
	b, _ := json.Marshal(m)
	return types.JSON(b)
}

func inputToGraphEntity(in *types.GraphEntityInput, tenantID uint64, kbID string, now time.Time) *types.GraphEntity {
	reviewStatus := strings.TrimSpace(in.ReviewStatus)
	if reviewStatus == "" {
		reviewStatus = types.GraphReviewStatusPending
	}
	return &types.GraphEntity{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		KnowledgeBaseID:  kbID,
		SourceEntityID:   in.SourceEntityID,
		EntityType:       in.EntityType,
		EntityName:       in.EntityName,
		EntityData:       jsonFromMap(in.EntityData),
		SourceDocUUID:    in.SourceDocUUID,
		SourceSite:       in.SourceSite,
		SourceText:       in.SourceText,
		ConfidenceScore:  in.ConfidenceScore,
		ConfidenceReason: in.ConfidenceReason,
		ReviewStatus:     reviewStatus,
		SyncStatus:       types.GraphSyncStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func inputToGraphRelationRecord(in *types.GraphRelationInput, tenantID uint64, kbID string, now time.Time) *types.GraphRelationRecord {
	reviewStatus := strings.TrimSpace(in.ReviewStatus)
	if reviewStatus == "" {
		reviewStatus = types.GraphReviewStatusPending
	}
	return &types.GraphRelationRecord{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		KnowledgeBaseID:  kbID,
		SourceRelationID: in.SourceRelationID,
		FromEntityID:     in.FromEntityID,
		ToEntityID:       in.ToEntityID,
		RelationType:     in.RelationType,
		RelationProps:    jsonFromMap(in.RelationProps),
		SourceDocUUID:    in.SourceDocUUID,
		SourceSite:       in.SourceSite,
		SourceText:       in.SourceText,
		ConfidenceScore:  in.ConfidenceScore,
		ConfidenceReason: in.ConfidenceReason,
		ReviewStatus:     reviewStatus,
		SyncStatus:       types.GraphSyncStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
