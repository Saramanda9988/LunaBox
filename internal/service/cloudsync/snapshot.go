package cloudsync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/utils/imageutils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (h *Helper) BuildLocalState() (LocalState, error) {
	state := LocalState{Covers: make(map[string]LocalCover)}
	snapshot := Snapshot{
		SchemaVersion: SchemaVersion,
		RevisionID:    uuid.New().String(),
		ExportedAt:    time.Now(),
		DeviceID:      h.currentDeviceID(),
	}

	games, err := h.listGames()
	if err != nil {
		return state, err
	}
	for _, game := range games {
		snapshot.Games = append(snapshot.Games, gameFromModel(game))
		coverPath, coverURL, err := imageutils.FindManagedCoverFile(game.ID)
		if err != nil {
			return state, fmt.Errorf("resolve cover for game %s: %w", game.ID, err)
		}
		if coverPath == "" {
			continue
		}
		localCover := newLocalCover(game, coverPath, coverURL)
		state.Covers[game.ID] = localCover
		snapshot.Covers = append(snapshot.Covers, localCover.Asset)
	}

	categories, err := h.listCategories()
	if err != nil {
		return state, err
	}
	for _, category := range categories {
		snapshot.Categories = append(snapshot.Categories, categoryFromModel(category))
	}

	relations, err := h.listRelations()
	if err != nil {
		return state, err
	}
	for _, relation := range relations {
		snapshot.GameCategories = append(snapshot.GameCategories, relationFromModel(relation))
	}

	sessions, err := h.listPlaySessions()
	if err != nil {
		return state, err
	}
	for _, session := range sessions {
		snapshot.PlaySessions = append(snapshot.PlaySessions, playSessionFromModel(session))
	}

	progresses, err := h.listGameProgresses()
	if err != nil {
		return state, err
	}
	for _, progress := range progresses {
		snapshot.GameProgresses = append(snapshot.GameProgresses, gameProgressFromModel(progress))
	}

	tags, err := h.listGameTags()
	if err != nil {
		return state, err
	}
	for _, tag := range tags {
		snapshot.GameTags = append(snapshot.GameTags, gameTagFromModel(tag))
	}

	tombstones, err := h.listTombstones()
	if err != nil {
		return state, err
	}
	for _, tombstone := range tombstones {
		snapshot.Tombstones = append(snapshot.Tombstones, tombstoneFromModel(tombstone))
	}

	sortSnapshot(&snapshot)
	state.Snapshot = snapshot
	return state, nil
}

