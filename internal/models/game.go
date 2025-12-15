package models

import (
	"lunabox/internal/enums"
	"time"
)

type Game struct {
	ID         string           `json:"id"`
	UserID     string           `json:"user_id"`
	Name       string           `json:"name"`
	CoverURL   string           `json:"cover_url"`
	Company    string           `json:"company"`
	Summary    string           `json:"summary"`
	Path       string           `json:"path"`        // 启动路径
	SourceType enums.SourceType `json:"source_type"` // "local", "bangumi", "vndb"
	CachedAt   time.Time        `json:"cached_at"`
	SourceID   string           `json:"source_id"`
	CreatedAt  time.Time        `json:"created_at"`
}
