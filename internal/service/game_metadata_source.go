package service

import (
	"database/sql"
	"errors"
	"fmt"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/service/gamehelper"
	"strings"
	"time"
)

func normalizeGameMetadataSource(source enums.SourceType, sourceID string) (enums.SourceType, string, error) {
	source = gamehelper.NormalizeMetadataSourceType(source)
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return "", "", fmt.Errorf("元数据来源 ID 不能为空")
	}
	switch source {
	case enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam, enums.DLsite,
		enums.TouchGal, enums.Hikarinagi, enums.ErogameScape:
		return source, sourceID, nil
	default:
		return "", "", fmt.Errorf("不支持的元数据来源: %s", source)
	}
}

func validateInitialMetadataSources(sources []models.GameMetadataSource) error {
	seen := make(map[enums.SourceType]struct{}, len(sources))
	for _, item := range sources {
		source, _, err := normalizeGameMetadataSource(item.SourceType, item.SourceID)
		if err != nil {
			return err
		}
		if _, exists := seen[source]; exists {
			return fmt.Errorf("同一游戏的 %s 元数据记录存在多个，请移除错误的候选项", source)
		}
		seen[source] = struct{}{}
	}
	return nil
}

func scanGameMetadataSources(rows *sql.Rows) ([]models.GameMetadataSource, error) {
	items := make([]models.GameMetadataSource, 0)
	for rows.Next() {
		var item models.GameMetadataSource
		var sourceType string
		if err := rows.Scan(
			&item.GameID,
			&sourceType,
			&item.SourceID,
			&item.CachedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("读取游戏元数据来源失败: %w", err)
		}
		item.SourceType = enums.SourceType(sourceType)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历游戏元数据来源失败: %w", err)
	}
	return items, nil
}

