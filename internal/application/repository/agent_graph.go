package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentGraphRepository struct {
	db *gorm.DB
}

// NewAgentGraphRepository creates a repository for agent graph streams.
func NewAgentGraphRepository(db *gorm.DB) interfaces.AgentGraphRepository {
	return &agentGraphRepository{db: db}
}

func (r *agentGraphRepository) DB() *gorm.DB { return r.db }

func (r *agentGraphRepository) WithTx(ctx context.Context, fn func(txRepo interfaces.AgentGraphRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&agentGraphRepository{db: tx})
	})
}

func (r *agentGraphRepository) UpsertRun(ctx context.Context, run *types.AgentGraphRun) error {
	now := time.Now()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = now
	}
	run.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_key"}},
		DoNothing: true,
	}).Create(run).Error
}

func (r *agentGraphRepository) GetRunByStreamKey(ctx context.Context, tenantID uint64, streamKey string) (*types.AgentGraphRun, error) {
	var run types.AgentGraphRun
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND stream_key = ?", tenantID, streamKey).
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *agentGraphRepository) GetLatestRunByMessage(
	ctx context.Context, tenantID uint64, sessionID, messageID string,
) (*types.AgentGraphRun, error) {
	var run types.AgentGraphRun
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND message_id = ?", tenantID, sessionID, messageID).
		Order("updated_at DESC").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// BumpMsgSeq allocates a message-level monotonic cursor shared across tool calls.
func (r *agentGraphRepository) BumpMsgSeq(ctx context.Context, tenantID uint64, messageID, streamKey string) (int64, error) {
	var eventMax, runMax int64
	if err := r.db.WithContext(ctx).Model(&types.AgentGraphEvent{}).
		Where("tenant_id = ? AND message_id = ?", tenantID, messageID).
		Select("COALESCE(MAX(msg_seq), 0)").Scan(&eventMax).Error; err != nil {
		return 0, err
	}
	if err := r.db.WithContext(ctx).Model(&types.AgentGraphRun{}).
		Where("tenant_id = ? AND message_id = ?", tenantID, messageID).
		Select("COALESCE(MAX(last_msg_seq), 0)").Scan(&runMax).Error; err != nil {
		return 0, err
	}
	next := eventMax + 1
	if runMax+1 > next {
		next = runMax + 1
	}
	if err := r.db.WithContext(ctx).Model(&types.AgentGraphRun{}).
		Where("tenant_id = ? AND stream_key = ?", tenantID, streamKey).
		Updates(map[string]interface{}{
			"last_msg_seq": next,
			"updated_at":   time.Now(),
		}).Error; err != nil {
		return 0, err
	}
	return next, nil
}

func (r *agentGraphRepository) AdvanceRunSeq(
	ctx context.Context, tenantID uint64, streamKey string, remoteSeq, msgSeq int64, extra map[string]interface{},
) error {
	fields := map[string]interface{}{
		"updated_at": time.Now(),
	}
	for k, v := range extra {
		fields[k] = v
	}
	// Monotonic remote seq + msg seq (never move backwards).
	res := r.db.WithContext(ctx).Model(&types.AgentGraphRun{}).
		Where("tenant_id = ? AND stream_key = ?", tenantID, streamKey).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	// Separate conditional bumps so dialect differences don't break Updates map.
	if err := r.db.WithContext(ctx).Exec(
		`UPDATE agent_graph_runs SET last_seq = CASE WHEN last_seq < ? THEN ? ELSE last_seq END,
		 last_msg_seq = CASE WHEN last_msg_seq < ? THEN ? ELSE last_msg_seq END
		 WHERE tenant_id = ? AND stream_key = ?`,
		remoteSeq, remoteSeq, msgSeq, msgSeq, tenantID, streamKey,
	).Error; err != nil {
		return err
	}
	return nil
}

func (r *agentGraphRepository) MarkRunFailed(ctx context.Context, tenantID uint64, streamKey string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&types.AgentGraphRun{}).
		Where("tenant_id = ? AND stream_key = ? AND status = ?", tenantID, streamKey, types.AgentGraphRunStatusRunning).
		Updates(map[string]interface{}{
			"status":       types.AgentGraphRunStatusFailed,
			"completed_at": now,
			"updated_at":   now,
		}).Error
}

