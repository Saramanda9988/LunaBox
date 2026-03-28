package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	cloudSyncEntityGame         = "game"
	cloudSyncEntityCategory     = "category"
	cloudSyncEntityGameCategory = "game_category"
	cloudSyncEntityPlaySession  = "play_session"

	systemFavoritesCategoryID   = "system:favorites"
	systemFavoritesCategoryName = "最喜欢的游戏"
)

type execContexter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func upsertSyncTombstone(ctx context.Context, exec execContexter, entityType, entityID string, deletedAt time.Time) error {
	if entityID == "" {
		return nil
	}

	_, err := exec.ExecContext(ctx, `
		INSERT INTO sync_tombstones (entity_type, entity_id, parent_id, secondary_id, deleted_at)
		VALUES (?, ?, '', '', ?)
		ON CONFLICT (entity_type, entity_id, parent_id, secondary_id) DO UPDATE SET deleted_at = EXCLUDED.deleted_at
	`, entityType, entityID, deletedAt)
	if err != nil {
		return fmt.Errorf("upsert sync tombstone %s/%s: %w", entityType, entityID, err)
	}
	return nil
}

func deleteSyncTombstone(ctx context.Context, exec execContexter, entityType, entityID string) error {
	if entityID == "" {
		return nil
	}

	_, err := exec.ExecContext(ctx, `DELETE FROM sync_tombstones WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	if err != nil {
		return fmt.Errorf("delete sync tombstone %s/%s: %w", entityType, entityID, err)
	}
	return nil
}

func relationTombstoneID(gameID, categoryID string) string {
	return gameID + "::" + categoryID
}
