package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/importpath"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service/importer"
	"lunabox/internal/utils/metadata"
	"strings"
	"sync"
	"time"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"
)

const (
	importStatusNew               = "new"
	importStatusExistsPath        = "exists_path"
	importStatusExistsSource      = "exists_source"
	importStatusExistsNamePath    = "exists_name_path"
	importStatusPossibleDuplicate = "possible_duplicate"
	importCoverWorkerCount        = 4
)

type importGameRef struct {
	ID         string
	Name       string
	Path       string
	SourceType enums.SourceType
	SourceID   string
	CreatedAt  time.Time
}

type importIndex struct {
	byPath     map[string]importGameRef
	bySource   map[string]importGameRef
	byNamePath map[string]importGameRef
	byName     map[string]importGameRef
}

type importItem struct {
	Game        models.Game
	Tags        []metadata.TagItem
	Sessions    []models.PlaySession
	Source      enums.SourceType
	Action      string
	CoverLoader func(models.Game) (string, error)
}

func normalizeImportPath(path string) string {
	return importpath.Normalize(path)
}

func importPathContainsNormalized(parentPath string, childPath string) bool {
	return importpath.ContainsNormalized(parentPath, childPath)
}

func normalizeImportName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func importSourceKey(source enums.SourceType, sourceID string) string {
	sourceID = strings.ToLower(strings.TrimSpace(sourceID))
	if source == "" || sourceID == "" {
		return ""
	}
	return strings.ToLower(string(source)) + "\x00" + sourceID
}

func importNamePathKey(name string, path string) string {
	nameKey := normalizeImportName(name)
	pathKey := normalizeImportPath(path)
	if nameKey == "" || pathKey == "" {
		return ""
	}
	return nameKey + "\x00" + pathKey
}

func newImportIndex(refs []importGameRef) importIndex {
	idx := importIndex{
		byPath:     make(map[string]importGameRef, len(refs)),
		bySource:   make(map[string]importGameRef, len(refs)),
		byNamePath: make(map[string]importGameRef, len(refs)),
		byName:     make(map[string]importGameRef, len(refs)),
	}

	for _, ref := range refs {
		idx.add(ref)
	}
	return idx
}

func (idx importIndex) add(ref importGameRef) {
	if key := normalizeImportPath(ref.Path); key != "" {
		if _, exists := idx.byPath[key]; !exists {
			idx.byPath[key] = ref
		}
	}
	if key := importSourceKey(ref.SourceType, ref.SourceID); key != "" {
		if _, exists := idx.bySource[key]; !exists {
			idx.bySource[key] = ref
		}
	}
	if key := importNamePathKey(ref.Name, ref.Path); key != "" {
		if _, exists := idx.byNamePath[key]; !exists {
			idx.byNamePath[key] = ref
		}
	}
	if key := normalizeImportName(ref.Name); key != "" {
		if _, exists := idx.byName[key]; !exists {
			idx.byName[key] = ref
		}
	}
}

func (idx importIndex) findByPath(path string) (importGameRef, bool) {
	ref, ok := idx.byPath[normalizeImportPath(path)]
	return ref, ok
}

func (idx importIndex) findByPathConflict(path string) (importGameRef, bool) {
	pathKey := normalizeImportPath(path)
	if pathKey == "" {
		return importGameRef{}, false
	}

	if ref, ok := idx.byPath[pathKey]; ok {
		return ref, true
	}

	for existingPath, ref := range idx.byPath {
		if importPathContainsNormalized(pathKey, existingPath) || importPathContainsNormalized(existingPath, pathKey) {
			return ref, true
		}
	}
	return importGameRef{}, false
}

func (idx importIndex) findBySource(source enums.SourceType, sourceID string) (importGameRef, bool) {
	key := importSourceKey(source, sourceID)
	if key == "" {
		return importGameRef{}, false
	}
	ref, ok := idx.bySource[key]
	return ref, ok
}

func (idx importIndex) findByNamePath(name string, path string) (importGameRef, bool) {
	key := importNamePathKey(name, path)
	if key == "" {
		return importGameRef{}, false
	}
	ref, ok := idx.byNamePath[key]
	return ref, ok
}

func (idx importIndex) findByName(name string) (importGameRef, bool) {
	ref, ok := idx.byName[normalizeImportName(name)]
	return ref, ok
}

