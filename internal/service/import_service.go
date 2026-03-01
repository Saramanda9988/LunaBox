package service

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/models/playnite"
	"lunabox/internal/models/potatovn"
	"lunabox/internal/models/vnite"
	"lunabox/internal/utils"
	"lunabox/internal/vo"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ImportResult 导入结果
type ImportResult struct {
	Success          int      `json:"success"`           // 成功导入数量
	Skipped          int      `json:"skipped"`           // 跳过数量（已存在）
	Failed           int      `json:"failed"`            // 失败数量
	FailedNames      []string `json:"failed_names"`      // 失败的游戏名称
	SkippedNames     []string `json:"skipped_names"`     // 跳过的游戏名称
	SessionsImported int      `json:"sessions_imported"` // 导入的游玩记录数量
}

type ImportService struct {
	ctx            context.Context
	db             *sql.DB
	config         *appconf.AppConfig
	gameService    *GameService
	sessionService *SessionService
}

func NewImportService() *ImportService {
	return &ImportService{}
}

func (s *ImportService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig, gameService *GameService) {
	s.ctx = ctx
	s.db = db
	s.config = config
	s.gameService = gameService
}

// SetSessionService SetStartService 设置 SessionService（用于导入游玩记录）
func (s *ImportService) SetSessionService(sessionService *SessionService) {
	s.sessionService = sessionService
}

// =================== PotatoVN 导入功能 ====================

// SelectZipFile 选择要导入的 ZIP 文件
func (s *ImportService) SelectZipFile() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择 PotatoVN 导出的 ZIP 文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "ZIP 文件",
				Pattern:     "*.zip",
			},
		},
	})
	return selection, err
}

// ImportFromPotatoVN 从 PotatoVN 导出的 ZIP 文件导入数据
func (s *ImportService) ImportFromPotatoVN(zipPath string, skipNoPath bool) (ImportResult, error) {
	result := ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}

	// 打开 ZIP 文件
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open ZIP file: %v", err)
		return result, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", "potatovn_import_*")
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to create temp dir: %v", err)
		return result, fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压文件
	if err := utils.ExtractZip(zipReader, tempDir); err != nil {
		applog.LogErrorf(s.ctx, "failed to extract ZIP: %v", err)
		return result, fmt.Errorf("解压失败: %w", err)
	}

	// 读取 data.galgames.json
	galgamesPath := filepath.Join(tempDir, "data.galgames.json")
	galgamesData, err := os.ReadFile(galgamesPath)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to read data.galgames.json: %v", err)
		return result, fmt.Errorf("无法读取 data.galgames.json: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(galgamesData, &galgames); err != nil {
		applog.LogErrorf(s.ctx, "failed to unmarshal data.galgames.json: %v", err)
		return result, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to get existing games: %v", err)
		return result, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	// 按名称和路径分别建立索引
	existingNames := make(map[string]string) // name -> id
	existingPaths := make(map[string]string) // path -> name
	for _, g := range existingGames {
		if g.Name != "" {
			existingNames[strings.ToLower(g.Name)] = g.ID
		}
		if g.Path != "" {
			existingPaths[g.Path] = g.Name
		}
	}

	// 导入每个游戏
	for _, galgame := range galgames {
		gameName := galgame.GetDisplayName()
		exePath := galgame.GetExePath()
		hasPath := exePath != ""

		// 检查启动路径是否已存在
		if hasPath {
			if existingName, exists := existingPaths[exePath]; exists {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, gameName+" (路径已存在: "+existingName+")")
				continue
			}
		}

		// 检查同名游戏是否存在（同名但路径不同允许导入）
		if existingID, exists := existingNames[strings.ToLower(gameName)]; exists {
			// 检查是否是同一路径（完全重复）
			for _, g := range existingGames {
				if g.ID == existingID && g.Path == exePath {
					result.Skipped++
					result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
					continue
				}
			}
			// 同名但路径不同，允许导入
			applog.LogInfof(s.ctx, "ImportFromPotatoVN: importing duplicate name %s with different path: %s", gameName, exePath)
		}

		// 如果设置跳过无路径的游戏，且当前游戏无路径，则跳过
		if skipNoPath && !hasPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		// 转换并导入游戏
		game, sessions := s.convertToGame(galgame, tempDir)

		if err := s.gameService.AddGame(game); err != nil {
			applog.LogErrorf(s.ctx, "failed to add game %s: %v", gameName, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, gameName)
			continue
		}

		// 导入游玩记录
		if len(sessions) > 0 && s.sessionService != nil {
			if err := s.sessionService.BatchAddPlaySessions(sessions); err != nil {
				applog.LogWarningf(s.ctx, "failed to import play sessions for game %s: %v", gameName, err)
				// 游玩记录导入失败不影响游戏导入成功
			} else {
				applog.LogInfof(s.ctx, "imported %d play sessions for game %s", len(sessions), gameName)
				result.SessionsImported += len(sessions)
			}
		}

		// 更新索引
		existingNames[strings.ToLower(gameName)] = game.ID
		if hasPath {
			existingPaths[exePath] = gameName
		}
		result.Success++
	}

	return result, nil
}