func (s *GameService) GetGameMetadataSources(gameID string) ([]models.GameMetadataSource, error) {
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT game_id, source_type, source_id,
		       COALESCE(cached_at, created_at, updated_at, CURRENT_TIMESTAMP),
		       COALESCE(created_at, CURRENT_TIMESTAMP),
		       COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		FROM game_metadata_sources
		WHERE game_id = ?
		ORDER BY source_type
	`, strings.TrimSpace(gameID))
	if err != nil {
		return nil, fmt.Errorf("查询游戏元数据来源失败: %w", err)
	}
	defer rows.Close()
	items, err := scanGameMetadataSources(rows)
	if err != nil || len(items) > 0 {
		return items, err
	}

	var legacySource string
	var legacyID string
	var cachedAt time.Time
	var createdAt time.Time
	var updatedAt time.Time
	err = s.db.QueryRowContext(s.ctx, `
		SELECT COALESCE(source_type, ''), COALESCE(source_id, ''),
		       COALESCE(cached_at, created_at, updated_at, CURRENT_TIMESTAMP),
		       COALESCE(created_at, CURRENT_TIMESTAMP),
		       COALESCE(updated_at, cached_at, created_at, CURRENT_TIMESTAMP)
		FROM games WHERE id = ?
	`, strings.TrimSpace(gameID)).Scan(&legacySource, &legacyID, &cachedAt, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return items, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取兼容元数据来源失败: %w", err)
	}
	source := gamehelper.NormalizeMetadataSourceType(enums.SourceType(legacySource))
	if source == "" || source == enums.Local || strings.TrimSpace(legacyID) == "" {
		return items, nil
	}
	return []models.GameMetadataSource{{
		GameID: gameID, SourceType: source, SourceID: strings.TrimSpace(legacyID),
		CachedAt: cachedAt, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}}, nil
}

func (s *GameService) getGameMetadataSource(gameID string, source enums.SourceType) (models.GameMetadataSource, error) {
	source = gamehelper.NormalizeMetadataSourceType(source)
	var item models.GameMetadataSource
	var sourceType string
	err := s.db.QueryRowContext(s.ctx, `
		SELECT game_id, source_type, source_id,
		       COALESCE(cached_at, created_at, updated_at, CURRENT_TIMESTAMP),
		       COALESCE(created_at, CURRENT_TIMESTAMP),
		       COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		FROM game_metadata_sources
		WHERE game_id = ? AND source_type = ?
	`, gameID, string(source)).Scan(
		&item.GameID,
		&sourceType,
		&item.SourceID,
		&item.CachedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.GameMetadataSource{}, fmt.Errorf("游戏未关联元数据来源 %s", source)
	}
	if err != nil {
		return models.GameMetadataSource{}, fmt.Errorf("查询游戏元数据来源失败: %w", err)
	}
	item.SourceType = enums.SourceType(sourceType)
	return item, nil
}

func (s *GameService) UpsertGameMetadataSource(gameID string, source enums.SourceType, sourceID string) error {
	source, sourceID, err := normalizeGameMetadataSource(source, sourceID)
	if err != nil {
		return err
	}
	gameID = strings.TrimSpace(gameID)
	if gameID == "" {
		return fmt.Errorf("游戏 ID 不能为空")
	}

	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("开始保存元数据来源事务失败: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(s.ctx, `SELECT EXISTS(SELECT 1 FROM games WHERE id = ?)`, gameID).Scan(&exists); err != nil {
		return fmt.Errorf("检查游戏记录失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("game not found with id: %s", gameID)
	}

	now := time.Now()
	if _, err := tx.ExecContext(s.ctx, `
		INSERT INTO game_metadata_sources (
			game_id, source_type, source_id, cached_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (game_id, source_type) DO UPDATE SET
			source_id = EXCLUDED.source_id,
			cached_at = EXCLUDED.cached_at,
			updated_at = EXCLUDED.updated_at
	`, gameID, string(source), sourceID, now, now, now); err != nil {
		return fmt.Errorf("保存游戏元数据来源失败: %w", err)
	}

	if _, err := tx.ExecContext(s.ctx, `
		UPDATE games
		SET source_type = CASE
				WHEN LOWER(TRIM(COALESCE(source_type, ''))) IN ('', 'local') THEN ?
				ELSE source_type
			END,
			source_id = CASE
				WHEN LOWER(TRIM(COALESCE(source_type, ''))) IN ('', 'local')
				  OR LOWER(TRIM(COALESCE(source_type, ''))) = ? THEN ?
				ELSE source_id
			END,
			updated_at = ?
		WHERE id = ?
	`, string(source), string(source), sourceID, now, gameID); err != nil {
		return fmt.Errorf("更新游戏默认元数据来源失败: %w", err)
	}

	if err := deleteSyncTombstone(s.ctx, tx, cloudSyncEntityGameMetadataSource, metadataSourceTombstoneID(gameID, source)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *GameService) SetDefaultMetadataSource(gameID string, source enums.SourceType) error {
	item, err := s.getGameMetadataSource(strings.TrimSpace(gameID), source)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(s.ctx, `
		UPDATE games
		SET source_type = ?, source_id = ?, updated_at = ?
		WHERE id = ?
	`, string(item.SourceType), item.SourceID, time.Now(), item.GameID)
	if err != nil {
		return fmt.Errorf("设置默认元数据来源失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("game not found with id: %s", gameID)
	}
	return nil
}

func (s *GameService) DeleteGameMetadataSource(gameID string, source enums.SourceType) error {
	gameID = strings.TrimSpace(gameID)
	source = gamehelper.NormalizeMetadataSourceType(source)
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("开始删除元数据来源事务失败: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(s.ctx, `
		DELETE FROM game_metadata_sources WHERE game_id = ? AND source_type = ?
	`, gameID, string(source))
	if err != nil {
		return fmt.Errorf("删除游戏元数据来源失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("游戏未关联元数据来源 %s", source)
	}

	var defaultSource string
	if err := tx.QueryRowContext(s.ctx, `SELECT COALESCE(source_type, '') FROM games WHERE id = ?`, gameID).Scan(&defaultSource); err != nil {
		return fmt.Errorf("读取默认元数据来源失败: %w", err)
	}
	if gamehelper.NormalizeMetadataSourceType(enums.SourceType(defaultSource)) == source {
		nextSource, nextID, selectErr := s.selectNextDefaultMetadataSource(tx, gameID)
		if selectErr != nil {
			return selectErr
		}
		if _, err := tx.ExecContext(s.ctx, `
			UPDATE games
			SET source_type = ?, source_id = ?, updated_at = ?
			WHERE id = ?
		`, defaultSourceTypeValue(nextSource), nextID, time.Now(), gameID); err != nil {
			return fmt.Errorf("更新默认元数据来源失败: %w", err)
		}
	}

	if err := upsertSyncTombstone(s.ctx, tx, cloudSyncEntityGameMetadataSource, metadataSourceTombstoneID(gameID, source), time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func defaultSourceTypeValue(source enums.SourceType) string {
	if source == "" {
		return string(enums.Local)
	}
	return string(source)
}

func (s *GameService) selectNextDefaultMetadataSource(tx *sql.Tx, gameID string) (enums.SourceType, string, error) {
	rows, err := tx.QueryContext(s.ctx, `SELECT source_type, source_id FROM game_metadata_sources WHERE game_id = ?`, gameID)
	if err != nil {
		return "", "", fmt.Errorf("查询备选元数据来源失败: %w", err)
	}
	defer rows.Close()
	available := make(map[enums.SourceType]string)
	for rows.Next() {
		var source string
		var sourceID string
		if err := rows.Scan(&source, &sourceID); err != nil {
			return "", "", fmt.Errorf("读取备选元数据来源失败: %w", err)
		}
		available[enums.SourceType(source)] = sourceID
	}
	for _, source := range s.getConfiguredMetadataSources() {
		if sourceID, ok := available[source]; ok {
			return source, sourceID, nil
		}
	}
	for source, sourceID := range available {
		return source, sourceID, nil
	}
	return "", "", nil
}

func (s *GameService) addInitialMetadataSources(game models.Game) error {
	sources := game.MetadataSources
	if len(sources) == 0 && game.SourceType != "" && game.SourceType != enums.Local && strings.TrimSpace(game.SourceID) != "" {
		sources = []models.GameMetadataSource{{SourceType: game.SourceType, SourceID: game.SourceID}}
	}
	for _, source := range sources {
		if err := s.UpsertGameMetadataSource(game.ID, source.SourceType, source.SourceID); err != nil {
			return err
		}
	}
	defaultSource := game.SourceType
	if defaultSource == enums.Local {
		defaultSource = ""
	}
	if defaultSource != "" {
		if err := s.SetDefaultMetadataSource(game.ID, defaultSource); err != nil {
			return err
		}
	}
	return nil
}