func (r *agentGraphRepository) InsertEvent(ctx context.Context, evt *types.AgentGraphEvent) (bool, error) {
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now()
	}
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stream_key"}, {Name: "seq"}},
		DoNothing: true,
	}).Create(evt)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *agentGraphRepository) ListEvents(
	ctx context.Context, tenantID uint64, sessionID, messageID string, afterMsgSeq int64,
) ([]*types.AgentGraphEvent, error) {
	var rows []*types.AgentGraphEvent
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND message_id = ? AND msg_seq > ?",
			tenantID, sessionID, messageID, afterMsgSeq).
		Order("msg_seq ASC").
		Limit(2000).
		Find(&rows).Error
	return rows, err
}

func (r *agentGraphRepository) UpsertNode(ctx context.Context, node *types.AgentGraphNode, overwriteConfirmed bool) error {
	now := time.Now()
	if node.CreatedAt.IsZero() {
		node.CreatedAt = now
	}
	node.UpdatedAt = now
	if len(node.Observations) == 0 {
		node.Observations = types.JSON([]byte("[]"))
	}

	if overwriteConfirmed {
		return r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "message_id"}, {Name: "entity_name"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"entity_type": gorm.Expr(
					"CASE WHEN ? <> '' THEN ? ELSE agent_graph_nodes.entity_type END",
					node.EntityType, node.EntityType,
				),
				"status":       types.AgentGraphNodeStatusConfirmed,
				"observations": node.Observations,
				"stream_key":   node.StreamKey,
				"last_seq":     gorm.Expr("CASE WHEN agent_graph_nodes.last_seq < ? THEN ? ELSE agent_graph_nodes.last_seq END", node.LastSeq, node.LastSeq),
				"last_msg_seq": gorm.Expr("CASE WHEN agent_graph_nodes.last_msg_seq < ? THEN ? ELSE agent_graph_nodes.last_msg_seq END", node.LastMsgSeq, node.LastMsgSeq),
				"source_kb":    gorm.Expr("CASE WHEN ? <> '' THEN ? ELSE agent_graph_nodes.source_kb END", node.SourceKB, node.SourceKB),
				"updated_at":   now,
			}),
		}).Create(node).Error
	}

	// EntityPlanned / EntitySearching: status only advances planned → searching →
	// confirmed, never downgrades an already-advanced node.
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "message_id"}, {Name: "entity_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stream_key": node.StreamKey,
			"last_seq": gorm.Expr(
				"CASE WHEN agent_graph_nodes.last_seq < ? THEN ? ELSE agent_graph_nodes.last_seq END",
				node.LastSeq, node.LastSeq,
			),
			"last_msg_seq": gorm.Expr(
				"CASE WHEN agent_graph_nodes.last_msg_seq < ? THEN ? ELSE agent_graph_nodes.last_msg_seq END",
				node.LastMsgSeq, node.LastMsgSeq,
			),
			"entity_type": gorm.Expr(
				"CASE WHEN ? <> '' THEN ? ELSE agent_graph_nodes.entity_type END",
				node.EntityType, node.EntityType,
			),
			"status": gorm.Expr(
				"CASE WHEN agent_graph_nodes.status = 'confirmed' OR ? = 'confirmed' THEN 'confirmed' "+
					"WHEN agent_graph_nodes.status = 'searching' OR ? = 'searching' THEN 'searching' ELSE 'planned' END",
				node.Status, node.Status,
			),
			"source_kb": gorm.Expr(
				"CASE WHEN agent_graph_nodes.source_kb = '' AND ? <> '' THEN ? ELSE agent_graph_nodes.source_kb END",
				node.SourceKB, node.SourceKB,
			),
			"updated_at": now,
		}),
	}).Create(node).Error
}

