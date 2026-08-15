package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"strings"
	"sync"
	"time"
)

const (
	gameReviewSyncSuccess     = "success"
	gameReviewSyncFailed      = "failed"
	gameReviewSyncUnavailable = "unavailable"
)

type GameReviewService struct {
	ctx               context.Context
	db                *sql.DB
	bangumiService    *BangumiService
	hikarinagiService *HikarinagiService
}

func NewGameReviewService() *GameReviewService {
	return &GameReviewService{}
}

//wails:ignore
func (s *GameReviewService) Init(ctx context.Context, db *sql.DB, _ *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
}

//wails:ignore
func (s *GameReviewService) SetBangumiService(service *BangumiService) {
	s.bangumiService = service
}

//wails:ignore
func (s *GameReviewService) SetHikarinagiService(service *HikarinagiService) {
	s.hikarinagiService = service
}

func (s *GameReviewService) GetGameReview(gameID string) (*models.GameReview, error) {
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return nil, fmt.Errorf("game_id is required")
	}

	var review models.GameReview
	var rating sql.NullInt64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT game_id, rating, COALESCE(content, ''), COALESCE(is_spoiler, FALSE), created_at, updated_at
		FROM game_reviews
		WHERE game_id = ?
	`, gameID).Scan(
		&review.GameID,
		&rating,
		&review.Content,
		&review.IsSpoiler,
		&review.CreatedAt,
		&review.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取游戏评价失败: %w", err)
	}
	if rating.Valid {
		value := int(rating.Int64)
		review.Rating = &value
	}
	return &review, nil
}

func (s *GameReviewService) SaveGameReview(review models.GameReview) (*models.GameReview, error) {
	review.GameID = strings.TrimSpace(review.GameID)
	if review.GameID == "" {
		return nil, fmt.Errorf("game_id is required")
	}
	if review.Rating != nil && (*review.Rating < 1 || *review.Rating > 10) {
		return nil, fmt.Errorf("评分必须在 1 到 10 之间")
	}

	var gameExists bool
	if err := s.db.QueryRowContext(s.ctx, `SELECT EXISTS(SELECT 1 FROM games WHERE id = ?)`, review.GameID).Scan(&gameExists); err != nil {
		return nil, fmt.Errorf("检查游戏记录失败: %w", err)
	}
	if !gameExists {
		return nil, fmt.Errorf("游戏不存在: %s", review.GameID)
	}

	now := time.Now()
	var rating any
	if review.Rating != nil {
		rating = *review.Rating
	}
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO game_reviews (game_id, rating, content, is_spoiler, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (game_id) DO UPDATE SET
			rating = EXCLUDED.rating,
			content = EXCLUDED.content,
			is_spoiler = EXCLUDED.is_spoiler,
			updated_at = EXCLUDED.updated_at
	`, review.GameID, rating, review.Content, review.IsSpoiler, now, now)
	if err != nil {
		return nil, fmt.Errorf("保存游戏评价失败: %w", err)
	}
	if err := deleteSyncTombstone(s.ctx, s.db, cloudSyncEntityGameReview, review.GameID); err != nil {
		return nil, err
	}
	return s.GetGameReview(review.GameID)
}

func (s *GameReviewService) SyncGameReview(gameID string, providers []enums.SourceType) (vo.GameReviewSyncResult, error) {
	review, err := s.GetGameReview(gameID)
	if err != nil {
		return vo.GameReviewSyncResult{}, err
	}
	if review == nil {
		return vo.GameReviewSyncResult{}, fmt.Errorf("请先保存评价")
	}

	uniqueProviders := make([]enums.SourceType, 0, len(providers))
	seen := make(map[enums.SourceType]struct{}, len(providers))
	for _, provider := range providers {
		if _, exists := seen[provider]; exists {
			continue
		}
		seen[provider] = struct{}{}
		uniqueProviders = append(uniqueProviders, provider)
	}

	result := vo.GameReviewSyncResult{
		Results: make([]vo.GameReviewProviderSyncResult, len(uniqueProviders)),
	}
	var wg sync.WaitGroup
	for index, provider := range uniqueProviders {
		wg.Add(1)
		go func(resultIndex int, selectedProvider enums.SourceType) {
			defer wg.Done()
			result.Results[resultIndex] = s.syncProviderReview(gameID, selectedProvider, *review)
		}(index, provider)
	}
	wg.Wait()

	for _, item := range result.Results {
		if item.Status == gameReviewSyncSuccess {
			result.Succeeded++
		} else {
			result.Failed++
		}
	}
	return result, nil
}

func (s *GameReviewService) syncProviderReview(gameID string, provider enums.SourceType, review models.GameReview) vo.GameReviewProviderSyncResult {
	item := vo.GameReviewProviderSyncResult{Provider: string(provider)}
	sourceID, err := s.findSourceID(gameID, provider)
	if err != nil {
		item.Status = gameReviewSyncFailed
		item.Error = err.Error()
		return item
	}
	if sourceID == "" {
		item.Status = gameReviewSyncUnavailable
		item.Error = "尚未关联该平台的条目"
		return item
	}

	switch provider {
	case enums.Bangumi:
		if s.bangumiService == nil {
			item.Status = gameReviewSyncUnavailable
			item.Error = "Bangumi 服务未初始化"
			return item
		}
		err = s.bangumiService.syncGameReview(s.ctx, sourceID, review)
	case enums.Hikarinagi:
		if s.hikarinagiService == nil {
			item.Status = gameReviewSyncUnavailable
			item.Error = "Hikarinagi 服务未初始化"
			return item
		}
		var timeToFinishMinutes int64
		timeToFinishMinutes, err = s.getGamePlayTimeMinutes(gameID)
		if err == nil {
			err = s.hikarinagiService.syncGameReview(s.ctx, sourceID, review, timeToFinishMinutes)
		}
	default:
		item.Status = gameReviewSyncUnavailable
		item.Error = "该平台暂不支持评价同步"
		return item
	}
	if err != nil {
		item.Status = gameReviewSyncFailed
		item.Error = err.Error()
		return item
	}
	item.Status = gameReviewSyncSuccess
	return item
}

func (s *GameReviewService) getGamePlayTimeMinutes(gameID string) (int64, error) {
	var totalSeconds int64
	err := s.db.QueryRowContext(s.ctx, `
		SELECT COALESCE(SUM(CASE WHEN duration > 0 THEN duration ELSE 0 END), 0)
		FROM play_sessions
		WHERE game_id = ?
	`, gameID).Scan(&totalSeconds)
	if err != nil {
		return 0, fmt.Errorf("读取本地游玩时间失败: %w", err)
	}
	return totalSeconds / 60, nil
}

func (s *GameReviewService) findSourceID(gameID string, provider enums.SourceType) (string, error) {
	var sourceID string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT source_id
		FROM game_metadata_sources
		WHERE game_id = ? AND source_type = ? AND TRIM(COALESCE(source_id, '')) <> ''
		UNION ALL
		SELECT source_id
		FROM games
		WHERE id = ? AND source_type = ? AND TRIM(COALESCE(source_id, '')) <> ''
		  AND NOT EXISTS (
			SELECT 1 FROM game_metadata_sources
			WHERE game_id = ? AND source_type = ?
		  )
		LIMIT 1
	`, gameID, string(provider), gameID, string(provider), gameID, string(provider)).Scan(&sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取平台条目标识失败: %w", err)
	}
	return strings.TrimSpace(sourceID), nil
}
