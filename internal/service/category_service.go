package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/models"
	"lunabox/internal/vo"
	"time"

	"github.com/google/uuid"
)

type CategoryService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewCategoryService() *CategoryService {
	return &CategoryService{}
}

func (s *CategoryService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
	s.ensureSystemCategories()
}

func (s *CategoryService) ensureSystemCategories() {
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM categories WHERE is_system = true AND name = ?", "最喜欢的游戏").Scan(&count)
	if err != nil {
		applog.LogErrorf(s.ctx, "Error checking system category: %v", err)
		return
	}

	if count == 0 {
		id := uuid.New().String()
		now := time.Now()
		_, err := s.db.Exec(`
			INSERT INTO categories (id, name, is_system, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, id, "最喜欢的游戏", true, now, now)
		if err != nil {
			applog.LogErrorf(s.ctx, "Error creating system category: %v", err)
		}
	}
}

func (s *CategoryService) GetCategories() ([]vo.CategoryVO, error) {
	query := `
		SELECT c.id, c.name, c.is_system, c.created_at, c.updated_at, COUNT(gc.game_id) as game_count
		FROM categories c
		LEFT JOIN game_categories gc ON c.id = gc.category_id
		GROUP BY c.id, c.name, c.is_system, c.created_at, c.updated_at
		ORDER BY c.created_at
	`
	rows, err := s.db.Query(query)
	if err != nil {
		applog.LogErrorf(s.ctx, "GetCategories: failed to query categories: %v", err)
		return nil, err
	}
	defer rows.Close()

	var categories []vo.CategoryVO
	for rows.Next() {
		var c vo.CategoryVO
		if err := rows.Scan(&c.ID, &c.Name, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt, &c.GameCount); err != nil {
			applog.LogErrorf(s.ctx, "GetCategories: failed to scan row: %v", err)
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (s *CategoryService) GetCategoryByID(id string) (vo.CategoryVO, error) {
	var c vo.CategoryVO
	query := `
		SELECT c.id, c.name, c.is_system, c.created_at, c.updated_at, COUNT(gc.game_id) as game_count
		FROM categories c
		LEFT JOIN game_categories gc ON c.id = gc.category_id
		WHERE c.id = ?
		GROUP BY c.id, c.name, c.is_system, c.created_at, c.updated_at
	`
	err := s.db.QueryRow(query, id).Scan(&c.ID, &c.Name, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt, &c.GameCount)
	if err != nil {
		if err == sql.ErrNoRows {
			applog.LogWarningf(s.ctx, "GetCategoryByID: category not found with id: %s", id)
		} else {
			applog.LogErrorf(s.ctx, "GetCategoryByID: failed to query category %s: %v", id, err)
		}
		return c, err
	}
	return c, nil
}

func (s *CategoryService) AddCategory(name string) error {
	id := uuid.New().String()
	now := time.Now()
	_, err := s.db.Exec(`
		       INSERT INTO categories (id, name, is_system, created_at, updated_at)
		       VALUES (?, ?, ?, ?, ?)
	       `, id, name, false, now, now)
	if err != nil {
		applog.LogErrorf(s.ctx, "AddCategory: failed to insert category %s: %v", name, err)
	}
	return err
}

func (s *CategoryService) AddGameToCategory(gameID, categoryID string) error {
	_, err := s.db.Exec("INSERT INTO game_categories (game_id, category_id) VALUES (?, ?)", gameID, categoryID)
	if err != nil {
		applog.LogErrorf(s.ctx, "AddGameToCategory: failed to add game %s to category %s: %v", gameID, categoryID, err)
	}
	return err
}

func (s *CategoryService) RemoveGameFromCategory(gameID, categoryID string) error {
	_, err := s.db.Exec("DELETE FROM game_categories WHERE game_id = ? AND category_id = ?", gameID, categoryID)
	if err != nil {
		applog.LogErrorf(s.ctx, "RemoveGameFromCategory: failed to remove game %s from category %s: %v", gameID, categoryID, err)
	}
	return err
}

func (s *CategoryService) DeleteCategory(id string) error {
	var isSystem bool
	err := s.db.QueryRow("SELECT is_system FROM categories WHERE id = ?", id).Scan(&isSystem)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteCategory: failed to query is_system for id %s: %v", id, err)
		return err
	}
	if isSystem {
		applog.LogWarningf(s.ctx, "DeleteCategory: attempt to delete system category %s", id)
		return fmt.Errorf("cannot delete system category")
	}

	tx, err := s.db.Begin()
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteCategory: failed to begin transaction for id %s: %v", id, err)
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM game_categories WHERE category_id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteCategory: failed to delete game_categories for id %s: %v", id, err)
		return err
	}

	_, err = tx.Exec("DELETE FROM categories WHERE id = ?", id)
	if err != nil {
		applog.LogErrorf(s.ctx, "DeleteCategory: failed to delete category for id %s: %v", id, err)
		return err
	}

	if err := tx.Commit(); err != nil {
		applog.LogErrorf(s.ctx, "DeleteCategory: failed to commit transaction for id %s: %v", id, err)
		return err
	}
	return nil
}

func (s *CategoryService) GetGamesByCategory(categoryID string) ([]models.Game, error) {
	query := `
		SELECT g.id, g.name,
			COALESCE(g.cover_url, '') as cover_url,
			COALESCE(g.company, '') as company,
			COALESCE(g.summary, '') as summary,
			COALESCE(g.path, '') as path,
			COALESCE(g.source_type, '') as source_type,
			g.cached_at,
			COALESCE(g.source_id, '') as source_id,
			g.created_at
		FROM games g
		JOIN game_categories gc ON g.id = gc.game_id
		WHERE gc.category_id = ?
		ORDER BY g.created_at DESC
	`
	rows, err := s.db.Query(query, categoryID)
	if err != nil {
		applog.LogErrorf(s.ctx, "GetGamesByCategory: failed to query games for category %s: %v", categoryID, err)
		return nil, err
	}
	defer rows.Close()

	var games []models.Game
	for rows.Next() {
		var g models.Game
		if err := rows.Scan(&g.ID, &g.Name, &g.CoverURL, &g.Company, &g.Summary, &g.Path, &g.SourceType, &g.CachedAt, &g.SourceID, &g.CreatedAt); err != nil {
			applog.LogErrorf(s.ctx, "GetGamesByCategory: failed to scan row for category %s: %v", categoryID, err)
			return nil, err
		}
		games = append(games, g)
	}
	return games, nil
}

func (s *CategoryService) GetCategoriesByGame(gameID string) ([]vo.CategoryVO, error) {
	query := `
		SELECT c.id, c.name, c.is_system, c.created_at, c.updated_at, COUNT(gc.game_id) as game_count
		FROM categories c
		INNER JOIN game_categories gc ON c.id = gc.category_id
		WHERE gc.game_id = ?
		GROUP BY c.id, c.name, c.is_system, c.created_at, c.updated_at
		ORDER BY c.created_at
	`
	rows, err := s.db.Query(query, gameID)
	if err != nil {
		applog.LogErrorf(s.ctx, "GetCategoriesByGame: failed to query categories for game %s: %v", gameID, err)
		return nil, err
	}
	defer rows.Close()

	var categories []vo.CategoryVO
	for rows.Next() {
		var c vo.CategoryVO
		if err := rows.Scan(&c.ID, &c.Name, &c.IsSystem, &c.CreatedAt, &c.UpdatedAt, &c.GameCount); err != nil {
			applog.LogErrorf(s.ctx, "GetCategoriesByGame: failed to scan row for game %s: %v", gameID, err)
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}
