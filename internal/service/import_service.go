package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/importer"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/metadata"
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

// PreviewGame 预览导入的游戏信息
type PreviewGame struct {
	Name       string    `json:"name"`
	Developer  string    `json:"developer"`
	SourceType string    `json:"source_type"`
	Exists     bool      `json:"exists"`
	AddTime    time.Time `json:"add_time"`
	HasPath    bool      `json:"has_path"`
}

type ImportService struct {
	ctx            context.Context
	db             *sql.DB
	config         *appconf.AppConfig
	gameService    *GameService
	bangumiService *BangumiService
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

func (s *ImportService) SetBangumiService(bangumiService *BangumiService) {
	s.bangumiService = bangumiService
}

func (s *ImportService) importerDependencies() importer.Dependencies {
	var addSessions func([]models.PlaySession) error
	if s.sessionService != nil {
		addSessions = s.sessionService.BatchAddPlaySessions
	}

	return importer.Dependencies{
		Ctx:                          s.ctx,
		ListGames:                    s.listImportGamesForImporter,
		AddGame:                      s.gameService.AddGameFromWebMetadata,
		AddItems:                     s.addImporterItems,
		AddSessions:                  addSessions,
		AllowDuplicateMetadataImport: s.config != nil && s.config.AllowDuplicateMetadataImport,
	}
}

func previewGamesFromImporter(previews []importer.PreviewGame) []PreviewGame {
	if previews == nil {
		return nil
	}

	result := make([]PreviewGame, 0, len(previews))
	for _, preview := range previews {
		result = append(result, PreviewGame(preview))
	}
	return result
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
	result, err := importer.NewPotatoVNImporter(s.importerDependencies()).Import(zipPath, skipNoPath)
	return ImportResult(result), err
}

// PreviewImport 预览 PotatoVN 导入内容（不实际导入）
func (s *ImportService) PreviewImport(zipPath string) ([]PreviewGame, error) {
	previews, err := importer.NewPotatoVNImporter(s.importerDependencies()).Preview(zipPath)
	return previewGamesFromImporter(previews), err
}

// =================== Playnite 导入功能 ====================

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
	previews, err := importer.NewPlayniteImporter(s.importerDependencies()).Preview(jsonPath)
	return previewGamesFromImporter(previews), err
}

// ImportFromPlaynite 从 Playnite 导出的 JSON 文件导入数据
func (s *ImportService) ImportFromPlaynite(jsonPath string, skipNoPath bool) (ImportResult, error) {
	result, err := importer.NewPlayniteImporter(s.importerDependencies()).Import(jsonPath, skipNoPath)
	return ImportResult(result), err
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
	previews, err := importer.NewVniteImporter(s.importerDependencies()).Preview(vniteDir)
	return previewGamesFromImporter(previews), err
}

// ImportFromVnite 从 Vnite 导出的数据库目录导入数据
func (s *ImportService) ImportFromVnite(vniteDir string, skipNoPath bool) (ImportResult, error) {
	result, err := importer.NewVniteImporter(s.importerDependencies()).Import(vniteDir, skipNoPath)
	return ImportResult(result), err
}

// ==================== 批量导入功能 ====================

// SelectLibraryDirectory 选择游戏库目录
func (s *ImportService) SelectLibraryDirectory() (string, error) {
	selection, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择游戏库目录",
	})
	return selection, err
}

// ScanLibraryDirectory 扫描游戏库目录，返回默认待导入候选项和路径阶段跳过项。
func (s *ImportService) ScanLibraryDirectory(libraryPath string) (vo.BatchImportScanResult, error) {
	return s.scanLibraryDirectory(libraryPath, vo.BatchImportScanOptions{})
}

// ScanLibraryDirectoryWithOptions 按指定模式扫描游戏库目录。
func (s *ImportService) ScanLibraryDirectoryWithOptions(libraryPath string, options vo.BatchImportScanOptions) (vo.BatchImportScanResult, error) {
	return s.scanLibraryDirectory(libraryPath, options)
}

