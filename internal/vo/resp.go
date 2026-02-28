package vo

import (
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"time"
)

type CategoryVO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Emoji     string    `json:"emoji"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	GameCount int       `json:"game_count"`
}

type GameMetadataFromWebVO struct {
	Source enums.SourceType
	Game   models.Game
}

// LastPlayedGame 上次游玩的游戏信息
type LastPlayedGame struct {
	Game           models.Game `json:"game"`
	LastPlayedAt   time.Time   `json:"last_played_at"`   // 上次游玩时间
	LastPlayedDur  int         `json:"last_played_dur"`  // 上次游玩时长（秒）
	TotalPlayedDur int         `json:"total_played_dur"` // 总游玩时长（秒）
	IsPlaying      bool        `json:"is_playing"`       // 是否正在游玩
}

type HomePageData struct {
	LastPlayed        *LastPlayedGame `json:"last_played"` // 上次游玩的游戏
	TodayPlayTimeSec  int             `json:"today_play_time_sec"`
	WeeklyPlayTimeSec int             `json:"weekly_play_time_sec"`
}

type DailyPlayTime struct {
	Date     string `json:"date"`     // YYYY-MM-DD
	Duration int    `json:"duration"` // seconds
}

type GameDetailStats struct {
	Dimension         string          `json:"dimension"`  // week, month, all
	StartDate         string          `json:"start_date"` // YYYY-MM-DD
	EndDate           string          `json:"end_date"`   // YYYY-MM-DD
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
	Dimension              enums.Period      `json:"dimension"`  // day, week, month
	StartDate              string            `json:"start_date"` // YYYY-MM-DD
	EndDate                string            `json:"end_date"`   // YYYY-MM-DD
	TotalPlayCount         int               `json:"total_play_count"`
	TotalPlayDuration      int               `json:"total_play_duration"`
	TotalGamesCount        int               `json:"total_games_count"`         // 本期间内游玩过的游戏数量
	CompletedGamesCount    int               `json:"completed_games_count"`     // 本期间内已通关游戏数量
	LibraryGamesCount      int               `json:"library_games_count"`       // 库中所有游戏数量
	AllSessionsCount       int               `json:"all_sessions_count"`        // 所有session数量
	AllSessionsDuration    int               `json:"all_sessions_duration"`     // 所有session总时长
	AllCompletedGamesCount int               `json:"all_completed_games_count"` // 所有已通关游戏数量
	PlayTimeLeaderboard    []GamePlayStats   `json:"play_time_leaderboard"`
	Timeline               []TimePoint       `json:"timeline"`
	LeaderboardSeries      []GameTrendSeries `json:"leaderboard_series"`
}

// AISummaryResponse AI总结响应
type AISummaryResponse struct {
	Summary   string `json:"summary"`
	Dimension string `json:"dimension"`
}

type ChatCompletionResponse struct {
	Choices []Choice  `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
}

type Choice struct {
	Message Message `json:"message"`
}

type APIError struct {
	Message string `json:"message"`
}

// CloudBackupStatus 云备份状态
type CloudBackupStatus struct {
	Enabled    bool   `json:"enabled"`    // 是否启用
	Configured bool   `json:"configured"` // 是否已配置
	UserID     string `json:"user_id"`    // 用户标识
	Provider   string `json:"provider"`   // 云备份提供商: s3, onedrive
}

// CloudBackupItem 云端备份项
type CloudBackupItem struct {
	Key       string    `json:"key"`        // S3 对象 key
	Name      string    `json:"name"`       // 显示名称
	Size      int64     `json:"size"`       // 文件大小
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// DBBackupInfo 数据库备份信息
type DBBackupInfo struct {
	Path      string    `json:"path"`       // 备份文件路径
	Name      string    `json:"name"`       // 显示名称
	Size      int64     `json:"size"`       // 文件大小
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// DBBackupStatus 数据库备份状态
type DBBackupStatus struct {
	LastBackupTime string         `json:"last_backup_time"` // 上次备份时间
	Backups        []DBBackupInfo `json:"backups"`          // 备份列表
}

// RenderTemplateResponse 渲染模板响应
type RenderTemplateResponse struct {
	HTML string `json:"html"` // 渲染后的HTML
}
