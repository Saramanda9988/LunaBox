package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/imageutils"
	"lunabox/internal/utils/metadata"
	"lunabox/internal/utils/processutils"
	"lunabox/internal/vo"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type GameService struct {
	ctx        context.Context
	db         *sql.DB
	config     *appconf.AppConfig
	tagService *TagService
}

type metadataSearchSource struct {
	getter metadata.Getter
	source enums.SourceType
	token  string
}

const metadataRefreshInterval = 300 * time.Millisecond

func NewGameService() *GameService {
	return &GameService{}
}

func (s *GameService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *GameService) SetTagService(ts *TagService) {
	s.tagService = ts
}

func (s *GameService) SelectGameExecutable() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "Select Game Executable",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Executables",
				Pattern:     "*.exe;*.bat;*.cmd;*.lnk",
			},
			{
				DisplayName: "All Files",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open file dialog: %v", err)
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

	info, err := os.Stat(trimmedPath)
	if err != nil {
		return "", fmt.Errorf("stat import path failed: %w", err)
	}

	if !info.IsDir() {
		return trimmedPath, nil
	}

	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title:            "选择游戏可执行文件",
		DefaultDirectory: trimmedPath,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Executables",
				Pattern:     "*.exe;*.bat;*.cmd;*.lnk",
			},
			{
				DisplayName: "All Files",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		applog.LogErrorf(s.ctx, "failed to open import executable dialog: %v", err)
		return "", err
	}

	return selection, nil
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
		source_type, cached_at, source_id, created_at,
		use_locale_emulator, use_magpie
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(s.ctx, query,
		game.ID,
		game.Name,
		game.CoverURL,
		game.Company,
		game.Summary,
		game.Rating,
		game.ReleaseDate,
		game.Path,
		string(game.SourceType),
		game.CachedAt,
		game.SourceID,
		game.CreatedAt,
		game.UseLocaleEmulator,
		game.UseMagpie,
	)
	if err != nil {
		applog.LogErrorf(s.ctx, "AddGame: failed to insert game %s: %v", game.Name, err)
		return err
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
		go s.asyncDownloadCoverImage(game.ID, game.Name, originalCoverURL)
	}

	return nil
}

func (s *GameService) syncScrapedTagsForGame(game models.Game) {
	if s.tagService == nil {
		return
	}
	if game.SourceType == enums.Local || game.SourceType == "" {
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
func (s *GameService) asyncDownloadCoverImage(gameID, gameName, coverURL string) {
	// 检查是否为远程URL
	if coverURL == "" || !strings.HasPrefix(coverURL, "http") || strings.Contains(coverURL, "wails.localhost") {
		return
	}

	applog.LogInfof(s.ctx, "asyncDownloadCoverImage: downloading cover for %s", gameName)

	// 下载并保存图片
	localPath, err := imageutils.DownloadAndSaveCoverImage(coverURL, gameID)
	if err != nil {
		applog.LogWarningf(s.ctx, "asyncDownloadCoverImage: failed to download cover for %s: %v", gameName, err)
		return
	}

	// 更新数据库中的封面路径
	if err := s.updateCoverURL(gameID, localPath); err != nil {
		applog.LogErrorf(s.ctx, "asyncDownloadCoverImage: failed to update cover URL for %s: %v", gameName, err)
		return
	}

	applog.LogInfof(s.ctx, "asyncDownloadCoverImage: successfully cached cover for %s", gameName)
}

// updateCoverURL 更新游戏的封面URL
func (s *GameService) updateCoverURL(gameID, coverURL string) error {
	query := `UPDATE games SET cover_url = ? WHERE id = ?`
	_, err := s.db.ExecContext(s.ctx, query, coverURL, gameID)
	return err
}

func (s *GameService) DeleteGame(id string) error {
	// 先删除关联的游戏分类记录
	_, err := s.db.ExecContext(s.ctx, "DELETE FROM game_categories WHERE game_id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete game_categories for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game categories: %w", err)
	}

	// 删除关联的游玩会话记录
	_, err = s.db.ExecContext(s.ctx, "DELETE FROM play_sessions WHERE game_id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete play_sessions for id %s: %v", id, err)
		return fmt.Errorf("failed to delete play sessions: %w", err)
	}
	// 删除游戏记录
	result, err := s.db.ExecContext(s.ctx, "DELETE FROM games WHERE id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to delete game for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGame: failed to get rows affected for id %s: %v", id, err)
		return err
	}

	if rowsAffected == 0 {
		applog.LogWarningf(s.ctx, "DeleteGame: game not found with id: %s", id)
		return fmt.Errorf("game not found with id: %s", id)
	}

	return nil
}