// convertToGame 将 PotatoVN 的 Galgame 转换为本地的 Game 模型
// 同时返回解析后的游玩记录
func (s *ImportService) convertToGame(galgame potatovn.Galgame, tempDir string) (models.Game, []models.PlaySession) {
	gameID := uuid.New().String()
	game := models.Game{
		ID:         gameID,
		Name:       galgame.GetDisplayName(),
		Company:    galgame.Developer.Value,
		Summary:    galgame.Description.Value,
		Path:       galgame.GetExePath(),
		SavePath:   galgame.GetSavePath(),
		SourceType: s.mapRssTypeToSourceType(galgame.RssType),
		SourceID:   galgame.GetSourceID(),
		CreatedAt:  galgame.AddTime.ToTime(),
		CachedAt:   time.Now(),
	}

	// 处理封面图片
	if galgame.ImagePath.Value != "" && galgame.ImagePath.Value != potatovn.DefaultImagePath {
		// 尝试从解压目录中获取封面图片
		coverPath := utils.ResolveCoverPath(galgame.ImagePath.Value, tempDir)
		if coverPath != "" {
			// 将封面图片复制到应用的封面目录
			savedPath, err := utils.SaveCoverImage(coverPath, game.ID)
			if err == nil {
				game.CoverURL = savedPath
			} else {
				applog.LogErrorf(s.ctx, "failed to save cover image for game %s: %v", game.Name, err)
			}
		} else {
			applog.LogErrorf(s.ctx, "cover image not found for game %s, path: %s", game.Name, galgame.ImagePath.Value)
		}
	}

	// 如果 CreatedAt 是零值，使用当前时间
	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	// 解析 PlayedTime 生成游玩记录
	var sessions []models.PlaySession
	if len(galgame.PlayedTime) > 0 {
		sessions = s.parsePlayedTime(gameID, galgame.PlayedTime)
	}

	return game, sessions
}

// mapRssTypeToSourceType 将 PotatoVN 的 RssType 映射到本地的 SourceType
func (s *ImportService) mapRssTypeToSourceType(rssType potatovn.RssType) enums.SourceType {
	switch rssType {
	case potatovn.RssTypeBangumi:
		return enums.Bangumi
	case potatovn.RssTypeVndb:
		return enums.VNDB
	case potatovn.RssTypeYmgal:
		return enums.Ymgal
	default:
		return enums.Local
	}
}

// parsePlayedTime 解析 PotatoVN 的 PlayedTime 字段，生成游玩记录
// PlayedTime 格式: map[string]int，key 为日期（如 "2026/1/12"），value 为游玩时长（分钟）
func (s *ImportService) parsePlayedTime(gameID string, playedTime map[string]int) []models.PlaySession {
	var sessions []models.PlaySession

	for dateStr, durationMinutes := range playedTime {
		if durationMinutes <= 0 {
			continue
		}

		// 解析日期，支持 "2026/1/12" 格式
		parsedTime, err := time.Parse("2006/1/2", dateStr)
		if err != nil {
			// 尝试其他格式
			parsedTime, err = time.Parse("2006/01/02", dateStr)
			if err != nil {
				applog.LogWarningf(s.ctx, "parsePlayedTime: failed to parse date %s: %v", dateStr, err)
				continue
			}
		}

		// 设置为当天中午12点作为开始时间（避免时区问题）
		startTime := time.Date(parsedTime.Year(), parsedTime.Month(), parsedTime.Day(), 12, 0, 0, 0, time.Local)
		durationSeconds := durationMinutes * 60
		endTime := startTime.Add(time.Duration(durationMinutes) * time.Minute)

		session := models.PlaySession{
			ID:        uuid.New().String(),
			GameID:    gameID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  durationSeconds,
		}
		sessions = append(sessions, session)
	}

	return sessions
}

