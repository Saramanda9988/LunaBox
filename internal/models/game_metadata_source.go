package models

import (
	"lunabox/internal/common/enums"
	"time"
)

// GameMetadataSource identifies a game at one remote metadata provider.
type GameMetadataSource struct {
	GameID     string           `json:"game_id"`
	SourceType enums.SourceType `json:"source_type"`
	SourceID   string           `json:"source_id"`
	CachedAt   time.Time        `json:"cached_at"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}
