package models

import "time"

// GameReview stores the user's own rating and review for a game.
type GameReview struct {
	GameID    string    `json:"game_id"`
	Rating    *int      `json:"rating"`
	Content   string    `json:"content"`
	IsSpoiler bool      `json:"is_spoiler"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
