package service

import (
	"context"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/common/vo"
	"lunabox/backend/utils/apputils"
	"lunabox/backend/utils/httputils"
	"lunabox/backend/version"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"lunabox/backend/appruntime"
	"resty.dev/v3"
)

//go:embed templates/*.html
var builtinTemplates embed.FS

type TemplateService struct {
	ctx     context.Context
	db      *sql.DB
	config  *appconf.AppConfig
	runtime appruntime.Runtime
}

func NewTemplateService() *TemplateService {
	return &TemplateService{runtime: appruntime.Unavailable()}
}

//wails:ignore
func (s *TemplateService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

//wails:ignore
func (s *TemplateService) SetRuntime(runtime appruntime.Runtime) {
	if runtime != nil {
		s.runtime = runtime
	}
}

// ListTemplates 列出所有可用模板
func (s *TemplateService) ListTemplates() ([]vo.TemplateInfo, error) {
	templates := []vo.TemplateInfo{}

	// 读取内置模板
	entries, err := builtinTemplates.ReadDir("templates")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
				continue
			}
			content, err := builtinTemplates.ReadFile("templates/" + entry.Name())
			if err != nil {
				continue
			}
			info := s.parseTemplateMetadata(string(content), entry.Name(), true)
			templates = append(templates, info)
		}
	}

	// 2. 读取用户自定义模板
	userDir, err := apputils.GetTemplatesDir()
	if err != nil {
		applog.LogWarningf(s.ctx, "Failed to get user templates dir: %v", err)
	} else {
		userEntries, err := os.ReadDir(userDir)
		if err == nil {
			for _, entry := range userEntries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
					continue
				}
				filePath := filepath.Join(userDir, entry.Name())
				content, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}
				info := s.parseTemplateMetadata(string(content), entry.Name(), false)
				info.FilePath = filePath
				templates = append(templates, info)
			}
		}
	}

	return templates, nil
}

// parseTemplateMetadata 解析模板元数据
// 模板文件头部可以包含如下格式的元数据：
// <!--
// @name: 模板名称
// @description: 模板描述
// @author: 作者
// @version: 1.0.0
// -->
func (s *TemplateService) parseTemplateMetadata(content, filename string, isBuiltin bool) vo.TemplateInfo {
	id := strings.TrimSuffix(filename, ".html")
	info := vo.TemplateInfo{
		ID:        id,
		Name:      id,
		IsBuiltin: isBuiltin,
		Version:   "1.0.0",
	}

	// 解析元数据注释
	metaRegex := regexp.MustCompile(`<!--\s*([\s\S]*?)\s*-->`)
	matches := metaRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		metaContent := matches[1]
		lines := strings.Split(metaContent, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "@name:") {
				info.Name = strings.TrimSpace(strings.TrimPrefix(line, "@name:"))
			} else if strings.HasPrefix(line, "@description:") {
				info.Description = strings.TrimSpace(strings.TrimPrefix(line, "@description:"))
			} else if strings.HasPrefix(line, "@author:") {
				info.Author = strings.TrimSpace(strings.TrimPrefix(line, "@author:"))
			} else if strings.HasPrefix(line, "@version:") {
				info.Version = strings.TrimSpace(strings.TrimPrefix(line, "@version:"))
			}
		}
	}

	return info
}

// GetTemplate 获取模板内容
func (s *TemplateService) GetTemplate(templateID string) (string, error) {
	// 1. 尝试从内置模板读取
	builtinPath := "templates/" + templateID + ".html"
	content, err := builtinTemplates.ReadFile(builtinPath)
	if err == nil {
		return string(content), nil
	}

	// 2. 尝试从用户目录读取
	userDir, err := apputils.GetTemplatesDir()
	if err != nil {
		return "", fmt.Errorf("failed to get templates dir: %w", err)
	}

	userPath := filepath.Join(userDir, templateID+".html")
	content, err = os.ReadFile(userPath)
	if err != nil {
		return "", fmt.Errorf("template not found: %s", templateID)
	}

	return string(content), nil
}

