package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/utils/imageutils"
	"lunabox/internal/vo"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	cloudSyncStateIdle    = "idle"
	cloudSyncStateSyncing = "syncing"
	cloudSyncStateSuccess = "success"
	cloudSyncStateFailed  = "failed"

	cloudSyncSchemaVersion = 1
	cloudSyncSnapshotKey   = "sync/library/latest.json"
	cloudSyncLibraryDir    = "sync/library"
	cloudSyncCoverDir      = "sync/covers"
)

type CloudSyncService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig

	mu       sync.Mutex
	syncing  bool
	debounce *time.Timer
}

type cloudSyncSnapshot struct {
	SchemaVersion  int                     `json:"schema_version"`
	RevisionID     string                  `json:"revision_id"`
	ExportedAt     time.Time               `json:"exported_at"`
	DeviceID       string                  `json:"device_id"`
	Games          []cloudSyncGame         `json:"games"`
	Categories     []cloudSyncCategory     `json:"categories"`
	GameCategories []cloudSyncRelation     `json:"game_categories"`
	PlaySessions   []cloudSyncPlaySession  `json:"play_sessions"`
	GameProgresses []cloudSyncGameProgress `json:"game_progresses"`
	GameTags       []cloudSyncGameTag      `json:"game_tags"`
	Tombstones     []cloudSyncTombstone    `json:"tombstones"`
	Covers         []cloudSyncCoverAsset   `json:"covers"`
}

