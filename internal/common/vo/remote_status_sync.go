package vo

// RemoteStatusSyncProgress describes the progress and result of uploading all
// local game statuses for one remote metadata provider.
type RemoteStatusSyncProgress struct {
	Provider        string   `json:"provider"`
	Status          string   `json:"status"`
	Current         int      `json:"current"`
	Total           int      `json:"total"`
	GameName        string   `json:"game_name"`
	SucceededGames  int      `json:"succeeded_games"`
	FailedGames     int      `json:"failed_games"`
	FailedGameNames []string `json:"failed_game_names"`
	LastError       string   `json:"last_error"`
}
