package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/models"
	"time"

	"github.com/google/uuid"
)

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
func (s *GameProgressService) GetGameProgress(gameID string) (*models.GameProgress, error) {
	row := s.db.QueryRowContext(s.ctx, `
		SELECT id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at
		FROM game_progress
		WHERE game_id = ?
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`, gameID)

	var gp models.GameProgress
	err := row.Scan(&gp.ID, &gp.GameID, &gp.Chapter, &gp.Route, &gp.ProgressNote, &gp.SpoilerBoundary, &gp.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get game progress: %w", err)
	}
	return &gp, nil
}

// ListGameProgresses 获取指定游戏的全部游玩进度记录
func (s *GameProgressService) ListGameProgresses(gameID string) ([]models.GameProgress, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at
		FROM game_progress
		WHERE game_id = ?
		ORDER BY updated_at DESC, id DESC
	`, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to list game progress: %w", err)
	}
	defer rows.Close()

	progresses := make([]models.GameProgress, 0)
	for rows.Next() {
		var gp models.GameProgress
		if err := rows.Scan(&gp.ID, &gp.GameID, &gp.Chapter, &gp.Route, &gp.ProgressNote, &gp.SpoilerBoundary, &gp.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan game progress: %w", err)
		}
		progresses = append(progresses, gp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate game progress rows: %w", err)
	}

	return progresses, nil
}

// UpsertGameProgress 追加保存游玩进度
func (s *GameProgressService) UpsertGameProgress(gp models.GameProgress) (*models.GameProgress, error) {
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

	now := time.Now()
	gp.ID = uuid.New().String()
	gp.UpdatedAt = now
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO game_progress (id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, gp.ID, gp.GameID, gp.Chapter, gp.Route, gp.ProgressNote, gp.SpoilerBoundary, gp.UpdatedAt)
	if err != nil {
		applog.LogError(s.ctx, "[GameProgressService] insert failed: "+err.Error())
		return nil, fmt.Errorf("failed to insert game progress: %w", err)
	}

	if err := deleteSyncTombstone(s.ctx, s.db, cloudSyncEntityGameProgress, gp.ID); err != nil {
		applog.LogWarningf(s.ctx, "UpsertGameProgress: failed to clear progress tombstone %s: %v", gp.ID, err)
	}

	return &gp, nil
}

// DeleteGameProgress 删除游玩进度（当游戏被删除时同步清理）
func (s *GameProgressService) DeleteGameProgress(gameID string) error {
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete game progress tx: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(s.ctx, "SELECT id FROM game_progress WHERE game_id = ?", gameID)
	if err != nil {
		return fmt.Errorf("failed to query game progress ids: %w", err)
	}

	var progressIDs []string
	for rows.Next() {
		var progressID string
		if err := rows.Scan(&progressID); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan game progress id: %w", err)
		}
		progressIDs = append(progressIDs, progressID)
	}
	rows.Close()

	if _, err := tx.ExecContext(s.ctx, "DELETE FROM game_progress WHERE game_id = ?", gameID); err != nil {
		return fmt.Errorf("failed to delete game progress: %w", err)
	}

	now := time.Now()
	for _, progressID := range progressIDs {
		if err := upsertSyncTombstone(s.ctx, tx, cloudSyncEntityGameProgress, progressID, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit delete game progress tx: %w", err)
	}

	return nil
}