func (s *GameService) DeleteGames(ids []string) error {
	ids = utils.UniqueNonEmptyStrings(ids)
	if len(ids) == 0 {
		return nil
	}

	placeholders := utils.BuildPlaceholders(len(ids))
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	tx, err := s.db.Begin()
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to begin transaction: %v", err)
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(s.ctx, fmt.Sprintf("DELETE FROM game_categories WHERE game_id IN (%s)", placeholders), args...); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to delete game_categories: %v", err)
		return fmt.Errorf("failed to delete game categories: %w", err)
	}

	if _, err := tx.ExecContext(s.ctx, fmt.Sprintf("DELETE FROM play_sessions WHERE game_id IN (%s)", placeholders), args...); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to delete play_sessions: %v", err)
		return fmt.Errorf("failed to delete play sessions: %w", err)
	}

	result, err := tx.ExecContext(s.ctx, fmt.Sprintf("DELETE FROM games WHERE id IN (%s)", placeholders), args...)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to delete games: %v", err)
		return fmt.Errorf("failed to delete games: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to get rows affected: %v", err)
		return err
	}
	if rowsAffected == 0 {
		applog.LogWarningf(s.ctx, "DeleteGames: no games deleted")
		return fmt.Errorf("no games deleted")
	}

	if err := tx.Commit(); err != nil {
		applog.LogErrorf(s.ctx, "DeleteGames: failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func (s *GameService) GetGames() ([]models.Game, error) {
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
		COALESCE(g.status, 'not_started') as status,
		COALESCE(g.source_type, '') as source_type, 
		g.cached_at, 
		COALESCE(g.source_id, '') as source_id, 
		g.created_at,
		latest.last_played_at,
		COALESCE(g.use_locale_emulator, FALSE) as use_locale_emulator,
		COALESCE(g.use_magpie, FALSE) as use_magpie
	FROM games g
	LEFT JOIN (
		SELECT game_id, MAX(start_time) as last_played_at
		FROM play_sessions
		GROUP BY game_id
	) latest ON latest.game_id = g.id
	ORDER BY g.created_at DESC`

	rows, err := s.db.QueryContext(s.ctx, query)
	if err != nil {
		applog.LogErrorf(s.ctx, "GetGames: failed to query games: %v", err)
		return nil, fmt.Errorf("failed to query games: %w", err)
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var game models.Game
		var sourceType string
		var status string
		var lastPlayedAt sql.NullTime

		err := rows.Scan(
			&game.ID,
			&game.Name,
			&game.CoverURL,
			&game.Company,
			&game.Summary,
			&game.Rating,
			&game.ReleaseDate,
			&game.Path,
			&game.SavePath,
			&status,
			&sourceType,
			&game.CachedAt,
			&game.SourceID,
			&game.CreatedAt,
			&lastPlayedAt,
			&game.UseLocaleEmulator,
			&game.UseMagpie,
		)
		if err != nil {
			applog.LogErrorf(s.ctx, "GetGames: failed to scan game row: %v", err)
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}

		game.SourceType = enums.SourceType(sourceType)
		game.Status = enums.GameStatus(status)
		if lastPlayedAt.Valid {
			lastPlayed := lastPlayedAt.Time
			game.LastPlayedAt = &lastPlayed
		}
		games = append(games, game)
	}

	if err = rows.Err(); err != nil {
		applog.LogErrorf(s.ctx, "GetGames: error iterating games: %v", err)
		return nil, fmt.Errorf("error iterating games: %w", err)
	}

	return games, nil
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
		COALESCE(g.status, 'not_started') as status,
		COALESCE(g.source_type, '') as source_type, 
		g.cached_at, 
		COALESCE(g.source_id, '') as source_id, 
		g.created_at,
		latest.last_played_at,
		COALESCE(g.use_locale_emulator, FALSE) as use_locale_emulator,
		COALESCE(g.use_magpie, FALSE) as use_magpie
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
		&status,
		&sourceType,
		&game.CachedAt,
		&game.SourceID,
		&game.CreatedAt,
		&lastPlayedAt,
		&game.UseLocaleEmulator,
		&game.UseMagpie,
	)

	if errors.Is(err, sql.ErrNoRows) {
		applog.LogWarningf(s.ctx, "GetGameByID: game not found with id: %s", id)
		return models.Game{}, fmt.Errorf("game not found with id: %s", id)
	}
	if err != nil {
		applog.LogErrorf(s.ctx, "GetGameByID: failed to query game %s: %v", id, err)
		return models.Game{}, fmt.Errorf("failed to query game: %w", err)
	}

	game.SourceType = enums.SourceType(sourceType)
	game.Status = enums.GameStatus(status)
	if lastPlayedAt.Valid {
		lastPlayed := lastPlayedAt.Time
		game.LastPlayedAt = &lastPlayed
	}
	return game, nil
}

func (s *GameService) UpdateGame(game models.Game) error {
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
		status = ?,
		source_type = ?,
		cached_at = ?,
		source_id = ?,
		use_locale_emulator = ?,
		use_magpie = ?
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
		string(game.Status),
		string(game.SourceType),
		game.CachedAt,
		game.SourceID,
		game.UseLocaleEmulator,
		game.UseMagpie,
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

	return coverPath, nil
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
			result, _ := src.getter.FetchMetadataByName(name, src.token)
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
	case enums.Bangumi:
		return s.fetchMetadataResultBySource(req.Source, strings.ToLower(sourceID))
	case enums.VNDB:
		if !isVndbId(strings.ToLower(sourceID)) {
			return metadata.MetadataResult{}, fmt.Errorf("invalid VNDB ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, strings.ToLower(sourceID))
	case enums.Ymgal:
		if !isYmgalId(strings.ToLower(sourceID)) {
			return metadata.MetadataResult{}, fmt.Errorf("invalid Ymgal ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, strings.ToLower(sourceID))
	case enums.Steam:
		if !isSteamAppID(sourceID) {
			return metadata.MetadataResult{}, fmt.Errorf("invalid Steam app ID format: %s", req.ID)
		}
		return s.fetchMetadataResultBySource(req.Source, sourceID)
	default:
		return metadata.MetadataResult{}, fmt.Errorf("unsupported source type: %s", req.Source)
	}
}

func (s *GameService) fetchMetadataResultBySource(source enums.SourceType, sourceID string) (metadata.MetadataResult, error) {
	switch source {
	case enums.Bangumi:
		getter := metadata.NewBangumiInfoGetter()
		return getter.FetchMetadata(sourceID, s.config.BangumiAccessToken)
	case enums.VNDB:
		getter := metadata.NewVNDBInfoGetterWithLanguage(s.config.Language)
		return getter.FetchMetadata(sourceID, s.config.VNDBAccessToken)
	case enums.Ymgal:
		getter := metadata.NewYmgalInfoGetter()
		return getter.FetchMetadata(sourceID, "")
	case enums.Steam:
		getter := metadata.NewSteamInfoGetterWithLanguage(s.config.Language)
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
	// 获取现有游戏信息
	existingGame, err := s.GetGameByID(gameID)
	if err != nil {
		return fmt.Errorf("failed to get game: %w", err)
	}

	if existingGame.SourceType == "" || existingGame.SourceID == "" {
		return fmt.Errorf("游戏缺少数据源信息，无法从远程更新")
	}

	sourceId := strings.ToLower(existingGame.SourceID)
	metaResult, err := s.fetchMetadataResultBySource(existingGame.SourceType, sourceId)
	if err != nil {
		return fmt.Errorf("failed to fetch metadata from remote: %w", err)
	}

	remoteGame := metaResult.Game

	// 保留本地重要字段，更新远程可获取的字段
	existingGame.Name = remoteGame.Name
	existingGame.Company = remoteGame.Company
	existingGame.Summary = remoteGame.Summary
	existingGame.Rating = remoteGame.Rating
	existingGame.ReleaseDate = remoteGame.ReleaseDate
	existingGame.CachedAt = time.Now()

	existingGame.CoverURL = remoteGame.CoverURL
	if remoteGame.CoverURL != "" {
		go s.asyncDownloadCoverImage(existingGame.ID, existingGame.Name, remoteGame.CoverURL)
	}

	if err := s.UpdateGame(existingGame); err != nil {
		return fmt.Errorf("failed to update game: %w", err)
	}

	// 写入 tags（先删除刮削来源的旧 tag，再批量插入新 tag，保留用户 tag）
	if s.tagService != nil && len(metaResult.Tags) > 0 {
		if err := s.tagService.upsertScrapedTags(gameID, metaResult.Tags); err != nil {
			applog.LogWarningf(s.ctx, "UpdateGameFromRemote: failed to upsert tags for game %s: %v", gameID, err)
		}
	}

	applog.LogInfof(s.ctx, "UpdateGameFromRemote: successfully updated game %s from %s", existingGame.Name, existingGame.SourceType)
	return nil
}

func (s *GameService) RefreshAllGamesMetadata() (vo.MetadataRefreshResult, error) {
	result := vo.MetadataRefreshResult{}

	games, err := s.GetGames()
	if err != nil {
		return result, fmt.Errorf("failed to get games: %w", err)
	}

	result.TotalGames = len(games)
	enabledSources := s.getConfiguredMetadataSourceSet()

	for _, game := range games {
		if game.SourceType == "" || game.SourceType == enums.Local || strings.TrimSpace(game.SourceID) == "" {
			result.SkippedGames++
			continue
		}

		if _, enabled := enabledSources[game.SourceType]; !enabled {
			result.SkippedGames++
			continue
		}

		if err := s.UpdateGameFromRemote(game.ID); err != nil {
			result.FailedGames++
			applog.LogWarningf(s.ctx, "RefreshAllGamesMetadata: failed to update game %s (%s): %v", game.Name, game.ID, err)
		} else {
			result.UpdatedGames++
		}

		// FIXME:哪天抽出专门的metadata_service来，这里和import_service中的方法有点重复了
		time.Sleep(metadataRefreshInterval)
	}

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

	return tx.Commit()
}

func (s *GameService) findGameIDBySource(source enums.SourceType, sourceID string) (string, bool) {
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
	bangumiToken := ""
	vndbToken := ""
	language := ""
	if s.config != nil {
		bangumiToken = s.config.BangumiAccessToken
		vndbToken = s.config.VNDBAccessToken
		language = s.config.Language
	}

	sources := make([]metadataSearchSource, 0, 4)
	for _, source := range s.getConfiguredMetadataSources() {
		switch source {
		case enums.Bangumi:
			sources = append(sources, metadataSearchSource{
				getter: metadata.NewBangumiInfoGetter(),
				source: enums.Bangumi,
				token:  bangumiToken,
			})
		case enums.VNDB:
			sources = append(sources, metadataSearchSource{
				getter: metadata.NewVNDBInfoGetterWithLanguage(language),
				source: enums.VNDB,
				token:  vndbToken,
			})
		case enums.Ymgal:
			sources = append(sources, metadataSearchSource{
				getter: metadata.NewYmgalInfoGetter(),
				source: enums.Ymgal,
				token:  "",
			})
		case enums.Steam:
			sources = append(sources, metadataSearchSource{
				getter: metadata.NewSteamInfoGetterWithLanguage(language),
				source: enums.Steam,
				token:  "",
			})
		}
	}
	return sources
}

func (s *GameService) getConfiguredMetadataSources() []enums.SourceType {
	defaultSources := []enums.SourceType{enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam}
	if s.config == nil || len(s.config.MetadataSources) == 0 {
		return defaultSources
	}

	result := make([]enums.SourceType, 0, len(defaultSources))
	seen := make(map[enums.SourceType]struct{}, len(defaultSources))
	for _, source := range s.config.MetadataSources {
		normalized := enums.SourceType(strings.ToLower(strings.TrimSpace(source)))
		switch normalized {
		case enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam:
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

func (s *GameService) getConfiguredMetadataSourceSet() map[enums.SourceType]struct{} {
	sourceSet := make(map[enums.SourceType]struct{})
	for _, source := range s.getConfiguredMetadataSources() {
		sourceSet[source] = struct{}{}
	}
	return sourceSet
}
