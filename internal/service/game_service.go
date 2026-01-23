package service

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"log/slog"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"lunabox/internal/vo"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type GameService struct {
	app    *application.App
	db     *sql.DB
	logger *slog.Logger
	config *appconf.AppConfig
}

func NewGameService(db *sql.DB, config *appconf.AppConfig, logger *slog.Logger) *GameService {
	return &GameService{
		db:     db,
		logger: logger,
		config: config,
	}
}

func (s *GameService) SelectGameExecutable() (string, error) {
	app := application.Get()
	selection, err := app.Dialog.OpenFile().
		SetTitle("选择游戏可执行文件").
		AddFilter("Executables", "*.exe;*.bat;*.cmd;*.lnk").
		AddFilter("All Files", "*.*").
		PromptForSingleSelection()
	if err != nil {
		s.logger.Error("failed to open file dialog: %v", err)
		return "", err
	}
	return selection, err
}

func (s *GameService) AddGame(game models.Game) error {
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
		newCoverURL, err := utils.RenameTempCover(game.CoverURL, game.ID)
		if err != nil {
			s.logger.Warn("AddGame: failed to rename temp cover: %v", err)
		} else {
			game.CoverURL = newCoverURL
			originalCoverURL = ""
		}
	}

	query := `INSERT INTO games (
		id, name, cover_url, company, summary, path, 
		source_type, cached_at, source_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query,
		game.ID,
		game.Name,
		game.CoverURL,
		game.Company,
		game.Summary,
		game.Path,
		string(game.SourceType),
		game.CachedAt,
		game.SourceID,
		game.CreatedAt,
	)
	if err != nil {
		s.logger.Error("AddGame: failed to insert game %s: %v", game.Name, err)
		return err
	}

	// 后台异步下载封面图片（不阻塞添加流程）
	if originalCoverURL != "" {
		go s.asyncDownloadCoverImage(game.ID, game.Name, originalCoverURL)
	}

	return nil
}

// asyncDownloadCoverImage 后台异步下载封面图片并更新数据库
func (s *GameService) asyncDownloadCoverImage(gameID, gameName, coverURL string) {
	// 检查是否为远程URL
	if coverURL == "" || !strings.HasPrefix(coverURL, "http") || strings.Contains(coverURL, "wails.localhost") {
		return
	}

	s.logger.Info("asyncDownloadCoverImage: downloading cover for %s", gameName)

	// 下载并保存图片
	localPath, err := utils.DownloadAndSaveCoverImage(coverURL, gameID)
	if err != nil {
		s.logger.Warn("asyncDownloadCoverImage: failed to download cover for %s: %v", gameName, err)
		return
	}

	// 更新数据库中的封面路径
	if err := s.updateCoverURL(gameID, localPath); err != nil {
		s.logger.Error("asyncDownloadCoverImage: failed to update cover URL for %s: %v", gameName, err)
		return
	}

	s.logger.Info("asyncDownloadCoverImage: successfully cached cover for %s", gameName)
}

// updateCoverURL 更新游戏的封面URL
func (s *GameService) updateCoverURL(gameID, coverURL string) error {
	query := `UPDATE games SET cover_url = ? WHERE id = ?`
	_, err := s.db.Exec(query, coverURL, gameID)
	return err
}

func (s *GameService) DeleteGame(id string) error {
	// 先删除关联的游戏分类记录
	_, err := s.db.Exec("DELETE FROM game_categories WHERE game_id = ?", id)
	if err != nil {
		s.logger.Error("DeleteGame: failed to delete game_categories for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game categories: %w", err)
	}

	// 删除关联的游玩会话记录
	_, err = s.db.Exec("DELETE FROM play_sessions WHERE game_id = ?", id)
	if err != nil {
		s.logger.Error("DeleteGame: failed to delete play_sessions for id %s: %v", id, err)
		return fmt.Errorf("failed to delete play sessions: %w", err)
	}
	// 删除游戏记录
	result, err := s.db.Exec("DELETE FROM games WHERE id = ?", id)
	if err != nil {
		s.logger.Error("DeleteGame: failed to delete game for id %s: %v", id, err)
		return fmt.Errorf("failed to delete game: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.Error("DeleteGame: failed to get rows affected for id %s: %v", id, err)
		return err
	}

	if rowsAffected == 0 {
		s.logger.Warn("DeleteGame: game not found with id: %s", id)
		return fmt.Errorf("game not found with id: %s", id)
	}

	return nil
}

func (s *GameService) GetGames() ([]models.Game, error) {
	query := `SELECT 
		id, name, 
		COALESCE(cover_url, '') as cover_url, 
		COALESCE(company, '') as company, 
		COALESCE(summary, '') as summary, 
		COALESCE(path, '') as path, 
		COALESCE(save_path, '') as save_path,
		COALESCE(status, 'not_started') as status,
		COALESCE(source_type, '') as source_type, 
		cached_at, 
		COALESCE(source_id, '') as source_id, 
		created_at 
	FROM games 
	ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		s.logger.Error("GetGames: failed to query games: %v", err)
		return nil, fmt.Errorf("failed to query games: %w", err)
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var game models.Game
		var sourceType string
		var status string

		err := rows.Scan(
			&game.ID,
			&game.Name,
			&game.CoverURL,
			&game.Company,
			&game.Summary,
			&game.Path,
			&game.SavePath,
			&status,
			&sourceType,
			&game.CachedAt,
			&game.SourceID,
			&game.CreatedAt,
		)
		if err != nil {
			s.logger.Error("GetGames: failed to scan game row: %v", err)
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}

		game.SourceType = enums.SourceType(sourceType)
		game.Status = enums.GameStatus(status)
		games = append(games, game)
	}

	if err = rows.Err(); err != nil {
		s.logger.Error("GetGames: error iterating games: %v", err)
		return nil, fmt.Errorf("error iterating games: %w", err)
	}

	return games, nil
}

