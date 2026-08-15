package models

import (
	"lunabox/internal/common/enums"
	"time"
)

type GameFilterPreset struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Tags          []string         `json:"tags"`
	ExcludeTags   bool             `json:"exclude_tags"`
	Status        enums.GameStatus `json:"status"`
	ExcludeStatus bool             `json:"exclude_status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}