// RenderTemplate 渲染模板
func (s *TemplateService) RenderTemplate(req vo.RenderTemplateRequest) (vo.RenderTemplateResponse, error) {
	var resp vo.RenderTemplateResponse

	// 获取模板内容
	templateContent, err := s.GetTemplate(req.TemplateID)
	if err != nil {
		return resp, err
	}

	// 填充应用信息
	req.Data.AppName = "LunaBox"
	req.Data.AppVersion = version.Version
	if req.Data.ExportTime == "" {
		req.Data.ExportTime = time.Now().Format("2006-01-02 15:04:05")
	}

	// 创建模板函数
	funcMap := template.FuncMap{
		"formatDuration": formatDuration,
		"json":           toJSON,
		"safeJS":         func(s string) template.JS { return template.JS(s) },
		"safeHTML":       func(s string) template.HTML { return template.HTML(s) },
		"add":            func(a, b int) int { return a + b },
		"sub":            func(a, b int) int { return a - b },
		"mul":            func(a, b int) int { return a * b },
		"div":            func(a, b int) int { return a / b },
	}

	// 解析并渲染模板
	tmpl, err := template.New("stats").Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return resp, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, newTemplateData(req.Data)); err != nil {
		return resp, fmt.Errorf("failed to execute template: %w", err)
	}

	resp.HTML = buf.String()
	return resp, nil
}

// templateData 是交给模板的数据。html/template 会把 src 里的 data: 地址替换成 #ZgotmplZ，
// 所以封面 Base64 以 template.URL 交给模板；同名字段遮住内嵌的 vo 字段，模板写法不变。
type templateData struct {
	vo.StatsExportData
	Leaderboard []templateGameItem `json:"leaderboard"`
}

type templateGameItem struct {
	vo.StatsGameItem
	CoverBase64 template.URL `json:"cover_base64"`
}

func newTemplateData(data vo.StatsExportData) templateData {
	out := templateData{StatsExportData: data, Leaderboard: make([]templateGameItem, len(data.Leaderboard))}
	for i, item := range data.Leaderboard {
		out.Leaderboard[i] = templateGameItem{StatsGameItem: item, CoverBase64: trustedImageURL(item.CoverBase64)}
	}
	return out
}

// trustedImageURL 只放行 fetchImageAsBase64 产生的地址：data:image/ 与 http(s)、站内路径，其余丢弃。
func trustedImageURL(s string) template.URL {
	for _, prefix := range []string{"data:image/", "http://", "https://", "/"} {
		if strings.HasPrefix(s, prefix) {
			return template.URL(s)
		}
	}
	return ""
}

// posterLeaderboardSize 是总结海报模板拿到的排行榜条数。
const posterLeaderboardSize = 10

// PrepareExportData 准备导出数据（处理图片等）
func (s *TemplateService) PrepareExportData(stats vo.PeriodStats, aiSummary string) (vo.StatsExportData, error) {
	data := vo.StatsExportData{
		ExportTime:        time.Now().Format("2006-01-02 15:04:05"),
		StartDate:         stats.StartDate,
		EndDate:           stats.EndDate,
		Period:            string(stats.Dimension),
		TotalPlayCount:    stats.TotalPlayCount,
		TotalPlayDuration: stats.TotalPlayDuration,
		TotalPlayTimeStr:  formatDuration(stats.TotalPlayDuration),
		AISummary:         aiSummary,
		AppName:           "LunaBox",
		AppVersion:        version.Version,
	}

	// 处理时间线数据（用于图表）
	var chartLabels []string
	var chartData []float64
	for _, point := range stats.Timeline {
		hours := float64(point.Duration) / 3600.0
		data.Timeline = append(data.Timeline, vo.StatsTimePoint{
			Label:       point.Label,
			Duration:    point.Duration,
			DurationStr: formatDuration(point.Duration),
			Hours:       hours,
		})
		chartLabels = append(chartLabels, point.Label)
		chartData = append(chartData, hours)
	}
	// 转为 JSON 字符串供模板使用
	if labelsJSON, err := json.Marshal(chartLabels); err == nil {
		data.ChartLabels = string(labelsJSON)
	}
	if dataJSON, err := json.Marshal(chartData); err == nil {
		data.ChartData = string(dataJSON)
	}

	// 处理游戏趋势数据
	colors := []string{
		"rgb(255, 99, 132)",
		"rgb(54, 162, 235)",
		"rgb(255, 206, 86)",
		"rgb(75, 192, 192)",
		"rgb(153, 102, 255)",
	}
	var gameTrendData []map[string]interface{}
	for i, series := range stats.LeaderboardSeries {
		trend := vo.StatsGameTrend{
			GameID:   series.GameID,
			GameName: series.GameName,
			Color:    colors[i%len(colors)],
		}
		var points []float64
		for _, point := range series.Points {
			hours := float64(point.Duration) / 3600.0
			trend.Points = append(trend.Points, vo.StatsTimePoint{
				Label:       point.Label,
				Duration:    point.Duration,
				DurationStr: formatDuration(point.Duration),
				Hours:       hours,
			})
			points = append(points, hours)
		}
		data.LeaderboardTrend = append(data.LeaderboardTrend, trend)
		gameTrendData = append(gameTrendData, map[string]interface{}{
			"label":           series.GameName,
			"data":            points,
			"borderColor":     colors[i%len(colors)],
			"backgroundColor": colors[i%len(colors)],
			"tension":         0.3,
		})
	}
	if trendJSON, err := json.Marshal(gameTrendData); err == nil {
		data.GameTrendData = string(trendJSON)
	}

	// 处理排行榜数据：海报只展示前 posterLeaderboardSize 名（统计接口返回的榜单更长，供“查看更多”使用）
	leaderboard := stats.PlayTimeLeaderboard[:min(posterLeaderboardSize, len(stats.PlayTimeLeaderboard))]
	for i, game := range leaderboard {
		item := vo.StatsGameItem{
			Rank:          i + 1,
			GameID:        game.GameID,
			GameName:      game.GameName,
			CoverURL:      game.CoverUrl,
			TotalDuration: game.TotalDuration,
			DurationStr:   formatDuration(game.TotalDuration),
		}

		// 尝试将封面转为Base64
		if game.CoverUrl != "" && !strings.HasPrefix(game.CoverUrl, "data:") {
			if base64Img, err := s.fetchImageAsBase64(game.CoverUrl); err == nil {
				item.CoverBase64 = base64Img
			}
		}

		data.Leaderboard = append(data.Leaderboard, item)
	}

	return data, nil
}

