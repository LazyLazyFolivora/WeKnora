package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Type mapping: PrimeKG entity type → WeKnora entity type.
var primekgTypeMap = map[string]string{
	"drug":         "Drug",
	"disease":      "Indication",
	"gene/protein": "Target",
	"pathway":      "Pathway",
}

type entityDictService struct {
	db          *gorm.DB
	graphSync   interfaces.GraphSyncService
}

// NewEntityDictService creates a new entity_dict sync service.
func NewEntityDictService(
	db *gorm.DB,
	graphSync interfaces.GraphSyncService,
) interfaces.EntityDictService {
	return &entityDictService{db: db, graphSync: graphSync}
}

// BatchUpsert upserts rows from ZK's entity_dict into WeKnora's mirror table.
func (s *entityDictService) BatchUpsert(ctx context.Context, rows []*types.EntityDictRecord) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	now := time.Now()
	for _, r := range rows {
		if r.SyncedAt == nil {
			r.SyncedAt = &now
		}
	}

	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"entity_type", "external_ids", "canonical_data",
			"canonical_source", "is_deleted", "synced_at", "updated_at",
		}),
	}).Create(rows).Error
	if err != nil {
		return 0, fmt.Errorf("batch upsert entity_dict: %w", err)
	}
	return len(rows), nil
}

// InitCopies reads PrimeKG rows from entity_dict and writes them to graph_entities.
// Marks upserted rows as approved so they pass the review filter during projection.
func (s *entityDictService) InitCopies(ctx context.Context, kbID string, tenantID uint64) (int, error) {
	var rows []types.EntityDictRecord
	if err := s.db.WithContext(ctx).
		Where("canonical_source = ? AND is_deleted = ?", "primekg", false).
		Order("id").
		Find(&rows).Error; err != nil {
		return 0, fmt.Errorf("query entity_dict: %w", err)
	}

	fmt.Printf("entity_dict 中共 %d 条 PrimeKG 行\n", len(rows))

	inputs := make([]*types.GraphEntityInput, 0, len(rows))
	for _, r := range rows {
		wkType, ok := primekgTypeMap[r.EntityType]
		if !ok {
			continue
		}
		// Parse canonical_data for entity_name and include external_ids.
		data := jsonToMap(r.CanonicalData)
		name := ""
		if n, ok := data["name"]; ok {
			name = fmt.Sprintf("%v", n)
		}
		// Merge external_ids into entity_data so projection can create REFERENCES edges.
		extIDs := jsonToMap(r.ExternalIDs)
		data["external_ids"] = extIDs

		inputs = append(inputs, &types.GraphEntityInput{
			SourceEntityID: fmt.Sprintf("dict:%d", r.ID),
			EntityType:     wkType,
			EntityName:     name,
			EntityData:     data,
			SourceSite:     "primekg",
			ReviewStatus:   types.GraphReviewStatusApproved,
		})
	}

	if len(inputs) == 0 {
		return 0, nil
	}

	// Batch write via GraphSyncService (handles validation and upsert).
	batchSize := 500
	total := 0
	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batch := inputs[i:end]
		result, err := s.graphSync.BatchUpsertEntities(ctx, kbID, &types.GraphEntityBatchUpsertRequest{
			Entities: batch,
		})
		if err != nil {
			return total, fmt.Errorf("batch upsert [%d:%d]: %w", i, end, err)
		}
		total += result.Upserted
		fmt.Printf("  graph_entities [%d:%d]: upserted=%d\n", i, end, result.Upserted)
	}
	return total, nil
}

func jsonToMap(j types.JSON) map[string]interface{} {
	m := make(map[string]interface{})
	if len(j) > 2 { // at least "{}"
		json.Unmarshal(json.RawMessage(j), &m)
	}
	return m
}