func (s *ImportService) scanLibraryDirectory(libraryPath string, options vo.BatchImportScanOptions) (vo.BatchImportScanResult, error) {
	var candidates []vo.BatchImportCandidate
	var result vo.BatchImportScanResult

	excludeKeywords := defaultImportExcludeKeywords()
	const maxDepth = 7
	candidatesMap := make(map[string]vo.BatchImportCandidate)

	scanOptions := normalizeBatchImportScanOptions(options)
	var err error
	if scanOptions.ScanMode == "hierarchy" {
		err = s.scanDirectoryByHierarchy(libraryPath, scanOptions.HierarchyDepth, candidatesMap)
	} else {
		err = s.scanDirectoryRecursive(libraryPath, libraryPath, 0, maxDepth, excludeKeywords, scanOptions.ScanNameMode, scanOptions.NameDepth, candidatesMap)
	}
	if err != nil {
		applog.LogErrorf(s.ctx, "ScanLibraryDirectory: failed to scan directory: %v", err)
		return result, fmt.Errorf("扫描目录失败: %w", err)
	}

	for _, candidate := range candidatesMap {
		candidates = append(candidates, candidate)
	}

	idx, err := s.loadImportIndex()
	if err != nil {
		applog.LogErrorf(s.ctx, "ScanLibraryDirectory: failed to load import index: %v", err)
		return result, fmt.Errorf("加载导入索引失败: %w", err)
	}

	result = splitScanCandidates(candidates, idx)
	applog.LogInfof(s.ctx, "ScanLibraryDirectory: found %d game candidates, %d importable, %d skipped", len(candidates), len(result.Candidates), result.Skipped)
	return result, nil
}

func normalizeBatchImportScanOptions(options vo.BatchImportScanOptions) vo.BatchImportScanOptions {
	if options.ScanMode != "hierarchy" {
		options.ScanMode = "scan"
	}
	if options.ScanNameMode != "parent" {
		options.ScanNameMode = "depth"
	}
	if options.NameDepth < 0 {
		options.NameDepth = 0
	}
	if options.HierarchyDepth < 0 {
		options.HierarchyDepth = 0
	}
	return options
}

// scanDirectoryRecursive 递归扫描目录，找到所有包含可执行文件的目录
func (s *ImportService) scanDirectoryRecursive(
	rootPath string,
	currentPath string,
	currentDepth int,
	maxDepth int,
	excludeKeywords []string,
	scanNameMode string,
	nameDepth int,
	candidatesMap map[string]vo.BatchImportCandidate,
) error {
	if currentDepth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		applog.LogWarningf(s.ctx, "scanDirectoryRecursive: failed to read dir %s: %v", currentPath, err)
		return nil
	}

	executables := apputils.FindExecutables(currentPath, excludeKeywords)
	if len(executables) > 0 {
		relativePath, _ := filepath.Rel(rootPath, currentPath)
		folderName := filepath.Base(currentPath)
		if relativePath != "." && relativePath != "" {
			folderName = relativePath
		}

		selectedExe := apputils.SelectBestExecutable(executables, folderName)
		searchName := scanSearchName(rootPath, currentPath, scanNameMode, nameDepth)
		candidatesMap[currentPath] = vo.BatchImportCandidate{
			FolderPath:  currentPath,
			FolderName:  folderName,
			Executables: executables,
			SelectedExe: selectedExe,
			SearchName:  searchName,
			IsSelected:  true,
			MatchStatus: "pending",
		}
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if shouldSkipImportDirectory(entry.Name()) {
			continue
		}

		subPath := filepath.Join(currentPath, entry.Name())
		if err := s.scanDirectoryRecursive(rootPath, subPath, currentDepth+1, maxDepth, excludeKeywords, scanNameMode, nameDepth, candidatesMap); err != nil {
			continue
		}
	}

	return nil
}

func (s *ImportService) scanDirectoryByHierarchy(
	rootPath string,
	hierarchyDepth int,
	candidatesMap map[string]vo.BatchImportCandidate,
) error {
	currentDirs := []string{rootPath}
	for depth := 0; depth <= hierarchyDepth; depth++ {
		nextDirs := make([]string, 0)
		for _, dir := range currentDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				applog.LogWarningf(s.ctx, "scanDirectoryByHierarchy: failed to read dir %s: %v", dir, err)
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() || shouldSkipImportDirectory(entry.Name()) {
					continue
				}
				nextDirs = append(nextDirs, filepath.Join(dir, entry.Name()))
			}
		}
		currentDirs = nextDirs
		if len(currentDirs) == 0 {
			break
		}
	}

	for _, dir := range currentDirs {
		relativePath, _ := filepath.Rel(rootPath, dir)
		folderName := filepath.Base(dir)
		if relativePath != "." && relativePath != "" {
			folderName = relativePath
		}
		candidatesMap[dir] = vo.BatchImportCandidate{
			FolderPath:  dir,
			FolderName:  folderName,
			Executables: []string{},
			SelectedExe: dir,
			SearchName:  filepath.Base(dir),
			IsSelected:  true,
			MatchStatus: "pending",
		}
	}
	return nil
}

