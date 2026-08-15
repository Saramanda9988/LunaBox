package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"time"
)

const remoteStatusSyncInterval = 250 * time.Millisecond

func loadGamesForRemoteStatusSync(
	ctx context.Context,
	db *sql.DB,
	source enums.SourceType,
) ([]models.Game, error) {
	if db == nil {
		return nil, fmt.Errorf("游戏数据库未初始化")
	}

	rows, err := db.QueryContext(ctx, `
		SELECT g.id, g.name, g.status, s.source_id
		FROM games g
		JOIN game_metadata_sources s ON s.game_id = g.id
		WHERE s.source_type = ?
		  AND TRIM(COALESCE(s.source_id, '')) <> ''
		UNION ALL
		SELECT g.id, g.name, g.status, g.source_id
		FROM games g
		WHERE g.source_type = ?
		  AND TRIM(COALESCE(g.source_id, '')) <> ''
		  AND NOT EXISTS (
			SELECT 1 FROM game_metadata_sources s WHERE s.game_id = g.id
		  )
		ORDER BY name, id
	`, string(source), string(source))
	if err != nil {
		return nil, fmt.Errorf("读取 %s 游戏条目失败: %w", source, err)
	}
	defer rows.Close()

	games := make([]models.Game, 0)
	for rows.Next() {
		var game models.Game
		var status string
		if err := rows.Scan(&game.ID, &game.Name, &status, &game.SourceID); err != nil {
			return nil, fmt.Errorf("读取 %s 游戏状态失败: %w", source, err)
		}
		game.Status = enums.GameStatus(status)
		game.SourceType = source
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 %s 游戏条目失败: %w", source, err)
	}
	return games, nil
}

func waitForRemoteStatusSync(ctx context.Context) error {
	timer := time.NewTimer(remoteStatusSyncInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