// PreviewImport 预览导入内容（不实际导入）
func (s *ImportService) PreviewImport(zipPath string) ([]PreviewGame, error) {
	// 打开 ZIP 文件
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewImport: failed to open ZIP file: %v", err)
		return nil, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", "potatovn_preview_*")
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewImport: failed to create temp dir: %v", err)
		return nil, fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 只解压 data.galgames.json
	found := false
	for _, file := range zipReader.File {
		if file.Name == "data.galgames.json" {
			filePath := filepath.Join(tempDir, file.Name)
			destFile, err := os.Create(filePath)
			if err != nil {
				applog.LogErrorf(s.ctx, "PreviewImport: failed to create data.galgames.json: %v", err)
				return nil, err
			}

			srcFile, err := file.Open()
			if err != nil {
				applog.LogErrorf(s.ctx, "PreviewImport: failed to open data.galgames.json in ZIP: %v", err)
				destFile.Close()
				return nil, err
			}

			_, err = io.Copy(destFile, srcFile)
			srcFile.Close()
			destFile.Close()

			if err != nil {
				applog.LogErrorf(s.ctx, "PreviewImport: failed to copy data.galgames.json: %v", err)
				return nil, err
			}
			found = true
			break
		}
	}
	if !found {
		applog.LogWarningf(s.ctx, "PreviewImport: data.galgames.json not found in ZIP: %s", zipPath)
	}

	// 读取 data.galgames.json
	galgamesPath := filepath.Join(tempDir, "data.galgames.json")
	galgamesData, err := os.ReadFile(galgamesPath)
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewImport: failed to read data.galgames.json: %v", err)
		return nil, fmt.Errorf("无法读取 data.galgames.json: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(galgamesData, &galgames); err != nil {
		applog.LogErrorf(s.ctx, "PreviewImport: failed to unmarshal data.galgames.json: %v", err)
		return nil, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewImport: failed to get existing games: %v", err)
		return nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	// 构建预览列表
	var previews []PreviewGame
	for _, galgame := range galgames {
		name := galgame.GetDisplayName()
		preview := PreviewGame{
			Name:       name,
			Developer:  galgame.Developer.Value,
			SourceType: string(s.mapRssTypeToSourceType(galgame.RssType)),
			Exists:     existingNames[strings.ToLower(name)],
			AddTime:    galgame.AddTime.ToTime(),
			HasPath:    galgame.GetExePath() != "",
		}
		previews = append(previews, preview)
	}

	return previews, nil
}

// =================== Playnite 导入功能 ====================

// PreviewGame 预览导入的游戏信息
type PreviewGame struct {
	Name       string    `json:"name"`
	Developer  string    `json:"developer"`
	SourceType string    `json:"source_type"`
	Exists     bool      `json:"exists"`
	AddTime    time.Time `json:"add_time"`
	HasPath    bool      `json:"has_path"` // 用于 Playnite 导入，标记是否有路径
}

// SelectJSONFile 选择要导入的 JSON 文件
func (s *ImportService) SelectJSONFile() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择 Playnite 导出的 JSON 文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "JSON 文件",
				Pattern:     "*.json",
			},
		},
	})
	return selection, err
}

// PreviewPlayniteImport 预览 Playnite 导入内容（不实际导入）
func (s *ImportService) PreviewPlayniteImport(jsonPath string) ([]PreviewGame, error) {
	// 读取 JSON 文件
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewPlayniteImport: failed to read JSON file: %v", err)
		return nil, fmt.Errorf("无法读取 JSON 文件: %w", err)
	}

	// 移除 UTF-8 BOM（如果存在）
	utf8BOM := []byte{0xEF, 0xBB, 0xBF}
	jsonData = bytes.TrimPrefix(jsonData, utf8BOM)

	var playniteGames []playnite.PlayniteGame
	if err := json.Unmarshal(jsonData, &playniteGames); err != nil {
		applog.LogErrorf(s.ctx, "PreviewPlayniteImport: failed to unmarshal JSON: %v", err)
		return nil, fmt.Errorf("解析 JSON 文件失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewPlayniteImport: failed to get existing games: %v", err)
		return nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	// 构建预览列表
	var previews []PreviewGame
	for _, pg := range playniteGames {
		preview := PreviewGame{
			Name:       pg.Name,
			Developer:  pg.Company,
			SourceType: pg.SourceType,
			Exists:     existingNames[strings.ToLower(pg.Name)],
			AddTime:    pg.CreatedAt,
			HasPath:    pg.Path != "",
		}
		previews = append(previews, preview)
	}

	return previews, nil
}

// ImportFromPlaynite 从 Playnite 导出的 JSON 文件导入数据
func (s *ImportService) ImportFromPlaynite(jsonPath string, skipNoPath bool) (ImportResult, error) {
	result := ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}

	// 读取 JSON 文件
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		applog.LogErrorf(s.ctx, "ImportFromPlaynite: failed to read JSON file: %v", err)
		return result, fmt.Errorf("无法读取 JSON 文件: %w", err)
	}

	// 移除 UTF-8 BOM（如果存在）
	utf8BOM := []byte{0xEF, 0xBB, 0xBF}
	jsonData = bytes.TrimPrefix(jsonData, utf8BOM)

	var playniteGames []playnite.PlayniteGame
	if err := json.Unmarshal(jsonData, &playniteGames); err != nil {
		applog.LogErrorf(s.ctx, "ImportFromPlaynite: failed to unmarshal JSON: %v", err)
		return result, fmt.Errorf("解析 JSON 文件失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		applog.LogErrorf(s.ctx, "ImportFromPlaynite: failed to get existing games: %v", err)
		return result, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	// 按名称和路径分别建立索引
	existingNames := make(map[string]string) // name -> id
	existingPaths := make(map[string]string) // path -> name
	for _, g := range existingGames {
		if g.Name != "" {
			existingNames[strings.ToLower(g.Name)] = g.ID
		}
		if g.Path != "" {
			existingPaths[g.Path] = g.Name
		}
	}

	// 导入每个游戏
	for _, pg := range playniteGames {
		// 检查启动路径是否已存在
		if pg.Path != "" {
			if existingName, exists := existingPaths[pg.Path]; exists {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, pg.Name+" (路径已存在: "+existingName+")")
				continue
			}
		}

		// 检查同名游戏是否存在（同名但路径不同允许导入）
		if existingID, exists := existingNames[strings.ToLower(pg.Name)]; exists {
			// 检查是否是同一路径（完全重复）
			for _, g := range existingGames {
				if g.ID == existingID && g.Path == pg.Path {
					result.Skipped++
					result.SkippedNames = append(result.SkippedNames, pg.Name+" (已存在)")
					continue
				}
			}
			// 同名但路径不同，允许导入
			applog.LogInfof(s.ctx, "ImportFromPlaynite: importing duplicate name %s with different path: %s", pg.Name, pg.Path)
		}

		// 如果设置跳过无路径的游戏，且当前游戏无路径，则跳过
		if skipNoPath && pg.Path == "" {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, pg.Name+" (无路径)")
			continue
		}

		// 转换并导入游戏
		game := s.convertPlayniteToGame(pg)

		if err := s.gameService.AddGame(game); err != nil {
			applog.LogErrorf(s.ctx, "ImportFromPlaynite: failed to add game %s: %v", pg.Name, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, pg.Name)
			continue
		}

		// 更新索引
		existingNames[strings.ToLower(pg.Name)] = game.ID
		if pg.Path != "" {
			existingPaths[pg.Path] = pg.Name
		}
		result.Success++
	}

	return result, nil
}

