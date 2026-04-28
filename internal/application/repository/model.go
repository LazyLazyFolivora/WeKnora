package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// modelRepository implements the model repository interface
type modelRepository struct {
	db *gorm.DB
}

// NewModelRepository creates a new model repository
func NewModelRepository(db *gorm.DB) interfaces.ModelRepository {
	return &modelRepository{db: db}
}

// Create creates a new model
func (r *modelRepository) Create(ctx context.Context, m *types.Model) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetByID retrieves a model by ID
func (r *modelRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.Model, error) {
	var m types.Model
	if err := r.db.WithContext(ctx).Where("id = ?", id).Where(
		"tenant_id = ? OR is_builtin = true OR is_global_default = true", tenantID,
	).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// List lists models with optional filtering
func (r *modelRepository) List(
	ctx context.Context, tenantID uint64, modelType types.ModelType, source types.ModelSource,
) ([]*types.Model, error) {
	var models []*types.Model
	query := r.db.WithContext(ctx).Where(
		"tenant_id = ? OR is_builtin = true", tenantID,
	)

	if modelType != "" {
		query = query.Where("type = ?", modelType)
	}

	if source != "" {
		query = query.Where("source = ?", source)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	return models, nil
}

// Update updates a model
func (r *modelRepository) Update(ctx context.Context, m *types.Model) error {
	// Use Select to explicitly update all fields, including zero values like false
	return r.db.WithContext(ctx).Debug().Model(&types.Model{}).Where(
		"id = ? AND tenant_id = ?", m.ID, m.TenantID,
	).Select("*").Updates(m).Error
}

// Delete deletes a model
func (r *modelRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).Where(
		"id = ? AND tenant_id = ?", id, tenantID,
	).Delete(&types.Model{}).Error
}

// ClearDefaultByType clears the default flag for all models of a specific type
// This is a batch operation that updates all matching records in one query
func (r *modelRepository) ClearDefaultByType(
	ctx context.Context,
	tenantID uint,
	modelType types.ModelType,
	excludeID string,
) error {
	query := r.db.WithContext(ctx).Model(&types.Model{}).Where(
		"tenant_id = ? AND type = ? AND is_default = ?", tenantID, modelType, true,
	)

	// If excludeID is provided, exclude that model from the update
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}

	// Batch update: set is_default to false for all matching records
	return query.Update("is_default", false).Error
}

// ListGlobalDefaults returns all models where is_global_default=true and deleted_at IS NULL.
// Does NOT filter by tenant_id — global default models are cross-tenant.
func (r *modelRepository) ListGlobalDefaults(ctx context.Context) ([]*types.Model, error) {
	var models []*types.Model
	if err := r.db.WithContext(ctx).
		Where("is_global_default = ? AND deleted_at IS NULL", true).
		Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// ClearGlobalDefaultByType clears is_global_default for all models of a given type,
// optionally excluding a specific model ID. Does NOT filter by tenant_id.
func (r *modelRepository) ClearGlobalDefaultByType(
	ctx context.Context,
	modelType types.ModelType,
	excludeID string,
) error {
	query := r.db.WithContext(ctx).Model(&types.Model{}).Where(
		"type = ? AND is_global_default = ?", modelType, true,
	)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	return query.Update("is_global_default", false).Error
}

// WithTransaction executes fn within a database transaction.
func (r *modelRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := &modelRepository{db: tx}
		txCtx := context.WithValue(ctx, modelRepoTxKey{}, txRepo)
		return fn(txCtx)
	})
}

// modelRepoTxKey is the context key for the transactional model repository.
type modelRepoTxKey struct{}
