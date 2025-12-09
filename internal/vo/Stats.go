package vo

type Stats struct {
	TotalPlayTimeSec   int    `json:"total_play_time_sec"`
	WeeklyPlayTimeSec  int    `json:"weekly_play_time_sec"`
	LongestGameID      string `json:"longest_game_id"`
	MostPlayedGameID   string `json:"most_played_game_id"`
	LongestGameName    string `json:"longest_game_name"`
	MostPlayedGameName string `json:"most_played_game_name"`
}
