package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/protocol"
	"lunabox/internal/utils"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/utils/imageutils"
	"lunabox/internal/utils/metadata"
	"lunabox/internal/utils/processutils"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxClipboardCoverImageBytes = 25 * 1024 * 1024

type GameService struct {
	ctx              context.Context
	db               *sql.DB
	config           *appconf.AppConfig
	tagService       *TagService
	bangumiService   *BangumiService
	emitEvent        func(context.Context, string, ...interface{})
	imageTaskStarter func([]CoverImageDownloadItem) string
}

type CoverImageDownloadItem struct {
	GameID   string
	GameName string
	CoverURL string
}

type metadataSearchSource struct {
	source      enums2.SourceType
	fetchByName func(string) (metadata.MetadataResult, error)
}

type metadataUpdateFieldSet map[enums2.MetadataUpdateField]struct{}

func metadataGetterOptions(config *appconf.AppConfig) []metadata.GetterOption {
	if config == nil {
		return nil
	}

	return []metadata.GetterOption{
		metadata.WithProxyConfig(config),
		metadata.WithTagLimit(config.ScrapedTagLimit),
	}
}

func configuredMetadataSources(config *appconf.AppConfig) []enums2.SourceType {
	defaultSources := []enums2.SourceType{enums2.Bangumi, enums2.VNDB, enums2.Ymgal, enums2.Steam}
	if config == nil || len(config.MetadataSources) == 0 {
		return defaultSources
	}

	result := make([]enums2.SourceType, 0, len(config.MetadataSources))
	seen := make(map[enums2.SourceType]struct{}, len(config.MetadataSources))
	for _, source := range config.MetadataSources {
		normalized := normalizeMetadataSourceType(enums2.SourceType(source))
		switch normalized {
		case enums2.Bangumi, enums2.VNDB, enums2.Ymgal, enums2.Steam, enums2.DLsite, enums2.ErogameScape:
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}

	if len(result) == 0 {
		return defaultSources
	}
	return result
}

func NewGameService() *GameService {
	return &GameService{
		emitEvent: runtime.EventsEmit,
	}
}

func (s *GameService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	if s.emitEvent == nil {
		s.emitEvent = runtime.EventsEmit
	}
}

func (s *GameService) SetTagService(ts *TagService) {
	s.tagService = ts
}

func (s *GameService) SetBangumiService(bangumiService *BangumiService) {
	s.bangumiService = bangumiService
}

func (s *GameService) SetImageDownloadTaskStarter(starter func([]CoverImageDownloadItem) string) {
	s.imageTaskStarter = starter
}

func (s *GameService) SetEventEmitter(emit func(context.Context, string, ...interface{})) {
	s.emitEvent = emit
}

func executableDialogDefaults(currentPath string) (string, string) {
	currentPath = strings.TrimSpace(currentPath)
	if currentPath == "" {
		return "", ""
	}

	cleanPath := filepath.Clean(currentPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		absPath = cleanPath
	}

	info, err := os.Stat(absPath)
	if err == nil {
		if info.IsDir() {
			if isMacAppBundlePath(absPath) {
				return filepath.Dir(absPath), filepath.Base(absPath)
			}
			return absPath, ""
		}
		return filepath.Dir(absPath), filepath.Base(absPath)
	}

	if filepath.Ext(absPath) == "" {
		return "", ""
	}

	parentDir := filepath.Dir(absPath)
	if parentInfo, statErr := os.Stat(parentDir); statErr == nil && parentInfo.IsDir() {
		return parentDir, filepath.Base(absPath)
	}

	return "", ""
}

func (s *GameService) SelectGameExecutable(currentPath string) (string, error) {
	defaultDirectory, defaultFilename := executableDialogDefaults(currentPath)
	selection, err := runtime.OpenFileDialog(
		s.ctx,
		executableOpenDialogOptions("Select Game Executable", defaultDirectory, defaultFilename),
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open file dialog: %v", err)
	}
	return selection, err
}

func (s *GameService) SelectWineRunnerExecutable(currentPath string) (string, error) {
	defaultDirectory, defaultFilename := executableDialogDefaults(currentPath)
	selection, err := runtime.OpenFileDialog(
		s.ctx,
		wineRunnerOpenDialogOptions("Select Wine Executable", defaultDirectory, defaultFilename),
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open wine runner dialog: %v", err)
	}
	return selection, err
}

// ResolveExecutablePathForImport 解析导入时的可执行路径：
// - 如果是可执行文件路径，直接返回
// - 如果是目录，弹出文件选择器让用户手动选择可执行文件
func (s *GameService) ResolveExecutablePathForImport(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", nil
	}

	normalizedPath, err := filepath.Abs(filepath.Clean(trimmedPath))
	if err != nil {
		return "", fmt.Errorf("normalize import path failed: %w", err)
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("stat import path failed: %w", err)
	}

	if !info.IsDir() || isMacAppBundlePath(normalizedPath) {
		return normalizedPath, nil
	}

	selection, err := runtime.OpenFileDialog(
		s.ctx,
		executableOpenDialogOptions("选择游戏可执行文件", normalizedPath, ""),
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open import executable dialog: %v", err)
		return "", err
	}

	return selection, nil
}

func isMacAppBundlePath(path string) bool {
	return goruntime.GOOS == "darwin" && strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), ".app")
}

func executableOpenDialogOptions(title string, defaultDirectory string, defaultFilename string) runtime.OpenDialogOptions {
	options := runtime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: defaultDirectory,
		DefaultFilename:  defaultFilename,
	}
	if goruntime.GOOS == "darwin" {
		// macOS .app bundles are directories that NSOpenPanel can expose as
		// package files. Avoid extension filters here so Unix executables with
		// no suffix remain selectable too.
		options.ResolvesAliases = true
		options.TreatPackagesAsDirectories = false
		return options
	}

	options.Filters = []runtime.FileFilter{
		executableFileFilter(),
		allFilesFileFilter(),
	}
	return options
}

func wineRunnerOpenDialogOptions(title string, defaultDirectory string, defaultFilename string) runtime.OpenDialogOptions {
	options := executableOpenDialogOptions(title, defaultDirectory, defaultFilename)
	if goruntime.GOOS == "darwin" {
		options.TreatPackagesAsDirectories = true
	}
	return options
}

func executableFileFilter() runtime.FileFilter {
	switch goruntime.GOOS {
	case "darwin":
		return runtime.FileFilter{
			DisplayName: "Applications and Executables",
			Pattern:     "*.app;*.exe;*.bat;*.cmd",
		}
	default:
		return runtime.FileFilter{
			DisplayName: "Executables",
			Pattern:     "*.exe;*.bat;*.cmd;*.lnk",
		}
	}
}

func allFilesFileFilter() runtime.FileFilter {
	return runtime.FileFilter{
		DisplayName: "All Files",
		Pattern:     "*.*",
	}
}

// AddGameFromWebMetadata 用于接收前端/导入流程中的完整刮削结果（含 tags）并一次性入库。
func (s *GameService) AddGameFromWebMetadata(meta vo.GameMetadataFromWebVO) error {
	game := meta.Game
	if game.SourceType == "" {
		game.SourceType = meta.Source
	}
	fallbackFetchTags := len(meta.Tags) == 0
	return s.addGameWithTags(game, meta.Tags, fallbackFetchTags)
}

