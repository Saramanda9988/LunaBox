package playnite

import "time"

// PlayniteGame Playnite 导出的游戏数据结构（与 Game model 一致）只用作接收导入
// relate to internal/modles/game.go
type PlayniteGame struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	CoverURL           string    `json:"cover_url"`
	CoverSourceURL     string    `json:"cover_source_url"`
	Company            string    `json:"company"`
	Summary            string    `json:"summary"`
	Rating             float64   `json:"rating"`
	ReleaseDate        string    `json:"release_date"`
	Path               string    `json:"path"`
	GameDirectory      string    `json:"game_directory"`
	SavePath           *string   `json:"save_path"`
	ProcessName        string    `json:"process_name"`
	Status             string    `json:"status"`
	SourceType         string    `json:"source_type"`
	SourceID           string    `json:"source_id"`
	LaunchMode         string    `json:"launch_mode"`
	SteamLaunchID      string    `json:"steam_launch_id"`
	SteamLaunchKind    string    `json:"steam_launch_kind"`
	SteamLaunchOptions string    `json:"steam_launch_options"`
	Tags               []string  `json:"tags"`
	CachedAt           time.Time `json:"cached_at"`
	CreatedAt          time.Time `json:"created_at"`
}
