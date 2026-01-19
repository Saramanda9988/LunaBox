package models

import (
	"lunabox/internal/enums"
	"time"
)

type Game struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	CoverURL   string           `json:"cover_url"`
	Company    string           `json:"company"`
	Summary    string           `json:"summary"`
	Path       string           `json:"path"`        // 启动路径
	SavePath   string           `json:"save_path"`   // 存档目录路径
	Status     enums.GameStatus `json:"status"`      // 游戏状态: not_started, playing, completed, on_hold
	SourceType enums.SourceType `json:"source_type"` // "local", "bangumi", "vndb"
	CachedAt   time.Time        `json:"cached_at"`
	SourceID   string           `json:"source_id"`
	CreatedAt  time.Time        `json:"created_at"`
}

// GameBackup 游戏存档备份记录（基于文件系统，不使用数据库）
type GameBackup struct {
	Path      string    `json:"path"` // 备份文件路径（作为唯一标识）
	Name      string    `json:"name"` // 文件名
	GameID    string    `json:"game_id"`
	Size      int64     `json:"size"`       // 备份文件大小（字节）
	CreatedAt time.Time `json:"created_at"` // 创建时间（来自文件修改时间）
}