func (s *GameService) addGameWithTags(game models.Game, tags []metadata.TagItem, fallbackFetchTags bool) error {
	if game.ID == "" {
		game.ID = uuid.New().String()
	}

	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	if game.CachedAt.IsZero() {
		game.CachedAt = time.Now()
	}
	if game.UpdatedAt.IsZero() {
		game.UpdatedAt = time.Now()
	}
	if game.Status == "" {
		game.Status = enums2.StatusNotStarted
	}

	// 保存原始封面URL用于后台下载
	originalCoverURL := game.CoverURL

	// 处理临时封面图片
	if strings.Contains(game.CoverURL, "/local/covers/temp_") {
		newCoverURL, err := imageutils.RenameTempCover(game.CoverURL, game.ID)
		if err != nil {
			applog.LogWarningf(s.ctx, "AddGame: failed to rename temp cover: %v", err)
		} else {
			game.CoverURL = newCoverURL
			originalCoverURL = ""
		}
	}

	query := `INSERT INTO games (
		id, name, cover_url, company, summary, rating, release_date, path, 
		save_path, process_name, status, source_type, cached_at, source_id, created_at, updated_at,
		use_locale_emulator, use_magpie, metadata_locked, wine_runner, wine_args, wine_prefix
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(s.ctx, query,
		game.ID,
		game.Name,
		game.CoverURL,
		game.Company,
		game.Summary,
		game.Rating,
		game.ReleaseDate,
		game.Path,
		game.SavePath,
		game.ProcessName,
		string(game.Status),
		string(game.SourceType),
		game.CachedAt,
		game.SourceID,
		game.CreatedAt,
		game.UpdatedAt,
		game.UseLocaleEmulator,
		game.UseMagpie,
		game.MetadataLocked,
		game.WineRunner,
		game.WineArgs,
		game.WinePrefix,
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "AddGame: failed to insert game %s: %v", game.Name, err)
		return err
	}

	if err := deleteSyncTombstone(s.ctx, s.db, cloudSyncEntityGame, game.ID); err != nil {
		applog.LogWarningf(s.ctx, "AddGame: failed to clear game tombstone for %s: %v", game.ID, err)
	}

	// 优先使用已刮削出的 tags，避免重复网络请求；无 tags 时再按 source_id 兜底拉取。
	if s.tagService != nil {
		switch {
		case len(tags) > 0:
			if err := s.tagService.upsertScrapedTags(game.ID, tags); err != nil {
				applog.LogWarningf(s.ctx, "AddGame: failed to upsert scraped tags for game %s: %v", game.Name, err)
			}
		case fallbackFetchTags:
			s.syncScrapedTagsForGame(game)
		}
	}

	// 后台异步下载封面图片（不阻塞添加流程）
	if originalCoverURL != "" {
		go s.asyncDownloadCoverImage(game.ID, game.Name, originalCoverURL, true)
	}

	return nil
}

func (s *GameService) syncScrapedTagsForGame(game models.Game) {
	if s.tagService == nil {
		return
	}
	if game.SourceType == enums2.Local || game.SourceType == "" {
		return
	}
	if strings.TrimSpace(game.SourceID) == "" {
		return
	}

	metaResult, err := s.fetchMetadataResultBySource(game.SourceType, game.SourceID)
	if err != nil {
		applog.LogWarningf(s.ctx, "syncScrapedTagsForGame: failed to fetch tags for game %s (%s/%s): %v", game.Name, game.SourceType, game.SourceID, err)
		return
	}
	if len(metaResult.Tags) == 0 {
		applog.LogInfof(s.ctx, "syncScrapedTagsForGame: no tags returned for game %s (%s/%s)", game.Name, game.SourceType, game.SourceID)
		return
	}
	if err := s.tagService.upsertScrapedTags(game.ID, metaResult.Tags); err != nil {
		applog.LogWarningf(s.ctx, "syncScrapedTagsForGame: failed to upsert tags for game %s (%s/%s): %v", game.Name, game.SourceType, game.SourceID, err)
		return
	}
	applog.LogInfof(s.ctx, "syncScrapedTagsForGame: synced %d tags for game %s", len(metaResult.Tags), game.Name)
}

// asyncDownloadCoverImage 后台异步下载封面图片并更新数据库
func (s *GameService) asyncDownloadCoverImage(gameID, gameName, coverURL string, emitToast bool) bool {
	// 检查是否为远程URL
	if !isDownloadableCoverURL(coverURL) {
		return false
	}

	applog.LogInfof(s.ctx, "asyncDownloadCoverImage: downloading cover for %s", gameName)
	if emitToast {
		s.emitCoverImageDownloadEvent(gameID, gameName, "started", "")
	}

	// 下载并保存图片
	localPath, err := imageutils.DownloadAndSaveCoverImageWithProxyConfig(coverURL, gameID, s.config)
	if err != nil {
		applog.LogWarningf(s.ctx, "asyncDownloadCoverImage: failed to download cover for %s: %v", gameName, err)
		if emitToast {
			s.emitCoverImageDownloadEvent(gameID, gameName, "failed", err.Error())
		}
		return false
	}

	// 更新数据库中的封面路径
	if err := s.updateCoverURL(gameID, localPath); err != nil {
		applog.LogErrorf(s.ctx, "asyncDownloadCoverImage: failed to update cover URL for %s: %v", gameName, err)
		if emitToast {
			s.emitCoverImageDownloadEvent(gameID, gameName, "failed", err.Error())
		}
		return false
	}

	applog.LogInfof(s.ctx, "asyncDownloadCoverImage: successfully cached cover for %s", gameName)
	if emitToast {
		s.emitCoverImageDownloadEvent(gameID, gameName, "done", "")
	}
	return true
}

// DownloadCoverImage 下载远程封面图片并替换为本地托管路径。
func (s *GameService) DownloadCoverImage(gameID string, coverURL string) (string, error) {
	gameID = strings.TrimSpace(gameID)
	coverURL = strings.TrimSpace(coverURL)
	if gameID == "" {
		return "", errors.New("game ID is required")
	}
	if !isDownloadableCoverURL(coverURL) {
		return "", fmt.Errorf("cover URL is not a downloadable remote URL")
	}

	localPath, err := imageutils.DownloadAndSaveCoverImageWithProxyConfig(coverURL, gameID, s.config)
	if err != nil {
		applog.LogWarningf(s.ctx, "DownloadCoverImage: failed to download cover for %s from %s: %v", gameID, coverURL, err)
		return "", fmt.Errorf("failed to download cover image: %w", err)
	}

	if err := s.updateCoverURL(gameID, localPath); err != nil {
		applog.LogErrorf(s.ctx, "DownloadCoverImage: failed to update cover URL for %s: %v", gameID, err)
		return "", fmt.Errorf("failed to update cover URL: %w", err)
	}

	return localPath, nil
}

func isDownloadableCoverURL(coverURL string) bool {
	coverURL = strings.TrimSpace(coverURL)
	normalizedURL := strings.ToLower(coverURL)
	if coverURL == "" || strings.Contains(normalizedURL, "wails.localhost") {
		return false
	}
	return strings.HasPrefix(normalizedURL, "http://") || strings.HasPrefix(normalizedURL, "https://")
}

func (s *GameService) emitCoverImageDownloadEvent(gameID, gameName, status, errorMsg string) {
	if s.ctx == nil || s.emitEvent == nil {
		return
	}
	s.emitEvent(s.ctx, "cover-image:download", map[string]string{
		"game_id":   gameID,
		"game_name": gameName,
		"status":    status,
		"error":     errorMsg,
	})
}

// updateCoverURL 更新游戏的封面URL
func (s *GameService) updateCoverURL(gameID, coverURL string) error {
	query := `UPDATE games SET cover_url = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(s.ctx, query, coverURL, time.Now(), gameID)
	return err
}

