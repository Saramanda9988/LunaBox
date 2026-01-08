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
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/models/playnite"
	"lunabox/internal/models/potatovn"
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
	Success      int      `json:"success"`       // 成功导入数量
	Skipped      int      `json:"skipped"`       // 跳过数量（已存在）
	Failed       int      `json:"failed"`        // 失败数量
	FailedNames  []string `json:"failed_names"`  // 失败的游戏名称
	SkippedNames []string `json:"skipped_names"` // 跳过的游戏名称
}

type ImportService struct {
	ctx         context.Context
	db          *sql.DB
	config      *appconf.AppConfig
	gameService *GameService
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
		runtime.LogErrorf(s.ctx, "failed to open ZIP file: %v", err)
		return result, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", "potatovn_import_*")
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to create temp dir: %v", err)
		return result, fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压文件
	if err := utils.ExtractZip(zipReader, tempDir); err != nil {
		runtime.LogErrorf(s.ctx, "failed to extract ZIP: %v", err)
		return result, fmt.Errorf("解压失败: %w", err)
	}

	// 读取 data.galgames.json
	galgamesPath := filepath.Join(tempDir, "data.galgames.json")
	galgamesData, err := os.ReadFile(galgamesPath)
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to read data.galgames.json: %v", err)
		return result, fmt.Errorf("无法读取 data.galgames.json: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(galgamesData, &galgames); err != nil {
		runtime.LogErrorf(s.ctx, "failed to unmarshal data.galgames.json: %v", err)
		return result, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		runtime.LogErrorf(s.ctx, "failed to get existing games: %v", err)
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
			runtime.LogInfof(s.ctx, "ImportFromPotatoVN: importing duplicate name %s with different path: %s", gameName, exePath)
		}

		// 如果设置跳过无路径的游戏，且当前游戏无路径，则跳过
		if skipNoPath && !hasPath {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (无路径)")
			continue
		}

		// 转换并导入游戏
		game := s.convertToGame(galgame, tempDir)

		if err := s.gameService.AddGame(game); err != nil {
			runtime.LogErrorf(s.ctx, "failed to add game %s: %v", gameName, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, gameName)
			continue
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
func (s *ImportService) convertToGame(galgame potatovn.Galgame, tempDir string) models.Game {
	game := models.Game{
		ID:         uuid.New().String(),
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
				runtime.LogErrorf(s.ctx, "failed to save cover image for game %s: %v", game.Name, err)
			}
		} else {
			runtime.LogErrorf(s.ctx, "cover image not found for game %s, path: %s", game.Name, galgame.ImagePath.Value)
		}
	}

	// 如果 CreatedAt 是零值，使用当前时间
	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	return game
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

// PreviewImport 预览导入内容（不实际导入）
func (s *ImportService) PreviewImport(zipPath string) ([]PreviewGame, error) {
	// 打开 ZIP 文件
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		runtime.LogErrorf(s.ctx, "PreviewImport: failed to open ZIP file: %v", err)
		return nil, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", "potatovn_preview_*")
	if err != nil {
		runtime.LogErrorf(s.ctx, "PreviewImport: failed to create temp dir: %v", err)
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
				runtime.LogErrorf(s.ctx, "PreviewImport: failed to create data.galgames.json: %v", err)
				return nil, err
			}

			srcFile, err := file.Open()
			if err != nil {
				runtime.LogErrorf(s.ctx, "PreviewImport: failed to open data.galgames.json in ZIP: %v", err)
				destFile.Close()
				return nil, err
			}

			_, err = io.Copy(destFile, srcFile)
			srcFile.Close()
			destFile.Close()

			if err != nil {
				runtime.LogErrorf(s.ctx, "PreviewImport: failed to copy data.galgames.json: %v", err)
				return nil, err
			}
			found = true
			break
		}
	}
	if !found {
		runtime.LogWarningf(s.ctx, "PreviewImport: data.galgames.json not found in ZIP: %s", zipPath)
	}

	// 读取 data.galgames.json
	galgamesPath := filepath.Join(tempDir, "data.galgames.json")
	galgamesData, err := os.ReadFile(galgamesPath)
	if err != nil {
		runtime.LogErrorf(s.ctx, "PreviewImport: failed to read data.galgames.json: %v", err)
		return nil, fmt.Errorf("无法读取 data.galgames.json: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(galgamesData, &galgames); err != nil {
		runtime.LogErrorf(s.ctx, "PreviewImport: failed to unmarshal data.galgames.json: %v", err)
		return nil, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		runtime.LogErrorf(s.ctx, "PreviewImport: failed to get existing games: %v", err)
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
		runtime.LogErrorf(s.ctx, "PreviewPlayniteImport: failed to read JSON file: %v", err)
		return nil, fmt.Errorf("无法读取 JSON 文件: %w", err)
	}

	// 移除 UTF-8 BOM（如果存在）
	utf8BOM := []byte{0xEF, 0xBB, 0xBF}
	jsonData = bytes.TrimPrefix(jsonData, utf8BOM)

	var playniteGames []playnite.PlayniteGame
	if err := json.Unmarshal(jsonData, &playniteGames); err != nil {
		runtime.LogErrorf(s.ctx, "PreviewPlayniteImport: failed to unmarshal JSON: %v", err)
		return nil, fmt.Errorf("解析 JSON 文件失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		runtime.LogErrorf(s.ctx, "PreviewPlayniteImport: failed to get existing games: %v", err)
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
		runtime.LogErrorf(s.ctx, "ImportFromPlaynite: failed to read JSON file: %v", err)
		return result, fmt.Errorf("无法读取 JSON 文件: %w", err)
	}

	// 移除 UTF-8 BOM（如果存在）
	utf8BOM := []byte{0xEF, 0xBB, 0xBF}
	jsonData = bytes.TrimPrefix(jsonData, utf8BOM)

	var playniteGames []playnite.PlayniteGame
	if err := json.Unmarshal(jsonData, &playniteGames); err != nil {
		runtime.LogErrorf(s.ctx, "ImportFromPlaynite: failed to unmarshal JSON: %v", err)
		return result, fmt.Errorf("解析 JSON 文件失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		runtime.LogErrorf(s.ctx, "ImportFromPlaynite: failed to get existing games: %v", err)
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
			runtime.LogInfof(s.ctx, "ImportFromPlaynite: importing duplicate name %s with different path: %s", pg.Name, pg.Path)
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
			runtime.LogErrorf(s.ctx, "ImportFromPlaynite: failed to add game %s: %v", pg.Name, err)
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
			runtime.LogErrorf(s.ctx, "convertPlayniteToGame: failed to save cover image for game %s: %v", game.Name, err)
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

	// 遍历一级子目录
	entries, err := os.ReadDir(libraryPath)
	if err != nil {
		runtime.LogErrorf(s.ctx, "ScanLibraryDirectory: failed to read dir %s: %v", libraryPath, err)
		return nil, fmt.Errorf("无法读取目录: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		folderPath := filepath.Join(libraryPath, entry.Name())
		folderName := entry.Name()

		// 扫描该文件夹下的可执行文件
		executables := utils.FindExecutables(folderPath, excludeKeywords)

		if len(executables) == 0 {
			continue // 没有可执行文件，跳过此文件夹
		}

		// 选择推荐的可执行文件
		selectedExe := utils.SelectBestExecutable(executables, folderName)

		candidate := vo.BatchImportCandidate{
			FolderPath:  folderPath,
			FolderName:  folderName,
			Executables: executables,
			SelectedExe: selectedExe,
			SearchName:  folderName,
			IsSelected:  true,
			MatchStatus: "pending",
		}

		candidates = append(candidates, candidate)
	}

	return candidates, nil
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
			runtime.LogWarningf(s.ctx, "FetchMetadataForCandidate: failed to fetch metadata from %v for %s: %v", src.source, searchName, err)
		}
		// 每个源之间添加短暂延迟以避免触发限流
		time.Sleep(300 * time.Millisecond)
	}

	runtime.LogWarningf(s.ctx, "FetchMetadataForCandidate: no metadata found for %s", searchName)
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
		runtime.LogErrorf(s.ctx, "BatchImportGames: failed to get existing games: %v", err)
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
				runtime.LogWarningf(s.ctx, "BatchImportGames: path already exists for game %s, skipping: %s", existingName, candidate.SelectedExe)
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
					runtime.LogWarningf(s.ctx, "BatchImportGames: game already exists with same path, skipping: %s", gameName)
					result.Skipped++
					result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
					continue
				}
			}
			// 同名但路径不同，允许导入，但记录日志
			runtime.LogInfof(s.ctx, "BatchImportGames: importing duplicate name %s with different path: %s", gameName, candidate.SelectedExe)
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
			runtime.LogErrorf(s.ctx, "BatchImportGames: failed to add game %s: %v", gameName, err)
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
