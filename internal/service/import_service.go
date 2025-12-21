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
	"sort"
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
	if err := s.extractZip(zipReader, tempDir); err != nil {
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
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	// 导入每个游戏
	for _, galgame := range galgames {
		gameName := galgame.GetDisplayName()
		hasPath := galgame.GetExePath() != ""

		// 检查是否已存在
		if existingNames[strings.ToLower(gameName)] {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName)
			continue
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

		existingNames[strings.ToLower(gameName)] = true
		result.Success++
	}

	return result, nil
}

// extractZip 解压 ZIP 文件到指定目录
func (s *ImportService) extractZip(zipReader *zip.ReadCloser, destDir string) error {
	for _, file := range zipReader.File {
		filePath := filepath.Join(destDir, file.Name)

		// 防止 ZIP Slip 攻击
		if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			runtime.LogErrorf(s.ctx, "illegal file path in ZIP: %s", file.Name)
			return fmt.Errorf("非法的文件路径: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				runtime.LogErrorf(s.ctx, "failed to create dir in extractZip: %v", err)
				return err
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			runtime.LogErrorf(s.ctx, "failed to create parent dir in extractZip: %v", err)
			return err
		}

		// 解压文件
		destFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			runtime.LogErrorf(s.ctx, "failed to open dest file in extractZip: %v", err)
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			runtime.LogErrorf(s.ctx, "failed to open src file in extractZip: %v", err)
			destFile.Close()
			return err
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()

		if err != nil {
			runtime.LogErrorf(s.ctx, "failed to copy file in extractZip: %v", err)
			return err
		}
	}
	return nil
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
		coverPath := s.resolveCoverPath(galgame.ImagePath.Value, tempDir)
		if coverPath != "" {
			// 将封面图片复制到应用的封面目录
			savedPath, err := s.saveCoverImage(coverPath, game.ID)
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

// resolveCoverPath 解析封面图片路径
func (s *ImportService) resolveCoverPath(imagePath string, tempDir string) string {
	// PotatoVN 的图片路径格式通常是 ".\\Images\\xxx_cover" 或相对路径
	// 需要转换为绝对路径

	// 移除开头的 ".\" 或 "./"
	cleanPath := strings.TrimPrefix(imagePath, ".\\")
	cleanPath = strings.TrimPrefix(cleanPath, "./")

	// 替换反斜杠为正斜杠
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

	// 构建完整路径
	fullPath := filepath.Join(tempDir, cleanPath)

	// 检查文件是否存在，可能需要添加扩展名
	extensions := []string{"", ".png", ".jpg", ".jpeg", ".webp", ".gif"}
	for _, ext := range extensions {
		testPath := fullPath + ext
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}
	}

	// 记录未找到封面图片的情况
	runtime.LogWarningf(s.ctx, "resolveCoverPath: cover image not found, imagePath=%s, tempDir=%s", imagePath, tempDir)
	return ""
}

// saveCoverImage 保存封面图片到应用的封面目录
func (s *ImportService) saveCoverImage(srcPath string, gameID string) (string, error) {
	// 获取应用程序目录
	execPath, err := os.Executable()
	if err != nil {
		runtime.LogErrorf(s.ctx, "saveCoverImage: failed to get executable path: %v", err)
		return "", err
	}
	appDir := filepath.Dir(execPath)

	// 获取封面保存目录
	coverDir := filepath.Join(appDir, "covers")
	if err := os.MkdirAll(coverDir, os.ModePerm); err != nil {
		runtime.LogErrorf(s.ctx, "saveCoverImage: failed to create cover dir: %v", err)
		return "", err
	}

	// 获取源文件的扩展名
	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	// 生成目标文件名
	destFileName := fmt.Sprintf("%s%s", gameID, ext)
	destPath := filepath.Join(coverDir, destFileName)

	// 复制文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		runtime.LogErrorf(s.ctx, "saveCoverImage: failed to open src file: %v", err)
		return "", err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		runtime.LogErrorf(s.ctx, "saveCoverImage: failed to create dest file: %v", err)
		return "", err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		runtime.LogErrorf(s.ctx, "saveCoverImage: failed to copy file: %v", err)
		return "", err
	}

	// 返回相对路径或可访问的 URL
	return fmt.Sprintf("/local/covers/%s", destFileName), nil
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
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	// 导入每个游戏
	for _, pg := range playniteGames {
		// 检查是否已存在
		if existingNames[strings.ToLower(pg.Name)] {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, pg.Name)
			continue
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

		existingNames[strings.ToLower(pg.Name)] = true
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
		savedPath, err := s.saveCoverImage(pg.CoverURL, game.ID)
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
		executables := s.findExecutables(folderPath, excludeKeywords)

		if len(executables) == 0 {
			continue // 没有可执行文件，跳过此文件夹
		}

		// 选择推荐的可执行文件
		selectedExe := s.selectBestExecutable(executables, folderName)

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

// findExecutables 在指定目录下查找可执行文件
func (s *ImportService) findExecutables(folderPath string, excludeKeywords []string) []string {
	var executables []string

	// 仅扫描一级目录
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		runtime.LogWarningf(s.ctx, "findExecutables: failed to read dir %s: %v", folderPath, err)
		return executables
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		lowerName := strings.ToLower(name)

		// 检查是否是可执行文件
		if !strings.HasSuffix(lowerName, ".exe") &&
			!strings.HasSuffix(lowerName, ".bat") &&
			!strings.HasSuffix(lowerName, ".lnk") {
			continue
		}

		// 检查是否应该排除
		excluded := false
		for _, keyword := range excludeKeywords {
			if strings.Contains(lowerName, keyword) {
				excluded = true
				break
			}
		}

		if !excluded {
			executables = append(executables, filepath.Join(folderPath, name))
		}
	}

	return executables
}

// selectBestExecutable 选择最佳可执行文件
func (s *ImportService) selectBestExecutable(executables []string, folderName string) string {
	if len(executables) == 0 {
		return ""
	}
	if len(executables) == 1 {
		return executables[0]
	}

	lowerFolderName := strings.ToLower(folderName)

	// 优先选择与文件夹名相似的
	for _, exe := range executables {
		exeName := strings.ToLower(filepath.Base(exe))
		exeName = strings.TrimSuffix(exeName, filepath.Ext(exeName))
		if strings.Contains(exeName, lowerFolderName) || strings.Contains(lowerFolderName, exeName) {
			return exe
		}
	}

	// 否则按文件大小排序，选择最大的
	type exeInfo struct {
		path string
		size int64
	}
	var exeInfos []exeInfo

	for _, exe := range executables {
		info, err := os.Stat(exe)
		if err == nil {
			exeInfos = append(exeInfos, exeInfo{path: exe, size: info.Size()})
		} else {
			runtime.LogWarningf(s.ctx, "selectBestExecutable: failed to stat file %s: %v", exe, err)
		}
	}

	if len(exeInfos) > 0 {
		sort.Slice(exeInfos, func(i, j int) bool {
			return exeInfos[i].size > exeInfos[j].size
		})
		return exeInfos[0].path
	}

	return executables[0]
}

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
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	for _, candidate := range candidates {
		if !candidate.IsSelected {
			continue
		}

		// 检查游戏名是否已存在
		gameName := candidate.SearchName
		if candidate.MatchedGame != nil && candidate.MatchedGame.Name != "" {
			gameName = candidate.MatchedGame.Name
		}

		if existingNames[strings.ToLower(gameName)] {
			runtime.LogWarningf(s.ctx, "BatchImportGames: game already exists, skipping: %s", gameName)
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (已存在)")
			continue
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

		// 保存游戏
		if err := s.gameService.AddGame(game); err != nil {
			runtime.LogErrorf(s.ctx, "BatchImportGames: failed to add game %s: %v", gameName, err)
			result.Failed++
			result.FailedNames = append(result.FailedNames, gameName)
			continue
		}

		existingNames[strings.ToLower(gameName)] = true
		result.Success++
	}

	return result, nil
}
