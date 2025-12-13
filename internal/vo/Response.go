package vo

import (
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"time"
)

type CategoryVO struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	GameCount int       `json:"game_count"`
}

type GameMetadataFromWebVO struct {
	Source enums.SourceType
	Game   models.Game
}