// convertPlayniteToGame 将 Playnite 的游戏数据转换为本地的 Game 模型
func (s *ImportService) convertPlayniteToGame(pg playnite.PlayniteGame) models.Game {
	game := models.Game{
		ID:         pg.ID,
		Name:       pg.Name,
		Company:    pg.Company,
		Summary:    pg.Summary,
		Path:       pg.Path,
		SourceType: s.stringToSourceType(pg.SourceType),
		SourceID:   pg.SourceID,
		CreatedAt:  pg.CreatedAt,
		CachedAt:   time.Now(),
	}

	// 处理 SavePath
	if pg.SavePath != nil {
		game.SavePath = *pg.SavePath
	}

	// 处理封面图片 - 从 Playnite 缓存目录复制到本地
	if pg.CoverURL != "" {
		savedPath, err := utils.SaveCoverImage(pg.CoverURL, game.ID)
		if err == nil {
			game.CoverURL = savedPath
		} else {
			applog.LogErrorf(s.ctx, "convertPlayniteToGame: failed to save cover image for game %s: %v", game.Name, err)
			// 如果复制失败，保留原路径
			game.CoverURL = pg.CoverURL
		}
	}

	// 如果 CreatedAt 是零值，使用当前时间
	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	return game
}

// stringToSourceType 将字符串转换为 SourceType
func (s *ImportService) stringToSourceType(sourceType string) enums.SourceType {
	switch strings.ToLower(sourceType) {
	case "bangumi":
		return enums.Bangumi
	case "vndb":
		return enums.VNDB
	case "ymgal":
		return enums.Ymgal
	default:
		return enums.Local
	}
}

// =================== Vnite 导入功能 ====================

// SelectVniteDirectory 选择 Vnite 导出的数据库目录
func (s *ImportService) SelectVniteDirectory() (string, error) {
	selection, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择 Vnite 导出的数据库目录",
	})
	return selection, err
}

// PreviewVniteImport 预览 Vnite 导入内容（不实际导入）
func (s *ImportService) PreviewVniteImport(vniteDir string) ([]PreviewGame, error) {
	data, err := vnite.LoadExportData(vniteDir)
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewVniteImport: failed to load vnite data: %v", err)
		return nil, fmt.Errorf("读取 Vnite 导出目录失败: %w", err)
	}

	existingGames, err := s.gameService.GetGames()
	if err != nil {
		applog.LogErrorf(s.ctx, "PreviewVniteImport: failed to get existing games: %v", err)
		return nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	allIDs := make(map[string]bool)
	for id := range data.GameDocs {
		allIDs[id] = true
	}
	for id := range data.GameLocalDocs {
		allIDs[id] = true
	}

	previews := make([]PreviewGame, 0, len(allIDs))
	for id := range allIDs {
		gameDoc, hasGame := data.GameDocs[id]
		localDoc, hasLocal := data.GameLocalDocs[id]

		if !hasGame {
			continue
		}

		name := s.pickVniteName(gameDoc)
		if name == "" {
			continue
		}

		preview := PreviewGame{
			Name:       name,
			Developer:  s.pickVniteDeveloper(gameDoc),
			SourceType: string(s.mapVniteSourceType(gameDoc)),
			Exists:     existingNames[strings.ToLower(name)],
			AddTime:    s.parseVniteTimeOrNow(gameDoc.Record.AddDate),
			HasPath:    hasLocal && s.pickVniteGamePath(localDoc) != "",
		}
		previews = append(previews, preview)
	}

	return previews, nil
}