func (s *GameService) DeleteGame(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to begin transaction: %v", err)
		return err
	}
	defer tx.Rollback()

	if err := s.deleteGameTx(tx, id, time.Now()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (s *GameService) DeleteGames(ids []string) error {
	ids = utils.UniqueNonEmptyStrings(ids)
	if len(ids) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to begin transaction: %v", err)
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, id := range ids {
		if err := s.deleteGameTx(tx, id, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (s *GameService) GetGames(req vo.GameListRequest) (vo.GameListResponse, error) {
	resp, err := queryGameList(s.ctx, s.db, req, gameListScope{})
	if err != nil {
		applog.LogErrorf(s.ctx, "GetGames: failed to query game list: %v", err)
		return resp, err
	}
	return resp, nil
}

func (s *GameService) listAllGamesInternal() ([]models.Game, error) {
	var all []models.Game
	req := vo.GameListRequest{
		Limit:     maxGameListLimit,
		SortBy:    enums2.GameListSortByCreatedAt,
		SortOrder: enums2.SortOrderDesc,
	}
	for {
		resp, err := queryGameList(s.ctx, s.db, req, gameListScope{})
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Games...)
		if !resp.HasMore {
			return all, nil
		}
		req.Offset += resp.Limit
	}
}

func (s *GameService) GetGameByID(id string) (models.Game, error) {
	// FIXME: 这里对于上次游玩时间查询使用了一个子查询，可能存在性能问题，后续可以考虑优化或者在 game 中增加一个 last_played_at 字段来直接存储每个游戏的最近游玩时间
	query := `SELECT 
		g.id, g.name, 
		COALESCE(g.cover_url, '') as cover_url, 
		COALESCE(g.company, '') as company, 
		COALESCE(g.summary, '') as summary, 
		COALESCE(g.rating, 0) as rating,
		COALESCE(g.release_date, '') as release_date,
		COALESCE(g.path, '') as path, 
		COALESCE(g.save_path, '') as save_path,
		COALESCE(g.process_name, '') as process_name,
		COALESCE(g.wine_runner, '') as wine_runner,
		COALESCE(g.wine_args, '') as wine_args,
		COALESCE(g.wine_prefix, '') as wine_prefix,
		COALESCE(g.status, 'not_started') as status,
		COALESCE(g.source_type, '') as source_type, 
		g.cached_at, 
		COALESCE(g.source_id, '') as source_id, 
		g.created_at,
		COALESCE(g.updated_at, g.created_at, g.cached_at) as updated_at,
		latest.last_played_at,
		COALESCE(g.use_locale_emulator, FALSE) as use_locale_emulator,
		COALESCE(g.use_magpie, FALSE) as use_magpie,
		COALESCE(g.metadata_locked, FALSE) as metadata_locked
	FROM games g
	LEFT JOIN (
		SELECT game_id, MAX(start_time) as last_played_at
		FROM play_sessions
		GROUP BY game_id
	) latest ON latest.game_id = g.id
	WHERE g.id = ?`

	var game models.Game
	var sourceType string
	var status string
	var lastPlayedAt sql.NullTime

	err := s.db.QueryRowContext(s.ctx, query, id).Scan(
		&game.ID,
		&game.Name,
		&game.CoverURL,
		&game.Company,
		&game.Summary,
		&game.Rating,
		&game.ReleaseDate,
		&game.Path,
		&game.SavePath,
		&game.ProcessName,
		&game.WineRunner,
		&game.WineArgs,
		&game.WinePrefix,
		&status,
		&sourceType,
		&game.CachedAt,
		&game.SourceID,
		&game.CreatedAt,
		&game.UpdatedAt,
		&lastPlayedAt,
		&game.UseLocaleEmulator,
		&game.UseMagpie,
		&game.MetadataLocked,
	)

	if errors.Is(err, sql.ErrNoRows) {
		applog.LogWarningf(s.ctx, "GetGameByID: game not found with id: %s", id)
		return models.Game{}, fmt.Errorf("game not found with id: %s", id)
	}
	if err != nil {
		applog.LogErrorf(s.ctx, "GetGameByID: failed to query game %s: %v", id, err)
		return models.Game{}, fmt.Errorf("failed to query game: %w", err)
	}

	game.SourceType = enums2.SourceType(sourceType)
	game.Status = enums2.GameStatus(status)
	if lastPlayedAt.Valid {
		lastPlayed := lastPlayedAt.Time
		game.LastPlayedAt = &lastPlayed
	}
	return game, nil
}

func (s *GameService) UpdateGame(game models.Game) error {
	previousGame, err := s.getGameStatusSyncSnapshot(game.ID)
	if err != nil {
		return err
	}

	game.UpdatedAt = time.Now()

	query := `UPDATE games SET 
		name = ?,
		cover_url = ?,
		company = ?,
		summary = ?,
		rating = ?,
		release_date = ?,
		path = ?,
		save_path = ?,
		process_name = ?,
		wine_runner = ?,
		wine_args = ?,
		wine_prefix = ?,
		status = ?,
		source_type = ?,
		cached_at = ?,
		source_id = ?,
		updated_at = ?,
		use_locale_emulator = ?,
		use_magpie = ?,
		metadata_locked = ?
	WHERE id = ?`

	result, err := s.db.ExecContext(s.ctx, query,
		game.Name,
		game.CoverURL,
		game.Company,
		game.Summary,
		game.Rating,
		game.ReleaseDate,
		game.Path,
		game.SavePath,
		game.ProcessName,
		game.WineRunner,
		game.WineArgs,
		game.WinePrefix,
		string(game.Status),
		string(game.SourceType),
		game.CachedAt,
		game.SourceID,
		game.UpdatedAt,
		game.UseLocaleEmulator,
		game.UseMagpie,
		game.MetadataLocked,
		game.ID,
	)

	if err != nil {
		applog.LogErrorf(s.ctx, "UpdateGame: failed to update game %s: %v", game.ID, err)
		return fmt.Errorf("failed to update game: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		applog.LogErrorf(s.ctx, "UpdateGame: failed to get rows affected for id %s: %v", game.ID, err)
		return err
	}

	if rowsAffected == 0 {
		applog.LogWarningf(s.ctx, "UpdateGame: game not found with id: %s", game.ID)
		return fmt.Errorf("game not found with id: %s", game.ID)
	}

	if err := deleteSyncTombstone(s.ctx, s.db, cloudSyncEntityGame, game.ID); err != nil {
		applog.LogWarningf(s.ctx, "UpdateGame: failed to clear game tombstone for %s: %v", game.ID, err)
	}

	s.pushBangumiStatusAfterLocalSave(previousGame, game)
	return nil
}

func (s *GameService) deleteGameTx(tx *sql.Tx, id string, deletedAt time.Time) error {
	var gameExists bool
	if err := tx.QueryRowContext(s.ctx, "SELECT EXISTS(SELECT 1 FROM games WHERE id = ?)", id).Scan(&gameExists); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to check game existence for id %s: %v", id, err)
		return fmt.Errorf("failed to check game existence: %w", err)
	}
	if !gameExists {
		applog.LogWarningf(s.ctx, "DeleteGame: game not found with id: %s", id)
		return fmt.Errorf("game not found with id: %s", id)
	}

	relRows, err := tx.QueryContext(s.ctx, "SELECT game_id, category_id FROM game_categories WHERE game_id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to query game_categories for id %s: %v", id, err)
		return fmt.Errorf("failed to query game categories: %w", err)
	}
	var relationIDs []string
	for relRows.Next() {
		var gameID string
		var categoryID string
		if scanErr := relRows.Scan(&gameID, &categoryID); scanErr != nil {
			relRows.Close()
			return fmt.Errorf("failed to scan game category relation: %w", scanErr)
		}
		relationIDs = append(relationIDs, relationTombstoneID(gameID, categoryID))
	}
	relRows.Close()

	sessionRows, err := tx.QueryContext(s.ctx, "SELECT id FROM play_sessions WHERE game_id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to query play_sessions for id %s: %v", id, err)
		return fmt.Errorf("failed to query play sessions: %w", err)
	}
	var sessionIDs []string
	for sessionRows.Next() {
		var sessionID string
		if scanErr := sessionRows.Scan(&sessionID); scanErr != nil {
			sessionRows.Close()
			return fmt.Errorf("failed to scan play session id: %w", scanErr)
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	sessionRows.Close()

	progressRows, err := tx.QueryContext(s.ctx, "SELECT id FROM game_progress WHERE game_id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to query game_progress for id %s: %v", id, err)
		return fmt.Errorf("failed to query game progress: %w", err)
	}
	var progressIDs []string
	for progressRows.Next() {
		var progressID string
		if scanErr := progressRows.Scan(&progressID); scanErr != nil {
			progressRows.Close()
			return fmt.Errorf("failed to scan game progress id: %w", scanErr)
		}
		progressIDs = append(progressIDs, progressID)
	}
	progressRows.Close()

	tagRows, err := tx.QueryContext(s.ctx, "SELECT game_id, source, name FROM game_tags WHERE game_id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to query game_tags for id %s: %v", id, err)
		return fmt.Errorf("failed to query game tags: %w", err)
	}
	var tagIDs []string
	for tagRows.Next() {
		var gameID string
		var source string
		var name string
		if scanErr := tagRows.Scan(&gameID, &source, &name); scanErr != nil {
			tagRows.Close()
			return fmt.Errorf("failed to scan game tag identity: %w", scanErr)
		}
		tagIDs = append(tagIDs, tagTombstoneID(gameID, source, name))
	}
	tagRows.Close()

	for _, relationID := range relationIDs {
		if err := upsertSyncTombstone(s.ctx, tx, cloudSyncEntityGameCategory, relationID, deletedAt); err != nil {
			return err
		}
	}
	for _, sessionID := range sessionIDs {
		if err := upsertSyncTombstone(s.ctx, tx, cloudSyncEntityPlaySession, sessionID, deletedAt); err != nil {
			return err
		}
	}
	for _, progressID := range progressIDs {
		if err := upsertSyncTombstone(s.ctx, tx, cloudSyncEntityGameProgress, progressID, deletedAt); err != nil {
			return err
		}
	}
	for _, tagID := range tagIDs {
		if err := upsertSyncTombstone(s.ctx, tx, cloudSyncEntityGameTag, tagID, deletedAt); err != nil {
			return err
		}
	}
	if err := upsertSyncTombstone(s.ctx, tx, cloudSyncEntityGame, id, deletedAt); err != nil {
		return err
	}

	if _, err := tx.ExecContext(s.ctx, "DELETE FROM game_categories WHERE game_id = ?", id); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete game_categories for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game categories: %w", err)
	}
	if _, err := tx.ExecContext(s.ctx, "DELETE FROM play_sessions WHERE game_id = ?", id); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete play_sessions for id %s: %v", id, err)
		return fmt.Errorf("failed to delete play sessions: %w", err)
	}
	if _, err := tx.ExecContext(s.ctx, "DELETE FROM game_progress WHERE game_id = ?", id); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete game_progress for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game progress: %w", err)
	}
	if _, err := tx.ExecContext(s.ctx, "DELETE FROM game_tags WHERE game_id = ?", id); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete game_tags for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game tags: %w", err)
	}
	if _, err := tx.ExecContext(s.ctx, "DELETE FROM games WHERE id = ?", id); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete game for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game: %w", err)
	}

	return nil
}