func (h *Helper) LoadRemoteSnapshot(provider cloudprovider.CloudStorageProvider) (Snapshot, bool, error) {
	var snapshot Snapshot

	prefix := provider.GetCloudPath(h.config.BackupUserID, LibraryDir+"/")
	keys, err := provider.ListObjects(h.ctx, prefix)
	if err != nil {
		return snapshot, false, fmt.Errorf("list cloud sync snapshots: %w", err)
	}

	latestKey := provider.GetCloudPath(h.config.BackupUserID, SnapshotKey)
	found := false
	for _, key := range keys {
		if key == latestKey || strings.HasSuffix(key, "/latest.json") {
			found = true
			break
		}
	}
	if !found {
		return snapshot, false, nil
	}

	tempFile, err := os.CreateTemp("", "lunabox_cloud_sync_*.json")
	if err != nil {
		return snapshot, false, fmt.Errorf("create temp cloud sync file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempPath)

	if err := provider.DownloadFile(h.ctx, latestKey, tempPath); err != nil {
		return snapshot, false, fmt.Errorf("download cloud sync snapshot: %w", err)
	}

	raw, err := os.ReadFile(tempPath)
	if err != nil {
		return snapshot, false, fmt.Errorf("read cloud sync snapshot: %w", err)
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, false, fmt.Errorf("decode cloud sync snapshot: %w", err)
	}

	sortSnapshot(&snapshot)
	return snapshot, true, nil
}

func (h *Helper) SaveRemoteSnapshot(provider cloudprovider.CloudStorageProvider, snapshot Snapshot) error {
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cloud sync snapshot: %w", err)
	}

	folderPath := provider.GetCloudPath(h.config.BackupUserID, LibraryDir)
	if err := provider.EnsureDir(h.ctx, folderPath); err != nil {
		return fmt.Errorf("ensure cloud sync dir: %w", err)
	}

	tempFile, err := os.CreateTemp("", "lunabox_cloud_sync_upload_*.json")
	if err != nil {
		return fmt.Errorf("create upload temp file: %w", err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(payload); err != nil {
		tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write cloud sync temp file: %w", err)
	}
	tempFile.Close()
	defer os.Remove(tempPath)

	latestKey := provider.GetCloudPath(h.config.BackupUserID, SnapshotKey)
	if err := provider.UploadFile(h.ctx, latestKey, tempPath); err != nil {
		return fmt.Errorf("upload cloud sync snapshot: %w", err)
	}

	return nil
}

func (h *Helper) ReconcileCoverAssets(provider cloudprovider.CloudStorageProvider, local LocalState, remote Snapshot, remoteExists bool, merged Snapshot) (map[string]string, error) {
	coverURLs := make(map[string]string)
	localAssets := local.Covers
	remoteAssets := mapCoverAssets(remote.Covers)
	mergedAssets := mapCoverAssets(merged.Covers)

	folderPath := provider.GetCloudPath(h.config.BackupUserID, CoverDir)
	if err := provider.EnsureDir(h.ctx, folderPath); err != nil {
		return coverURLs, fmt.Errorf("ensure cloud cover dir: %w", err)
	}

	for _, game := range merged.Games {
		asset, hasMerged := mergedAssets[game.ID]
		localAsset, hasLocal := localAssets[game.ID]
		remoteAsset, hasRemote := remoteAssets[game.ID]

		switch {
		case hasMerged && hasLocal && localAsset.Asset.Ext == asset.Ext && localAsset.Asset.UpdatedAt.Equal(asset.UpdatedAt):
			coverURLs[game.ID] = localAsset.LocalURL
			if !hasRemote || remoteAsset.Ext != asset.Ext || !remoteAsset.UpdatedAt.Equal(asset.UpdatedAt) {
				if err := provider.UploadFile(h.ctx, h.coverCloudKey(provider, asset), localAsset.LocalPath); err != nil {
					return coverURLs, fmt.Errorf("upload cover for game %s: %w", game.ID, err)
				}
			}
		case hasMerged && hasRemote && remoteAsset.Ext == asset.Ext && remoteAsset.UpdatedAt.Equal(asset.UpdatedAt):
			destPath, localURL, err := imageutils.PrepareManagedCoverDestination(game.ID, asset.Ext)
			if err != nil {
				return coverURLs, fmt.Errorf("prepare cover destination for game %s: %w", game.ID, err)
			}
			if err := provider.DownloadFile(h.ctx, h.coverCloudKey(provider, asset), destPath); err != nil {
				return coverURLs, fmt.Errorf("download cover for game %s: %w", game.ID, err)
			}
			coverURLs[game.ID] = localURL
		case hasMerged && hasLocal:
			coverURLs[game.ID] = localAsset.LocalURL
			if err := provider.UploadFile(h.ctx, h.coverCloudKey(provider, asset), localAsset.LocalPath); err != nil {
				return coverURLs, fmt.Errorf("upload local cover fallback for game %s: %w", game.ID, err)
			}
		case hasMerged && hasRemote:
			destPath, localURL, err := imageutils.PrepareManagedCoverDestination(game.ID, asset.Ext)
			if err != nil {
				return coverURLs, fmt.Errorf("prepare remote cover destination for game %s: %w", game.ID, err)
			}
			if err := provider.DownloadFile(h.ctx, h.coverCloudKey(provider, asset), destPath); err != nil {
				return coverURLs, fmt.Errorf("download remote cover fallback for game %s: %w", game.ID, err)
			}
			coverURLs[game.ID] = localURL
		default:
			if err := imageutils.RemoveManagedCover(game.ID); err != nil {
				return coverURLs, fmt.Errorf("remove local cover for game %s: %w", game.ID, err)
			}
		}
	}

	if remoteExists {
		for gameID, asset := range remoteAssets {
			if _, keep := mergedAssets[gameID]; keep {
				continue
			}
			if err := provider.DeleteObject(h.ctx, h.coverCloudKey(provider, asset)); err != nil {
				applog.LogWarningf(h.ctx, "CloudSync: failed to delete stale remote cover for game %s: %v", gameID, err)
			}
		}
	}

	return coverURLs, nil
}

func (h *Helper) ApplyMergedSnapshot(snapshot Snapshot, coverURLs map[string]string) error {
	tx, err := h.db.BeginTx(h.ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud sync apply tx: %w", err)
	}
	defer tx.Rollback()

	for _, tombstone := range snapshot.Tombstones {
		if err := h.applyTombstone(tx, tombstoneToModel(tombstone)); err != nil {
			return err
		}
	}
	for _, categoryDTO := range snapshot.Categories {
		if err := h.upsertCategory(tx, categoryToModel(categoryDTO)); err != nil {
			return err
		}
	}
	for _, gameDTO := range snapshot.Games {
		coverURL := coverURLs[gameDTO.ID]
		if coverURL == "" {
			url, _, _ := h.lookupExistingGameCover(gameDTO.ID)
			coverURL = url
		}
		if err := h.upsertGame(tx, gameToModel(gameDTO, coverURL)); err != nil {
			return err
		}
	}
	for _, relationDTO := range snapshot.GameCategories {
		if err := h.upsertRelation(tx, relationToModel(relationDTO)); err != nil {
			return err
		}
	}
	for _, sessionDTO := range snapshot.PlaySessions {
		if err := h.upsertPlaySession(tx, playSessionToModel(sessionDTO)); err != nil {
			return err
		}
	}
	for _, progressDTO := range snapshot.GameProgresses {
		if err := h.upsertGameProgress(tx, gameProgressToModel(progressDTO)); err != nil {
			return err
		}
	}
	for _, tagDTO := range snapshot.GameTags {
		if err := h.upsertGameTag(tx, gameTagToModel(tagDTO)); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(h.ctx, `DELETE FROM sync_tombstones`); err != nil {
		return fmt.Errorf("clear local sync tombstones: %w", err)
	}
	for _, tombstone := range snapshot.Tombstones {
		if err := h.insertTombstone(tx, tombstoneToModel(tombstone)); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cloud sync apply tx: %w", err)
	}
	return nil
}

func (h *Helper) listGames() ([]models.Game, error) {
	rows, err := h.db.QueryContext(h.ctx, `SELECT id, name, COALESCE(company, ''), COALESCE(summary, ''), COALESCE(rating, 0), COALESCE(release_date, ''), COALESCE(status, 'not_started'), COALESCE(source_type, ''), COALESCE(source_id, ''), created_at, COALESCE(updated_at, created_at, cached_at) FROM games`)
	if err != nil {
		return nil, fmt.Errorf("query games for cloud sync: %w", err)
	}
	defer rows.Close()
	var items []models.Game
	for rows.Next() {
		var item models.Game
		var status string
		var sourceType string
		if err := rows.Scan(&item.ID, &item.Name, &item.Company, &item.Summary, &item.Rating, &item.ReleaseDate, &status, &sourceType, &item.SourceID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan game for cloud sync: %w", err)
		}
		item.Status = enums.GameStatus(status)
		item.SourceType = enums.SourceType(sourceType)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate games for cloud sync: %w", err)
	}
	return items, nil
}

func (h *Helper) listCategories() ([]models.Category, error) {
	rows, err := h.db.QueryContext(h.ctx, `SELECT id, name, COALESCE(emoji, ''), COALESCE(is_system, FALSE), created_at, updated_at FROM categories`)
	if err != nil {
		return nil, fmt.Errorf("query categories for cloud sync: %w", err)
	}
	defer rows.Close()
	var items []models.Category
	for rows.Next() {
		var item models.Category
		if err := rows.Scan(&item.ID, &item.Name, &item.Emoji, &item.IsSystem, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan category for cloud sync: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories for cloud sync: %w", err)
	}
	return items, nil
}

func (h *Helper) listRelations() ([]models.GameCategory, error) {
	rows, err := h.db.QueryContext(h.ctx, `SELECT game_id, category_id, COALESCE(updated_at, CURRENT_TIMESTAMP) FROM game_categories`)
	if err != nil {
		return nil, fmt.Errorf("query game categories for cloud sync: %w", err)
	}
	defer rows.Close()
	var items []models.GameCategory
	for rows.Next() {
		var item models.GameCategory
		if err := rows.Scan(&item.GameID, &item.CategoryID, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan game category for cloud sync: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game categories for cloud sync: %w", err)
	}
	return items, nil
}

func (h *Helper) listPlaySessions() ([]models.PlaySession, error) {
	rows, err := h.db.QueryContext(h.ctx, `SELECT id, game_id, start_time, COALESCE(end_time, start_time), duration, COALESCE(updated_at, end_time, start_time) FROM play_sessions`)
	if err != nil {
		return nil, fmt.Errorf("query play sessions for cloud sync: %w", err)
	}
	defer rows.Close()
	var items []models.PlaySession
	for rows.Next() {
		var item models.PlaySession
		if err := rows.Scan(&item.ID, &item.GameID, &item.StartTime, &item.EndTime, &item.Duration, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan play session for cloud sync: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate play sessions for cloud sync: %w", err)
	}
	return items, nil
}

func (h *Helper) listGameProgresses() ([]models.GameProgress, error) {
	rows, err := h.db.QueryContext(h.ctx, `SELECT id, game_id, COALESCE(chapter, ''), COALESCE(route, ''), COALESCE(progress_note, ''), COALESCE(spoiler_boundary, 'none'), COALESCE(updated_at, CURRENT_TIMESTAMP) FROM game_progress`)
	if err != nil {
		return nil, fmt.Errorf("query game progress for cloud sync: %w", err)
	}
	defer rows.Close()
	var items []models.GameProgress
	for rows.Next() {
		var item models.GameProgress
		if err := rows.Scan(&item.ID, &item.GameID, &item.Chapter, &item.Route, &item.ProgressNote, &item.SpoilerBoundary, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan game progress for cloud sync: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game progress for cloud sync: %w", err)
	}
	return items, nil
}

func (h *Helper) listGameTags() ([]models.GameTag, error) {
	rows, err := h.db.QueryContext(h.ctx, `SELECT id, game_id, name, source, COALESCE(weight, 1.0), COALESCE(is_spoiler, FALSE), COALESCE(created_at, CURRENT_TIMESTAMP), COALESCE(updated_at, created_at, CURRENT_TIMESTAMP) FROM game_tags`)
	if err != nil {
		return nil, fmt.Errorf("query game tags for cloud sync: %w", err)
	}
	defer rows.Close()
	var items []models.GameTag
	for rows.Next() {
		var item models.GameTag
		if err := rows.Scan(&item.ID, &item.GameID, &item.Name, &item.Source, &item.Weight, &item.IsSpoiler, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan game tag for cloud sync: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate game tags for cloud sync: %w", err)
	}
	return items, nil
}

func (h *Helper) listTombstones() ([]models.SyncTombstone, error) {
	rows, err := h.db.QueryContext(h.ctx, `SELECT entity_type, entity_id, COALESCE(parent_id, ''), COALESCE(secondary_id, ''), deleted_at FROM sync_tombstones WHERE COALESCE(parent_id, '') = '' AND COALESCE(secondary_id, '') = ''`)
	if err != nil {
		return nil, fmt.Errorf("query tombstones for cloud sync: %w", err)
	}
	defer rows.Close()
	var items []models.SyncTombstone
	for rows.Next() {
		var item models.SyncTombstone
		if err := rows.Scan(&item.EntityType, &item.EntityID, &item.ParentID, &item.SecondaryID, &item.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan tombstone for cloud sync: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tombstones for cloud sync: %w", err)
	}
	return items, nil
}

func (h *Helper) applyTombstone(tx *sql.Tx, tombstone models.SyncTombstone) error {
	switch tombstone.EntityType {
	case entityGameCategory:
		parts := strings.SplitN(tombstone.EntityID, "::", 2)
		if len(parts) == 2 {
			if _, err := tx.ExecContext(h.ctx, `DELETE FROM game_categories WHERE game_id = ? AND category_id = ?`, parts[0], parts[1]); err != nil {
				return fmt.Errorf("delete synced relation: %w", err)
			}
		}
	case entityPlaySession:
		if _, err := tx.ExecContext(h.ctx, `DELETE FROM play_sessions WHERE id = ?`, tombstone.EntityID); err != nil {
			return fmt.Errorf("delete synced play session: %w", err)
		}
	case entityGameProgress:
		if _, err := tx.ExecContext(h.ctx, `DELETE FROM game_progress WHERE id = ?`, tombstone.EntityID); err != nil {
			return fmt.Errorf("delete synced game progress: %w", err)
		}
	case entityGameTag:
		parts := strings.SplitN(tombstone.EntityID, "::", 3)
		if len(parts) == 3 {
			if _, err := tx.ExecContext(h.ctx, `DELETE FROM game_tags WHERE game_id = ? AND source = ? AND name = ?`, parts[0], parts[1], parts[2]); err != nil {
				return fmt.Errorf("delete synced game tag: %w", err)
			}
		}
	case entityCategory:
		if tombstone.EntityID != systemFavoritesCategoryID {
			if _, err := tx.ExecContext(h.ctx, `DELETE FROM game_categories WHERE category_id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced category relations: %w", err)
			}
			if _, err := tx.ExecContext(h.ctx, `DELETE FROM categories WHERE id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced category: %w", err)
			}
		}
	case entityGame:
		if _, err := tx.ExecContext(h.ctx, `DELETE FROM game_categories WHERE game_id = ?`, tombstone.EntityID); err != nil {
			return fmt.Errorf("delete synced game relations: %w", err)
		}
		if _, err := tx.ExecContext(h.ctx, `DELETE FROM play_sessions WHERE game_id = ?`, tombstone.EntityID); err != nil {
			return fmt.Errorf("delete synced game sessions: %w", err)
		}
		if _, err := tx.ExecContext(h.ctx, `DELETE FROM game_progress WHERE game_id = ?`, tombstone.EntityID); err != nil {
			return fmt.Errorf("delete synced game progress: %w", err)
		}
		if _, err := tx.ExecContext(h.ctx, `DELETE FROM game_tags WHERE game_id = ?`, tombstone.EntityID); err != nil {
			return fmt.Errorf("delete synced game tags: %w", err)
		}
		if _, err := tx.ExecContext(h.ctx, `DELETE FROM games WHERE id = ?`, tombstone.EntityID); err != nil {
			return fmt.Errorf("delete synced game: %w", err)
		}
	}
	return nil
}

func (h *Helper) upsertCategory(tx *sql.Tx, category models.Category) error {
	_, err := tx.ExecContext(h.ctx, `INSERT INTO categories (id, name, emoji, is_system, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, emoji = EXCLUDED.emoji, is_system = EXCLUDED.is_system, created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at`, category.ID, category.Name, category.Emoji, category.IsSystem, category.CreatedAt, category.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert synced category %s: %w", category.ID, err)
	}
	return nil
}

func (h *Helper) upsertGame(tx *sql.Tx, game models.Game) error {
	_, err := tx.ExecContext(h.ctx, `INSERT INTO games (id, name, cover_url, company, summary, rating, release_date, path, save_path, process_name, status, source_type, cached_at, source_id, created_at, updated_at, use_locale_emulator, use_magpie) VALUES (?, ?, ?, ?, ?, ?, ?, '', '', '', ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, FALSE, FALSE) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, cover_url = EXCLUDED.cover_url, company = EXCLUDED.company, summary = EXCLUDED.summary, rating = EXCLUDED.rating, release_date = EXCLUDED.release_date, status = EXCLUDED.status, source_type = EXCLUDED.source_type, source_id = EXCLUDED.source_id, created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at`, game.ID, game.Name, game.CoverURL, game.Company, game.Summary, game.Rating, game.ReleaseDate, game.Status, game.SourceType, game.SourceID, game.CreatedAt, game.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert synced game %s: %w", game.ID, err)
	}
	return nil
}

func (h *Helper) upsertRelation(tx *sql.Tx, relation models.GameCategory) error {
	_, err := tx.ExecContext(h.ctx, `INSERT INTO game_categories (game_id, category_id, updated_at) VALUES (?, ?, ?) ON CONFLICT (game_id, category_id) DO UPDATE SET updated_at = EXCLUDED.updated_at`, relation.GameID, relation.CategoryID, relation.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert synced relation %s/%s: %w", relation.GameID, relation.CategoryID, err)
	}
	return nil
}

func (h *Helper) upsertPlaySession(tx *sql.Tx, session models.PlaySession) error {
	_, err := tx.ExecContext(h.ctx, `INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO UPDATE SET game_id = EXCLUDED.game_id, start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time, duration = EXCLUDED.duration, updated_at = EXCLUDED.updated_at`, session.ID, session.GameID, session.StartTime, session.EndTime, session.Duration, session.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert synced play session %s: %w", session.ID, err)
	}
	return nil
}

func (h *Helper) upsertGameProgress(tx *sql.Tx, progress models.GameProgress) error {
	_, err := tx.ExecContext(h.ctx, `INSERT INTO game_progress (id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO UPDATE SET game_id = EXCLUDED.game_id, chapter = EXCLUDED.chapter, route = EXCLUDED.route, progress_note = EXCLUDED.progress_note, spoiler_boundary = EXCLUDED.spoiler_boundary, updated_at = EXCLUDED.updated_at`, progress.ID, progress.GameID, progress.Chapter, progress.Route, progress.ProgressNote, progress.SpoilerBoundary, progress.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert synced game progress %s: %w", progress.ID, err)
	}
	return nil
}

func (h *Helper) upsertGameTag(tx *sql.Tx, tag models.GameTag) error {
	_, err := tx.ExecContext(h.ctx, `INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (game_id, name, source) DO UPDATE SET id = EXCLUDED.id, weight = EXCLUDED.weight, is_spoiler = EXCLUDED.is_spoiler, created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at`, tag.ID, tag.GameID, tag.Name, tag.Source, tag.Weight, tag.IsSpoiler, tag.CreatedAt, tag.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert synced game tag %s/%s/%s: %w", tag.GameID, tag.Source, tag.Name, err)
	}
	return nil
}

func (h *Helper) insertTombstone(tx *sql.Tx, tombstone models.SyncTombstone) error {
	_, err := tx.ExecContext(h.ctx, `INSERT INTO sync_tombstones (entity_type, entity_id, parent_id, secondary_id, deleted_at) VALUES (?, ?, ?, ?, ?)`, tombstone.EntityType, tombstone.EntityID, tombstone.ParentID, tombstone.SecondaryID, tombstone.DeletedAt)
	if err != nil {
		return fmt.Errorf("insert merged tombstone %s/%s: %w", tombstone.EntityType, tombstone.EntityID, err)
	}
	return nil
}

func (h *Helper) coverCloudKey(provider cloudprovider.CloudStorageProvider, asset CoverAsset) string {
	return provider.GetCloudPath(h.config.BackupUserID, filepath.ToSlash(filepath.Join(CoverDir, asset.GameID+asset.Ext)))
}

func (h *Helper) currentDeviceID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "unknown-device"
	}
	return host
}

func (h *Helper) lookupExistingGameCover(gameID string) (string, string, error) {
	path, url, err := imageutils.FindManagedCoverFile(gameID)
	if err != nil {
		return "", "", err
	}
	return url, path, nil
}
