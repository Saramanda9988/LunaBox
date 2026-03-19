package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/models"
	"lunabox/internal/utils/metadata"
	"time"

	"github.com/google/uuid"
)

type TagService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewTagService() *TagService {
	return &TagService{}
}

func (s *TagService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

// GetTagsByGame 获取指定游戏的所有 tag
func (s *TagService) GetTagsByGame(gameID string) ([]models.GameTag, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT id, game_id, name, source, weight, is_spoiler, created_at
		FROM game_tags
		WHERE game_id = ?
		ORDER BY weight DESC
	`, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	var tags []models.GameTag
	for rows.Next() {
		var t models.GameTag
		if err := rows.Scan(&t.ID, &t.GameID, &t.Name, &t.Source, &t.Weight, &t.IsSpoiler, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// AddUserTag 用户手动添加 tag
func (s *TagService) AddUserTag(gameID string, tagName string) error {
	if tagName == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	id := uuid.New().String()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at)
		VALUES (?, ?, ?, 'user', 1.0, false, ?)
		ON CONFLICT (game_id, name, source) DO NOTHING
	`, id, gameID, tagName, time.Now())
	if err != nil {
		applog.LogErrorf(s.ctx, "AddUserTag: failed for game %s tag %s: %v", gameID, tagName, err)
		return fmt.Errorf("failed to add user tag: %w", err)
	}
	return nil
}

// DeleteTag 删除 tag（仅允许删除 source='user' 的 tag）
func (s *TagService) DeleteTag(tagID string) error {
	result, err := s.db.ExecContext(s.ctx, `
		DELETE FROM game_tags WHERE id = ? AND source = 'user'
	`, tagID)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tag not found or not deletable (only user tags can be deleted)")
	}
	return nil
}

// SearchTagsInLibrary 搜索库中匹配的 tag 名称（用于游戏库筛选）
func (s *TagService) SearchTagsInLibrary(query string) ([]string, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT DISTINCT name FROM game_tags
		WHERE name ILIKE ?
		ORDER BY name
		LIMIT 50
	`, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to search tags: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// GetGameIDsByTag 获取包含指定 tag 的所有游戏 ID（用于游戏库筛选）
func (s *TagService) GetGameIDsByTag(tagName string) ([]string, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT DISTINCT game_id FROM game_tags WHERE name = ?
	`, tagName)
	if err != nil {
		return nil, fmt.Errorf("failed to get game ids by tag: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// upsertScrapedTags 删除指定游戏的刮削来源 tag，再批量插入新 tag（保留用户 tag）
func (s *TagService) upsertScrapedTags(gameID string, tags []metadata.TagItem) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 删除旧的刮削 tag（保留 source='user'）
	if _, err := tx.ExecContext(s.ctx, `
		DELETE FROM game_tags WHERE game_id = ? AND source != 'user'
	`, gameID); err != nil {
		return fmt.Errorf("failed to delete old scraped tags: %w", err)
	}

	// 批量插入新 tag
	for _, t := range tags {
		id := uuid.New().String()
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (game_id, name, source) DO UPDATE SET weight = excluded.weight, is_spoiler = excluded.is_spoiler
		`, id, gameID, t.Name, t.Source, t.Weight, t.IsSpoiler, time.Now()); err != nil {
			return fmt.Errorf("failed to insert tag %s: %w", t.Name, err)
		}
	}

	return tx.Commit()
}
