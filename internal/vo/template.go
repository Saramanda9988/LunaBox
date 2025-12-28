package vo

// StatsExportData 统计导出数据，用于模板渲染
type StatsExportData struct {
	// 元数据
	ExportTime string `json:"export_time"` // 导出时间
	StartDate  string `json:"start_date"`  // 统计开始日期
	EndDate    string `json:"end_date"`    // 统计结束日期
	Period     string `json:"period"`      // 统计周期：week/month/custom

	// 概览数据
	TotalPlayCount    int    `json:"total_play_count"`    // 总游玩次数
	TotalPlayDuration int    `json:"total_play_duration"` // 总游玩时长（秒）
	TotalPlayTimeStr  string `json:"total_play_time_str"` // 格式化的总游玩时长

	// 排行榜数据
	Leaderboard []StatsGameItem `json:"leaderboard"` // 排行榜

	// 图表数据（Base64图片）
	TrendChartImage     string `json:"trend_chart_image"`      // 趋势图表图片
	GameTrendChartImage string `json:"game_trend_chart_image"` // 游戏趋势图表图片

	// AI总结
	AISummary string `json:"ai_summary"` // AI总结内容

	// 应用信息
	AppName    string `json:"app_name"`    // 应用名称
	AppVersion string `json:"app_version"` // 应用版本
}

// StatsGameItem 统计游戏项
type StatsGameItem struct {
	Rank          int    `json:"rank"`           // 排名
	GameID        string `json:"game_id"`        // 游戏ID
	GameName      string `json:"game_name"`      // 游戏名称
	CoverURL      string `json:"cover_url"`      // 封面URL
	CoverBase64   string `json:"cover_base64"`   // 封面Base64（避免CORS）
	TotalDuration int    `json:"total_duration"` // 总时长（秒）
	DurationStr   string `json:"duration_str"`   // 格式化时长
}

// TemplateInfo 模板信息
type TemplateInfo struct {
	ID          string `json:"id"`          // 模板唯一标识（文件名不含扩展名）
	Name        string `json:"name"`        // 模板显示名称
	Description string `json:"description"` // 模板描述
	Author      string `json:"author"`      // 模板作者
	Version     string `json:"version"`     // 模板版本
	Preview     string `json:"preview"`     // 预览图（Base64或URL）
	IsBuiltin   bool   `json:"is_builtin"`  // 是否内置模板
	FilePath    string `json:"file_path"`   // 模板文件路径
}
