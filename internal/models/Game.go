package models

import "time"

type Game struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	CoverURL   string    `json:"cover_url"`
	Company    string    `json:"company"`
	Summary    string    `json:"summary"`
	SourceType string    `json:"source_type"` // "local", "bangumi", "vndb"
	SourceID   string    `json:"source_id"`
	CachedAt   time.Time `json:"cached_at"`
	CreatedAt  time.Time `json:"created_at"`
}