func scanSearchName(rootPath string, currentPath string, scanNameMode string, nameDepth int) string {
	if scanNameMode == "parent" {
		return filepath.Base(currentPath)
	}

	relativePath, err := filepath.Rel(rootPath, currentPath)
	if err != nil || relativePath == "." || relativePath == "" {
		return filepath.Base(currentPath)
	}

	parts := strings.FieldsFunc(relativePath, func(r rune) bool {
		return r == filepath.Separator || r == '/' || r == '\\'
	})
	if nameDepth >= 0 && nameDepth < len(parts) && strings.TrimSpace(parts[nameDepth]) != "" {
		return parts[nameDepth]
	}
	return filepath.Base(currentPath)
}

func shouldSkipImportDirectory(name string) bool {
	lowerName := strings.ToLower(name)
	return lowerName == "system" || lowerName == "windows" ||
		lowerName == "program files" || lowerName == "program files (x86)" ||
		strings.HasPrefix(lowerName, ".") ||
		lowerName == "node_modules" || lowerName == "__pycache__"
}

// ==================== 元数据获取与批量导入 ====================

// FetchMetadataForCandidate 为单个候选项获取元数据（带限流）
func (s *ImportService) FetchMetadataForCandidate(searchName string) (vo.BatchImportCandidate, error) {
	result := vo.BatchImportCandidate{
		SearchName:  searchName,
		MatchStatus: "not_found",
	}
	getterOptions := metadataGetterOptions(s.config)
	sources := s.getConfiguredMetadataSearchSources(getterOptions)

	for _, src := range sources {
		metaResult, err := src.fetchByName(searchName)
		if err == nil && metaResult.Game.Name != "" {
			game := metaResult.Game
			result.MatchedGame = &game
			result.MatchedTags = metaResult.Tags
			result.MatchSource = src.source
			result.MatchStatus = "matched"
			return result, nil
		}
		if err != nil {
			applog.LogWarningf(s.ctx, "FetchMetadataForCandidate: failed to fetch metadata from %v for %s: %v", src.source, searchName, err)
		}
	}

	applog.LogWarningf(s.ctx, "FetchMetadataForCandidate: no metadata found for %s", searchName)
	return result, nil
}

func (s *ImportService) getConfiguredMetadataSearchSources(getterOptions []metadata.GetterOption) []metadataSearchSource {
	vndbToken := ""
	language := ""
	if s.config != nil {
		vndbToken = s.config.VNDBAccessToken
		language = s.config.Language
	}

	configuredSources := configuredMetadataSources(s.config)
	sources := make([]metadataSearchSource, 0, len(configuredSources))
	for _, source := range configuredSources {
		switch source {
		case enums.Bangumi:
			if s.bangumiService == nil {
				continue
			}
			sources = append(sources, metadataSearchSource{
				source: enums.Bangumi,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return s.bangumiService.fetchMetadataByName(s.ctx, name)
				},
			})
		case enums.VNDB:
			sources = append(sources, metadataSearchSource{
				source: enums.VNDB,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewVNDBInfoGetterWithLanguage(language, getterOptions...).FetchMetadataByName(name, vndbToken)
				},
			})
		case enums.Ymgal:
			sources = append(sources, metadataSearchSource{
				source: enums.Ymgal,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewYmgalInfoGetter(getterOptions...).FetchMetadataByName(name, "")
				},
			})
		case enums.Steam:
			sources = append(sources, metadataSearchSource{
				source: enums.Steam,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewSteamInfoGetterWithLanguage(language, getterOptions...).FetchMetadataByName(name, "")
				},
			})
		case enums.DLsite:
			sources = append(sources, metadataSearchSource{
				source: enums.DLsite,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewDLsiteInfoGetter(getterOptions...).FetchMetadataByName(name, "")
				},
			})
		case enums.ErogameScape:
			sources = append(sources, metadataSearchSource{
				source: enums.ErogameScape,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewErogameScapeInfoGetter(getterOptions...).FetchMetadataByName(name, "")
				},
			})
		}
	}
	return sources
}

