package types

import "time"

// Agent graph run status values.
const (
	AgentGraphRunStatusRunning   = "running"
	AgentGraphRunStatusCompleted = "completed"
	AgentGraphRunStatusFailed    = "failed"
)

// Agent graph node status values.
const (
	AgentGraphNodeStatusSearching = "searching"
	AgentGraphNodeStatusConfirmed = "confirmed"
)

// BioDSA narrative event type names (class names from NarrativeEvent).
const (
	AgentGraphEventEntitySearching     = "EntitySearching"
	AgentGraphEventLiteratureSearching = "LiteratureSearching"
	AgentGraphEventEntityConfirmed     = "EntityConfirmed"
	AgentGraphEventRelationFound       = "RelationFound"
	AgentGraphEventPhaseChange         = "PhaseChange"
	AgentGraphEventProgress            = "Progress"
	AgentGraphEventRunComplete         = "RunComplete"
)

// AgentGraphRun is one MCP tool-call's streaming knowledge-graph run.
type AgentGraphRun struct {
	ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64     `json:"tenant_id"`
	SessionID       string     `json:"session_id" gorm:"type:varchar(36)"`
	MessageID       string     `json:"message_id" gorm:"type:varchar(36)"`
	StreamKey       string     `json:"stream_key" gorm:"type:varchar(200)"`
	ToolCallID      string     `json:"tool_call_id" gorm:"type:varchar(128)"`
	Status          string     `json:"status" gorm:"type:varchar(32)"`
	Phase           string     `json:"phase" gorm:"type:varchar(64)"`
	Step            int        `json:"step"`
	EntityCount     int        `json:"entity_count"`
	RelationCount   int        `json:"relation_count"`
	LastSeq         int64      `json:"last_seq"`
	LastMsgSeq      int64      `json:"last_msg_seq"`
	DurationSeconds *float64   `json:"duration_seconds"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (AgentGraphRun) TableName() string { return "agent_graph_runs" }

// AgentGraphEvent is one append-only graph notification row.
type AgentGraphEvent struct {
	ID        string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64     `json:"tenant_id"`
	SessionID string     `json:"session_id" gorm:"type:varchar(36)"`
	MessageID string     `json:"message_id" gorm:"type:varchar(36)"`
	StreamKey string     `json:"stream_key" gorm:"type:varchar(200)"`
	Seq       int64      `json:"seq"`
	MsgSeq    int64      `json:"msg_seq"`
	EventType string     `json:"event_type" gorm:"type:varchar(64)"`
	EventID   string     `json:"event_id" gorm:"type:varchar(64)"`
	Payload   JSON       `json:"payload" gorm:"type:json"`
	EmittedAt *time.Time `json:"emitted_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (AgentGraphEvent) TableName() string { return "agent_graph_events" }

// AgentGraphNode is the projected node view for one assistant message.
type AgentGraphNode struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID     uint64    `json:"tenant_id"`
	SessionID    string    `json:"session_id" gorm:"type:varchar(36)"`
	MessageID    string    `json:"message_id" gorm:"type:varchar(36)"`
	StreamKey    string    `json:"stream_key" gorm:"type:varchar(200)"`
	EntityName   string    `json:"entity_name" gorm:"type:varchar(500)"`
	EntityType   string    `json:"entity_type" gorm:"type:varchar(100)"`
	Status       string    `json:"status" gorm:"type:varchar(32)"`
	SourceKB     string    `json:"source_kb" gorm:"type:varchar(100);column:source_kb"`
	Observations JSON      `json:"observations" gorm:"type:json"`
	FirstSeq     int64     `json:"first_seq"`
	LastSeq      int64     `json:"last_seq"`
	FirstMsgSeq  int64     `json:"first_msg_seq"`
	LastMsgSeq   int64     `json:"last_msg_seq"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AgentGraphNode) TableName() string { return "agent_graph_nodes" }

// AgentGraphEdge is the projected edge view for one assistant message.
type AgentGraphEdge struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID     uint64    `json:"tenant_id"`
	SessionID    string    `json:"session_id" gorm:"type:varchar(36)"`
	MessageID    string    `json:"message_id" gorm:"type:varchar(36)"`
	StreamKey    string    `json:"stream_key" gorm:"type:varchar(200)"`
	SourceEntity string    `json:"source_entity" gorm:"type:varchar(500)"`
	TargetEntity string    `json:"target_entity" gorm:"type:varchar(500)"`
	RelationType string    `json:"relation_type" gorm:"type:varchar(200)"`
	FirstSeq     int64     `json:"first_seq"`
	LastSeq      int64     `json:"last_seq"`
	FirstMsgSeq  int64     `json:"first_msg_seq"`
	LastMsgSeq   int64     `json:"last_msg_seq"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AgentGraphEdge) TableName() string { return "agent_graph_edges" }

// AgentGraphSnapshot is the API response for incremental graph queries.
// after_seq / last_seq use message-level msg_seq so multiple tool calls in one
// turn share a single monotonic cursor.
type AgentGraphSnapshot struct {
	Run     *AgentGraphRun     `json:"run"`
	Nodes   []*AgentGraphNode  `json:"nodes"`
	Edges   []*AgentGraphEdge  `json:"edges"`
	Events  []*AgentGraphEvent `json:"events"`
	LastSeq int64              `json:"last_seq"`
}
