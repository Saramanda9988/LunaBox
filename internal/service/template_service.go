package service

import (
	"context"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/utils"
	"lunabox/internal/version"
	"lunabox/internal/vo"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed templates/*.html
var builtinTemplates embed.FS

type TemplateService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewTemplateService() *TemplateService {
	return &TemplateService{}
}

func (s *TemplateService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
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
	userDir, err := utils.GetTemplatesDir()
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
	userDir, err := utils.GetTemplatesDir()
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
	if err := tmpl.Execute(&buf, req.Data); err != nil {
		return resp, fmt.Errorf("failed to execute template: %w", err)
	}

	resp.HTML = buf.String()
	return resp, nil
}

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

	// 处理排行榜数据
	for i, game := range stats.PlayTimeLeaderboard {
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
	// 跳过本地图片
	if strings.Contains(url, "wails.localhost") {
		return url, nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data), nil
}

// OpenTemplatesDir 打开模板目录
func (s *TemplateService) OpenTemplatesDir() error {
	dir, err := utils.GetTemplatesDir()
	if err != nil {
		return err
	}

	return utils.OpenDirectory(dir)
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

	filename, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		DefaultFilename: fmt.Sprintf("lunabox-stats-%s.png", time.Now().Format("20060102-150405")),
		Title:           "保存统计图片",
		Filters: []runtime.FileFilter{
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