func (s *GameService) listImportGameRefs() ([]importGameRef, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	rows, err := s.db.QueryContext(s.ctx, `
		SELECT
			id,
			COALESCE(name, ''),
			COALESCE(path, ''),
			COALESCE(source_type, ''),
			COALESCE(source_id, ''),
			COALESCE(created_at, cached_at, updated_at, CURRENT_TIMESTAMP)
		FROM games
	`)
	if err != nil {
		return nil, fmt.Errorf("query import game refs: %w", err)
	}
	defer rows.Close()

	refs := make([]importGameRef, 0)
	for rows.Next() {
		var ref importGameRef
		var sourceType string
		if err := rows.Scan(&ref.ID, &ref.Name, &ref.Path, &sourceType, &ref.SourceID, &ref.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan import game ref: %w", err)
		}
		ref.SourceType = enums.SourceType(sourceType)
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate import game refs: %w", err)
	}
	return refs, nil
}

func (s *ImportService) loadImportIndex() (importIndex, error) {
	refs, err := s.gameService.listImportGameRefs()
	if err != nil {
		return importIndex{}, err
	}
	return newImportIndex(refs), nil
}

func (s *ImportService) allowDuplicateMetadataImport() bool {
	return s.config != nil && s.config.AllowDuplicateMetadataImport
}

func (s *ImportService) listImportGamesForImporter() ([]models.Game, error) {
	refs, err := s.gameService.listImportGameRefs()
	if err != nil {
		return nil, err
	}
	games := make([]models.Game, 0, len(refs))
	for _, ref := range refs {
		games = append(games, models.Game{
			ID:         ref.ID,
			Name:       ref.Name,
			Path:       ref.Path,
			SourceType: ref.SourceType,
			SourceID:   ref.SourceID,
			CreatedAt:  ref.CreatedAt,
		})
	}
	return games, nil
}

func (s *ImportService) addImporterItems(items []importer.ImportItem) (importer.ImportResult, error) {
	result := importer.ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}
	if len(items) == 0 {
		return result, nil
	}

	startedAt := time.Now()
	stepStartedAt := time.Now()
	idx, err := s.loadImportIndex()
	if err != nil {
		return result, fmt.Errorf("加载导入索引失败: %w", err)
	}
	applog.LogInfof(s.ctx, "addImporterItems: loaded import index for items=%d elapsed=%s", len(items), time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	toCommit := make([]importItem, 0, len(items))
	for _, item := range items {
		game := item.Source.Game
		displayName := item.DisplayName
		if displayName == "" {
			displayName = game.Name
		}
		if item.Path == "" {
			item.Path = game.Path
		}

		source := item.Source.Source
		if source == "" {
			source = game.SourceType
		}
		if game.SourceType == "" {
			game.SourceType = source
		}
		if game.ID == "" {
			game.ID = uuid.New().String()
		}

		action := item.Action
		if action == "" {
			action = importer.ImportActionCreate
		}
		if action == importer.ImportActionUpdateExisting {
			if item.ExistingGameID == "" {
				result.Failed++
				result.FailedNames = append(result.FailedNames, displayName)
				continue
			}
			game.ID = item.ExistingGameID
			for i := range item.Sessions {
				item.Sessions[i].GameID = item.ExistingGameID
			}
		} else {
			if ref, ok := idx.findByPathConflict(item.Path); ok {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, displayName+" (路径已存在: "+ref.Name+")")
				continue
			}
		}

		if !s.allowDuplicateMetadataImport() {
			if ref, ok := idx.findBySource(source, game.SourceID); ok && (action != importer.ImportActionUpdateExisting || ref.ID != item.ExistingGameID) {
				result.Skipped++
				result.SkippedNames = append(result.SkippedNames, displayName+" (元数据已存在: "+ref.Name+")")
				continue
			}
		}
		if ref, ok := idx.findByNamePath(game.Name, item.Path); ok && (action != importer.ImportActionUpdateExisting || ref.ID != item.ExistingGameID) {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, displayName+" (已存在: "+ref.Name+")")
			continue
		}

		converted := importItem{
			Game:        game,
			Tags:        item.Source.Tags,
			Sessions:    item.Sessions,
			Source:      source,
			Action:      action,
			CoverLoader: item.CoverLoader,
		}
		toCommit = append(toCommit, converted)
		if action == importer.ImportActionCreate {
			idx.add(importGameRef{
				ID:         game.ID,
				Name:       game.Name,
				Path:       game.Path,
				SourceType: game.SourceType,
				SourceID:   game.SourceID,
				CreatedAt:  game.CreatedAt,
			})
		}
	}
	applog.LogInfof(s.ctx, "addImporterItems: filtered commit_items=%d skipped=%d elapsed=%s", len(toCommit), result.Skipped, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	success, sessionsImported, err := s.commitImportedItems(toCommit)
	if err != nil {
		result.Failed += len(toCommit)
		for _, item := range toCommit {
			result.FailedNames = append(result.FailedNames, item.Game.Name)
		}
		return result, err
	}
	result.Success += success
	result.SessionsImported += sessionsImported
	applog.LogInfof(s.ctx, "addImporterItems: committed success=%d skipped=%d failed=%d sessions=%d elapsed=%s total=%s", result.Success, result.Skipped, result.Failed, result.SessionsImported, time.Since(stepStartedAt), time.Since(startedAt))
	return result, nil
}