func (s *GameService) GetGameByID(id string) (models.Game, error) {
	query := `SELECT 
		id, name, 
		COALESCE(cover_url, '') as cover_url, 
		COALESCE(company, '') as company, 
		COALESCE(summary, '') as summary, 
		COALESCE(path, '') as path, 
		COALESCE(save_path, '') as save_path,
		COALESCE(status, 'not_started') as status,
		COALESCE(source_type, '') as source_type, 
		cached_at, 
		COALESCE(source_id, '') as source_id, 
		created_at 
	FROM games 
	WHERE id = ?`

	var game models.Game
	var sourceType string
	var status string

	err := s.db.QueryRow(query, id).Scan(
		&game.ID,
		&game.Name,
		&game.CoverURL,
		&game.Company,
		&game.Summary,
		&game.Path,
		&game.SavePath,
		&status,
		&sourceType,
		&game.CachedAt,
		&game.SourceID,
		&game.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		s.logger.Warn("GetGameByID: game not found with id: %s", id)
		return models.Game{}, fmt.Errorf("game not found with id: %s", id)
	}
	if err != nil {
		s.logger.Error("GetGameByID: failed to query game %s: %v", id, err)
		return models.Game{}, fmt.Errorf("failed to query game: %w", err)
	}

	game.SourceType = enums.SourceType(sourceType)
	game.Status = enums.GameStatus(status)
	return game, nil
}