// CheckImportMetadataDuplicates 批量检查元数据 source/id 是否已存在。
func (s *ImportService) CheckImportMetadataDuplicates(requests []vo.ImportMetadataDuplicateRequest) ([]vo.ImportMetadataDuplicateResult, error) {
	results := make([]vo.ImportMetadataDuplicateResult, 0, len(requests))
	if len(requests) == 0 {
		return results, nil
	}

	idx, err := s.loadImportIndex()
	if err != nil {
		applog.LogErrorf(s.ctx, "CheckImportMetadataDuplicates: failed to load import index: %v", err)
		return results, fmt.Errorf("加载导入索引失败: %w", err)
	}

	for _, request := range requests {
		result := vo.ImportMetadataDuplicateResult{
			Source:   request.Source,
			SourceID: request.SourceID,
		}
		if ref, ok := idx.findBySource(request.Source, request.SourceID); ok {
			result.Exists = true
			result.ExistingID = ref.ID
			result.ExistingName = ref.Name
		}
		results = append(results, result)
	}

	return results, nil
}

// BatchImportGames 批量导入游戏
func (s *ImportService) BatchImportGames(candidates []vo.BatchImportCandidate) (ImportResult, error) {
	result := ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}

	startedAt := time.Now()
	stepStartedAt := time.Now()
	idx, err := s.loadImportIndex()
	if err != nil {
		applog.LogErrorf(s.ctx, "BatchImportGames: failed to load import index: %v", err)
		return result, fmt.Errorf("加载导入索引失败: %w", err)
	}
	applog.LogInfof(s.ctx, "BatchImportGames: loaded import index for candidates=%d elapsed=%s", len(candidates), time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	items := make([]importItem, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.IsSelected {
			continue
		}

		if ref, exists := idx.findByPathConflict(candidate.SelectedExe); exists {
			applog.LogWarningf(s.ctx, "BatchImportGames: path already exists for game %s, skipping: %s", ref.Name, candidate.SelectedExe)
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, candidate.SearchName+" (路径已存在: "+ref.Name+")")
			continue
		}

		gameName := candidate.SearchName
		if candidate.MatchedGame != nil && candidate.MatchedGame.Name != "" {
			gameName = candidate.MatchedGame.Name
		}

		if ref, exists := idx.findByNamePath(gameName, candidate.SelectedExe); exists {
			applog.LogWarningf(s.ctx, "BatchImportGames: game already exists with same path, skipping: %s", gameName)
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName+" (已存在: "+ref.Name+")")
			continue
		}
		if ref, exists := idx.findByName(gameName); exists && normalizeImportPath(ref.Path) != normalizeImportPath(candidate.SelectedExe) {
			applog.LogInfof(s.ctx, "BatchImportGames: importing duplicate name %s with different path: %s", gameName, candidate.SelectedExe)
		}

		var game models.Game
		if candidate.MatchedGame != nil {
			game = *candidate.MatchedGame
		} else {
			game = models.Game{
				Name:       candidate.SearchName,
				SourceType: enums.Local,
			}
		}

		game.ID = uuid.New().String()
		game.Path = candidate.SelectedExe
		game.CreatedAt = time.Now()
		game.CachedAt = time.Now()
		game.UpdatedAt = time.Now()

		source := candidate.MatchSource
		if source == "" {
			source = game.SourceType
		}
		if game.SourceType == "" {
			game.SourceType = source
		}
		if !s.allowDuplicateMetadataImport() {
			if sourceRef, exists := idx.findBySource(source, game.SourceID); exists {
				applog.LogWarningf(s.ctx, "BatchImportGames: source already exists for game %s, skipping: %s/%s", sourceRef.Name, source, game.SourceID)
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, gameName+" (元数据已存在: "+sourceRef.Name+")")
				continue
			}
		}

		item := importItem{
			Game:   game,
			Tags:   candidate.MatchedTags,
			Source: source,
		}
		items = append(items, item)
		idx.add(importGameRef{
			ID:         game.ID,
			Name:       game.Name,
			Path:       game.Path,
			SourceType: game.SourceType,
			SourceID:   game.SourceID,
			CreatedAt:  game.CreatedAt,
		})
	}
	applog.LogInfof(s.ctx, "BatchImportGames: built commit items=%d skipped=%d elapsed=%s", len(items), result.Skipped, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	success, sessionsImported, err := s.commitImportedItems(items)
	if err != nil {
		applog.LogErrorf(s.ctx, "BatchImportGames: batch import failed: %v", err)
		result.Failed += len(items)
		for _, item := range items {
			result.FailedNames = append(result.FailedNames, item.Game.Name)
		}
		return result, err
	}
	result.Success += success
	result.SessionsImported += sessionsImported
	applog.LogInfof(s.ctx, "BatchImportGames: complete success=%d skipped=%d failed=%d sessions=%d commit_elapsed=%s total=%s", result.Success, result.Skipped, result.Failed, result.SessionsImported, time.Since(stepStartedAt), time.Since(startedAt))

	return result, nil
}

