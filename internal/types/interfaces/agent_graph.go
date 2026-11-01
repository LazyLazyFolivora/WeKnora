package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// AgentGraphRepository persists incremental agent knowledge-graph streams.
type AgentGraphRepository interface {
	WithTx(ctx context.Context, fn func(txRepo AgentGraphRepository) error) error

	UpsertRun(ctx context.Context, run *types.AgentGraphRun) error
	GetRunByStreamKey(ctx context.Context, tenantID uint64, streamKey string) (*types.AgentGraphRun, error)
	GetLatestRunByMessage(ctx context.Context, tenantID uint64, sessionID, messageID string) (*types.AgentGraphRun, error)
	BumpMsgSeq(ctx context.Context, tenantID uint64, messageID, streamKey string) (int64, error)
	AdvanceRunSeq(ctx context.Context, tenantID uint64, streamKey string, remoteSeq, msgSeq int64, extra map[string]interface{}) error
	MarkRunFailed(ctx context.Context, tenantID uint64, streamKey string) error

	InsertEvent(ctx context.Context, evt *types.AgentGraphEvent) (inserted bool, err error)
	ListEvents(ctx context.Context, tenantID uint64, sessionID, messageID string, afterMsgSeq int64) ([]*types.AgentGraphEvent, error)

	UpsertNode(ctx context.Context, node *types.AgentGraphNode, overwriteConfirmed bool) error
	UpsertNodeReconcile(ctx context.Context, node *types.AgentGraphNode) error
	ListNodes(ctx context.Context, tenantID uint64, sessionID, messageID string, afterMsgSeq int64) ([]*types.AgentGraphNode, error)
	CountConfirmedNodes(ctx context.Context, tenantID uint64, messageID string) (int, error)

	UpsertEdge(ctx context.Context, edge *types.AgentGraphEdge) error
	UpsertEdgeReconcile(ctx context.Context, edge *types.AgentGraphEdge) error
	ListEdges(ctx context.Context, tenantID uint64, sessionID, messageID string, afterMsgSeq int64) ([]*types.AgentGraphEdge, error)
	CountEdges(ctx context.Context, tenantID uint64, messageID string) (int, error)

	DB() *gorm.DB
}

// AgentGraphService records streaming graph notifications and serves snapshots.
type AgentGraphService interface {
	Record(ctx context.Context, sessionID, messageID string, data event.AgentGraphData) error
	MarkFailed(ctx context.Context, streamKey string) error
	GetSnapshot(ctx context.Context, tenantID uint64, sessionID, messageID string, afterSeq int64, include map[string]bool) (*types.AgentGraphSnapshot, error)
}
