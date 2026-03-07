package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"time"

	"github.com/google/uuid"
)

// GameProgress 游玩进度记录
type GameProgress struct {
	ID              string    `json:"id"`
	GameID          string    `json:"game_id"`
	Chapter         string    `json:"chapter"`
	Route           string    `json:"route"`
	ProgressNote    string    `json:"progress_note"`
	SpoilerBoundary string    `json:"spoiler_boundary"` // none | chapter_end | route_end | full
	UpdatedAt       time.Time `json:"updated_at"`
}

type GameProgressService struct {
	ctx       context.Context
	db        *sql.DB
	appConfig *appconf.AppConfig
}

func NewGameProgressService() *GameProgressService {
	return &GameProgressService{}
}

func (s *GameProgressService) Init(ctx context.Context, db *sql.DB, appConfig *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.appConfig = appConfig
}

// GetGameProgress 获取指定游戏的游玩进度记录
func (s *GameProgressService) GetGameProgress(gameID string) (*GameProgress, error) {
	row := s.db.QueryRowContext(s.ctx, `
		SELECT id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at
		FROM game_progress
		WHERE game_id = ?
	`, gameID)

	var gp GameProgress
	err := row.Scan(&gp.ID, &gp.GameID, &gp.Chapter, &gp.Route, &gp.ProgressNote, &gp.SpoilerBoundary, &gp.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get game progress: %w", err)
	}
	return &gp, nil
}

// UpsertGameProgress 创建或更新游玩进度
func (s *GameProgressService) UpsertGameProgress(gp GameProgress) (*GameProgress, error) {
	if gp.GameID == "" {
		return nil, fmt.Errorf("game_id is required")
	}

	// 校验 spoiler_boundary
	validBoundaries := map[string]bool{"none": true, "chapter_end": true, "route_end": true, "full": true}
	if gp.SpoilerBoundary == "" {
		gp.SpoilerBoundary = "none"
	}
	if !validBoundaries[gp.SpoilerBoundary] {
		return nil, fmt.Errorf("invalid spoiler_boundary: %s", gp.SpoilerBoundary)
	}

	// 检查是否已存在
	existing, err := s.GetGameProgress(gp.GameID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if existing == nil {
		// 新增
		gp.ID = uuid.New().String()
		gp.UpdatedAt = now
		_, err = s.db.ExecContext(s.ctx, `
			INSERT INTO game_progress (id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, gp.ID, gp.GameID, gp.Chapter, gp.Route, gp.ProgressNote, gp.SpoilerBoundary, gp.UpdatedAt)
		if err != nil {
			applog.LogError(s.ctx, "[GameProgressService] insert failed: "+err.Error())
			return nil, fmt.Errorf("failed to insert game progress: %w", err)
		}
	} else {
		// 更新
		gp.ID = existing.ID
		gp.UpdatedAt = now
		_, err = s.db.ExecContext(s.ctx, `
			UPDATE game_progress
			SET chapter = ?, route = ?, progress_note = ?, spoiler_boundary = ?, updated_at = ?
			WHERE game_id = ?
		`, gp.Chapter, gp.Route, gp.ProgressNote, gp.SpoilerBoundary, gp.UpdatedAt, gp.GameID)
		if err != nil {
			applog.LogError(s.ctx, "[GameProgressService] update failed: "+err.Error())
			return nil, fmt.Errorf("failed to update game progress: %w", err)
		}
	}

	return &gp, nil
}

// DeleteGameProgress 删除游玩进度（当游戏被删除时同步清理）
func (s *GameProgressService) DeleteGameProgress(gameID string) error {
	_, err := s.db.ExecContext(s.ctx, "DELETE FROM game_progress WHERE game_id = ?", gameID)
	if err != nil {
		return fmt.Errorf("failed to delete game progress: %w", err)
	}
	return nil
}