// ProcessDroppedPaths 处理拖拽导入的路径，支持文件夹和可执行文件
// 返回候选游戏列表供前端展示和确认
func (s *ImportService) ProcessDroppedPaths(paths []string) (vo.BatchImportScanResult, error) {
	var candidates []vo.BatchImportCandidate
	var result vo.BatchImportScanResult

	excludeKeywords := defaultImportExcludeKeywords()
	const maxDepth = 3
	candidatesMap := make(map[string]vo.BatchImportCandidate)

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			applog.LogWarningf(s.ctx, "ProcessDroppedPaths: failed to stat path %s: %v", path, err)
			continue
		}

		if info.IsDir() {
			err := s.scanDirectoryRecursive(path, path, 0, maxDepth, excludeKeywords, "parent", 0, candidatesMap)
			if err != nil {
				applog.LogWarningf(s.ctx, "ProcessDroppedPaths: failed to scan directory %s: %v", path, err)
				continue
			}

			if len(candidatesMap) == 0 {
				applog.LogInfof(s.ctx, "ProcessDroppedPaths: no executable found in folder %s", path)
			}
			continue
		}

		lowerName := strings.ToLower(path)
		if !strings.HasSuffix(lowerName, ".exe") && !strings.HasSuffix(lowerName, ".bat") {
			applog.LogInfof(s.ctx, "ProcessDroppedPaths: skipping non-executable file %s", path)
			continue
		}

		fileName := filepath.Base(path)
		if shouldExcludeExecutable(fileName, excludeKeywords) {
			applog.LogInfof(s.ctx, "ProcessDroppedPaths: skipping excluded file %s", path)
			continue
		}

		folderPath := filepath.Dir(path)
		candidatesMap[folderPath] = vo.BatchImportCandidate{
			FolderPath:  folderPath,
			FolderName:  filepath.Base(folderPath),
			Executables: []string{path},
			SelectedExe: path,
			SearchName:  searchNameForExecutable(fileName, folderPath),
			IsSelected:  true,
			MatchStatus: "pending",
		}
	}

	for _, candidate := range candidatesMap {
		candidates = append(candidates, candidate)
	}

	idx, err := s.loadImportIndex()
	if err != nil {
		applog.LogErrorf(s.ctx, "ProcessDroppedPaths: failed to load import index: %v", err)
		return result, fmt.Errorf("加载导入索引失败: %w", err)
	}

	result = splitScanCandidates(candidates, idx)
	applog.LogInfof(s.ctx, "ProcessDroppedPaths: processed %d paths, found %d candidates, %d importable, %d skipped", len(paths), len(candidates), len(result.Candidates), result.Skipped)
	return result, nil
}

func defaultImportExcludeKeywords() []string {
	return []string{
		"unins", "setup", "config", "patch", "update", "crashpad",
		"vc_redist", "dxwebsetup", "directx", "vcredist", "dotnet",
		"redistributable", "installer", "launcher_helper", "crashreporter",
		"updater", "uninstall", "删除", "卸载",
	}
}

func shouldExcludeExecutable(fileName string, excludeKeywords []string) bool {
	lowerFileName := strings.ToLower(fileName)
	for _, keyword := range excludeKeywords {
		if strings.Contains(lowerFileName, keyword) {
			return true
		}
	}
	return false
}

func searchNameForExecutable(fileName string, folderPath string) string {
	searchName := filepath.Base(folderPath)
	exeName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	genericNames := []string{"game", "main", "start", "launch", "run", "play"}
	for _, generic := range genericNames {
		if strings.ToLower(exeName) == generic {
			return searchName
		}
	}
	if len(exeName) > 3 {
		return exeName
	}
	return searchName
}
