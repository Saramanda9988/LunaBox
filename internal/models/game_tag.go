package models

import "time"

type GameTag struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"` // 'bangumi' | 'vndb' | 'user'
	Weight    float64   `json:"weight"`
	IsSpoiler bool      `json:"is_spoiler"`
	CreatedAt time.Time `json:"created_at"`
}