func (s *GameService) UpdateGame(game models.Game) error {
	query := `UPDATE games SET 
		name = ?,
		cover_url = ?,
		company = ?,
		summary = ?,
		path = ?,
		save_path = ?,
		status = ?,
		source_type = ?,
		cached_at = ?,
		source_id = ?
	WHERE id = ?`

	result, err := s.db.Exec(query,
		game.Name,
		game.CoverURL,
		game.Company,
		game.Summary,
		game.Path,
		game.SavePath,
		string(game.Status),
		string(game.SourceType),
		game.CachedAt,
		game.SourceID,
		game.ID,
	)

	if err != nil {
		s.logger.Error("UpdateGame: failed to update game %s: %v", game.ID, err)
		return fmt.Errorf("failed to update game: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.logger.Error("UpdateGame: failed to get rows affected for id %s: %v", game.ID, err)
		return err
	}

	if rowsAffected == 0 {
		s.logger.Warn("UpdateGame: game not found with id: %s", game.ID)
		return fmt.Errorf("game not found with id: %s", game.ID)
	}

	return nil
}

// SelectSaveDirectory 选择存档目录
func (s *GameService) SelectSaveDirectory() (string, error) {
	app := application.Get()
	selection, err := app.Dialog.OpenFile().
		SetTitle("选择存档目录").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	return selection, err
}

// SelectCoverImage 选择封面图片并保存到 covers 目录
func (s *GameService) SelectCoverImage(gameID string) (string, error) {
	app := application.Get()
	selection, err := app.Dialog.OpenFile().
		SetTitle("选择封面图片").
		AddFilter("图片文件", "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp").
		AddFilter("All Files", "*.*").
		PromptForSingleSelection()
	if err != nil {
		s.logger.Error("failed to open file dialog: %v", err)
		return "", err
	}
	if selection == "" {
		return "", nil
	}

	coverPath, err := utils.SaveCoverImage(selection, gameID)
	if err != nil {
		s.logger.Error("failed to save cover image: %v", err)
		return "", fmt.Errorf("failed to save cover image: %w", err)
	}

	return coverPath, nil
}

// SelectCoverImageWithTempID 选择封面图片并使用临时ID保存（用于新增游戏时）
func (s *GameService) SelectCoverImageWithTempID() (string, error) {
	app := application.Get()
	selection, err := app.Dialog.OpenFile().
		SetTitle("选择封面图片").
		AddFilter("图片文件", "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.bmp").
		AddFilter("All Files", "*.*").
		PromptForSingleSelection()
	if err != nil {
		s.logger.Error("failed to open file dialog: %v", err)
		return "", err
	}
	if selection == "" {
		return "", nil
	}

	// 使用时间戳作为临时ID
	tempID := fmt.Sprintf("temp_%d", time.Now().UnixNano())
	coverPath, err := utils.SaveCoverImage(selection, tempID)
	if err != nil {
		s.logger.Error("failed to save cover image: %v", err)
		return "", fmt.Errorf("failed to save cover image: %w", err)
	}

	return coverPath, nil
}

func (s *GameService) FetchMetadataByName(name string) ([]vo.GameMetadataFromWebVO, error) {
	var games []vo.GameMetadataFromWebVO
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 这里暂不处理任何错误，直接尝试从多个来源并发获取数据，空就是网络问题或未找到，不管它
	wg.Add(3)

	go func() {
		defer wg.Done()
		bgmGetter := utils.NewBangumiInfoGetter()
		bgm, _ := bgmGetter.FetchMetadataByName(name, s.config.BangumiAccessToken)
		if bgm != (models.Game{}) {
			mu.Lock()
			games = append(games, vo.GameMetadataFromWebVO{Source: enums.Bangumi, Game: bgm})
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		vndbGetter := utils.NewVNDBInfoGetter()
		vndb, _ := vndbGetter.FetchMetadataByName(name, s.config.VNDBAccessToken)
		if vndb != (models.Game{}) {
			mu.Lock()
			games = append(games, vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: vndb})
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		ymgalGetter := utils.NewYmgalInfoGetter()
		ymgal, _ := ymgalGetter.FetchMetadataByName(name, "")
		if ymgal != (models.Game{}) {
			mu.Lock()
			games = append(games, vo.GameMetadataFromWebVO{Source: enums.Ymgal, Game: ymgal})
			mu.Unlock()
		}
	}()

	wg.Wait()

	return games, nil
}

func (s *GameService) FetchMetadata(req vo.MetadataRequest) (models.Game, error) {
	var game = models.Game{}
	var e error

	if game, e = fetchFromLocal(req.ID); e == nil {
		return game, nil
	}

	switch req.Source {
	case enums.Bangumi:
		bgmGetter := utils.NewBangumiInfoGetter()
		game, e = bgmGetter.FetchMetadata(req.ID, s.config.BangumiAccessToken)
	case enums.VNDB:
		vndbGetter := utils.NewVNDBInfoGetter()
		game, e = vndbGetter.FetchMetadata(req.ID, s.config.VNDBAccessToken)
	case enums.Ymgal:
		ymgalGetter := utils.NewYmgalInfoGetter()
		game, e = ymgalGetter.FetchMetadata(req.ID, "")
	}
	return game, e
}

func fetchFromLocal(id string) (models.Game, error) {
	// 这个函数暂时返回错误，表示未实现从本地数据库获取
	// 如果需要实现，应该在这里查询数据库
	return models.Game{}, fmt.Errorf("game not found in local cache")
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

	// 从远程获取最新数据
	req := vo.MetadataRequest{
		Source: existingGame.SourceType,
		ID:     existingGame.SourceID,
	}

	remoteGame, err := s.FetchMetadata(req)
	if err != nil {
		return fmt.Errorf("failed to fetch metadata from remote: %w", err)
	}

	// 保留本地重要字段，更新远程可获取的字段
	existingGame.Name = remoteGame.Name
	existingGame.Company = remoteGame.Company
	existingGame.Summary = remoteGame.Summary
	existingGame.CachedAt = time.Now()

	existingGame.CoverURL = remoteGame.CoverURL
	if remoteGame.CoverURL != "" {
		go s.asyncDownloadCoverImage(existingGame.ID, existingGame.Name, remoteGame.CoverURL)
	}

	if err := s.UpdateGame(existingGame); err != nil {
		return fmt.Errorf("failed to update game: %w", err)
	}

	s.logger.Info("UpdateGameFromRemote: successfully updated game %s from %s", existingGame.Name, existingGame.SourceType)
	return nil
}
