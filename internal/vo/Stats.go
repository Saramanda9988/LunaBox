package vo

type DailyPlayTime struct {
	Date     string `json:"date"`     // YYYY-MM-DD
	Duration int    `json:"duration"` // seconds
}

type GameDetailStats struct {
	TotalPlayTime     int             `json:"total_play_time"`
	TodayPlayTime     int             `json:"today_play_time"`
	RecentPlayHistory []DailyPlayTime `json:"recent_play_history"`
}

type GamePlayStats struct {
	GameID        string `json:"game_id"`
	GameName      string `json:"game_name"`
	TotalDuration int    `json:"total_duration"`
}

type GamePlayCount struct {
	GameID    string `json:"game_id"`
	GameName  string `json:"game_name"`
	PlayCount int    `json:"play_count"`
}

type GlobalStats struct {
	TotalPlayTime       int             `json:"total_play_time"`
	WeeklyPlayTime      int             `json:"weekly_play_time"`
	PlayTimeLeaderboard []GamePlayStats `json:"play_time_leaderboard"`
	MostPlayedGame      GamePlayCount   `json:"most_played_game"`
}
