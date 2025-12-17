package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/utils"
	"lunabox/internal/vo"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type GameService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewGameService() *GameService {
	return &GameService{}
}

func (s *GameService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
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

	query := `INSERT INTO games (
		id, user_id, name, cover_url, company, summary, path, 
		source_type, cached_at, source_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(s.ctx, query,
		game.ID,
		game.UserID,
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

	return err
}

func (s *GameService) DeleteGame(id string) error {
	// 先删除关联的游戏分类记录
	_, err := s.db.ExecContext(s.ctx, "DELETE FROM game_categories WHERE game_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete game categories: %w", err)
	}

	// 删除游戏记录
	result, err := s.db.ExecContext(s.ctx, "DELETE FROM games WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game not found with id: %s", id)
	}

	return nil
}

func (s *GameService) GetGames() ([]models.Game, error) {
	query := `SELECT 
		id, user_id, name, cover_url, company, summary, path, 
		COALESCE(save_path, '') as save_path,
		source_type, cached_at, source_id, created_at 
	FROM games 
	ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(s.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query games: %w", err)
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var game models.Game
		var sourceType string

		err := rows.Scan(
			&game.ID,
			&game.UserID,
			&game.Name,
			&game.CoverURL,
			&game.Company,
			&game.Summary,
			&game.Path,
			&game.SavePath,
			&sourceType,
			&game.CachedAt,
			&game.SourceID,
			&game.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}

		game.SourceType = enums.SourceType(sourceType)
		games = append(games, game)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating games: %w", err)
	}

	return games, nil
}

func (s *GameService) GetGameByID(id string) (models.Game, error) {
	query := `SELECT 
		id, user_id, name, cover_url, company, summary, path, 
		COALESCE(save_path, '') as save_path,
		source_type, cached_at, source_id, created_at 
	FROM games 
	WHERE id = ?`

	var game models.Game
	var sourceType string

	err := s.db.QueryRowContext(s.ctx, query, id).Scan(
		&game.ID,
		&game.UserID,
		&game.Name,
		&game.CoverURL,
		&game.Company,
		&game.Summary,
		&game.Path,
		&game.SavePath,
		&sourceType,
		&game.CachedAt,
		&game.SourceID,
		&game.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return models.Game{}, fmt.Errorf("game not found with id: %s", id)
	}
	if err != nil {
		return models.Game{}, fmt.Errorf("failed to query game: %w", err)
	}

	game.SourceType = enums.SourceType(sourceType)
	return game, nil
}

func (s *GameService) UpdateGame(game models.Game) error {
	query := `UPDATE games SET 
		user_id = ?,
		name = ?,
		cover_url = ?,
		company = ?,
		summary = ?,
		path = ?,
		save_path = ?,
		source_type = ?,
		cached_at = ?,
		source_id = ?
	WHERE id = ?`

	result, err := s.db.ExecContext(s.ctx, query,
		game.UserID,
		game.Name,
		game.CoverURL,
		game.Company,
		game.Summary,
		game.Path,
		game.SavePath,
		string(game.SourceType),
		game.CachedAt,
		game.SourceID,
		game.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update game: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("game not found with id: %s", game.ID)
	}

	return nil
}

// SelectSaveDirectory 选择存档目录
func (s *GameService) SelectSaveDirectory() (string, error) {
	selection, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择存档目录",
	})
	return selection, err
}

func (s *GameService) FetchMetadataByName(name string) ([]vo.GameMetadataFromWebVO, error) {
	var games []vo.GameMetadataFromWebVO

	// 这里暂不处理任何错误，直接尝试从两个来源获取数据，空就是网络问题或未找到，不管它
	bgmGetter := utils.NewBangumiInfoGetter()
	bgm, _ := bgmGetter.FetchMetadataByName(name, s.config.BangumiAccessToken)
	if bgm != (models.Game{}) {
		games = append(games, vo.GameMetadataFromWebVO{Source: enums.Bangumi, Game: bgm})
	}

	vndbGetter := utils.NewVNDBInfoGetter()
	vndb, _ := vndbGetter.FetchMetadataByName(name, s.config.VNDBAccessToken)
	if vndb != (models.Game{}) {
		games = append(games, vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: vndb})
	}

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
	}
	return game, e
}

func fetchFromLocal(id string) (models.Game, error) {
	// 这个函数暂时返回错误，表示未实现从本地数据库获取
	// 如果需要实现，应该在这里查询数据库
	return models.Game{}, fmt.Errorf("game not found in local cache")
}
