package models

import "time"

type SyncTombstone struct {
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	ParentID    string    `json:"parent_id"`
	SecondaryID string    `json:"secondary_id"`
	DeletedAt   time.Time `json:"deleted_at"`
}