// SelectSaveFile 选择存档文件
func (s *GameService) SelectSaveFile() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择存档文件",
	})
	return selection, err
}

// SelectSaveDirectory 选择存档目录
func (s *GameService) SelectSaveDirectory() (string, error) {
	selection, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择存档文件夹",
	})
	return selection, err
}

// SelectCoverImage 选择封面图片并保存到 covers 目录
func (s *GameService) SelectCoverImage(gameID string) (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择封面图片",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "图片文件",
				Pattern:     "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp",
			},
		},
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open file dialog: %v", err)
		return "", err
	}
	if selection == "" {
		return "", nil
	}

	coverPath, err := imageutils.SaveCoverImage(selection, gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to save cover image: %v", err)
		return "", fmt.Errorf("failed to save cover image: %w", err)
	}
	if err := s.updateCoverURL(gameID, coverPath); err != nil {
		applog.LogErrorf(s.ctx, "failed to update cover URL: %v", err)
		return "", fmt.Errorf("failed to update cover URL: %w", err)
	}

	return coverPath, nil
}

// SaveCoverImageDataURL 保存前端剪贴板读取到的图片 data URL。
func (s *GameService) SaveCoverImageDataURL(gameID string, dataURL string) (string, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return "", errors.New("game ID is required")
	}

	contentType, encodedData, err := splitImageDataURL(dataURL)
	if err != nil {
		return "", err
	}
	if base64.StdEncoding.DecodedLen(len(encodedData)) > maxClipboardCoverImageBytes {
		return "", fmt.Errorf("cover image is too large")
	}

	imageData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return "", fmt.Errorf("decode cover image data: %w", err)
	}
	if len(imageData) > maxClipboardCoverImageBytes {
		return "", fmt.Errorf("cover image is too large")
	}

	coverPath, err := imageutils.SaveCoverImageBytes(imageData, gameID, contentType)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to save cover image from clipboard: %v", err)
		return "", fmt.Errorf("failed to save cover image from clipboard: %w", err)
	}
	if err := s.updateCoverURL(gameID, coverPath); err != nil {
		applog.LogErrorf(s.ctx, "failed to update cover URL from clipboard: %v", err)
		return "", fmt.Errorf("failed to update cover URL from clipboard: %w", err)
	}

	return coverPath, nil
}