func annotateScanCandidate(candidate vo.BatchImportCandidate, idx importIndex) vo.BatchImportCandidate {
	candidate.ImportStatus = importStatusNew
	candidate.IsSelected = true

	pathsToCheck := candidate.Executables
	if len(pathsToCheck) == 0 && candidate.SelectedExe != "" {
		pathsToCheck = []string{candidate.SelectedExe}
	}
	for _, exePath := range pathsToCheck {
		if ref, ok := idx.findByPathConflict(exePath); ok {
			candidate.ImportStatus = importStatusExistsPath
			candidate.IsSelected = false
			candidate.ExistingID = ref.ID
			candidate.ExistingName = ref.Name
			candidate.SkipReason = "路径已存在: " + ref.Name
			return candidate
		}
	}

	if ref, ok := idx.findByPathConflict(candidate.SelectedExe); ok {
		candidate.ImportStatus = importStatusExistsPath
		candidate.IsSelected = false
		candidate.ExistingID = ref.ID
		candidate.ExistingName = ref.Name
		candidate.SkipReason = "路径已存在: " + ref.Name
		return candidate
	}

	if ref, ok := idx.findByNamePath(candidate.SearchName, candidate.SelectedExe); ok {
		candidate.ImportStatus = importStatusExistsNamePath
		candidate.IsSelected = false
		candidate.ExistingID = ref.ID
		candidate.ExistingName = ref.Name
		candidate.SkipReason = "已存在: " + ref.Name
		return candidate
	}

	if ref, ok := idx.findByName(candidate.SearchName); ok && normalizeImportPath(ref.Path) != normalizeImportPath(candidate.SelectedExe) {
		candidate.ImportStatus = importStatusPossibleDuplicate
		candidate.ExistingID = ref.ID
		candidate.ExistingName = ref.Name
		candidate.SkipReason = "存在同名游戏: " + ref.Name
	}

	return candidate
}

func splitScanCandidates(candidates []vo.BatchImportCandidate, idx importIndex) vo.BatchImportScanResult {
	result := vo.BatchImportScanResult{
		Candidates:        make([]vo.BatchImportCandidate, 0, len(candidates)),
		SkippedCandidates: make([]vo.BatchImportCandidate, 0),
		TotalDetected:     len(candidates),
	}

	for _, candidate := range candidates {
		annotated := annotateScanCandidate(candidate, idx)
		switch annotated.ImportStatus {
		case importStatusExistsPath, importStatusExistsNamePath:
			result.SkippedCandidates = append(result.SkippedCandidates, annotated)
		default:
			result.Candidates = append(result.Candidates, annotated)
		}
	}
	result.Skipped = len(result.SkippedCandidates)
	return result
}

func appendImportRows(ctx context.Context, conn *sql.Conn, table string, appendRows func(*duckdb.Appender) error) error {
	return conn.Raw(func(driverConn any) error {
		duckConn, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("duckdb raw connection has unexpected type %T", driverConn)
		}
		appender, err := duckdb.NewAppenderFromConn(duckConn, "", table)
		if err != nil {
			return fmt.Errorf("create appender for %s: %w", table, err)
		}
		if err := appendRows(appender); err != nil {
			_ = appender.Close()
			return err
		}
		if err := appender.Close(); err != nil {
			return fmt.Errorf("close appender for %s: %w", table, err)
		}
		return nil
	})
}

