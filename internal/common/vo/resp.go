package vo

import (
	enums "lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
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
	Tags   []metadata.TagItem
}

type MetadataRefreshResult struct {
	TotalGames   int `json:"total_games"`
	UpdatedGames int `json:"updated_games"`
	SkippedGames int `json:"skipped_games"`
	FailedGames  int `json:"failed_games"`
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
	Summary       string `json:"summary"`
	Dimension     string `json:"dimension"`
	WebSearchUsed bool   `json:"web_search_used"` // 是否使用了 WebSearch 增强
}

type ChatCompletionResponse struct {
	Choices []Choice  `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
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

// CloudSyncStatus 云同步状态
type CloudSyncStatus struct {
	Enabled        bool   `json:"enabled"`
	Configured     bool   `json:"configured"`
	Syncing        bool   `json:"syncing"`
	LastSyncTime   string `json:"last_sync_time"`
	LastSyncStatus string `json:"last_sync_status"`
	LastSyncError  string `json:"last_sync_error"`
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

type SpoilerContext struct {
	GlobalLevel string `json:"global_level"`
}

type MCPGameCatalogEntry struct {
	GameID       string     `json:"game_id"`
	Name         string     `json:"name"`
	Company      string     `json:"company,omitempty"`
	Status       string     `json:"status"`
	SourceType   string     `json:"source_type"`
	Rating       float64    `json:"rating"`
	ReleaseDate  string     `json:"release_date,omitempty"`
	LastPlayedAt *time.Time `json:"last_played_at,omitempty"`
}

type MCPListGamesResponse struct {
	Games   []MCPGameCatalogEntry `json:"games"`
	Limit   int                   `json:"limit"`
	Offset  int                   `json:"offset"`
	Total   int                   `json:"total"`
	HasMore bool                  `json:"has_more"`
}

type MCPGameTag struct {
	Name      string  `json:"name"`
	Source    string  `json:"source"`
	Weight    float64 `json:"weight"`
	IsSpoiler bool    `json:"is_spoiler"`
}

type MCPGameProgressSnapshot struct {
	Chapter         string    `json:"chapter,omitempty"`
	Route           string    `json:"route,omitempty"`
	ProgressNote    string    `json:"progress_note,omitempty"`
	SpoilerBoundary string    `json:"spoiler_boundary"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MCPGameDetail struct {
	GameID         string                   `json:"game_id"`
	Name           string                   `json:"name"`
	CoverURL       string                   `json:"cover_url,omitempty"`
	Company        string                   `json:"company,omitempty"`
	Summary        string                   `json:"summary,omitempty"`
	Rating         float64                  `json:"rating"`
	ReleaseDate    string                   `json:"release_date,omitempty"`
	Status         string                   `json:"status"`
	SourceType     string                   `json:"source_type"`
	SourceID       string                   `json:"source_id,omitempty"`
	LastPlayedAt   *time.Time               `json:"last_played_at,omitempty"`
	Categories     []string                 `json:"categories,omitempty"`
	Tags           []MCPGameTag             `json:"tags,omitempty"`
	LatestProgress *MCPGameProgressSnapshot `json:"latest_progress,omitempty"`
}

type MCPGetGameResponse struct {
	Game           MCPGameDetail  `json:"game"`
	SpoilerContext SpoilerContext `json:"spoiler_context"`
}

type MCPStartGameResponse struct {
	GameID  string `json:"game_id"`
	Name    string `json:"name,omitempty"`
	Started bool   `json:"started"`
	Message string `json:"message,omitempty"`
}

type MCPPlaySession struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  int       `json:"duration"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MCPPlaySessionsResponse struct {
	GameID   string           `json:"game_id"`
	Sessions []MCPPlaySession `json:"sessions"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	Total    int              `json:"total"`
	HasMore  bool             `json:"has_more"`
}

type MCPMetadataCandidate struct {
	Source      string       `json:"source"`
	SourceID    string       `json:"source_id"`
	Name        string       `json:"name"`
	CoverURL    string       `json:"cover_url,omitempty"`
	Company     string       `json:"company,omitempty"`
	Summary     string       `json:"summary,omitempty"`
	Rating      float64      `json:"rating"`
	ReleaseDate string       `json:"release_date,omitempty"`
	Tags        []MCPGameTag `json:"tags,omitempty"`
}

type MCPMetadataSearchResponse struct {
	Query          string                 `json:"query"`
	Results        []MCPMetadataCandidate `json:"results"`
	TotalResults   int                    `json:"total_results"`
	Limit          int                    `json:"limit"`
	SpoilerContext SpoilerContext         `json:"spoiler_context"`
}

type MCPGameStatisticTopGame struct {
	GameID          string   `json:"game_id"`
	Name            string   `json:"name"`
	Company         string   `json:"company,omitempty"`
	Duration        int      `json:"duration"`
	Summary         string   `json:"summary,omitempty"`
	Categories      []string `json:"categories,omitempty"`
	Status          string   `json:"status"`
	SpoilerBoundary string   `json:"spoiler_boundary"`
	ProgressNote    string   `json:"progress_note,omitempty"`
	Route           string   `json:"route,omitempty"`
}

type MCPGameStatisticSession struct {
	GameID    string    `json:"game_id"`
	GameName  string    `json:"game_name"`
	StartTime time.Time `json:"start_time"`
	Duration  int       `json:"duration"`
	DayOfWeek int       `json:"day_of_week"`
	Hour      int       `json:"hour"`
}

type MCPGameStatisticResponse struct {
	Period            string                    `json:"period"`
	StartDate         string                    `json:"start_date"`
	EndDate           string                    `json:"end_date"`
	DateRange         string                    `json:"date_range"`
	TotalPlayCount    int                       `json:"total_play_count"`
	TotalPlayDuration int                       `json:"total_play_duration"`
	TopGames          []MCPGameStatisticTopGame `json:"top_games"`
	RecentSessions    []MCPGameStatisticSession `json:"recent_sessions"`
	SpoilerContext    SpoilerContext            `json:"spoiler_context"`
}