func splitImageDataURL(dataURL string) (string, string, error) {
	meta, encodedData, ok := strings.Cut(strings.TrimSpace(dataURL), ",")
	if !ok || !strings.HasPrefix(meta, "data:") {
		return "", "", fmt.Errorf("invalid image data URL")
	}

	metaParts := strings.Split(strings.TrimPrefix(meta, "data:"), ";")
	contentType := strings.ToLower(strings.TrimSpace(metaParts[0]))
	if !strings.HasPrefix(contentType, "image/") {
		return "", "", fmt.Errorf("unsupported cover image type: %s", contentType)
	}

	isBase64 := false
	for _, part := range metaParts[1:] {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			isBase64 = true
			break
		}
	}
	if !isBase64 {
		return "", "", fmt.Errorf("image data URL must be base64 encoded")
	}

	encodedData = strings.TrimSpace(encodedData)
	if encodedData == "" {
		return "", "", fmt.Errorf("cover image data is empty")
	}

	return contentType, encodedData, nil
}

// SelectCoverImageWithTempID 选择封面图片并使用临时ID保存（用于新增游戏时）
func (s *GameService) SelectCoverImageWithTempID() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择封面图片",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "图片文件",
				Pattern:     "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp",
			},
		},
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open file dialog: %v", err)
		return "", err
	}
	if selection == "" {
		return "", nil
	}

	// 使用时间戳作为临时ID
	tempID := fmt.Sprintf("temp_%d", time.Now().UnixNano())
	coverPath, err := imageutils.SaveCoverImage(selection, tempID)
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to save cover image: %v", err)
		return "", fmt.Errorf("failed to save cover image: %w", err)
	}

	return coverPath, nil
}

// ExportLaunchShortcut exports a per-game .url shortcut that re-enters LunaBox via protocol.
func (s *GameService) ExportLaunchShortcut(gameID string) (string, error) {
	game, err := s.GetGameByID(gameID)
	if err != nil {
		return "", fmt.Errorf("加载游戏失败: %w", err)
	}

	launchURL, err := protocol.BuildLaunchURL(game.ID)
	if err != nil {
		return "", fmt.Errorf("生成快捷启动链接失败: %w", err)
	}

	defaultName := strings.TrimSpace(downloadutils.SanitizeFileName(game.Name))
	if defaultName == "" {
		defaultName = strings.TrimSpace(game.ID)
	}
	defaultName += ".url"

	defaultDir := ""
	if desktopDir, err := apputils.GetDesktopDir(); err == nil {
		if info, statErr := os.Stat(desktopDir); statErr == nil && info.IsDir() {
			defaultDir = desktopDir
		}
	}

	savePath, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		Title:            "导出快捷启动方式",
		DefaultDirectory: defaultDir,
		DefaultFilename:  defaultName,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Internet Shortcut (*.url)",
				Pattern:     "*.url",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(savePath) == "" {
		return "", nil
	}

	iconPath := resolveLaunchShortcutIconPath(game.Path)
	if iconPath != "" {
		cachePath, cacheErr := apputils.ExportShortcutIconCache(iconPath, game.ID)
		if cacheErr != nil {
			applog.LogWarningf(s.ctx, "ExportLaunchShortcut: failed to cache icon from %s: %v", iconPath, cacheErr)
		} else {
			iconPath = cachePath
		}
	}
	if err := apputils.WriteInternetShortcut(savePath, apputils.InternetShortcut{
		URL:       launchURL,
		IconFile:  iconPath,
		IconIndex: 0,
	}); err != nil {
		return "", fmt.Errorf("写入快捷方式文件失败: %w", err)
	}

	if !strings.EqualFold(filepath.Ext(savePath), ".url") {
		savePath += ".url"
	}
	return savePath, nil
}