// UpsertNodeReconcile fills missing confirmed nodes without rewriting last_msg_seq on existing rows.
func (r *agentGraphRepository) UpsertNodeReconcile(ctx context.Context, node *types.AgentGraphNode) error {
	now := time.Now()
	if node.CreatedAt.IsZero() {
		node.CreatedAt = now
	}
	node.UpdatedAt = now
	if len(node.Observations) == 0 {
		node.Observations = types.JSON([]byte("[]"))
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "message_id"}, {Name: "entity_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			// Never blank out a type we already have (RunComplete historically
			// omitted camelCase entityType → empty overwrite).
			"entity_type": gorm.Expr(
				"CASE WHEN ? <> '' THEN ? ELSE agent_graph_nodes.entity_type END",
				node.EntityType, node.EntityType,
			),
			"status":       types.AgentGraphNodeStatusConfirmed,
			"observations": node.Observations,
			"stream_key":   node.StreamKey,
			"updated_at":   now,
		}),
	}).Create(node).Error
}

func (r *agentGraphRepository) ListNodes(
	ctx context.Context, tenantID uint64, sessionID, messageID string, afterMsgSeq int64,
) ([]*types.AgentGraphNode, error) {
	var rows []*types.AgentGraphNode
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND message_id = ?", tenantID, sessionID, messageID)
	if afterMsgSeq > 0 {
		q = q.Where("last_msg_seq > ?", afterMsgSeq)
	}
	err := q.Order("last_msg_seq ASC").Find(&rows).Error
	return rows, err
}

func (r *agentGraphRepository) CountConfirmedNodes(ctx context.Context, tenantID uint64, messageID string) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&types.AgentGraphNode{}).
		Where("tenant_id = ? AND message_id = ? AND status = ?",
			tenantID, messageID, types.AgentGraphNodeStatusConfirmed).
		Count(&n).Error
	return int(n), err
}

func (r *agentGraphRepository) UpsertEdge(ctx context.Context, edge *types.AgentGraphEdge) error {
	now := time.Now()
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = now
	}
	edge.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "message_id"},
			{Name: "source_entity"},
			{Name: "target_entity"},
			{Name: "relation_type"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stream_key": edge.StreamKey,
			"last_seq": gorm.Expr(
				"CASE WHEN agent_graph_edges.last_seq < ? THEN ? ELSE agent_graph_edges.last_seq END",
				edge.LastSeq, edge.LastSeq,
			),
			"last_msg_seq": gorm.Expr(
				"CASE WHEN agent_graph_edges.last_msg_seq < ? THEN ? ELSE agent_graph_edges.last_msg_seq END",
				edge.LastMsgSeq, edge.LastMsgSeq,
			),
			"updated_at": now,
		}),
	}).Create(edge).Error
}

func (r *agentGraphRepository) UpsertEdgeReconcile(ctx context.Context, edge *types.AgentGraphEdge) error {
	now := time.Now()
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = now
	}
	edge.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "message_id"},
			{Name: "source_entity"},
			{Name: "target_entity"},
			{Name: "relation_type"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"stream_key": edge.StreamKey,
			"updated_at": now,
		}),
	}).Create(edge).Error
}

func (r *agentGraphRepository) ListEdges(
	ctx context.Context, tenantID uint64, sessionID, messageID string, afterMsgSeq int64,
) ([]*types.AgentGraphEdge, error) {
	var rows []*types.AgentGraphEdge
	q := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND message_id = ?", tenantID, sessionID, messageID)
	if afterMsgSeq > 0 {
		q = q.Where("last_msg_seq > ?", afterMsgSeq)
	}
	err := q.Order("last_msg_seq ASC").Find(&rows).Error
	return rows, err
}

func (r *agentGraphRepository) CountEdges(ctx context.Context, tenantID uint64, messageID string) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&types.AgentGraphEdge{}).
		Where("tenant_id = ? AND message_id = ?", tenantID, messageID).Count(&n).Error
	return int(n), err
}
