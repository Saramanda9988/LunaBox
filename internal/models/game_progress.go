package models

import "time"

// GameProgress 游玩进度记录
type GameProgress struct {
	ID              string    `json:"id"`
	GameID          string    `json:"game_id"`
	Chapter         string    `json:"chapter"`
	Route           string    `json:"route"`
	ProgressNote    string    `json:"progress_note"`
	SpoilerBoundary string    `json:"spoiler_boundary"` // none | chapter_end | route_end | full
	UpdatedAt       time.Time `json:"updated_at"`
}
