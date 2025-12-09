package vo

import "lunabox/internal/models"

type HomePageData struct {
	RecentGames      []models.Game `json:"recent_games"`
	RecentlyAdded    []models.Game `json:"recently_added"`
	TodayPlayTimeSec int           `json:"today_play_time_sec"`
}
