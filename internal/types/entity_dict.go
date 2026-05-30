package types

import "time"

// EntityDictRecord mirrors a row from ZK's entity_dict table.
type EntityDictRecord struct {
	ID              int64     `json:"id"               gorm:"primaryKey"`
	EntityType      string    `json:"entity_type"`
	ExternalIDs     JSON      `json:"external_ids"     gorm:"type:json"`
	CanonicalData   JSON      `json:"canonical_data"   gorm:"type:json"`
	CanonicalSource string    `json:"canonical_source"`
	IsDeleted       bool      `json:"is_deleted"`
	SyncedAt        *time.Time `json:"synced_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName returns the table name for EntityDictRecord.
func (EntityDictRecord) TableName() string { return "entity_dict" }

// EntityDictBatchUpsertRequest is the request payload for batch upserting entity_dict rows.
type EntityDictBatchUpsertRequest struct {
	Rows []*EntityDictRecord `json:"rows"`
}
