package models

import (
	"lunabox/internal/common/enums"
	"time"
)

type Game struct {
	ID                 string               `json:"id"`
	Name               string               `json:"name"`
	Aliases            []string             `json:"aliases"`
	CoverURL           string               `json:"cover_url"`
	CoverSourceURL     string               `json:"cover_source_url"` // 刮削封面的远程源地址，本地封面不可用时用于回退
	Company            string               `json:"company"`
	Summary            string               `json:"summary"`
	Rating             float64              `json:"rating"`            // 游戏评分（统一按 10 分制存储）
	ReleaseDate        string               `json:"release_date"`      // 发售日期（源站原始日期字符串）
	Path               string               `json:"path"`              // 启动路径
	GameDirectory      string               `json:"game_directory"`    // 游戏根目录，不一定是启动文件的上一级
	SavePath           string               `json:"save_path"`         // 存档目录路径
	ProcessName        string               `json:"process_name"`      // 实际监控的进程名（当启动器和游戏进程不同时使用）
	WineRunner         string               `json:"wine_runner"`       // macOS/Linux：兼容层类型（system/crossover/custom）
	WineArgs           string               `json:"wine_args"`         // macOS/Linux：追加给 Wine/Proton 的启动参数
	WinePrefix         string               `json:"wine_prefix"`       // macOS/Linux：WINEPREFIX、CrossOver bottle 或 Proton prefix
	LaunchMode         enums.LaunchMode     `json:"launch_mode"`       // 启动方式: normal, steam, compatibility
	SteamLaunchID      string               `json:"steam_launch_id"`   // 本机 Steam 启动标识：原生 AppID 或非 Steam 快捷方式的长 ID
	SteamLaunchKind    string               `json:"steam_launch_kind"` // 本机关联类型：native 或 shortcut
	SteamUserID        string               `json:"steam_user_id"`     // 非 Steam 快捷方式所属的 Steam account ID
	SteamLaunchOptions string               `json:"steam_launch_options"`
	Status             enums.GameStatus     `json:"status"`      // 游戏状态: not_started, want_to_play, playing, completed, on_hold
	SourceType         enums.SourceType     `json:"source_type"` // 默认元数据来源
	MetadataSources    []GameMetadataSource `json:"metadata_sources"`
	CachedAt           time.Time            `json:"cached_at"`
	SourceID           string               `json:"source_id"` // 默认元数据来源 ID
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
	UseLocaleEmulator  bool                 `json:"use_locale_emulator"`      // 是否使用 Locale Emulator 转区启动
	UseMagpie          bool                 `json:"use_magpie"`               // 是否使用 Magpie 超分辨率缩放
	IsNSFW             bool                 `json:"is_nsfw"`                  // 是否为 NSFW 游戏
	MetadataLocked     bool                 `json:"metadata_locked"`          // 是否锁定远程元数据更新
	LastPlayedAt       *time.Time           `json:"last_played_at,omitempty"` // 最近一次游玩开始时间（由 play_sessions 聚合）
}

// GameBackup 游戏存档备份记录（基于文件系统，不使用数据库）
type GameBackup struct {
	Path      string    `json:"path"` // 备份文件路径（作为唯一标识）
	Name      string    `json:"name"` // 文件名
	GameID    string    `json:"game_id"`
	Size      int64     `json:"size"`       // 备份文件大小（字节）
	CreatedAt time.Time `json:"created_at"` // 创建时间（来自文件修改时间）
}