type cloudSyncGame struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Company     string    `json:"company"`
	Summary     string    `json:"summary"`
	Rating      float64   `json:"rating"`
	ReleaseDate string    `json:"release_date"`
	Status      string    `json:"status"`
	SourceType  string    `json:"source_type"`
	SourceID    string    `json:"source_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type cloudSyncCategory struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Emoji     string    `json:"emoji"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cloudSyncRelation struct {
	GameID     string    `json:"game_id"`
	CategoryID string    `json:"category_id"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type cloudSyncPlaySession struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  int       `json:"duration"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cloudSyncGameProgress struct {
	ID              string    `json:"id"`
	GameID          string    `json:"game_id"`
	Chapter         string    `json:"chapter"`
	Route           string    `json:"route"`
	ProgressNote    string    `json:"progress_note"`
	SpoilerBoundary string    `json:"spoiler_boundary"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type cloudSyncGameTag struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	Weight    float64   `json:"weight"`
	IsSpoiler bool      `json:"is_spoiler"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cloudSyncTombstone struct {
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	DeletedAt  time.Time `json:"deleted_at"`
}

type cloudSyncCoverAsset struct {
	GameID    string    `json:"game_id"`
	Ext       string    `json:"ext"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cloudSyncLocalCover struct {
	Asset     cloudSyncCoverAsset
	LocalPath string
	LocalURL  string
}

type cloudSyncLocalState struct {
	Snapshot cloudSyncSnapshot
	Covers   map[string]cloudSyncLocalCover
}

type cloudSyncCandidate struct {
	Timestamp time.Time
	Source    int
	Deleted   bool
}

func NewCloudSyncService() *CloudSyncService {
	return &CloudSyncService{}
}

func (s *CloudSyncService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *CloudSyncService) GetCloudSyncStatus() vo.CloudSyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return vo.CloudSyncStatus{
		Enabled:        s.config.CloudSyncEnabled,
		Configured:     cloudprovider.IsConfigured(s.config),
		Syncing:        s.syncing,
		LastSyncTime:   s.config.LastCloudSyncTime,
		LastSyncStatus: s.config.LastCloudSyncStatus,
		LastSyncError:  s.config.LastCloudSyncError,
	}
}

func (s *CloudSyncService) NotifyLibraryChanged() {
	if !s.config.CloudSyncEnabled || !cloudprovider.IsConfigured(s.config) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.debounce != nil {
		s.debounce.Stop()
	}
	s.debounce = time.AfterFunc(2*time.Second, func() {
		if _, err := s.SyncNow(); err != nil {
			applog.LogWarningf(s.ctx, "CloudSyncService.NotifyLibraryChanged: sync failed: %v", err)
		}
	})
}

func (s *CloudSyncService) SyncNow() (vo.CloudSyncStatus, error) {
	s.mu.Lock()
	if s.syncing {
		status := s.currentStatusLocked()
		s.mu.Unlock()
		return status, nil
	}
	s.syncing = true
	s.config.LastCloudSyncStatus = cloudSyncStateSyncing
	s.config.LastCloudSyncError = ""
	_ = appconf.SaveConfig(s.config)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.syncing = false
		s.mu.Unlock()
	}()

	if !s.config.CloudSyncEnabled {
		return s.finishSync(cloudSyncStateIdle, "", nil)
	}

	provider, err := cloudprovider.NewCloudProvider(s.ctx, s.config)
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	localState, err := s.buildLocalState()
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	remoteSnapshot, remoteExists, err := s.loadRemoteSnapshot(provider)
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	merged := s.mergeSnapshots(localState.Snapshot, remoteSnapshot, remoteExists)
	coverURLs, err := s.reconcileCoverAssets(provider, localState, remoteSnapshot, remoteExists, merged)
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	if err := s.applyMergedSnapshot(merged, coverURLs); err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	if err := s.saveRemoteSnapshot(provider, merged); err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	return s.finishSync(cloudSyncStateSuccess, "", nil)
}

func (s *CloudSyncService) currentStatusLocked() vo.CloudSyncStatus {
	return vo.CloudSyncStatus{
		Enabled:        s.config.CloudSyncEnabled,
		Configured:     cloudprovider.IsConfigured(s.config),
		Syncing:        s.syncing,
		LastSyncTime:   s.config.LastCloudSyncTime,
		LastSyncStatus: s.config.LastCloudSyncStatus,
		LastSyncError:  s.config.LastCloudSyncError,
	}
}

func (s *CloudSyncService) finishSync(state, lastError string, syncErr error) (vo.CloudSyncStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.LastCloudSyncStatus = state
	s.config.LastCloudSyncError = lastError
	if state == cloudSyncStateSuccess {
		s.config.LastCloudSyncTime = time.Now().Format(time.RFC3339)
	}
	_ = appconf.SaveConfig(s.config)

	return s.currentStatusLocked(), syncErr
}

func (s *CloudSyncService) buildLocalState() (cloudSyncLocalState, error) {
	state := cloudSyncLocalState{
		Covers: make(map[string]cloudSyncLocalCover),
	}

	snapshot := cloudSyncSnapshot{
		SchemaVersion: cloudSyncSchemaVersion,
		RevisionID:    uuid.New().String(),
		ExportedAt:    time.Now(),
		DeviceID:      s.currentDeviceID(),
	}

	gameRows, err := s.db.QueryContext(s.ctx, `
		SELECT id, name, COALESCE(company, ''), COALESCE(summary, ''), COALESCE(rating, 0),
		       COALESCE(release_date, ''), COALESCE(status, 'not_started'),
		       COALESCE(source_type, ''), COALESCE(source_id, ''), created_at,
		       COALESCE(updated_at, created_at, cached_at)
		FROM games
	`)
	if err != nil {
		return state, fmt.Errorf("query games for cloud sync: %w", err)
	}
	defer gameRows.Close()

	for gameRows.Next() {
		var game cloudSyncGame
		if err := gameRows.Scan(
			&game.ID,
			&game.Name,
			&game.Company,
			&game.Summary,
			&game.Rating,
			&game.ReleaseDate,
			&game.Status,
			&game.SourceType,
			&game.SourceID,
			&game.CreatedAt,
			&game.UpdatedAt,
		); err != nil {
			return state, fmt.Errorf("scan game for cloud sync: %w", err)
		}
		snapshot.Games = append(snapshot.Games, game)

		coverPath, coverURL, err := imageutils.FindManagedCoverFile(game.ID)
		if err != nil {
			return state, fmt.Errorf("resolve cover for game %s: %w", game.ID, err)
		}
		if coverPath != "" {
			state.Covers[game.ID] = cloudSyncLocalCover{
				Asset: cloudSyncCoverAsset{
					GameID:    game.ID,
					Ext:       strings.ToLower(filepath.Ext(coverPath)),
					UpdatedAt: game.UpdatedAt,
				},
				LocalPath: coverPath,
				LocalURL:  coverURL,
			}
			snapshot.Covers = append(snapshot.Covers, state.Covers[game.ID].Asset)
		}
	}

	categoryRows, err := s.db.QueryContext(s.ctx, `
		SELECT id, name, COALESCE(emoji, ''), COALESCE(is_system, FALSE), created_at, updated_at
		FROM categories
	`)
	if err != nil {
		return state, fmt.Errorf("query categories for cloud sync: %w", err)
	}
	defer categoryRows.Close()

	for categoryRows.Next() {
		var category cloudSyncCategory
		if err := categoryRows.Scan(&category.ID, &category.Name, &category.Emoji, &category.IsSystem, &category.CreatedAt, &category.UpdatedAt); err != nil {
			return state, fmt.Errorf("scan category for cloud sync: %w", err)
		}
		snapshot.Categories = append(snapshot.Categories, category)
	}

	relationRows, err := s.db.QueryContext(s.ctx, `
		SELECT game_id, category_id, COALESCE(updated_at, CURRENT_TIMESTAMP)
		FROM game_categories
	`)
	if err != nil {
		return state, fmt.Errorf("query game categories for cloud sync: %w", err)
	}
	defer relationRows.Close()

	for relationRows.Next() {
		var relation cloudSyncRelation
		if err := relationRows.Scan(&relation.GameID, &relation.CategoryID, &relation.UpdatedAt); err != nil {
			return state, fmt.Errorf("scan game category for cloud sync: %w", err)
		}
		snapshot.GameCategories = append(snapshot.GameCategories, relation)
	}

	sessionRows, err := s.db.QueryContext(s.ctx, `
		SELECT id, game_id, start_time, COALESCE(end_time, start_time), duration,
		       COALESCE(updated_at, end_time, start_time)
		FROM play_sessions
	`)
	if err != nil {
		return state, fmt.Errorf("query play sessions for cloud sync: %w", err)
	}
	defer sessionRows.Close()

	for sessionRows.Next() {
		var session cloudSyncPlaySession
		if err := sessionRows.Scan(&session.ID, &session.GameID, &session.StartTime, &session.EndTime, &session.Duration, &session.UpdatedAt); err != nil {
			return state, fmt.Errorf("scan play session for cloud sync: %w", err)
		}
		snapshot.PlaySessions = append(snapshot.PlaySessions, session)
	}

	progressRows, err := s.db.QueryContext(s.ctx, `
		SELECT id, game_id, COALESCE(chapter, ''), COALESCE(route, ''), COALESCE(progress_note, ''),
		       COALESCE(spoiler_boundary, 'none'), COALESCE(updated_at, CURRENT_TIMESTAMP)
		FROM game_progress
	`)
	if err != nil {
		return state, fmt.Errorf("query game progress for cloud sync: %w", err)
	}
	defer progressRows.Close()

	for progressRows.Next() {
		var progress cloudSyncGameProgress
		if err := progressRows.Scan(
			&progress.ID,
			&progress.GameID,
			&progress.Chapter,
			&progress.Route,
			&progress.ProgressNote,
			&progress.SpoilerBoundary,
			&progress.UpdatedAt,
		); err != nil {
			return state, fmt.Errorf("scan game progress for cloud sync: %w", err)
		}
		snapshot.GameProgresses = append(snapshot.GameProgresses, progress)
	}

	tagRows, err := s.db.QueryContext(s.ctx, `
		SELECT id, game_id, name, source, COALESCE(weight, 1.0), COALESCE(is_spoiler, FALSE),
		       COALESCE(created_at, CURRENT_TIMESTAMP), COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		FROM game_tags
	`)
	if err != nil {
		return state, fmt.Errorf("query game tags for cloud sync: %w", err)
	}
	defer tagRows.Close()

	for tagRows.Next() {
		var tag cloudSyncGameTag
		if err := tagRows.Scan(
			&tag.ID,
			&tag.GameID,
			&tag.Name,
			&tag.Source,
			&tag.Weight,
			&tag.IsSpoiler,
			&tag.CreatedAt,
			&tag.UpdatedAt,
		); err != nil {
			return state, fmt.Errorf("scan game tag for cloud sync: %w", err)
		}
		snapshot.GameTags = append(snapshot.GameTags, tag)
	}

	tombstoneRows, err := s.db.QueryContext(s.ctx, `
		SELECT entity_type, entity_id, deleted_at
		FROM sync_tombstones
		WHERE COALESCE(parent_id, '') = '' AND COALESCE(secondary_id, '') = ''
	`)
	if err != nil {
		return state, fmt.Errorf("query tombstones for cloud sync: %w", err)
	}
	defer tombstoneRows.Close()

	for tombstoneRows.Next() {
		var tombstone cloudSyncTombstone
		if err := tombstoneRows.Scan(&tombstone.EntityType, &tombstone.EntityID, &tombstone.DeletedAt); err != nil {
			return state, fmt.Errorf("scan tombstone for cloud sync: %w", err)
		}
		snapshot.Tombstones = append(snapshot.Tombstones, tombstone)
	}

	sort.Slice(snapshot.Games, func(i, j int) bool { return snapshot.Games[i].ID < snapshot.Games[j].ID })
	sort.Slice(snapshot.Categories, func(i, j int) bool { return snapshot.Categories[i].ID < snapshot.Categories[j].ID })
	sort.Slice(snapshot.GameCategories, func(i, j int) bool {
		left := snapshot.GameCategories[i].GameID + "::" + snapshot.GameCategories[i].CategoryID
		right := snapshot.GameCategories[j].GameID + "::" + snapshot.GameCategories[j].CategoryID
		return left < right
	})
	sort.Slice(snapshot.PlaySessions, func(i, j int) bool { return snapshot.PlaySessions[i].ID < snapshot.PlaySessions[j].ID })
	sort.Slice(snapshot.GameProgresses, func(i, j int) bool { return snapshot.GameProgresses[i].ID < snapshot.GameProgresses[j].ID })
	sort.Slice(snapshot.GameTags, func(i, j int) bool {
		return tagTombstoneID(snapshot.GameTags[i].GameID, snapshot.GameTags[i].Source, snapshot.GameTags[i].Name) <
			tagTombstoneID(snapshot.GameTags[j].GameID, snapshot.GameTags[j].Source, snapshot.GameTags[j].Name)
	})
	sort.Slice(snapshot.Tombstones, func(i, j int) bool {
		left := snapshot.Tombstones[i].EntityType + "::" + snapshot.Tombstones[i].EntityID
		right := snapshot.Tombstones[j].EntityType + "::" + snapshot.Tombstones[j].EntityID
		return left < right
	})
	sort.Slice(snapshot.Covers, func(i, j int) bool { return snapshot.Covers[i].GameID < snapshot.Covers[j].GameID })

	state.Snapshot = snapshot
	return state, nil
}

func (s *CloudSyncService) loadRemoteSnapshot(provider cloudprovider.CloudStorageProvider) (cloudSyncSnapshot, bool, error) {
	var snapshot cloudSyncSnapshot

	prefix := provider.GetCloudPath(s.config.BackupUserID, cloudSyncLibraryDir+"/")
	keys, err := provider.ListObjects(s.ctx, prefix)
	if err != nil {
		return snapshot, false, fmt.Errorf("list cloud sync snapshots: %w", err)
	}

	latestKey := provider.GetCloudPath(s.config.BackupUserID, cloudSyncSnapshotKey)
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

	if err := provider.DownloadFile(s.ctx, latestKey, tempPath); err != nil {
		return snapshot, false, fmt.Errorf("download cloud sync snapshot: %w", err)
	}

	raw, err := os.ReadFile(tempPath)
	if err != nil {
		return snapshot, false, fmt.Errorf("read cloud sync snapshot: %w", err)
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, false, fmt.Errorf("decode cloud sync snapshot: %w", err)
	}

	return snapshot, true, nil
}

func (s *CloudSyncService) saveRemoteSnapshot(provider cloudprovider.CloudStorageProvider, snapshot cloudSyncSnapshot) error {
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cloud sync snapshot: %w", err)
	}

	folderPath := provider.GetCloudPath(s.config.BackupUserID, cloudSyncLibraryDir)
	if err := provider.EnsureDir(s.ctx, folderPath); err != nil {
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

	latestKey := provider.GetCloudPath(s.config.BackupUserID, cloudSyncSnapshotKey)
	if err := provider.UploadFile(s.ctx, latestKey, tempPath); err != nil {
		return fmt.Errorf("upload cloud sync snapshot: %w", err)
	}

	return nil
}

func (s *CloudSyncService) RunStartupSync() {
	if !s.config.CloudSyncEnabled || !cloudprovider.IsConfigured(s.config) {
		return
	}

	go func() {
		if _, err := s.SyncNow(); err != nil {
			applog.LogWarningf(s.ctx, "CloudSyncService.RunStartupSync: sync failed: %v", err)
		}
	}()
}

func (s *CloudSyncService) mergeSnapshots(local, remote cloudSyncSnapshot, remoteExists bool) cloudSyncSnapshot {
	if !remoteExists {
		local.RevisionID = uuid.New().String()
		local.ExportedAt = time.Now()
		local.DeviceID = s.currentDeviceID()
		return local
	}

	merged := cloudSyncSnapshot{
		SchemaVersion: cloudSyncSchemaVersion,
		RevisionID:    uuid.New().String(),
		ExportedAt:    time.Now(),
		DeviceID:      s.currentDeviceID(),
	}

	localGameMap := mapGames(local.Games)
	remoteGameMap := mapGames(remote.Games)
	localGameTombstones := mapTombstones(local.Tombstones, cloudSyncEntityGame)
	remoteGameTombstones := mapTombstones(remote.Tombstones, cloudSyncEntityGame)
	for _, id := range unionKeys4(localGameMap, remoteGameMap, localGameTombstones, remoteGameTombstones) {
		if game, ok, deletedAt := mergeRecord(localGameMap[id], remoteGameMap[id], localGameTombstones[id], remoteGameTombstones[id]); ok {
			merged.Games = append(merged.Games, game)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, cloudSyncTombstone{EntityType: cloudSyncEntityGame, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localCategoryMap := mapCategories(local.Categories)
	remoteCategoryMap := mapCategories(remote.Categories)
	localCategoryTombstones := mapTombstones(local.Tombstones, cloudSyncEntityCategory)
	remoteCategoryTombstones := mapTombstones(remote.Tombstones, cloudSyncEntityCategory)
	for _, id := range unionKeys4(localCategoryMap, remoteCategoryMap, localCategoryTombstones, remoteCategoryTombstones) {
		if category, ok, deletedAt := mergeCategory(localCategoryMap[id], remoteCategoryMap[id], localCategoryTombstones[id], remoteCategoryTombstones[id]); ok {
			merged.Categories = append(merged.Categories, category)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, cloudSyncTombstone{EntityType: cloudSyncEntityCategory, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localRelationMap := mapRelations(local.GameCategories)
	remoteRelationMap := mapRelations(remote.GameCategories)
	localRelationTombstones := mapTombstones(local.Tombstones, cloudSyncEntityGameCategory)
	remoteRelationTombstones := mapTombstones(remote.Tombstones, cloudSyncEntityGameCategory)
	for _, id := range unionKeys4(localRelationMap, remoteRelationMap, localRelationTombstones, remoteRelationTombstones) {
		if relation, ok, deletedAt := mergeRelation(localRelationMap[id], remoteRelationMap[id], localRelationTombstones[id], remoteRelationTombstones[id]); ok {
			merged.GameCategories = append(merged.GameCategories, relation)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, cloudSyncTombstone{EntityType: cloudSyncEntityGameCategory, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localSessionMap := mapPlaySessions(local.PlaySessions)
	remoteSessionMap := mapPlaySessions(remote.PlaySessions)
	localSessionTombstones := mapTombstones(local.Tombstones, cloudSyncEntityPlaySession)
	remoteSessionTombstones := mapTombstones(remote.Tombstones, cloudSyncEntityPlaySession)
	for _, id := range unionKeys4(localSessionMap, remoteSessionMap, localSessionTombstones, remoteSessionTombstones) {
		if session, ok, deletedAt := mergeSession(localSessionMap[id], remoteSessionMap[id], localSessionTombstones[id], remoteSessionTombstones[id]); ok {
			merged.PlaySessions = append(merged.PlaySessions, session)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, cloudSyncTombstone{EntityType: cloudSyncEntityPlaySession, EntityID: id, DeletedAt: deletedAt})
		}
	}

	mergedGameMap := mapGames(merged.Games)

	localProgressMap := mapGameProgresses(local.GameProgresses)
	remoteProgressMap := mapGameProgresses(remote.GameProgresses)
	localProgressTombstones := mapTombstones(local.Tombstones, cloudSyncEntityGameProgress)
	remoteProgressTombstones := mapTombstones(remote.Tombstones, cloudSyncEntityGameProgress)
	for _, id := range unionKeys4(localProgressMap, remoteProgressMap, localProgressTombstones, remoteProgressTombstones) {
		if progress, ok, deletedAt := mergeGameProgress(localProgressMap[id], remoteProgressMap[id], localProgressTombstones[id], remoteProgressTombstones[id]); ok {
			if _, gameExists := mergedGameMap[progress.GameID]; gameExists {
				merged.GameProgresses = append(merged.GameProgresses, progress)
			}
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, cloudSyncTombstone{EntityType: cloudSyncEntityGameProgress, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localTagMap := mapGameTags(local.GameTags)
	remoteTagMap := mapGameTags(remote.GameTags)
	localTagTombstones := mapTombstones(local.Tombstones, cloudSyncEntityGameTag)
	remoteTagTombstones := mapTombstones(remote.Tombstones, cloudSyncEntityGameTag)
	for _, id := range unionKeys4(localTagMap, remoteTagMap, localTagTombstones, remoteTagTombstones) {
		if tag, ok, deletedAt := mergeGameTag(localTagMap[id], remoteTagMap[id], localTagTombstones[id], remoteTagTombstones[id]); ok {
			if _, gameExists := mergedGameMap[tag.GameID]; gameExists {
				merged.GameTags = append(merged.GameTags, tag)
			}
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, cloudSyncTombstone{EntityType: cloudSyncEntityGameTag, EntityID: id, DeletedAt: deletedAt})
		}
	}

	merged.Covers = s.mergeCovers(local, remote, merged.Games)

	sort.Slice(merged.Games, func(i, j int) bool { return merged.Games[i].ID < merged.Games[j].ID })
	sort.Slice(merged.Categories, func(i, j int) bool { return merged.Categories[i].ID < merged.Categories[j].ID })
	sort.Slice(merged.GameCategories, func(i, j int) bool {
		left := merged.GameCategories[i].GameID + "::" + merged.GameCategories[i].CategoryID
		right := merged.GameCategories[j].GameID + "::" + merged.GameCategories[j].CategoryID
		return left < right
	})
	sort.Slice(merged.PlaySessions, func(i, j int) bool { return merged.PlaySessions[i].ID < merged.PlaySessions[j].ID })
	sort.Slice(merged.GameProgresses, func(i, j int) bool { return merged.GameProgresses[i].ID < merged.GameProgresses[j].ID })
	sort.Slice(merged.GameTags, func(i, j int) bool {
		return tagTombstoneID(merged.GameTags[i].GameID, merged.GameTags[i].Source, merged.GameTags[i].Name) <
			tagTombstoneID(merged.GameTags[j].GameID, merged.GameTags[j].Source, merged.GameTags[j].Name)
	})
	sort.Slice(merged.Tombstones, func(i, j int) bool {
		left := merged.Tombstones[i].EntityType + "::" + merged.Tombstones[i].EntityID
		right := merged.Tombstones[j].EntityType + "::" + merged.Tombstones[j].EntityID
		return left < right
	})
	sort.Slice(merged.Covers, func(i, j int) bool { return merged.Covers[i].GameID < merged.Covers[j].GameID })

	return merged
}

func (s *CloudSyncService) mergeCovers(local, remote cloudSyncSnapshot, mergedGames []cloudSyncGame) []cloudSyncCoverAsset {
	localCovers := mapCoverAssets(local.Covers)
	remoteCovers := mapCoverAssets(remote.Covers)
	localGames := mapGames(local.Games)
	remoteGames := mapGames(remote.Games)

	merged := make([]cloudSyncCoverAsset, 0, len(mergedGames))
	for _, game := range mergedGames {
		localGame, hasLocalGame := localGames[game.ID]
		remoteGame, hasRemoteGame := remoteGames[game.ID]
		localCover, hasLocalCover := localCovers[game.ID]
		remoteCover, hasRemoteCover := remoteCovers[game.ID]

		best := cloudSyncCandidate{}
		hasBest := false
		bestDeleted := false
		chosen := cloudSyncCoverAsset{}

		if hasLocalCover {
			best = cloudSyncCandidate{Timestamp: localCover.UpdatedAt, Source: 0}
			chosen = localCover
			hasBest = true
		}
		if hasRemoteCover {
			candidate := cloudSyncCandidate{Timestamp: remoteCover.UpdatedAt, Source: 1}
			if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
				best = candidate
				chosen = remoteCover
				hasBest = true
				bestDeleted = false
			}
		}
		if hasLocalGame && !hasLocalCover {
			candidate := cloudSyncCandidate{Timestamp: localGame.UpdatedAt, Source: 0, Deleted: true}
			if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
				best = candidate
				hasBest = true
				bestDeleted = true
			}
		}
		if hasRemoteGame && !hasRemoteCover {
			candidate := cloudSyncCandidate{Timestamp: remoteGame.UpdatedAt, Source: 1, Deleted: true}
			if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
				best = candidate
				hasBest = true
				bestDeleted = true
			}
		}

		if hasBest && !bestDeleted {
			merged = append(merged, chosen)
		}
	}

	return merged
}

func (s *CloudSyncService) reconcileCoverAssets(provider cloudprovider.CloudStorageProvider, local cloudSyncLocalState, remote cloudSyncSnapshot, remoteExists bool, merged cloudSyncSnapshot) (map[string]string, error) {
	coverURLs := make(map[string]string)
	localAssets := local.Covers
	remoteAssets := mapCoverAssets(remote.Covers)
	mergedAssets := mapCoverAssets(merged.Covers)

	folderPath := provider.GetCloudPath(s.config.BackupUserID, cloudSyncCoverDir)
	if err := provider.EnsureDir(s.ctx, folderPath); err != nil {
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
				if err := provider.UploadFile(s.ctx, s.coverCloudKey(provider, asset), localAsset.LocalPath); err != nil {
					return coverURLs, fmt.Errorf("upload cover for game %s: %w", game.ID, err)
				}
			}
		case hasMerged && hasRemote && remoteAsset.Ext == asset.Ext && remoteAsset.UpdatedAt.Equal(asset.UpdatedAt):
			destPath, localURL, err := imageutils.PrepareManagedCoverDestination(game.ID, asset.Ext)
			if err != nil {
				return coverURLs, fmt.Errorf("prepare cover destination for game %s: %w", game.ID, err)
			}
			if err := provider.DownloadFile(s.ctx, s.coverCloudKey(provider, asset), destPath); err != nil {
				return coverURLs, fmt.Errorf("download cover for game %s: %w", game.ID, err)
			}
			coverURLs[game.ID] = localURL
		case hasMerged && hasLocal:
			coverURLs[game.ID] = localAsset.LocalURL
			if err := provider.UploadFile(s.ctx, s.coverCloudKey(provider, asset), localAsset.LocalPath); err != nil {
				return coverURLs, fmt.Errorf("upload local cover fallback for game %s: %w", game.ID, err)
			}
		case hasMerged && hasRemote:
			destPath, localURL, err := imageutils.PrepareManagedCoverDestination(game.ID, asset.Ext)
			if err != nil {
				return coverURLs, fmt.Errorf("prepare remote cover destination for game %s: %w", game.ID, err)
			}
			if err := provider.DownloadFile(s.ctx, s.coverCloudKey(provider, asset), destPath); err != nil {
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
			if err := provider.DeleteObject(s.ctx, s.coverCloudKey(provider, asset)); err != nil {
				applog.LogWarningf(s.ctx, "CloudSyncService: failed to delete stale remote cover for game %s: %v", gameID, err)
			}
		}
	}

	return coverURLs, nil
}

func (s *CloudSyncService) applyMergedSnapshot(snapshot cloudSyncSnapshot, coverURLs map[string]string) error {
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud sync apply tx: %w", err)
	}
	defer tx.Rollback()

	for _, tombstone := range snapshot.Tombstones {
		switch tombstone.EntityType {
		case cloudSyncEntityGameCategory:
			parts := strings.SplitN(tombstone.EntityID, "::", 2)
			if len(parts) == 2 {
				if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_categories WHERE game_id = ? AND category_id = ?`, parts[0], parts[1]); err != nil {
					return fmt.Errorf("delete synced relation: %w", err)
				}
			}
		case cloudSyncEntityPlaySession:
			if _, err := tx.ExecContext(s.ctx, `DELETE FROM play_sessions WHERE id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced play session: %w", err)
			}
		case cloudSyncEntityGameProgress:
			if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_progress WHERE id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced game progress: %w", err)
			}
		case cloudSyncEntityGameTag:
			parts := strings.SplitN(tombstone.EntityID, "::", 3)
			if len(parts) == 3 {
				if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_tags WHERE game_id = ? AND source = ? AND name = ?`, parts[0], parts[1], parts[2]); err != nil {
					return fmt.Errorf("delete synced game tag: %w", err)
				}
			}
		case cloudSyncEntityCategory:
			if tombstone.EntityID != systemFavoritesCategoryID {
				if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_categories WHERE category_id = ?`, tombstone.EntityID); err != nil {
					return fmt.Errorf("delete synced category relations: %w", err)
				}
				if _, err := tx.ExecContext(s.ctx, `DELETE FROM categories WHERE id = ?`, tombstone.EntityID); err != nil {
					return fmt.Errorf("delete synced category: %w", err)
				}
			}
		case cloudSyncEntityGame:
			if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_categories WHERE game_id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced game relations: %w", err)
			}
			if _, err := tx.ExecContext(s.ctx, `DELETE FROM play_sessions WHERE game_id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced game sessions: %w", err)
			}
			if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_progress WHERE game_id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced game progress: %w", err)
			}
			if _, err := tx.ExecContext(s.ctx, `DELETE FROM game_tags WHERE game_id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced game tags: %w", err)
			}
			if _, err := tx.ExecContext(s.ctx, `DELETE FROM games WHERE id = ?`, tombstone.EntityID); err != nil {
				return fmt.Errorf("delete synced game: %w", err)
			}
		}
	}

	for _, category := range snapshot.Categories {
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO categories (id, name, emoji, is_system, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				emoji = EXCLUDED.emoji,
				is_system = EXCLUDED.is_system,
				created_at = EXCLUDED.created_at,
				updated_at = EXCLUDED.updated_at
		`, category.ID, category.Name, category.Emoji, category.IsSystem, category.CreatedAt, category.UpdatedAt); err != nil {
			return fmt.Errorf("upsert synced category %s: %w", category.ID, err)
		}
	}

	for _, game := range snapshot.Games {
		coverURL := coverURLs[game.ID]
		if coverURL == "" {
			url, _, _ := s.lookupExistingGameCover(game.ID)
			coverURL = url
		}
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO games (
				id, name, cover_url, company, summary, rating, release_date, path, save_path,
				process_name, status, source_type, cached_at, source_id, created_at, updated_at,
				use_locale_emulator, use_magpie
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, '', '', '', ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, FALSE, FALSE)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				cover_url = EXCLUDED.cover_url,
				company = EXCLUDED.company,
				summary = EXCLUDED.summary,
				rating = EXCLUDED.rating,
				release_date = EXCLUDED.release_date,
				status = EXCLUDED.status,
				source_type = EXCLUDED.source_type,
				source_id = EXCLUDED.source_id,
				created_at = EXCLUDED.created_at,
				updated_at = EXCLUDED.updated_at
		`, game.ID, game.Name, coverURL, game.Company, game.Summary, game.Rating, game.ReleaseDate, game.Status, game.SourceType, game.SourceID, game.CreatedAt, game.UpdatedAt); err != nil {
			return fmt.Errorf("upsert synced game %s: %w", game.ID, err)
		}
	}

	for _, relation := range snapshot.GameCategories {
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO game_categories (game_id, category_id, updated_at)
			VALUES (?, ?, ?)
			ON CONFLICT (game_id, category_id) DO UPDATE SET updated_at = EXCLUDED.updated_at
		`, relation.GameID, relation.CategoryID, relation.UpdatedAt); err != nil {
			return fmt.Errorf("upsert synced relation %s/%s: %w", relation.GameID, relation.CategoryID, err)
		}
	}

	for _, session := range snapshot.PlaySessions {
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				game_id = EXCLUDED.game_id,
				start_time = EXCLUDED.start_time,
				end_time = EXCLUDED.end_time,
				duration = EXCLUDED.duration,
				updated_at = EXCLUDED.updated_at
		`, session.ID, session.GameID, session.StartTime, session.EndTime, session.Duration, session.UpdatedAt); err != nil {
			return fmt.Errorf("upsert synced play session %s: %w", session.ID, err)
		}
	}

	for _, progress := range snapshot.GameProgresses {
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO game_progress (id, game_id, chapter, route, progress_note, spoiler_boundary, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				game_id = EXCLUDED.game_id,
				chapter = EXCLUDED.chapter,
				route = EXCLUDED.route,
				progress_note = EXCLUDED.progress_note,
				spoiler_boundary = EXCLUDED.spoiler_boundary,
				updated_at = EXCLUDED.updated_at
		`, progress.ID, progress.GameID, progress.Chapter, progress.Route, progress.ProgressNote, progress.SpoilerBoundary, progress.UpdatedAt); err != nil {
			return fmt.Errorf("upsert synced game progress %s: %w", progress.ID, err)
		}
	}

	for _, tag := range snapshot.GameTags {
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (game_id, name, source) DO UPDATE SET
				id = EXCLUDED.id,
				weight = EXCLUDED.weight,
				is_spoiler = EXCLUDED.is_spoiler,
				created_at = EXCLUDED.created_at,
				updated_at = EXCLUDED.updated_at
		`, tag.ID, tag.GameID, tag.Name, tag.Source, tag.Weight, tag.IsSpoiler, tag.CreatedAt, tag.UpdatedAt); err != nil {
			return fmt.Errorf("upsert synced game tag %s/%s/%s: %w", tag.GameID, tag.Source, tag.Name, err)
		}
	}

	if _, err := tx.ExecContext(s.ctx, `DELETE FROM sync_tombstones`); err != nil {
		return fmt.Errorf("clear local sync tombstones: %w", err)
	}
	for _, tombstone := range snapshot.Tombstones {
		if _, err := tx.ExecContext(s.ctx, `
			INSERT INTO sync_tombstones (entity_type, entity_id, parent_id, secondary_id, deleted_at)
			VALUES (?, ?, '', '', ?)
		`, tombstone.EntityType, tombstone.EntityID, tombstone.DeletedAt); err != nil {
			return fmt.Errorf("insert merged tombstone %s/%s: %w", tombstone.EntityType, tombstone.EntityID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cloud sync apply tx: %w", err)
	}

	return nil
}

func (s *CloudSyncService) coverCloudKey(provider cloudprovider.CloudStorageProvider, asset cloudSyncCoverAsset) string {
	return provider.GetCloudPath(s.config.BackupUserID, filepath.ToSlash(filepath.Join(cloudSyncCoverDir, asset.GameID+asset.Ext)))
}

func (s *CloudSyncService) currentDeviceID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "unknown-device"
	}
	return host
}

func (s *CloudSyncService) lookupExistingGameCover(gameID string) (string, string, error) {
	path, url, err := imageutils.FindManagedCoverFile(gameID)
	if err != nil {
		return "", "", err
	}
	return url, path, nil
}

func mapGames(items []cloudSyncGame) map[string]cloudSyncGame {
	result := make(map[string]cloudSyncGame, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapCategories(items []cloudSyncCategory) map[string]cloudSyncCategory {
	result := make(map[string]cloudSyncCategory, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapRelations(items []cloudSyncRelation) map[string]cloudSyncRelation {
	result := make(map[string]cloudSyncRelation, len(items))
	for _, item := range items {
		result[relationTombstoneID(item.GameID, item.CategoryID)] = item
	}
	return result
}

func mapPlaySessions(items []cloudSyncPlaySession) map[string]cloudSyncPlaySession {
	result := make(map[string]cloudSyncPlaySession, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapGameProgresses(items []cloudSyncGameProgress) map[string]cloudSyncGameProgress {
	result := make(map[string]cloudSyncGameProgress, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapGameTags(items []cloudSyncGameTag) map[string]cloudSyncGameTag {
	result := make(map[string]cloudSyncGameTag, len(items))
	for _, item := range items {
		result[tagTombstoneID(item.GameID, item.Source, item.Name)] = item
	}
	return result
}

func mapCoverAssets(items []cloudSyncCoverAsset) map[string]cloudSyncCoverAsset {
	result := make(map[string]cloudSyncCoverAsset, len(items))
	for _, item := range items {
		result[item.GameID] = item
	}
	return result
}

func mapTombstones(items []cloudSyncTombstone, entityType string) map[string]time.Time {
	result := make(map[string]time.Time)
	for _, item := range items {
		if item.EntityType == entityType {
			result[item.EntityID] = item.DeletedAt
		}
	}
	return result
}

func unionKeys4[T any, U any](left map[string]T, right map[string]T, third map[string]U, fourth map[string]U) []string {
	seen := make(map[string]struct{}, len(left)+len(right)+len(third)+len(fourth))
	for key := range left {
		seen[key] = struct{}{}
	}
	for key := range right {
		seen[key] = struct{}{}
	}
	for key := range third {
		seen[key] = struct{}{}
	}
	for key := range fourth {
		seen[key] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compareCloudSyncCandidate(left, right cloudSyncCandidate) int {
	switch {
	case left.Timestamp.After(right.Timestamp):
		return 1
	case right.Timestamp.After(left.Timestamp):
		return -1
	}
	if left.Source != right.Source {
		if left.Source > right.Source {
			return 1
		}
		return -1
	}
	if left.Deleted != right.Deleted {
		if left.Deleted {
			return 1
		}
		return -1
	}
	return 0
}

func mergeRecord(local, remote cloudSyncGame, localDeleted, remoteDeleted time.Time) (cloudSyncGame, bool, time.Time) {
	best := cloudSyncCandidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := cloudSyncGame{}

	if !local.UpdatedAt.IsZero() {
		best = cloudSyncCandidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return cloudSyncGame{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeCategory(local, remote cloudSyncCategory, localDeleted, remoteDeleted time.Time) (cloudSyncCategory, bool, time.Time) {
	best := cloudSyncCandidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := cloudSyncCategory{}

	if !local.UpdatedAt.IsZero() {
		best = cloudSyncCandidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return cloudSyncCategory{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeRelation(local, remote cloudSyncRelation, localDeleted, remoteDeleted time.Time) (cloudSyncRelation, bool, time.Time) {
	best := cloudSyncCandidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := cloudSyncRelation{}

	if !local.UpdatedAt.IsZero() {
		best = cloudSyncCandidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return cloudSyncRelation{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeSession(local, remote cloudSyncPlaySession, localDeleted, remoteDeleted time.Time) (cloudSyncPlaySession, bool, time.Time) {
	best := cloudSyncCandidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := cloudSyncPlaySession{}

	if !local.UpdatedAt.IsZero() {
		best = cloudSyncCandidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return cloudSyncPlaySession{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeGameProgress(local, remote cloudSyncGameProgress, localDeleted, remoteDeleted time.Time) (cloudSyncGameProgress, bool, time.Time) {
	best := cloudSyncCandidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := cloudSyncGameProgress{}

	if !local.UpdatedAt.IsZero() {
		best = cloudSyncCandidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return cloudSyncGameProgress{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeGameTag(local, remote cloudSyncGameTag, localDeleted, remoteDeleted time.Time) (cloudSyncGameTag, bool, time.Time) {
	best := cloudSyncCandidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := cloudSyncGameTag{}

	if !local.UpdatedAt.IsZero() {
		best = cloudSyncCandidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := cloudSyncCandidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCloudSyncCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return cloudSyncGameTag{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}