// fetchImageAsBase64 获取图片并转为Base64
func (s *TemplateService) fetchImageAsBase64(url string) (string, error) {
	// 本地图片从数据目录读取：MyGo 版的 WebView 没有 Wails 的资源服务，html2canvas 也不会被跨域图片污染画布
	if strings.HasPrefix(url, "/local/") || strings.Contains(url, "wails.localhost") {
		if i := strings.Index(url, "/local/"); i >= 0 {
			return readLocalImageAsBase64(url[i+len("/local/"):])
		}
		return url, nil
	}

	client, _, err := httputils.NewRestyClient(httputils.ClientOptions{
		Timeout:     30 * time.Second,
		ProxyConfig: s.config,
	})
	if err != nil {
		return "", fmt.Errorf("create image fetch client: %w", err)
	}
	resp, err := client.R().
		SetRetryCount(3).
		AddRetryConditions(
			resty.RetryConditionStatusTooManyRequests,
			resty.RetryConditionStatus5XX,
		).
		Get(url)
	if err != nil {
		return "", err
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode())
	}

	data := resp.Bytes()

	contentType := resp.Header().Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

// readLocalImageAsBase64 读取数据目录下的本地图片（/local/ 之后的路径，可带查询串）并转为Base64
func readLocalImageAsBase64(rel string) (string, error) {
	if i := strings.IndexAny(rel, "?#"); i >= 0 {
		rel = rel[:i]
	}
	dataDir, err := apputils.GetDataDir()
	if err != nil {
		return "", err
	}
	base, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, filepath.FromSlash(rel))
	if !strings.HasPrefix(path, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("local image outside data dir: %s", rel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") { // 嗅探不出 AVIF 等格式时按扩展名
		if byExt := mime.TypeByExtension(filepath.Ext(path)); byExt != "" {
			contentType = byExt
		}
	}
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data)), nil
}

// OpenTemplatesDir 打开模板目录
func (s *TemplateService) OpenTemplatesDir() error {
	dir, err := apputils.GetTemplatesDir()
	if err != nil {
		return err
	}

	return apputils.OpenDirectory(dir)
}

// ExportRenderedHTML 导出渲染后的HTML为图片
func (s *TemplateService) ExportRenderedHTML(base64Data string) error {
	// Remove header if present
	if idx := strings.Index(base64Data, ","); idx != -1 {
		base64Data = base64Data[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64 data: %w", err)
	}

	filename, err := s.runtime.SaveFile(appruntime.SaveDialogOptions{
		Filename: fmt.Sprintf("lunabox-stats-%s.png", time.Now().Format("20060102-150405")),
		Title:    "保存统计图片",
		Filters: []appruntime.FileFilter{
			{
				DisplayName: "PNG Images (*.png)",
				Pattern:     "*.png",
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to open save dialog: %w", err)
	}

	if filename == "" {
		return nil
	}

	return os.WriteFile(filename, data, 0644)
}

// formatDuration 格式化时长（秒转为可读格式）
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%d分钟", seconds/60)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if minutes == 0 {
		return fmt.Sprintf("%d小时", hours)
	}
	return fmt.Sprintf("%d小时%d分钟", hours, minutes)
}

// toJSON 将对象转为JSON字符串
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
