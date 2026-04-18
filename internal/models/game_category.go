package models

import "time"

type GameCategory struct {
	GameID     string    `json:"game_id"`
	CategoryID string    `json:"category_id"`
	UpdatedAt  time.Time `json:"updated_at"`
}
