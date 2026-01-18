package vo

import (
	"lunabox/internal/enums"
	"lunabox/internal/models"
)

type AISummaryRequest struct {
	Dimension string `json:"dimension"` // week, month, year
}

type MetadataRequest struct {
	Source enums.SourceType `json:"source"` // "bangumi" or "vndb"
	ID     string           `json:"id"`
}

// BatchImportCandidate 批量导入候选项
type BatchImportCandidate struct {
	FolderPath  string           `json:"folder_path"`            // 文件夹路径
	FolderName  string           `json:"folder_name"`            // 文件夹名
	Executables []string         `json:"executables"`            // 检测到的可执行文件列表
	SelectedExe string           `json:"selected_exe"`           // 选中的可执行文件
	SearchName  string           `json:"search_name"`            // 用于搜索的名称（用户可编辑）
	IsSelected  bool             `json:"is_selected"`            // 是否选中导入
	MatchedGame *models.Game     `json:"matched_game,omitempty"` // 匹配到的游戏信息
	MatchSource enums.SourceType `json:"match_source,omitempty"` // 匹配来源
	MatchStatus string           `json:"match_status"`           // 匹配状态: pending, matched, not_found, error
}

// BatchImportRequest 批量导入请求
type BatchImportRequest struct {
	Candidates []BatchImportCandidate `json:"candidates"`
}

// ChatCompletionRequest OpenAI兼容的API请求/响应结构
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PeriodStatsRequest 统计请求参数
type PeriodStatsRequest struct {
	Dimension enums.Period `json:"dimension"`  // day, week, month
	StartDate string       `json:"start_date"` // YYYY-MM-DD (可选，不传则使用默认范围)
	EndDate   string       `json:"end_date"`   // YYYY-MM-DD (可选，不传则使用默认范围)
}

// GameStatsRequest 游戏统计请求参数
type GameStatsRequest struct {
	GameID    string       `json:"game_id"`
	Dimension enums.Period `json:"dimension"`  // week, month, all
	StartDate string       `json:"start_date"` // YYYY-MM-DD (可选，不传则使用默认范围)
	EndDate   string       `json:"end_date"`   // YYYY-MM-DD (可选，不传则使用默认范围)
}

// RenderTemplateRequest 渲染模板请求
type RenderTemplateRequest struct {
	TemplateID string          `json:"template_id"` // 模板ID
	Data       StatsExportData `json:"data"`        // 导出数据
}