// ImportFromVnite 从 Vnite 导出的数据库目录导入数据
func (s *ImportService) ImportFromVnite(vniteDir string, skipNoPath bool) (ImportResult, error) {
	result := ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}

	data, err := vnite.LoadExportData(vniteDir)
	if err != nil {
		applog.LogErrorf(s.ctx, "ImportFromVnite: failed to load vnite data: %v", err)
		return result, fmt.Errorf("读取 Vnite 导出目录失败: %w", err)
	}

	existingGames, err := s.gameService.GetGames()
	if err != nil {
		applog.LogErrorf(s.ctx, "ImportFromVnite: failed to get existing games: %v", err)
		return result, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}

	existingNames := make(map[string]string)
	existingPaths := make(map[string]string)
	for _, g := range existingGames {
		if g.Name != "" {
			existingNames[strings.ToLower(g.Name)] = g.ID
		}
		if g.Path != "" {
			existingPaths[g.Path] = g.Name
		}
	}

	allIDs := make(map[string]bool)
	for id := range data.GameDocs {
		allIDs[id] = true
	}
	for id := range data.GameLocalDocs {
		allIDs[id] = true
	}

	for id := range allIDs {
		gameDoc, hasGame := data.GameDocs[id]
		localDoc := data.GameLocalDocs[id]
		if !hasGame {
			continue
		}

		gameName := s.pickVniteName(gameDoc)
		if gameName == "" {
			continue
		}

		exePath := s.pickVniteGamePath(localDoc)
		hasPath := exePath != ""

		if hasPath {
			if existingName, exists := existingPaths[exePath]; exists {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, gameName+" (路径已存在: "+existingName+")")
				continue
			}
		}

		if existingID, exists := existingNames[strings.ToLower(gameName)]; exists {
			isSame := false
			for _, g := range existingGames {
				if g.ID == existingID && g.Path == exePath {
					isSame = true
					break
				}
			}
			if isSame {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
				continue
			}
			applog.LogInfof(s.ctx, "ImportFromVnite: importing duplicate name %s with different path: %s", gameName, exePath)
		}

		if skipNoPath && !hasPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		game, sessions := s.convertVniteToGame(gameDoc, localDoc)
		s.applyVniteCover(&game, vniteDir, gameDoc)
		if err := s.gameService.AddGame(game); err != nil {
			applog.LogErrorf(s.ctx, "ImportFromVnite: failed to add game %s: %v", gameName, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, gameName)
			continue
		}

		if len(sessions) > 0 && s.sessionService != nil {
			if err := s.sessionService.BatchAddPlaySessions(sessions); err != nil {
				applog.LogWarningf(s.ctx, "ImportFromVnite: failed to import play sessions for game %s: %v", gameName, err)
			} else {
				result.SessionsImported += len(sessions)
			}
		}

		existingNames[strings.ToLower(gameName)] = game.ID
		if hasPath {
			existingPaths[exePath] = gameName
		}
		result.Success++
	}

	return result, nil
}

func (s *ImportService) convertVniteToGame(gameDoc vnite.GameDoc, localDoc vnite.GameLocalDoc) (models.Game, []models.PlaySession) {
	gameID := uuid.New().String()
	game := models.Game{
		ID:         gameID,
		Name:       s.pickVniteName(gameDoc),
		Company:    s.pickVniteDeveloper(gameDoc),
		Summary:    gameDoc.Metadata.Description,
		Path:       s.pickVniteGamePath(localDoc),
		SavePath:   s.pickVniteSavePath(localDoc),
		SourceType: s.mapVniteSourceType(gameDoc),
		SourceID:   s.pickVniteSourceID(gameDoc),
		CreatedAt:  s.parseVniteTimeOrNow(gameDoc.Record.AddDate),
		CachedAt:   time.Now(),
		UseMagpie:  localDoc.Launcher.UseMagpie,
	}

	sessions := s.parseVniteTimers(gameID, gameDoc.Record.Timers)
	return game, sessions
}

func (s *ImportService) applyVniteCover(game *models.Game, vniteDir string, gameDoc vnite.GameDoc) {
	coverBytes, ext, err := vnite.LoadGameCoverBytes(vniteDir, gameDoc)
	if err != nil {
		applog.LogWarningf(s.ctx, "ImportFromVnite: failed to load cover bytes for game %s: %v", game.Name, err)
		return
	}
	if len(coverBytes) == 0 {
		return
	}

	if ext == "" {
		ext = ".jpg"
	}

	tempFile, err := os.CreateTemp("", "vnite_cover_*"+ext)
	if err != nil {
		applog.LogWarningf(s.ctx, "ImportFromVnite: failed to create temp cover file for game %s: %v", game.Name, err)
		return
	}
	tempFilePath := tempFile.Name()
	if _, err := tempFile.Write(coverBytes); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		applog.LogWarningf(s.ctx, "ImportFromVnite: failed to write temp cover for game %s: %v", game.Name, err)
		return
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFilePath)
		applog.LogWarningf(s.ctx, "ImportFromVnite: failed to close temp cover for game %s: %v", game.Name, err)
		return
	}
	defer os.Remove(tempFilePath)

	savedPath, err := utils.SaveCoverImage(tempFilePath, game.ID)
	if err != nil {
		applog.LogWarningf(s.ctx, "ImportFromVnite: failed to save cover image for game %s: %v", game.Name, err)
		return
	}

	game.CoverURL = savedPath
}

