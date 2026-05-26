package repository

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// graphEntityRepository implements interfaces.GraphEntityRepository.
type graphEntityRepository struct {
	db *gorm.DB
}

// NewGraphEntityRepository creates a new graph entity repository.
func NewGraphEntityRepository(db *gorm.DB) interfaces.GraphEntityRepository {
	return &graphEntityRepository{db: db}
}

func (r *graphEntityRepository) BatchUpsert(ctx context.Context, rows []*types.GraphEntity) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "knowledge_base_id"},
			{Name: "source_entity_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"entity_type", "entity_name", "entity_data",
			"source_doc_uuid", "source_site", "source_text",
			"confidence_score", "confidence_reason", "review_status",
			"sync_status", "is_deleted",
			"updated_at",
		}),
	}).Create(rows).Error
}

func (r *graphEntityRepository) ListForProjection(
	ctx context.Context, tenantID uint64, kbID string, limit int,
) ([]*types.GraphEntity, error) {
	var rows []*types.GraphEntity
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ? AND sync_status IN ?",
			tenantID, kbID, []string{types.GraphSyncStatusPending, types.GraphSyncStatusFailed}).
		Order("updated_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *graphEntityRepository) MarkSynced(ctx context.Context, id string, neo4jNodeID string, syncedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&types.GraphEntity{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"sync_status":   types.GraphSyncStatusSynced,
			"neo4j_node_id": neo4jNodeID,
			"synced_at":     syncedAt,
			"sync_error":    "",
		}).Error
}

func (r *graphEntityRepository) MarkFailed(ctx context.Context, id string, errMsg string) error {
	return r.db.WithContext(ctx).Model(&types.GraphEntity{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"sync_status": types.GraphSyncStatusFailed,
			"sync_error":  errMsg,
		}).Error
}

func (r *graphEntityRepository) MarkDeleted(ctx context.Context, id string, syncedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&types.GraphEntity{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"sync_status": types.GraphSyncStatusDeleted,
			"is_deleted":  true,
			"synced_at":   syncedAt,
		}).Error
}

func (r *graphEntityRepository) FindBySourceIDs(
	ctx context.Context, tenantID uint64, kbID string, sourceIDs []string,
) ([]*types.GraphEntity, error) {
	if len(sourceIDs) == 0 {
		return nil, nil
	}
	var rows []*types.GraphEntity
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ? AND source_entity_id IN ?",
			tenantID, kbID, sourceIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// graphRelationRepository implements interfaces.GraphRelationRepository.
type graphRelationRepository struct {
	db *gorm.DB
}

// NewGraphRelationRepository creates a new graph relation repository.
func NewGraphRelationRepository(db *gorm.DB) interfaces.GraphRelationRepository {
	return &graphRelationRepository{db: db}
}

func (r *graphRelationRepository) BatchUpsert(ctx context.Context, rows []*types.GraphRelationRecord) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "knowledge_base_id"},
			{Name: "source_relation_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"from_entity_id", "to_entity_id", "relation_type", "relation_props",
			"source_doc_uuid", "source_site", "source_text",
			"confidence_score", "confidence_reason", "review_status",
			"sync_status", "is_deleted",
			"updated_at",
		}),
	}).Create(rows).Error
}

func (r *graphRelationRepository) ListForProjection(
	ctx context.Context, tenantID uint64, kbID string, limit int,
) ([]*types.GraphRelationRecord, error) {
	var rows []*types.GraphRelationRecord
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ? AND sync_status IN ?",
			tenantID, kbID, []string{types.GraphSyncStatusPending, types.GraphSyncStatusFailed}).
		Order("updated_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *graphRelationRepository) MarkSynced(ctx context.Context, id string, neo4jRelationID string, syncedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&types.GraphRelationRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"sync_status":       types.GraphSyncStatusSynced,
			"neo4j_relation_id": neo4jRelationID,
			"synced_at":         syncedAt,
			"sync_error":        "",
		}).Error
}

func (r *graphRelationRepository) MarkFailed(ctx context.Context, id string, errMsg string) error {
	return r.db.WithContext(ctx).Model(&types.GraphRelationRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"sync_status": types.GraphSyncStatusFailed,
			"sync_error":  errMsg,
		}).Error
}

func (r *graphRelationRepository) ListByEntityIDs(
	ctx context.Context, tenantID uint64, kbID string, entitySourceIDs []string,
) ([]*types.GraphRelationRecord, error) {
	if len(entitySourceIDs) == 0 {
		return nil, nil
	}
	var rows []*types.GraphRelationRecord
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ? AND from_entity_id IN ? AND to_entity_id IN ?",
			tenantID, kbID, entitySourceIDs, entitySourceIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
