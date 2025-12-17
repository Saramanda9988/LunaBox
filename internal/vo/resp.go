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

type HomePageData struct {
	RecentGames       []models.Game `json:"recent_games"`
	RecentlyAdded     []models.Game `json:"recently_added"`
	TodayPlayTimeSec  int           `json:"today_play_time_sec"`
	WeeklyPlayTimeSec int           `json:"weekly_play_time_sec"`
}

type DailyPlayTime struct {
	Date     string `json:"date"`     // YYYY-MM-DD
	Duration int    `json:"duration"` // seconds
}

type GameDetailStats struct {
	TotalPlayCount    int             `json:"total_play_count"`
	TotalPlayTime     int             `json:"total_play_time"`
	TodayPlayTime     int             `json:"today_play_time"`
	RecentPlayHistory []DailyPlayTime `json:"recent_play_history"`
}

type GamePlayStats struct {
	GameID        string `json:"game_id"`
	GameName      string `json:"game_name"`
	CoverUrl      string `json:"cover_url"`
	TotalDuration int    `json:"total_duration"`
}

type GamePlayCount struct {
	GameID    string `json:"game_id"`
	GameName  string `json:"game_name"`
	PlayCount int    `json:"play_count"`
}

type TimePoint struct {
	Label    string `json:"label"`    // YYYY-MM-DD or YYYY-MM
	Duration int    `json:"duration"` // seconds
}

type GameTrendSeries struct {
	GameID   string      `json:"game_id"`
	GameName string      `json:"game_name"`
	Points   []TimePoint `json:"points"`
}

type PeriodStats struct {
	Dimension           enums.Period      `json:"dimension"` // week, month, year
	TotalPlayCount      int               `json:"total_play_count"`
	TotalPlayDuration   int               `json:"total_play_duration"`
	PlayTimeLeaderboard []GamePlayStats   `json:"play_time_leaderboard"`
	Timeline            []TimePoint       `json:"timeline"`
	LeaderboardSeries   []GameTrendSeries `json:"leaderboard_series"`
}

// AISummaryResponse AI总结响应
type AISummaryResponse struct {
	Summary   string `json:"summary"`
	Dimension string `json:"dimension"`
}
