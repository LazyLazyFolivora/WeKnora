package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// GraphEntityRepository persists graph entities to the database.
type GraphEntityRepository interface {
	BatchUpsert(ctx context.Context, rows []*types.GraphEntity) error
	ListForProjection(ctx context.Context, tenantID uint64, limit int) ([]*types.GraphEntity, error)
	MarkSynced(ctx context.Context, id string, neo4jNodeID string, syncedAt time.Time) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	MarkDeleted(ctx context.Context, id string, syncedAt time.Time) error
	FindBySourceIDs(ctx context.Context, tenantID uint64, sourceIDs []string) ([]*types.GraphEntity, error)
}

// GraphRelationRepository persists graph relations to the database.
type GraphRelationRepository interface {
	BatchUpsert(ctx context.Context, rows []*types.GraphRelationRecord) error
	ListForProjection(ctx context.Context, tenantID uint64, limit int) ([]*types.GraphRelationRecord, error)
	MarkSynced(ctx context.Context, id string, neo4jRelationID string, syncedAt time.Time) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	ListByEntityIDs(ctx context.Context, tenantID uint64, entitySourceIDs []string) ([]*types.GraphRelationRecord, error)
}

// GraphSyncService imports graph entities and relations into the database.
// It validates inputs and writes rows with sync_status = pending.
// Neo4j is never written directly — a separate projection step handles that.
type GraphSyncService interface {
	BatchUpsertEntities(ctx context.Context, tenantID uint64, req *types.GraphEntityBatchUpsertRequest) (*types.GraphBatchUpsertResult, error)
	BatchUpsertRelations(ctx context.Context, tenantID uint64, req *types.GraphRelationBatchUpsertRequest) (*types.GraphBatchUpsertResult, error)
}

// GraphProjectionService projects graph_entities/graph_relations rows to Neo4j.
// Reads rows where sync_status IN (pending, failed), writes to Neo4j,
// then updates sync_status accordingly.
type GraphProjectionService interface {
	ProjectEntities(ctx context.Context, tenantID uint64, limit int) (synced int, err error)
	ProjectRelations(ctx context.Context, tenantID uint64, limit int) (synced int, err error)
}
