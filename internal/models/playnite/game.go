package playnite

import "time"

// PlayniteGame Playnite 导出的游戏数据结构（与 Game model 一致）只用作接收导入
// relate to internal/modles/game.go
type PlayniteGame struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CoverURL   string    `json:"cover_url"`
	Company    string    `json:"company"`
	Summary    string    `json:"summary"`
	Path       string    `json:"path"`
	SavePath   *string   `json:"save_path"`
	SourceType string    `json:"source_type"`
	SourceID   string    `json:"source_id"`
	CachedAt   time.Time `json:"cached_at"`
	CreatedAt  time.Time `json:"created_at"`
}