func (s *ImportService) pickVniteName(gameDoc vnite.GameDoc) string {
	if gameDoc.Metadata.Name != "" {
		return gameDoc.Metadata.Name
	}
	return gameDoc.Metadata.OriginalName
}

func (s *ImportService) pickVniteDeveloper(gameDoc vnite.GameDoc) string {
	if len(gameDoc.Metadata.Developers) > 0 {
		return gameDoc.Metadata.Developers[0]
	}
	if len(gameDoc.Metadata.Publishers) > 0 {
		return gameDoc.Metadata.Publishers[0]
	}
	return ""
}

func (s *ImportService) pickVniteGamePath(localDoc vnite.GameLocalDoc) string {
	if localDoc.Path.GamePath != "" {
		return localDoc.Path.GamePath
	}
	return localDoc.Launcher.FileConfig.Path
}

func (s *ImportService) pickVniteSavePath(localDoc vnite.GameLocalDoc) string {
	if len(localDoc.Path.SavePaths) > 0 {
		return localDoc.Path.SavePaths[0]
	}
	return ""
}

func (s *ImportService) pickVniteSourceID(gameDoc vnite.GameDoc) string {
	if gameDoc.Metadata.VNDBID != "" {
		return gameDoc.Metadata.VNDBID
	}
	if gameDoc.Metadata.YmgalID != "" {
		return gameDoc.Metadata.YmgalID
	}
	if gameDoc.Metadata.BangumiID != "" {
		return gameDoc.Metadata.BangumiID
	}
	if gameDoc.Metadata.SteamID != "" {
		return gameDoc.Metadata.SteamID
	}
	return ""
}

func (s *ImportService) mapVniteSourceType(gameDoc vnite.GameDoc) enums.SourceType {
	if gameDoc.Metadata.VNDBID != "" {
		return enums.VNDB
	}
	if gameDoc.Metadata.YmgalID != "" {
		return enums.Ymgal
	}
	if gameDoc.Metadata.BangumiID != "" {
		return enums.Bangumi
	}
	return enums.Local
}

func (s *ImportService) parseVniteTimeOrNow(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}

	return time.Now()
}

func (s *ImportService) parseVniteTimers(gameID string, timers []vnite.GameTimer) []models.PlaySession {
	sessions := make([]models.PlaySession, 0, len(timers))
	for _, timer := range timers {
		if timer.Start == "" || timer.End == "" {
			continue
		}

		startTime := s.parseVniteTimeOrNow(timer.Start)
		endTime := s.parseVniteTimeOrNow(timer.End)
		duration := int(endTime.Sub(startTime).Seconds())
		if duration <= 0 {
			continue
		}

		sessions = append(sessions, models.PlaySession{
			ID:        uuid.New().String(),
			GameID:    gameID,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
		})
	}

	return sessions
}

// ==================== 批量导入功能 ====================

// SelectLibraryDirectory 选择游戏库目录
func (s *ImportService) SelectLibraryDirectory() (string, error) {
	selection, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择游戏库目录",
	})
	return selection, err
}

// ScanLibraryDirectory 扫描游戏库目录，返回候选游戏列表
func (s *ImportService) ScanLibraryDirectory(libraryPath string) ([]vo.BatchImportCandidate, error) {
	var candidates []vo.BatchImportCandidate

	// 需要排除的可执行文件关键词
	excludeKeywords := []string{
		"unins", "setup", "config", "patch", "update", "crashpad",
		"vc_redist", "dxwebsetup", "directx", "vcredist", "dotnet",
		"redistributable", "installer", "launcher_helper", "crashreporter",
		"updater", "uninstall", "删除", "卸载",
	}

	// 最大递归 7 层
	const maxDepth = 7
	candidatesMap := make(map[string]vo.BatchImportCandidate) // 使用 map 去重

	err := s.scanDirectoryRecursive(libraryPath, libraryPath, 0, maxDepth, excludeKeywords, candidatesMap)
	if err != nil {
		applog.LogErrorf(s.ctx, "ScanLibraryDirectory: failed to scan directory: %v", err)
		return nil, fmt.Errorf("扫描目录失败: %w", err)
	}

	// 将 map 转换为 slice
	for _, candidate := range candidatesMap {
		candidates = append(candidates, candidate)
	}

	applog.LogInfof(s.ctx, "ScanLibraryDirectory: found %d game candidates", len(candidates))
	return candidates, nil
}