func splitImportItemsByAction(items []importItem) ([]importItem, []importItem) {
	createItems := make([]importItem, 0, len(items))
	updateItems := make([]importItem, 0)
	for _, item := range items {
		if item.Action == importer.ImportActionUpdateExisting {
			updateItems = append(updateItems, item)
			continue
		}
		createItems = append(createItems, item)
	}
	return createItems, updateItems
}

func (s *ImportService) addImportedItems(ctx context.Context, conn *sql.Conn, items []importItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_games`); err != nil {
		return 0, fmt.Errorf("drop temp_import_games: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_games (
		id TEXT,
		name TEXT,
		cover_url TEXT,
		company TEXT,
		summary TEXT,
		rating DOUBLE,
		release_date TEXT,
		path TEXT,
		save_path TEXT,
		process_name TEXT,
		source_type TEXT,
		cached_at TIMESTAMPTZ,
		source_id TEXT,
		created_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ,
		use_locale_emulator BOOLEAN,
		use_magpie BOOLEAN
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_import_games: %w", err)
	}

	now := time.Now()
	if err := appendImportRows(ctx, conn, "temp_import_games", func(appender *duckdb.Appender) error {
		for i := range items {
			game := items[i].Game
			if game.ID == "" {
				game.ID = uuid.New().String()
			}
			if game.CreatedAt.IsZero() {
				game.CreatedAt = now
			}
			if game.CachedAt.IsZero() {
				game.CachedAt = now
			}
			if game.UpdatedAt.IsZero() {
				game.UpdatedAt = now
			}
			if game.SourceType == "" {
				game.SourceType = items[i].Source
			}
			items[i].Game = game

			if err := appender.AppendRow(
				game.ID,
				game.Name,
				game.CoverURL,
				game.Company,
				game.Summary,
				game.Rating,
				game.ReleaseDate,
				game.Path,
				game.SavePath,
				game.ProcessName,
				string(game.SourceType),
				game.CachedAt,
				game.SourceID,
				game.CreatedAt,
				game.UpdatedAt,
				game.UseLocaleEmulator,
				game.UseMagpie,
			); err != nil {
				return fmt.Errorf("append imported game %s: %w", game.Name, err)
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO games (
		id, name, cover_url, company, summary, rating, release_date, path,
		save_path, process_name, source_type, cached_at, source_id, created_at, updated_at,
		use_locale_emulator, use_magpie
	)
	SELECT
		id, name, cover_url, company, summary, rating, release_date, path,
		save_path, process_name, source_type, cached_at, source_id, created_at, updated_at,
		use_locale_emulator, use_magpie
	FROM temp_import_games`); err != nil {
		return 0, fmt.Errorf("insert imported games from staging: %w", err)
	}

	return len(items), nil
}

