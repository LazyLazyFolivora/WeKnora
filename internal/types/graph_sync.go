package types

import "time"

// Sync status constants for graph entities and relations.
const (
	GraphSyncStatusPending = "pending"
	GraphSyncStatusSynced  = "synced"
	GraphSyncStatusFailed  = "failed"
	GraphSyncStatusDeleted = "deleted"
	GraphSyncStatusSkipped = "skipped"
)

// GraphReviewStatus constants.
const (
	GraphReviewStatusPending  = "pending"
	GraphReviewStatusApproved = "approved"
	GraphReviewStatusRejected = "rejected"
)

// GraphEntity is a row in the graph_entities table.
// The database is the source of truth; Neo4j is a derived projection.
type GraphEntity struct {
	ID               string     `json:"id"                 gorm:"type:varchar(36);primaryKey"`
	TenantID         uint64     `json:"tenant_id"`
	SourceEntityID   string     `json:"source_entity_id"`
	EntityType       string     `json:"entity_type"`
	EntityName       string     `json:"entity_name"`
	EntityData       JSON       `json:"entity_data"        gorm:"type:json"`
	SourceDocUUID    string     `json:"source_doc_uuid"`
	SourceSite       string     `json:"source_site"`
	SourceText       string     `json:"source_text"`
	ConfidenceScore  *float64   `json:"confidence_score"`
	ConfidenceReason string     `json:"confidence_reason"`
	ReviewStatus     string     `json:"review_status"`
	SyncStatus       string     `json:"sync_status"`
	Neo4jNodeID      string     `json:"neo4j_node_id"`
	IsDeleted        bool       `json:"is_deleted"`
	SyncedAt         *time.Time `json:"synced_at"`
	SyncError        string     `json:"sync_error"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at"`
}

// TableName returns the table name for GraphEntity.
func (GraphEntity) TableName() string { return "graph_entities" }

// GraphRelationRecord is a row in the graph_relations table.
type GraphRelationRecord struct {
	ID                string     `json:"id"                  gorm:"type:varchar(36);primaryKey"`
	TenantID          uint64     `json:"tenant_id"`
	SourceRelationID  string     `json:"source_relation_id"`
	FromEntityID      string     `json:"from_entity_id"`
	ToEntityID        string     `json:"to_entity_id"`
	RelationType      string     `json:"relation_type"`
	RelationProps     JSON       `json:"relation_props"      gorm:"type:json"`
	SourceDocUUID     string     `json:"source_doc_uuid"`
	SourceSite        string     `json:"source_site"`
	SourceText        string     `json:"source_text"`
	ConfidenceScore   *float64   `json:"confidence_score"`
	ConfidenceReason  string     `json:"confidence_reason"`
	ReviewStatus      string     `json:"review_status"`
	SyncStatus        string     `json:"sync_status"`
	Neo4jRelationID   string     `json:"neo4j_relation_id"`
	IsDeleted         bool       `json:"is_deleted"`
	SyncedAt          *time.Time `json:"synced_at"`
	SyncError         string     `json:"sync_error"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at"`
}

// TableName returns the table name for GraphRelationRecord.
func (GraphRelationRecord) TableName() string { return "graph_relations" }

// GraphEntityInput is the request payload for a single entity in a batch upsert.
type GraphEntityInput struct {
	SourceEntityID   string                 `json:"source_entity_id"`
	EntityType       string                 `json:"entity_type"`
	EntityName       string                 `json:"entity_name"`
	EntityData       map[string]interface{} `json:"entity_data,omitempty"`
	SourceDocUUID    string                 `json:"source_doc_uuid,omitempty"`
	SourceSite       string                 `json:"source_site,omitempty"`
	SourceText       string                 `json:"source_text,omitempty"`
	ConfidenceScore  *float64               `json:"confidence_score,omitempty"`
	ConfidenceReason string                 `json:"confidence_reason,omitempty"`
	ReviewStatus     string                 `json:"review_status,omitempty"`
	IsDeleted        bool                   `json:"is_deleted,omitempty"`
}

// GraphRelationInput is the request payload for a single relation in a batch upsert.
type GraphRelationInput struct {
	SourceRelationID string                 `json:"source_relation_id"`
	FromEntityID     string                 `json:"from_entity_id"`
	ToEntityID       string                 `json:"to_entity_id"`
	RelationType     string                 `json:"relation_type"`
	RelationProps    map[string]interface{} `json:"relation_props,omitempty"`
	SourceDocUUID    string                 `json:"source_doc_uuid,omitempty"`
	SourceSite       string                 `json:"source_site,omitempty"`
	SourceText       string                 `json:"source_text,omitempty"`
	ConfidenceScore  *float64               `json:"confidence_score,omitempty"`
	ConfidenceReason string                 `json:"confidence_reason,omitempty"`
	ReviewStatus     string                 `json:"review_status,omitempty"`
	IsDeleted        bool                   `json:"is_deleted,omitempty"`
}

// GraphEntityBatchUpsertRequest is the request body for batch entity upsert.
type GraphEntityBatchUpsertRequest struct {
	Entities []*GraphEntityInput `json:"entities"`
}

// GraphRelationBatchUpsertRequest is the request body for batch relation upsert.
type GraphRelationBatchUpsertRequest struct {
	Relations []*GraphRelationInput `json:"relations"`
}

// GraphBatchUpsertResult reports the outcome of a batch upsert operation.
type GraphBatchUpsertResult struct {
	Upserted int `json:"upserted"`
	Deleted  int `json:"deleted"`
	Skipped  int `json:"skipped"`
}