// scanDirectoryRecursive 递归扫描目录，找到所有包含可执行文件的目录
func (s *ImportService) scanDirectoryRecursive(
	rootPath string,
	currentPath string,
	currentDepth int,
	maxDepth int,
	excludeKeywords []string,
	candidatesMap map[string]vo.BatchImportCandidate,
) error {
	// 达到最大深度，停止递归
	if currentDepth > maxDepth {
		return nil
	}

	// 读取当前目录
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		// 忽略无法读取的目录（可能是权限问题）
		applog.LogWarningf(s.ctx, "scanDirectoryRecursive: failed to read dir %s: %v", currentPath, err)
		return nil
	}

	// 扫描当前目录下的可执行文件
	executables := utils.FindExecutables(currentPath, excludeKeywords)

	// 如果当前目录包含可执行文件，将其作为候选游戏
	if len(executables) > 0 {
		// 使用相对于根路径的路径作为文件夹名
		relativePath, _ := filepath.Rel(rootPath, currentPath)
		folderName := filepath.Base(currentPath)

		// 如果是根目录，使用相对路径作为名称（更直观）
		if relativePath != "." && relativePath != "" {
			folderName = relativePath
		}

		// 选择推荐的可执行文件
		selectedExe := utils.SelectBestExecutable(executables, folderName)

		candidate := vo.BatchImportCandidate{
			FolderPath:  currentPath,
			FolderName:  folderName,
			Executables: executables,
			SelectedExe: selectedExe,
			SearchName:  filepath.Base(currentPath), // 使用最底层目录名作为搜索名
			IsSelected:  true,
			MatchStatus: "pending",
		}

		// 使用路径作为 key 去重（避免重复添加）
		candidatesMap[currentPath] = candidate

		// 找到游戏目录后，不再向下递归
		// 这样可以避免将父目录和子目录都作为候选游戏
		return nil
	}

	// 如果当前目录没有可执行文件，继续递归扫描子目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 跳过常见的非游戏目录
		lowerName := strings.ToLower(entry.Name())
		if lowerName == "system" || lowerName == "windows" ||
			lowerName == "program files" || lowerName == "program files (x86)" ||
			strings.HasPrefix(lowerName, ".") || // 隐藏目录
			lowerName == "node_modules" || lowerName == "__pycache__" {
			continue
		}

		subPath := filepath.Join(currentPath, entry.Name())
		// 递归扫描子目录
		if err := s.scanDirectoryRecursive(rootPath, subPath, currentDepth+1, maxDepth, excludeKeywords, candidatesMap); err != nil {
			// 继续扫描其他目录，不因单个目录失败而停止
			continue
		}
	}

	return nil
}

// ==================== 元数据获取与批量导入 ====================

// FetchMetadataForCandidate 为单个候选项获取元数据（带限流）
func (s *ImportService) FetchMetadataForCandidate(searchName string) (vo.BatchImportCandidate, error) {
	result := vo.BatchImportCandidate{
		SearchName:  searchName,
		MatchStatus: "not_found",
	}

	// 优先级顺序：Bangumi > VNDB > Ymgal
	sources := []struct {
		getter utils.Getter
		source enums.SourceType
		token  string
	}{
		{utils.NewBangumiInfoGetter(), enums.Bangumi, s.config.BangumiAccessToken},
		{utils.NewVNDBInfoGetter(), enums.VNDB, s.config.VNDBAccessToken},
		{utils.NewYmgalInfoGetter(), enums.Ymgal, ""},
	}

	for _, src := range sources {
		game, err := src.getter.FetchMetadataByName(searchName, src.token)
		if err == nil && game.Name != "" {
			result.MatchedGame = &game
			result.MatchSource = src.source
			result.MatchStatus = "matched"
			return result, nil
		}
		if err != nil {
			applog.LogWarningf(s.ctx, "FetchMetadataForCandidate: failed to fetch metadata from %v for %s: %v", src.source, searchName, err)
		}
		// 每个源之间添加短暂延迟以避免触发限流
		time.Sleep(300 * time.Millisecond)
	}

	applog.LogWarningf(s.ctx, "FetchMetadataForCandidate: no metadata found for %s", searchName)
	return result, nil
}

// BatchImportGames 批量导入游戏
func (s *ImportService) BatchImportGames(candidates []vo.BatchImportCandidate) (ImportResult, error) {
	result := ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}

	// 获取现有游戏列表用于去重
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		applog.LogErrorf(s.ctx, "BatchImportGames: failed to get existing games: %v", err)
		return result, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	// 按名称和路径分别建立索引，用于不同维度的去重检查
	existingNames := make(map[string]string) // name -> id (用于检查同名但不同路径的情况)
	existingPaths := make(map[string]string) // path -> name (用于检查同一路径)
	for _, g := range existingGames {
		if g.Name != "" {
			existingNames[strings.ToLower(g.Name)] = g.ID
		}
		if g.Path != "" {
			existingPaths[g.Path] = g.Name
		}
	}

	for _, candidate := range candidates {
		if !candidate.IsSelected {
			continue
		}

		// 检查启动路径是否已存在（路径是唯一标识，同一路径不能对应多个游戏）
		if candidate.SelectedExe != "" {
			if existingName, exists := existingPaths[candidate.SelectedExe]; exists {
				applog.LogWarningf(s.ctx, "BatchImportGames: path already exists for game %s, skipping: %s", existingName, candidate.SelectedExe)
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, candidate.SearchName+" (路径已存在: "+existingName+")")
				continue
			}
		}

		// 确定最终的游戏名（优先使用匹配后的元数据名称）
		gameName := candidate.SearchName
		if candidate.MatchedGame != nil && candidate.MatchedGame.Name != "" {
			gameName = candidate.MatchedGame.Name
		}

		// 检查同名游戏是否存在
		// 注意：同名但路径不同的游戏允许导入（可能是不同版本/安装位置）
		if existingID, exists := existingNames[strings.ToLower(gameName)]; exists {
			// 检查是否是同一路径（完全重复的情况）
			for _, g := range existingGames {
				if g.ID == existingID && g.Path == candidate.SelectedExe {
					applog.LogWarningf(s.ctx, "BatchImportGames: game already exists with same path, skipping: %s", gameName)
					result.Skipped++
					result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
					continue
				}
			}
			// 同名但路径不同，允许导入，但记录日志
			applog.LogInfof(s.ctx, "BatchImportGames: importing duplicate name %s with different path: %s", gameName, candidate.SelectedExe)
		}

		// 构建游戏对象
		var game models.Game
		if candidate.MatchedGame != nil {
			game = *candidate.MatchedGame
		} else {
			// 没有匹配到元数据，创建基本游戏信息
			game = models.Game{
				Name:       candidate.SearchName,
				SourceType: enums.Local,
			}
		}

		game.ID = uuid.New().String()
		game.Path = candidate.SelectedExe
		game.CreatedAt = time.Now()
		game.CachedAt = time.Now()

		// 保存游戏（图片会在后台异步下载）
		if err := s.gameService.AddGame(game); err != nil {
			applog.LogErrorf(s.ctx, "BatchImportGames: failed to add game %s: %v", gameName, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, gameName)
			continue
		}

		// 更新索引，防止同批次内的重复
		existingNames[strings.ToLower(gameName)] = game.ID
		if candidate.SelectedExe != "" {
			existingPaths[candidate.SelectedExe] = gameName
		}
		result.Success++
	}

	return result, nil
}