func (s *ImportService) updateImportedItemMetadata(ctx context.Context, conn *sql.Conn, items []importItem) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_update_import_games`); err != nil {
		return 0, fmt.Errorf("drop temp_update_import_games: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_update_import_games (
		id TEXT,
		name TEXT,
		cover_url TEXT,
		company TEXT,
		summary TEXT,
		rating DOUBLE,
		release_date TEXT,
		source_type TEXT,
		cached_at TIMESTAMPTZ,
		source_id TEXT,
		updated_at TIMESTAMPTZ
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_update_import_games: %w", err)
	}

	now := time.Now()
	inserted := 0
	if err := appendImportRows(ctx, conn, "temp_update_import_games", func(appender *duckdb.Appender) error {
		for i := range items {
			game := items[i].Game
			if game.ID == "" {
				continue
			}
			if game.CachedAt.IsZero() {
				game.CachedAt = now
			}
			if game.UpdatedAt.IsZero() {
				game.UpdatedAt = now
			}
			if game.SourceType == "" {
				game.SourceType = items[i].Source
			}
			items[i].Game = game
			if err := appender.AppendRow(
				game.ID,
				game.Name,
				game.CoverURL,
				game.Company,
				game.Summary,
				game.Rating,
				game.ReleaseDate,
				string(game.SourceType),
				game.CachedAt,
				game.SourceID,
				game.UpdatedAt,
			); err != nil {
				return fmt.Errorf("append import metadata update %s: %w", game.Name, err)
			}
			inserted++
		}
		return nil
	}); err != nil {
		return inserted, err
	}

	if inserted == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `
		UPDATE games
		SET
			name = temp_update_import_games.name,
			cover_url = CASE
				WHEN temp_update_import_games.cover_url <> '' THEN temp_update_import_games.cover_url
				ELSE games.cover_url
			END,
			company = temp_update_import_games.company,
			summary = temp_update_import_games.summary,
			rating = temp_update_import_games.rating,
			release_date = temp_update_import_games.release_date,
			source_type = temp_update_import_games.source_type,
			cached_at = temp_update_import_games.cached_at,
			source_id = temp_update_import_games.source_id,
			updated_at = temp_update_import_games.updated_at
		FROM temp_update_import_games
		WHERE games.id = temp_update_import_games.id
	`); err != nil {
		return inserted, fmt.Errorf("update imported game metadata from staging: %w", err)
	}

	return inserted, nil
}

func (s *ImportService) addImportedItemTags(ctx context.Context, conn *sql.Conn, items []importItem) (int, error) {
	total := 0
	for _, item := range items {
		total += len(item.Tags)
	}
	if total == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_game_tags`); err != nil {
		return 0, fmt.Errorf("drop temp_import_game_tags: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_game_tags (
		id TEXT,
		game_id TEXT,
		name TEXT,
		source TEXT,
		weight DOUBLE,
		is_spoiler BOOLEAN,
		created_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_import_game_tags: %w", err)
	}

	now := time.Now()
	inserted := 0
	if err := appendImportRows(ctx, conn, "temp_import_game_tags", func(appender *duckdb.Appender) error {
		for _, item := range items {
			for _, tag := range item.Tags {
				name := strings.TrimSpace(tag.Name)
				source := strings.TrimSpace(tag.Source)
				if name == "" {
					continue
				}
				if source == "" {
					source = "user"
				}
				id := uuid.New().String()
				if err := appender.AppendRow(id, item.Game.ID, name, source, tag.Weight, tag.IsSpoiler, now, now); err != nil {
					return fmt.Errorf("append imported tag %s for %s: %w", name, item.Game.Name, err)
				}
				inserted++
			}
		}
		return nil
	}); err != nil {
		return inserted, err
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
		SELECT id, game_id, name, source, weight, is_spoiler, created_at, updated_at
		FROM temp_import_game_tags
		ON CONFLICT (game_id, name, source) DO UPDATE SET
			id = EXCLUDED.id,
			weight = EXCLUDED.weight,
			is_spoiler = EXCLUDED.is_spoiler,
			updated_at = EXCLUDED.updated_at
	`); err != nil {
		return inserted, fmt.Errorf("insert imported tags from staging: %w", err)
	}
	return inserted, nil
}

func (s *ImportService) addImportedItemSessions(ctx context.Context, conn *sql.Conn, items []importItem) (int, error) {
	total := 0
	for _, item := range items {
		total += len(item.Sessions)
	}
	if total == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_play_sessions`); err != nil {
		return 0, fmt.Errorf("drop temp_import_play_sessions: %w", err)
	}
	_, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_play_sessions (
		id TEXT,
		game_id TEXT,
		start_time TIMESTAMPTZ,
		end_time TIMESTAMPTZ,
		duration INTEGER,
		updated_at TIMESTAMPTZ
	)`)
	if err != nil {
		return 0, fmt.Errorf("create temp_import_play_sessions: %w", err)
	}

	now := time.Now()
	inserted := 0
	if err := appendImportRows(ctx, conn, "temp_import_play_sessions", func(appender *duckdb.Appender) error {
		for itemIndex := range items {
			for sessionIndex := range items[itemIndex].Sessions {
				session := items[itemIndex].Sessions[sessionIndex]
				if session.ID == "" {
					session.ID = uuid.New().String()
				}
				if session.GameID == "" {
					session.GameID = items[itemIndex].Game.ID
				}
				if session.UpdatedAt.IsZero() {
					session.UpdatedAt = now
				}
				items[itemIndex].Sessions[sessionIndex] = session
				if err := appender.AppendRow(session.ID, session.GameID, session.StartTime, session.EndTime, int64(session.Duration), session.UpdatedAt); err != nil {
					return fmt.Errorf("append imported session for %s: %w", items[itemIndex].Game.Name, err)
				}
				inserted++
			}
		}
		return nil
	}); err != nil {
		return inserted, err
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_play_sessions_dedup`); err != nil {
		return inserted, fmt.Errorf("drop temp_import_play_sessions_dedup: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TEMP TABLE temp_import_play_sessions_dedup AS
		SELECT id, game_id, start_time, end_time, duration, updated_at
		FROM (
			SELECT
				id,
				game_id,
				start_time,
				end_time,
				duration,
				updated_at,
				ROW_NUMBER() OVER (
					PARTITION BY game_id, start_time, end_time
					ORDER BY updated_at DESC, id
				) AS row_num
			FROM temp_import_play_sessions
		)
		WHERE row_num = 1
	`); err != nil {
		return inserted, fmt.Errorf("deduplicate imported sessions: %w", err)
	}

	var toInsert int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM temp_import_play_sessions_dedup t
		WHERE NOT EXISTS (
			SELECT 1
			FROM play_sessions p
			WHERE p.game_id = t.game_id
			  AND p.start_time = t.start_time
			  AND p.end_time = t.end_time
		)
	`).Scan(&toInsert); err != nil {
		return inserted, fmt.Errorf("count deduplicated imported sessions: %w", err)
	}
	if toInsert == 0 {
		return 0, nil
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		SELECT id, game_id, start_time, end_time, duration, updated_at
		FROM temp_import_play_sessions_dedup t
		WHERE NOT EXISTS (
			SELECT 1
			FROM play_sessions p
			WHERE p.game_id = t.game_id
			  AND p.start_time = t.start_time
			  AND p.end_time = t.end_time
		)
	`); err != nil {
		return inserted, fmt.Errorf("insert imported sessions from staging: %w", err)
	}
	return toInsert, nil
}

func (s *ImportService) deleteImportedItemTombstones(ctx context.Context, conn *sql.Conn, items []importItem) error {
	if len(items) == 0 {
		return nil
	}
	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS temp_import_tombstones`); err != nil {
		return fmt.Errorf("drop temp_import_tombstones: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TEMP TABLE temp_import_tombstones (
		entity_type TEXT,
		entity_id TEXT
	)`); err != nil {
		return fmt.Errorf("create temp_import_tombstones: %w", err)
	}

	count := 0
	if err := appendImportRows(ctx, conn, "temp_import_tombstones", func(appender *duckdb.Appender) error {
		for _, item := range items {
			if item.Game.ID != "" {
				if err := appender.AppendRow(cloudSyncEntityGame, item.Game.ID); err != nil {
					return fmt.Errorf("append game tombstone %s: %w", item.Game.ID, err)
				}
				count++
			}
			for _, tag := range item.Tags {
				name := strings.TrimSpace(tag.Name)
				source := strings.TrimSpace(tag.Source)
				if name == "" {
					continue
				}
				if source == "" {
					source = "user"
				}
				if err := appender.AppendRow(cloudSyncEntityGameTag, tagTombstoneID(item.Game.ID, source, name)); err != nil {
					return fmt.Errorf("append tag tombstone %s/%s: %w", item.Game.ID, name, err)
				}
				count++
			}
			for _, session := range item.Sessions {
				if session.ID == "" {
					continue
				}
				if err := appender.AppendRow(cloudSyncEntityPlaySession, session.ID); err != nil {
					return fmt.Errorf("append session tombstone %s: %w", session.ID, err)
				}
				count++
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, `
		DELETE FROM sync_tombstones
		WHERE EXISTS (
			SELECT 1
			FROM temp_import_tombstones t
			WHERE t.entity_type = sync_tombstones.entity_type
			  AND t.entity_id = sync_tombstones.entity_id
		)
	`); err != nil {
		return fmt.Errorf("delete import tombstones from staging: %w", err)
	}
	return nil
}

func (s *ImportService) commitImportedItems(items []importItem) (int, int, error) {
	if len(items) == 0 {
		return 0, 0, nil
	}

	startedAt := time.Now()
	applog.LogInfof(s.ctx, "commitImportedItems: start items=%d", len(items))

	conn, err := s.db.Conn(s.ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("获取导入数据库连接失败: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(s.ctx, `BEGIN TRANSACTION`); err != nil {
		return 0, 0, fmt.Errorf("开始导入事务失败: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(s.ctx, `ROLLBACK`)
		}
	}()

	createItems, updateItems := splitImportItemsByAction(items)

	stepStartedAt := time.Now()
	insertedGames, err := s.addImportedItems(s.ctx, conn, createItems)
	if err != nil {
		return 0, 0, err
	}
	applog.LogInfof(s.ctx, "commitImportedItems: staged and inserted games=%d elapsed=%s", insertedGames, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	updatedGames, err := s.updateImportedItemMetadata(s.ctx, conn, updateItems)
	if err != nil {
		return insertedGames, 0, err
	}
	applog.LogInfof(s.ctx, "commitImportedItems: staged and updated games=%d elapsed=%s", updatedGames, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	insertedTags, err := s.addImportedItemTags(s.ctx, conn, items)
	if err != nil {
		return insertedGames + updatedGames, 0, err
	}
	applog.LogInfof(s.ctx, "commitImportedItems: staged and upserted tags=%d elapsed=%s", insertedTags, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	insertedSessions, err := s.addImportedItemSessions(s.ctx, conn, items)
	if err != nil {
		return insertedGames + updatedGames, 0, err
	}
	applog.LogInfof(s.ctx, "commitImportedItems: staged and inserted sessions=%d elapsed=%s", insertedSessions, time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	if err := s.deleteImportedItemTombstones(s.ctx, conn, items); err != nil {
		return insertedGames + updatedGames, insertedSessions, err
	}
	applog.LogInfof(s.ctx, "commitImportedItems: deleted sync tombstones elapsed=%s", time.Since(stepStartedAt))

	stepStartedAt = time.Now()
	if _, err := conn.ExecContext(s.ctx, `COMMIT`); err != nil {
		return 0, 0, fmt.Errorf("提交导入事务失败: %w", err)
	}
	committed = true
	applog.LogInfof(s.ctx, "commitImportedItems: committed elapsed=%s total=%s", time.Since(stepStartedAt), time.Since(startedAt))

	s.startImportCoverProcessing(items)
	return insertedGames + updatedGames, insertedSessions, nil
}

func (s *ImportService) startImportCoverProcessing(items []importItem) {
	if s.gameService == nil {
		return
	}

	jobs := make([]importItem, 0)
	for _, item := range items {
		if item.Game.CoverURL != "" || item.CoverLoader != nil {
			jobs = append(jobs, item)
		}
	}
	if len(jobs) == 0 {
		return
	}

	workerCount := importCoverWorkerCount
	if len(jobs) < workerCount {
		workerCount = len(jobs)
	}

	go func() {
		startedAt := time.Now()
		applog.LogInfof(s.ctx, "import cover processing: start jobs=%d workers=%d", len(jobs), workerCount)

		jobCh := make(chan importItem)
		var wg sync.WaitGroup
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range jobCh {
					s.processImportCover(item)
				}
			}()
		}
		for _, item := range jobs {
			jobCh <- item
		}
		close(jobCh)
		wg.Wait()

		applog.LogInfof(s.ctx, "import cover processing: complete jobs=%d elapsed=%s", len(jobs), time.Since(startedAt))
	}()
}

func (s *ImportService) processImportCover(item importItem) {
	if item.Game.CoverURL != "" {
		s.gameService.asyncDownloadCoverImage(item.Game.ID, item.Game.Name, item.Game.CoverURL, false)
	}
	if item.CoverLoader == nil {
		return
	}

	coverURL, err := item.CoverLoader(item.Game)
	if err != nil {
		applog.LogWarningf(s.ctx, "import cover processing failed for %s: %v", item.Game.Name, err)
		return
	}
	if coverURL == "" {
		return
	}
	if err := s.gameService.updateCoverURL(item.Game.ID, coverURL); err != nil {
		applog.LogWarningf(s.ctx, "import cover update failed for %s: %v", item.Game.Name, err)
	}
}
