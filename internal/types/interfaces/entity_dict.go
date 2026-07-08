package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// EntityDictService manages the entity_dict mirror table and PrimeKG copy initialization.
type EntityDictService interface {
	BatchUpsert(ctx context.Context, rows []*types.EntityDictRecord) (int, error)
	InitCopies(ctx context.Context, tenantID uint64) (int, error)
}