// ProcessDroppedPaths 处理拖拽导入的路径，支持文件夹和可执行文件
// 返回候选游戏列表供前端展示和确认
func (s *ImportService) ProcessDroppedPaths(paths []string) ([]vo.BatchImportCandidate, error) {
	var candidates []vo.BatchImportCandidate

	// 需要排除的可执行文件关键词
	excludeKeywords := []string{
		"unins", "setup", "config", "patch", "update", "crashpad",
		"vc_redist", "dxwebsetup", "directx", "vcredist", "dotnet",
		"redistributable", "installer", "launcher_helper", "crashreporter",
		"updater", "uninstall", "删除", "卸载",
	}

	// 最大递归深度设为 3 层（拖拽场景通常不会太深）
	const maxDepth = 3
	candidatesMap := make(map[string]vo.BatchImportCandidate) // 使用 map 去重（按路径）

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			applog.LogWarningf(s.ctx, "ProcessDroppedPaths: failed to stat path %s: %v", path, err)
			continue
		}

		if info.IsDir() {
			// 处理文件夹：使用递归扫描查找所有包含可执行文件的子目录
			err := s.scanDirectoryRecursive(path, path, 0, maxDepth, excludeKeywords, candidatesMap)
			if err != nil {
				applog.LogWarningf(s.ctx, "ProcessDroppedPaths: failed to scan directory %s: %v", path, err)
				continue
			}

			// 如果没有找到任何候选，记录日志
			if len(candidatesMap) == 0 {
				applog.LogInfof(s.ctx, "ProcessDroppedPaths: no executable found in folder %s", path)
			}
		} else {
			// 处理可执行文件
			lowerName := strings.ToLower(path)
			if !strings.HasSuffix(lowerName, ".exe") && !strings.HasSuffix(lowerName, ".bat") {
				applog.LogInfof(s.ctx, "ProcessDroppedPaths: skipping non-executable file %s", path)
				continue
			}

			// 检查是否应该排除
			excluded := false
			fileName := filepath.Base(path)
			lowerFileName := strings.ToLower(fileName)
			for _, keyword := range excludeKeywords {
				if strings.Contains(lowerFileName, keyword) {
					excluded = true
					break
				}
			}
			if excluded {
				applog.LogInfof(s.ctx, "ProcessDroppedPaths: skipping excluded file %s", path)
				continue
			}

			folderPath := filepath.Dir(path)
			folderName := filepath.Base(folderPath)
			// 如果文件名更有意义（不是通用名称），使用文件名作为搜索名
			searchName := folderName
			exeName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
			genericNames := []string{"game", "main", "start", "launch", "run", "play"}
			isGeneric := false
			for _, generic := range genericNames {
				if strings.ToLower(exeName) == generic {
					isGeneric = true
					break
				}
			}
			if !isGeneric && len(exeName) > 3 {
				searchName = exeName
			}

			candidate := vo.BatchImportCandidate{
				FolderPath:  folderPath,
				FolderName:  folderName,
				Executables: []string{path},
				SelectedExe: path,
				SearchName:  searchName,
				IsSelected:  true,
				MatchStatus: "pending",
			}
			// 使用路径作为 key，与 scanDirectoryRecursive 保持一致
			candidatesMap[folderPath] = candidate
		}
	}

	// 将 map 转换为 slice
	for _, candidate := range candidatesMap {
		candidates = append(candidates, candidate)
	}

	applog.LogInfof(s.ctx, "ProcessDroppedPaths: processed %d paths, found %d candidates", len(paths), len(candidates))
	return candidates, nil
}