func (s *GameService) FetchMetadataByName(name string) ([]vo.GameMetadataFromWebVO, error) {
	var games []vo.GameMetadataFromWebVO
	var wg sync.WaitGroup
	var mu sync.Mutex

	searchSources := s.getConfiguredMetadataSearchSources()
	// 这里暂不处理任何错误，直接尝试从多个来源并发获取数据，空就是网络问题或未找到，不管它
	wg.Add(len(searchSources))
	for _, searchSource := range searchSources {
		src := searchSource
		go func() {
			defer wg.Done()
			result, _ := src.fetchByName(name)
			if result.Game != (models.Game{}) {
				mu.Lock()
				games = append(games, vo.GameMetadataFromWebVO{Source: src.source, Game: result.Game, Tags: result.Tags})
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	return games, nil
}

func (s *GameService) FetchMetadata(req vo.MetadataRequest) (models.Game, error) {
	result, err := s.FetchMetadataFromWeb(req)
	if err != nil {
		return models.Game{}, err
	}
	return result.Game, nil
}

func (s *GameService) FetchMetadataFromWeb(req vo.MetadataRequest) (vo.GameMetadataFromWebVO, error) {
	result, err := s.fetchMetadataResultByRequest(req)
	if err != nil {
		return vo.GameMetadataFromWebVO{}, err
	}

	return vo.GameMetadataFromWebVO{
		Source: req.Source,
		Game:   result.Game,
		Tags:   result.Tags,
	}, nil
}

func (s *GameService) fetchMetadataResultByRequest(req vo.MetadataRequest) (metadata.MetadataResult, error) {
	sourceID := strings.TrimSpace(req.ID)
	if sourceID == "" {
		return metadata.MetadataResult{}, errors.New("metadata id is empty")
	}

	switch req.Source {
	case enums2.Bangumi:
		return s.fetchMetadataResultBySource(req.Source, strings.ToLower(sourceID))
	case enums2.VNDB:
		if !isVndbId(strings.ToLower(sourceID)) {
			return metadata.MetadataResult{}, fmt.Errorf("invalid VNDB ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, strings.ToLower(sourceID))
	case enums2.Ymgal:
		if !isYmgalId(strings.ToLower(sourceID)) {
			return metadata.MetadataResult{}, fmt.Errorf("invalid Ymgal ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, strings.ToLower(sourceID))
	case enums2.Steam:
		if !isSteamAppID(sourceID) {
			return metadata.MetadataResult{}, fmt.Errorf("invalid Steam app ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, sourceID)
	case enums2.DLsite:
		normalizedID, ok := metadata.NormalizeDLsiteID(sourceID)
		if !ok {
			return metadata.MetadataResult{}, fmt.Errorf("invalid DLsite ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, normalizedID)
	case enums2.ErogameScape:
		normalizedID, ok := metadata.NormalizeErogameScapeID(sourceID)
		if !ok {
			return metadata.MetadataResult{}, fmt.Errorf("invalid ErogameScape ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, normalizedID)
	default:
		return metadata.MetadataResult{}, fmt.Errorf("unsupported source type: %s", req.Source)
	}
}

func (s *GameService) fetchMetadataResultBySource(source enums2.SourceType, sourceID string) (metadata.MetadataResult, error) {
	getterOptions := metadataGetterOptions(s.config)
	switch source {
	case enums2.Bangumi:
		if s.bangumiService == nil {
			return metadata.MetadataResult{}, fmt.Errorf("Bangumi 服务未初始化")
		}
		return s.bangumiService.fetchMetadataByID(s.ctx, sourceID)
	case enums2.VNDB:
		getter := metadata.NewVNDBInfoGetterWithLanguage(s.config.Language, getterOptions...)
		return getter.FetchMetadata(sourceID, s.config.VNDBAccessToken)
	case enums2.Ymgal:
		getter := metadata.NewYmgalInfoGetter(getterOptions...)
		return getter.FetchMetadata(sourceID, "")
	case enums2.Steam:
		getter := metadata.NewSteamInfoGetterWithLanguage(s.config.Language, getterOptions...)
		return getter.FetchMetadata(sourceID, "")
	case enums2.DLsite:
		getter := metadata.NewDLsiteInfoGetter(getterOptions...)
		return getter.FetchMetadata(sourceID, "")
	case enums2.ErogameScape:
		getter := metadata.NewErogameScapeInfoGetter(getterOptions...)
		return getter.FetchMetadata(sourceID, "")
	default:
		return metadata.MetadataResult{}, fmt.Errorf("unsupported source type: %s", source)
	}
}

// isVndbId 判断是否符合VNDB ID的格式（以字母v开头，后面跟数字）
func isVndbId(sourceId string) bool {
	return strings.HasPrefix(sourceId, "v") && len(sourceId) > 1
}

// isYmgalId 判断是否符合Ymgal ID的格式（以字母ga开头，后面跟数字）
func isYmgalId(sourceId string) bool {
	return strings.HasPrefix(sourceId, "ga") && len(sourceId) > 2
}

// isSteamAppID 判断是否包含可解析的 Steam AppID（支持纯数字或常见 URL/协议前缀）。
func isSteamAppID(sourceId string) bool {
	id := strings.TrimSpace(sourceId)
	if id == "" {
		return false
	}

	// 支持纯 appid，也支持带前缀/URL 的形式（如 steam://rungameid/620、.../app/620/...）
	inDigits := false
	for i := 0; i < len(id); i++ {
		if id[i] >= '0' && id[i] <= '9' {
			inDigits = true
			continue
		}
		if inDigits {
			return true
		}
	}
	return inDigits
}

// UpdateGameFromRemote 从远程数据源更新游戏信息
func (s *GameService) UpdateGameFromRemote(gameID string) error {
	_, err := s.updateGameMetadataFromRemote(gameID, true, nil)
	return err
}

// UpdateGameFromRemoteWithFields 从远程数据源更新指定字段。
func (s *GameService) UpdateGameFromRemoteWithFields(gameID string, fields []enums2.MetadataUpdateField) error {
	_, err := s.updateGameMetadataFromRemote(gameID, true, fields)
	return err
}

func (s *GameService) updateGameMetadataFromRemote(gameID string, downloadCoverImmediately bool, fields []enums2.MetadataUpdateField) (string, error) {
	// 获取现有游戏信息
	existingGame, err := s.GetGameByID(gameID)
	if err != nil {
		return "", fmt.Errorf("failed to get game: %w", err)
	}
	fieldSet := normalizeMetadataUpdateFields(fields)

	sourceType := normalizeMetadataSourceType(existingGame.SourceType)
	sourceID := strings.TrimSpace(existingGame.SourceID)
	if existingGame.MetadataLocked {
		return "", fmt.Errorf("游戏元数据已锁定，请先解锁后再更新")
	}
	if sourceType == "" || sourceType == enums2.Local || sourceID == "" {
		return "", fmt.Errorf("游戏缺少数据源信息，无法从远程更新")
	}

	if !s.isMetadataSourceEnabled(sourceType) {
		return "", fmt.Errorf("元数据源 %s 未启用，无法更新游戏元数据", sourceType)
	}

	sourceId := strings.ToLower(sourceID)
	metaResult, err := s.fetchMetadataResultBySource(sourceType, sourceId)
	if err != nil {
		return "", fmt.Errorf("failed to fetch metadata from remote: %w", err)
	}

	remoteGame := metaResult.Game
	remoteCoverURL := strings.TrimSpace(remoteGame.CoverURL)

	// 保留本地重要字段，更新远程可获取的字段
	if fieldSet.has(enums2.MetadataUpdateFieldName) {
		existingGame.Name = remoteGame.Name
	}
	if fieldSet.has(enums2.MetadataUpdateFieldCompany) {
		existingGame.Company = remoteGame.Company
	}
	if fieldSet.has(enums2.MetadataUpdateFieldSummary) {
		existingGame.Summary = remoteGame.Summary
	}
	if fieldSet.has(enums2.MetadataUpdateFieldRating) {
		existingGame.Rating = remoteGame.Rating
	}
	if fieldSet.has(enums2.MetadataUpdateFieldReleaseDate) {
		existingGame.ReleaseDate = remoteGame.ReleaseDate
	}
	existingGame.CachedAt = time.Now()

	if fieldSet.has(enums2.MetadataUpdateFieldCover) {
		existingGame.CoverURL = remoteCoverURL
	}

	if err := s.UpdateGame(existingGame); err != nil {
		return "", fmt.Errorf("failed to update game: %w", err)
	}

	if downloadCoverImmediately && fieldSet.has(enums2.MetadataUpdateFieldCover) && remoteCoverURL != "" {
		go s.asyncDownloadCoverImage(existingGame.ID, existingGame.Name, remoteCoverURL, true)
	}

	// 写入 tags（先删除刮削来源的旧 tag，再批量插入新 tag，保留用户 tag）
	if fieldSet.has(enums2.MetadataUpdateFieldTags) && s.tagService != nil && len(metaResult.Tags) > 0 {
		if err := s.tagService.upsertScrapedTags(gameID, metaResult.Tags); err != nil {
			applog.LogWarningf(s.ctx, "UpdateGameFromRemote: failed to upsert tags for game %s: %v", gameID, err)
		}
	}

	applog.LogInfof(s.ctx, "UpdateGameFromRemote: successfully updated game %s from %s", existingGame.Name, sourceType)
	if !fieldSet.has(enums2.MetadataUpdateFieldCover) {
		return "", nil
	}
	return remoteCoverURL, nil
}

func normalizeMetadataUpdateFields(fields []enums2.MetadataUpdateField) metadataUpdateFieldSet {
	fieldSet := make(metadataUpdateFieldSet, len(enums2.DefaultMetadataUpdateFields))
	for _, field := range fields {
		normalized := enums2.MetadataUpdateField(strings.ToLower(strings.TrimSpace(string(field))))
		switch normalized {
		case enums2.MetadataUpdateFieldName,
			enums2.MetadataUpdateFieldCover,
			enums2.MetadataUpdateFieldCompany,
			enums2.MetadataUpdateFieldSummary,
			enums2.MetadataUpdateFieldRating,
			enums2.MetadataUpdateFieldReleaseDate,
			enums2.MetadataUpdateFieldTags:
			fieldSet[normalized] = struct{}{}
		}
	}

	if len(fieldSet) > 0 {
		return fieldSet
	}

	for _, field := range enums2.DefaultMetadataUpdateFields {
		fieldSet[field] = struct{}{}
	}
	return fieldSet
}

func (fields metadataUpdateFieldSet) has(field enums2.MetadataUpdateField) bool {
	_, ok := fields[field]
	return ok
}

func (s *GameService) emitMetadataRefreshProgress(result vo.MetadataRefreshResult, current int, gameName string, status string) {
	if s.ctx == nil || s.emitEvent == nil {
		return
	}
	s.emitEvent(s.ctx, "metadata:refresh-progress", map[string]interface{}{
		"status":            status,
		"current":           current,
		"total":             result.TotalGames,
		"game_name":         gameName,
		"updated_games":     result.UpdatedGames,
		"skipped_games":     result.SkippedGames,
		"failed_games":      result.FailedGames,
		"locked_games":      result.LockedGames,
		"failed_game_ids":   result.FailedGameIDs,
		"failed_game_names": result.FailedGameNames,
	})
}

func (s *GameService) RefreshAllGamesMetadata() (vo.MetadataRefreshResult, error) {
	return s.RefreshAllGamesMetadataWithFields(nil)
}

func (s *GameService) RefreshAllGamesMetadataWithFields(fields []enums2.MetadataUpdateField) (vo.MetadataRefreshResult, error) {
	games, err := s.listAllGamesInternal()
	if err != nil {
		return newMetadataRefreshResult(), fmt.Errorf("failed to get games: %w", err)
	}

	return s.refreshGamesMetadata(games, "RefreshAllGamesMetadata", fields)
}

func (s *GameService) RefreshGamesMetadata(gameIDs []string) (vo.MetadataRefreshResult, error) {
	return s.RefreshGamesMetadataWithFields(gameIDs, nil)
}

func (s *GameService) RefreshGamesMetadataWithFields(gameIDs []string, fields []enums2.MetadataUpdateField) (vo.MetadataRefreshResult, error) {
	gameIDs = utils.UniqueNonEmptyStrings(gameIDs)
	if len(gameIDs) == 0 {
		return newMetadataRefreshResult(), nil
	}

	allGames, err := s.listAllGamesInternal()
	if err != nil {
		return newMetadataRefreshResult(), fmt.Errorf("failed to get games: %w", err)
	}

	idSet := make(map[string]struct{}, len(gameIDs))
	for _, id := range gameIDs {
		idSet[id] = struct{}{}
	}

	games := make([]models.Game, 0, len(gameIDs))
	for _, game := range allGames {
		if _, ok := idSet[game.ID]; ok {
			games = append(games, game)
		}
	}

	return s.refreshGamesMetadata(games, "RefreshGamesMetadata", fields)
}

func newMetadataRefreshResult() vo.MetadataRefreshResult {
	return vo.MetadataRefreshResult{
		FailedGameIDs:   []string{},
		FailedGameNames: []string{},
	}
}

func (s *GameService) refreshGamesMetadata(games []models.Game, logPrefix string, fields []enums2.MetadataUpdateField) (vo.MetadataRefreshResult, error) {
	result := newMetadataRefreshResult()
	result.TotalGames = len(games)
	enabledSources := s.getConfiguredMetadataSourceSet()
	imageItems := make([]CoverImageDownloadItem, 0)
	s.emitMetadataRefreshProgress(result, 0, "", "started")

	for index, game := range games {
		current := index + 1
		s.emitMetadataRefreshProgress(result, current, game.Name, "running")

		if game.MetadataLocked {
			result.SkippedGames++
			result.LockedGames++
			s.emitMetadataRefreshProgress(result, current, game.Name, "running")
			continue
		}

		if game.SourceType == "" || game.SourceType == enums2.Local || strings.TrimSpace(game.SourceID) == "" {
			result.SkippedGames++
			s.emitMetadataRefreshProgress(result, current, game.Name, "running")
			continue
		}

		if _, enabled := enabledSources[normalizeMetadataSourceType(game.SourceType)]; !enabled {
			result.SkippedGames++
			s.emitMetadataRefreshProgress(result, current, game.Name, "running")
			continue
		}

		remoteCoverURL, err := s.updateGameMetadataFromRemote(game.ID, false, fields)
		if err != nil {
			result.FailedGames++
			result.FailedGameIDs = append(result.FailedGameIDs, game.ID)
			result.FailedGameNames = append(result.FailedGameNames, game.Name)
			applog.LogWarningf(s.ctx, "%s: failed to update game %s (%s): %v", logPrefix, game.Name, game.ID, err)
		} else {
			result.UpdatedGames++
			if remoteCoverURL != "" {
				imageItems = append(imageItems, CoverImageDownloadItem{
					GameID:   game.ID,
					GameName: game.Name,
					CoverURL: remoteCoverURL,
				})
			}
		}

		s.emitMetadataRefreshProgress(result, current, game.Name, "running")
	}

	if len(imageItems) > 0 && s.imageTaskStarter != nil {
		s.imageTaskStarter(imageItems)
	}
	s.emitMetadataRefreshProgress(result, result.TotalGames, "", "done")

	return result, nil
}

// GetRunningProcesses 获取系统中正在运行的进程列表（过滤掉系统进程）
func (s *GameService) GetRunningProcesses() ([]processutils.ProcessInfo, error) {
	return processutils.GetRunningProcesses()
}

// OpenLocalPath 打开指定的本地文件或目录（通过资源管理器）
func (s *GameService) OpenLocalPath(path string) error {
	err := apputils.OpenFileOrFolder(path)
	if err != nil {
		applog.LogErrorf(s.ctx, "OpenLocalPath failed for path %s: %v", path, err)
		return fmt.Errorf("打开路径失败: %w", err)
	}
	return nil
}

// UpdateGameProcessName 更新游戏的进程名
// 当用户选择了实际的游戏进程时调用
func (s *GameService) UpdateGameProcessName(gameID string, processName string) error {
	result, err := s.db.ExecContext(
		s.ctx,
		`UPDATE games SET process_name = ? WHERE id = ?`,
		processName,
		gameID,
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "UpdateGameProcessName: failed to update process_name for game %s: %v", gameID, err)
		return fmt.Errorf("failed to update process_name: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("game not found with id: %s", gameID)
	}

	applog.LogInfof(s.ctx, "UpdateGameProcessName: updated process_name for game %s to %s", gameID, processName)
	return nil
}

// BatchUpdateStatus 批量更新多个游戏的游玩状态
func (s *GameService) BatchUpdateStatus(ids []string, status string) error {
	ids = utils.UniqueNonEmptyStrings(ids)
	if len(ids) == 0 {
		return nil
	}

	placeholders := utils.BuildPlaceholders(len(ids))
	// args: status + all ids
	args := make([]interface{}, 0, 1+len(ids))
	args = append(args, status)
	for _, id := range ids {
		args = append(args, id)
	}

	tx, err := s.db.Begin()
	if err != nil {
		applog.LogErrorf(s.ctx, "BatchUpdateStatus: failed to begin transaction: %v", err)
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(
		s.ctx,
		fmt.Sprintf("UPDATE games SET status = ? WHERE id IN (%s)", placeholders),
		args...,
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "BatchUpdateStatus: failed to update games status: %v", err)
		return fmt.Errorf("failed to batch update status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	applog.LogInfof(s.ctx, "BatchUpdateStatus: updated %d games to status %s", rowsAffected, status)

	if err := tx.Commit(); err != nil {
		return err
	}

	s.pushBangumiStatusAfterBatch(ids, enums2.GameStatus(status))
	return nil
}

func (s *GameService) getGameStatusSyncSnapshot(gameID string) (models.Game, error) {
	var snapshot models.Game
	var sourceType string
	var status string

	err := s.db.QueryRowContext(s.ctx, `
		SELECT id, name, COALESCE(status, 'not_started'), COALESCE(source_type, ''), COALESCE(source_id, '')
		FROM games
		WHERE id = ?
	`, gameID).Scan(
		&snapshot.ID,
		&snapshot.Name,
		&status,
		&sourceType,
		&snapshot.SourceID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Game{}, fmt.Errorf("game not found with id: %s", gameID)
	}
	if err != nil {
		return models.Game{}, fmt.Errorf("failed to load game status snapshot: %w", err)
	}

	snapshot.Status = enums2.GameStatus(status)
	snapshot.SourceType = enums2.SourceType(sourceType)
	return snapshot, nil
}

func (s *GameService) listGamesForBangumiStatusPush(ids []string) ([]models.Game, error) {
	ids = utils.UniqueNonEmptyStrings(ids)
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := utils.BuildPlaceholders(len(ids))
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	rows, err := s.db.QueryContext(s.ctx, fmt.Sprintf(`
		SELECT id, name, COALESCE(status, 'not_started'), COALESCE(source_type, ''), COALESCE(source_id, '')
		FROM games
		WHERE id IN (%s)
	`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list games for Bangumi status push: %w", err)
	}
	defer rows.Close()

	games := make([]models.Game, 0, len(ids))
	for rows.Next() {
		var game models.Game
		var sourceType string
		var status string
		if err := rows.Scan(&game.ID, &game.Name, &status, &sourceType, &game.SourceID); err != nil {
			return nil, fmt.Errorf("failed to scan Bangumi status push game: %w", err)
		}
		game.Status = enums2.GameStatus(status)
		game.SourceType = enums2.SourceType(sourceType)
		games = append(games, game)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate Bangumi status push games: %w", err)
	}

	return games, nil
}

func (s *GameService) pushBangumiStatusAfterLocalSave(previousGame models.Game, updatedGame models.Game) {
	if previousGame.Status == updatedGame.Status {
		return
	}
	if s.bangumiService == nil {
		return
	}
	if !s.bangumiService.isGameEligibleForStatusPush(updatedGame) {
		return
	}

	if err := s.bangumiService.syncGameStatus(s.ctx, updatedGame); err != nil {
		s.handleBangumiStatusPushFailure(updatedGame, err)
	}
}

func (s *GameService) pushBangumiStatusAfterBatch(ids []string, status enums2.GameStatus) {
	if s.bangumiService == nil {
		return
	}

	games, err := s.listGamesForBangumiStatusPush(ids)
	if err != nil {
		applog.LogWarningf(s.ctx, "pushBangumiStatusAfterBatch: failed to load games: %v", err)
		return
	}

	for _, game := range games {
		if !s.bangumiService.isGameEligibleForStatusPush(game) {
			continue
		}
		game.Status = status
		if err := s.bangumiService.syncGameStatus(s.ctx, game); err != nil {
			game.Status = status
			s.handleBangumiStatusPushFailure(game, err)
		}
	}
}

func (s *GameService) handleBangumiStatusPushFailure(game models.Game, err error) {
	applog.LogWarningf(
		s.ctx,
		"Bangumi status push failed for game %s (%s -> %s): %v",
		game.Name,
		game.SourceID,
		game.Status,
		err,
	)

	if s.ctx == nil {
		return
	}

	if s.ctx != nil && s.emitEvent != nil {
		s.emitEvent(s.ctx, "bangumi:status-push-failed", vo.BangumiStatusPushFailureEvent{
			GameID:      game.ID,
			GameName:    game.Name,
			SubjectID:   strings.TrimSpace(game.SourceID),
			LocalStatus: string(game.Status),
			Error:       err.Error(),
		})
	}
}

func (s *GameService) findGameIDBySource(source enums2.SourceType, sourceID string) (string, bool) {
	if s.db == nil || sourceID == "" {
		return "", false
	}
	var id string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT id FROM games
		WHERE source_type = ? AND source_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, string(source), sourceID).Scan(&id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			applog.LogWarningf(s.ctx, "findGameIDBySource query failed: %v", err)
		}
		return "", false
	}
	return id, true
}

func (s *GameService) findGameIDByPath(path string) (string, bool) {
	if s.db == nil || path == "" {
		return "", false
	}
	var id string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT id FROM games
		WHERE path = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, path).Scan(&id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			applog.LogWarningf(s.ctx, "findGameIDByPath query failed: %v", err)
		}
		return "", false
	}
	return id, true
}

func (s *GameService) getConfiguredMetadataSearchSources() []metadataSearchSource {
	vndbToken := ""
	language := ""
	getterOptions := metadataGetterOptions(s.config)
	if s.config != nil {
		vndbToken = s.config.VNDBAccessToken
		language = s.config.Language
	}

	sources := make([]metadataSearchSource, 0, 4)
	for _, source := range s.getConfiguredMetadataSources() {
		switch source {
		case enums2.Bangumi:
			if s.bangumiService == nil {
				continue
			}
			sources = append(sources, metadataSearchSource{
				source: enums2.Bangumi,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return s.bangumiService.fetchMetadataByName(s.ctx, name)
				},
			})
		case enums2.VNDB:
			sources = append(sources, metadataSearchSource{
				source: enums2.VNDB,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewVNDBInfoGetterWithLanguage(language, getterOptions...).FetchMetadataByName(name, vndbToken)
				},
			})
		case enums2.Ymgal:
			sources = append(sources, metadataSearchSource{
				source: enums2.Ymgal,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewYmgalInfoGetter(getterOptions...).FetchMetadataByName(name, "")
				},
			})
		case enums2.Steam:
			sources = append(sources, metadataSearchSource{
				source: enums2.Steam,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewSteamInfoGetterWithLanguage(language, getterOptions...).FetchMetadataByName(name, "")
				},
			})
		case enums2.DLsite:
			sources = append(sources, metadataSearchSource{
				source: enums2.DLsite,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewDLsiteInfoGetter(getterOptions...).FetchMetadataByName(name, "")
				},
			})
		case enums2.ErogameScape:
			sources = append(sources, metadataSearchSource{
				source: enums2.ErogameScape,
				fetchByName: func(name string) (metadata.MetadataResult, error) {
					return metadata.NewErogameScapeInfoGetter(getterOptions...).FetchMetadataByName(name, "")
				},
			})
		}
	}
	return sources
}

func resolveLaunchShortcutIconPath(gamePath string) string {
	trimmedPath := strings.TrimSpace(gamePath)
	if trimmedPath != "" {
		absPath, err := filepath.Abs(filepath.Clean(trimmedPath))
		if err == nil {
			if info, statErr := os.Stat(absPath); statErr == nil && !info.IsDir() && canUseShortcutIconSource(absPath) {
				return absPath
			}
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	absExePath, err := filepath.Abs(exePath)
	if err != nil {
		return exePath
	}
	return absExePath
}

func canUseShortcutIconSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".ico", ".dll":
		return true
	default:
		return false
	}
}

func (s *GameService) getConfiguredMetadataSources() []enums2.SourceType {
	return configuredMetadataSources(s.config)
}

func (s *GameService) getConfiguredMetadataSourceSet() map[enums2.SourceType]struct{} {
	sourceSet := make(map[enums2.SourceType]struct{})
	for _, source := range s.getConfiguredMetadataSources() {
		sourceSet[source] = struct{}{}
	}
	return sourceSet
}

func (s *GameService) isMetadataSourceEnabled(source enums2.SourceType) bool {
	source = normalizeMetadataSourceType(source)
	if source == "" || source == enums2.Local {
		return false
	}

	_, enabled := s.getConfiguredMetadataSourceSet()[source]
	return enabled
}

func normalizeMetadataSourceType(source enums2.SourceType) enums2.SourceType {
	return enums2.SourceType(strings.ToLower(strings.TrimSpace(string(source))))
}
