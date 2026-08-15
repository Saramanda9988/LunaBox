package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/migrations"
	"lunabox/internal/models"
	"lunabox/internal/service/importer"
	"lunabox/internal/utils/metadata"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

func TestScanGameDirectoryUsesConfiguredNameDepth(t *testing.T) {
	root := filepath.Join("library")
	current := filepath.Join(root, "game-root", "bin", "x64")

	if got := scanGameDirectory(root, current, "depth", 0); got != filepath.Join(root, "game-root") {
		t.Fatalf("unexpected first-level game directory: %q", got)
	}
	if got := scanGameDirectory(root, current, "depth", 1); got != filepath.Join(root, "game-root", "bin") {
		t.Fatalf("unexpected second-level game directory: %q", got)
	}
	if got := scanGameDirectory(root, current, "parent", 0); got != current {
		t.Fatalf("parent scan mode should keep executable directory: %q", got)
	}
}

func setupImportServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	applog.SetMode(applog.ModeCLI)

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("init test schema: %v", err)
	}
	return db
}

func TestBatchImportGamesPersistsEveryMatchedMetadataSource(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()
	config := &appconf.AppConfig{}

	gameService := NewGameService()
	gameService.Init(ctx, db, config)
	importService := NewImportService()
	importService.Init(ctx, db, config)
	importService.SetGameService(gameService)

	matchedGame := models.Game{
		Name:       "SEQUEL blight",
		SourceType: enums.Bangumi,
		SourceID:   "101",
		MetadataSources: []models.GameMetadataSource{
			{SourceType: enums.Bangumi, SourceID: "101"},
			{SourceType: enums.VNDB, SourceID: "v202"},
			{SourceType: enums.DLsite, SourceID: "RJ303"},
		},
	}

	result, err := importService.BatchImportGames([]vo.BatchImportCandidate{
		{
			FolderPath:  `D:\Games\SEQUEL blight`,
			SelectedExe: `D:\Games\SEQUEL blight\Game.exe`,
			SearchName:  "SEQUEL blight",
			IsSelected:  true,
			MatchedGame: &matchedGame,
			MatchSource: enums.Bangumi,
			MatchStatus: "matched",
		},
	})
	if err != nil {
		t.Fatalf("BatchImportGames returned error: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("expected one imported game, got %+v", result)
	}

	var gameID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM games WHERE name = ?`, matchedGame.Name).Scan(&gameID); err != nil {
		t.Fatalf("query imported game ID: %v", err)
	}
	savedSources, err := gameService.GetGameMetadataSources(gameID)
	if err != nil {
		t.Fatalf("GetGameMetadataSources returned error: %v", err)
	}
	if len(savedSources) != 3 {
		t.Fatalf("expected three saved metadata sources, got %+v", savedSources)
	}
	bySource := make(map[enums.SourceType]string, len(savedSources))
	for _, source := range savedSources {
		bySource[source.SourceType] = source.SourceID
	}
	if bySource[enums.Bangumi] != "101" || bySource[enums.VNDB] != "v202" || bySource[enums.DLsite] != "RJ303" {
		t.Fatalf("unexpected saved metadata sources: %+v", savedSources)
	}

	duplicateGame := models.Game{
		Name:       "Another title",
		SourceType: enums.Steam,
		SourceID:   "404",
		MetadataSources: []models.GameMetadataSource{
			{SourceType: enums.Steam, SourceID: "404"},
			{SourceType: enums.VNDB, SourceID: "v202"},
		},
	}
	duplicateResult, err := importService.BatchImportGames([]vo.BatchImportCandidate{
		{
			FolderPath:  `D:\Games\Another title`,
			SelectedExe: `D:\Games\Another title\Game.exe`,
			SearchName:  "Another title",
			IsSelected:  true,
			MatchedGame: &duplicateGame,
			MatchSource: enums.Steam,
			MatchStatus: "matched",
		},
	})
	if err != nil {
		t.Fatalf("duplicate BatchImportGames returned error: %v", err)
	}
	if duplicateResult.Skipped != 1 || duplicateResult.Success != 0 {
		t.Fatalf("expected secondary-source duplicate to be skipped, got %+v", duplicateResult)
	}
}

func TestCommitImportedItemsUpdateExistingMergesMetadataTagsAndSessions(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()

	gameService := NewGameService()
	gameService.Init(ctx, db, &appconf.AppConfig{})
	importService := NewImportService()
	importService.Init(ctx, db, &appconf.AppConfig{})
	importService.SetGameService(gameService)

	createdAt := time.Date(2023, 1, 2, 3, 4, 5, 0, time.Local)
	existing := models.Game{
		ID:                "existing-game",
		Name:              "Existing Name",
		CoverURL:          "/local/covers/existing.jpg",
		Company:           "Old Studio",
		Summary:           "Old summary",
		Rating:            2,
		ReleaseDate:       "2020-01-01",
		Path:              `D:\Games\Same\game.exe`,
		SavePath:          `D:\Saves\Same`,
		ProcessName:       "actual.exe",
		Status:            enums.StatusPlaying,
		SourceType:        enums.Local,
		SourceID:          "local-old",
		CreatedAt:         createdAt,
		CachedAt:          createdAt,
		UpdatedAt:         createdAt,
		UseLocaleEmulator: true,
		UseMagpie:         true,
		MetadataLocked:    true,
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO games (
			id, name, cover_url, company, summary, rating, release_date, path,
			save_path, process_name, status, source_type, cached_at, source_id, created_at, updated_at,
			use_locale_emulator, use_magpie, metadata_locked
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		existing.ID,
		existing.Name,
		existing.CoverURL,
		existing.Company,
		existing.Summary,
		existing.Rating,
		existing.ReleaseDate,
		existing.Path,
		existing.SavePath,
		existing.ProcessName,
		string(existing.Status),
		string(existing.SourceType),
		existing.CachedAt,
		existing.SourceID,
		existing.CreatedAt,
		existing.UpdatedAt,
		existing.UseLocaleEmulator,
		existing.UseMagpie,
		existing.MetadataLocked,
	); err != nil {
		t.Fatalf("insert existing game: %v", err)
	}

	sessionStart := time.Date(2024, 5, 6, 12, 0, 0, 0, time.Local)
	success, sessionsImported, err := importService.commitImportedItems([]importItem{
		{
			Game: models.Game{
				ID:          existing.ID,
				Name:        "Imported Name",
				Company:     "New Studio",
				Summary:     "New summary",
				Rating:      8.5,
				ReleaseDate: "2024-05-01",
				Path:        `D:\Imported\ShouldNotReplace.exe`,
				SavePath:    `D:\Imported\Saves`,
				ProcessName: "imported.exe",
				SourceType:  enums.VNDB,
				SourceID:    "v123",
				IsNSFW:      true,
				CachedAt:    sessionStart,
				UpdatedAt:   sessionStart,
			},
			Tags: []metadata.TagItem{
				{Name: "Drama", Source: "vndb", Weight: 0.8},
			},
			Sessions: []models.PlaySession{
				{
					ID:        "session-imported",
					GameID:    existing.ID,
					StartTime: sessionStart,
					EndTime:   sessionStart.Add(30 * time.Minute),
					Duration:  1800,
				},
			},
			Source: enums.VNDB,
			Action: importer.ImportActionUpdateExisting,
		},
	})
	if err != nil {
		t.Fatalf("commitImportedItems returned error: %v", err)
	}
	if success != 1 || sessionsImported != 1 {
		t.Fatalf("expected success=1 sessions=1, got success=%d sessions=%d", success, sessionsImported)
	}

	saved, err := gameService.GetGameByID(existing.ID)
	if err != nil {
		t.Fatalf("GetGameByID returned error: %v", err)
	}
	if saved.Name != "Imported Name" || saved.Company != "New Studio" || saved.Summary != "New summary" {
		t.Fatalf("metadata was not updated: %+v", saved)
	}
	if saved.SourceType != enums.VNDB || saved.SourceID != "v123" {
		t.Fatalf("source metadata was not updated: %+v", saved)
	}
	if !saved.IsNSFW {
		t.Fatalf("VNDB NSFW metadata was not updated: %+v", saved)
	}
	if saved.Path != existing.Path || saved.SavePath != existing.SavePath || saved.ProcessName != existing.ProcessName {
		t.Fatalf("local launch fields should be preserved: %+v", saved)
	}
	if saved.Status != existing.Status || !saved.UseLocaleEmulator || !saved.UseMagpie || !saved.MetadataLocked {
		t.Fatalf("local state flags should be preserved: %+v", saved)
	}

	var sessionCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM play_sessions WHERE game_id = ?`, existing.ID).Scan(&sessionCount); err != nil {
		t.Fatalf("count imported sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("expected 1 imported session for existing game, got %d", sessionCount)
	}

	var tagCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_tags WHERE game_id = ? AND name = ? AND source = ?`, existing.ID, "Drama", "vndb").Scan(&tagCount); err != nil {
		t.Fatalf("count imported tags: %v", err)
	}
	if tagCount != 1 {
		t.Fatalf("expected imported tag to be upserted, got %d", tagCount)
	}
}

func TestCommitImportedItemsUpdatesOnlyFlaggedLocalLaunchFields(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()
	importService := NewImportService()
	importService.Init(ctx, db, &appconf.AppConfig{})
	now := time.Now().Truncate(time.Second)

	if _, err := db.ExecContext(ctx, `
		INSERT INTO games (
			id, name, path, game_directory, process_name, status, source_type, cached_at, source_id,
			launch_mode, steam_launch_id, steam_launch_kind, steam_user_id, created_at, updated_at
		) VALUES
			('missing-path', 'Missing Path', '', '', '', 'not_started', 'steam', ?, '123456', 'normal', '', '', '', ?, ?),
			('ordinary', 'Ordinary', '/local/original', '/local', 'original.exe', 'not_started', 'steam', ?, '654321', 'normal', 'local-id', 'shortcut', 'local-user', ?, ?)
	`, now, now, now, now, now, now); err != nil {
		t.Fatalf("insert existing games: %v", err)
	}

	_, _, err := importService.commitImportedItems([]importItem{
		{
			Game: models.Game{
				ID:              "missing-path",
				Name:            "Hydrated",
				Path:            "/Applications/Steam Game",
				GameDirectory:   "/Applications/Steam Game",
				LaunchMode:      enums.LaunchModeSteam,
				SteamLaunchID:   "123456",
				SteamLaunchKind: "native",
				SourceType:      enums.Steam,
				SourceID:        "123456",
				CachedAt:        now,
				UpdatedAt:       now,
			},
			Source:                  enums.Steam,
			Action:                  importer.ImportActionUpdateExisting,
			UpdateLocalLaunchFields: true,
		},
		{
			Game: models.Game{
				ID:              "ordinary",
				Name:            "Ordinary Updated",
				Path:            "/imported/should-not-replace",
				GameDirectory:   "/imported",
				ProcessName:     "imported.exe",
				LaunchMode:      enums.LaunchModeSteam,
				SteamLaunchID:   "imported-id",
				SteamLaunchKind: "native",
				SourceType:      enums.Steam,
				SourceID:        "654321",
				CachedAt:        now,
				UpdatedAt:       now,
			},
			Source: enums.Steam,
			Action: importer.ImportActionUpdateExisting,
		},
	})
	if err != nil {
		t.Fatalf("commitImportedItems() error = %v", err)
	}

	var path, gameDirectory, processName, launchMode, steamLaunchID, steamLaunchKind, steamUserID string
	if err := db.QueryRowContext(ctx, `
		SELECT path, game_directory, process_name, launch_mode, steam_launch_id, steam_launch_kind, steam_user_id
		FROM games WHERE id = 'missing-path'
	`).Scan(&path, &gameDirectory, &processName, &launchMode, &steamLaunchID, &steamLaunchKind, &steamUserID); err != nil {
		t.Fatalf("query hydrated game: %v", err)
	}
	if path != "/Applications/Steam Game" || gameDirectory != path || processName != "" || launchMode != "steam" || steamLaunchID != "123456" || steamLaunchKind != "native" || steamUserID != "" {
		t.Fatalf("unexpected hydrated launch fields: path=%q directory=%q process=%q mode=%q id=%q kind=%q user=%q", path, gameDirectory, processName, launchMode, steamLaunchID, steamLaunchKind, steamUserID)
	}

	if err := db.QueryRowContext(ctx, `
		SELECT path, game_directory, process_name, launch_mode, steam_launch_id, steam_launch_kind, steam_user_id
		FROM games WHERE id = 'ordinary'
	`).Scan(&path, &gameDirectory, &processName, &launchMode, &steamLaunchID, &steamLaunchKind, &steamUserID); err != nil {
		t.Fatalf("query ordinary update: %v", err)
	}
	if path != "/local/original" || gameDirectory != "/local" || processName != "original.exe" || launchMode != "normal" || steamLaunchID != "local-id" || steamLaunchKind != "shortcut" || steamUserID != "local-user" {
		t.Fatalf("ordinary metadata update changed local launch fields: path=%q directory=%q process=%q mode=%q id=%q kind=%q user=%q", path, gameDirectory, processName, launchMode, steamLaunchID, steamLaunchKind, steamUserID)
	}
}

func TestCommitImportedItemsMergeSessionsPreservesGameInformation(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()

	gameService := NewGameService()
	gameService.Init(ctx, db, &appconf.AppConfig{})
	importService := NewImportService()
	importService.Init(ctx, db, &appconf.AppConfig{})
	importService.SetGameService(gameService)

	createdAt := time.Date(2023, 2, 3, 4, 5, 6, 0, time.Local)
	existing := models.Game{
		ID:          "merge-sessions-game",
		Name:        "Existing Name",
		CoverURL:    "/local/covers/existing.jpg",
		Company:     "Existing Studio",
		Summary:     "Existing summary",
		Rating:      7.5,
		ReleaseDate: "2021-02-03",
		Path:        `D:\Games\MergeOnly\game.exe`,
		SourceType:  enums.Local,
		SourceID:    "existing-source",
		CreatedAt:   createdAt,
		CachedAt:    createdAt,
		UpdatedAt:   createdAt,
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO games (
			id, name, cover_url, company, summary, rating, release_date, path,
			save_path, process_name, status, source_type, cached_at, source_id, created_at, updated_at,
			use_locale_emulator, use_magpie, metadata_locked
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', '', 'not_started', ?, ?, ?, ?, ?, FALSE, FALSE, FALSE)
	`,
		existing.ID,
		existing.Name,
		existing.CoverURL,
		existing.Company,
		existing.Summary,
		existing.Rating,
		existing.ReleaseDate,
		existing.Path,
		string(existing.SourceType),
		existing.CachedAt,
		existing.SourceID,
		existing.CreatedAt,
		existing.UpdatedAt,
	); err != nil {
		t.Fatalf("insert existing game: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at)
		VALUES ('existing-tag', ?, 'Existing Tag', 'user', 1, FALSE, ?, ?)
	`, existing.ID, createdAt, createdAt); err != nil {
		t.Fatalf("insert existing tag: %v", err)
	}

	sessionStart := time.Date(2024, 7, 8, 12, 0, 0, 0, time.Local)
	success, sessionsImported, err := importService.commitImportedItems([]importItem{
		{
			Game: models.Game{
				ID:          existing.ID,
				Name:        "Imported Name",
				CoverURL:    "https://example.com/imported.jpg",
				Company:     "Imported Studio",
				Summary:     "Imported summary",
				Rating:      9,
				ReleaseDate: "2024-07-08",
				Path:        existing.Path,
				SourceType:  enums.VNDB,
				SourceID:    "v999",
				MetadataSources: []models.GameMetadataSource{
					{GameID: existing.ID, SourceType: enums.VNDB, SourceID: "v999"},
				},
				CachedAt:  sessionStart,
				UpdatedAt: sessionStart,
			},
			Tags: []metadata.TagItem{
				{Name: "Imported Tag", Source: "vndb", Weight: 0.9},
			},
			Sessions: []models.PlaySession{
				{
					ID:        "merge-only-session",
					GameID:    existing.ID,
					StartTime: sessionStart,
					EndTime:   sessionStart.Add(20 * time.Minute),
					Duration:  1200,
				},
			},
			Source: enums.VNDB,
			Action: importer.ImportActionMergeSessions,
		},
	})
	if err != nil {
		t.Fatalf("commitImportedItems returned error: %v", err)
	}
	if success != 1 || sessionsImported != 1 {
		t.Fatalf("expected success=1 sessions=1, got success=%d sessions=%d", success, sessionsImported)
	}

	saved, err := gameService.GetGameByID(existing.ID)
	if err != nil {
		t.Fatalf("GetGameByID returned error: %v", err)
	}
	if saved.Name != existing.Name || saved.CoverURL != existing.CoverURL || saved.Company != existing.Company ||
		saved.Summary != existing.Summary || saved.Rating != existing.Rating || saved.ReleaseDate != existing.ReleaseDate ||
		saved.SourceType != existing.SourceType || saved.SourceID != existing.SourceID {
		t.Fatalf("merge sessions changed existing game information: %+v", saved)
	}

	var importedTagCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_tags
		WHERE game_id = ? AND name = 'Imported Tag'
	`, existing.ID).Scan(&importedTagCount); err != nil {
		t.Fatalf("count imported tags: %v", err)
	}
	if importedTagCount != 0 {
		t.Fatalf("expected imported tags to remain absent, got %d", importedTagCount)
	}

	var importedSourceCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_metadata_sources
		WHERE game_id = ? AND source_type = ? AND source_id = ?
	`, existing.ID, string(enums.VNDB), "v999").Scan(&importedSourceCount); err != nil {
		t.Fatalf("count imported metadata sources: %v", err)
	}
	if importedSourceCount != 0 {
		t.Fatalf("expected imported metadata sources to remain absent, got %d", importedSourceCount)
	}

	var existingTagCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_tags
		WHERE game_id = ? AND name = 'Existing Tag'
	`, existing.ID).Scan(&existingTagCount); err != nil {
		t.Fatalf("count existing tags: %v", err)
	}
	if existingTagCount != 1 {
		t.Fatalf("expected existing tag to remain, got %d", existingTagCount)
	}
}

func TestMergeMetadataPreservesManualNSFWForUnsupportedSource(t *testing.T) {
	target := models.Game{IsNSFW: true}
	changed := mergeMetadataIntoGame(&target, models.Game{SourceType: enums.DLsite})

	if target.IsNSFW != true {
		t.Fatal("unsupported metadata source cleared the manual NSFW flag")
	}
	if !changed {
		t.Fatal("expected source metadata to be merged")
	}
}

func TestCommitImportedItemsDeduplicatesImportedSessions(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()

	gameService := NewGameService()
	gameService.Init(ctx, db, &appconf.AppConfig{})
	importService := NewImportService()
	importService.Init(ctx, db, &appconf.AppConfig{})
	importService.SetGameService(gameService)

	game := models.Game{
		ID:         "session-dedupe-game",
		Name:       "Session Dedupe Game",
		Path:       `D:\Games\Dedupe\game.exe`,
		SourceType: enums.Local,
		CreatedAt:  time.Now(),
		CachedAt:   time.Now(),
		UpdatedAt:  time.Now(),
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO games (
			id, name, cover_url, company, summary, rating, release_date, path,
			save_path, process_name, status, source_type, cached_at, source_id, created_at, updated_at,
			use_locale_emulator, use_magpie, metadata_locked
		) VALUES (?, ?, '', '', '', 0, '', ?, '', '', 'not_started', ?, ?, '', ?, ?, FALSE, FALSE, FALSE)
	`, game.ID, game.Name, game.Path, string(game.SourceType), game.CachedAt, game.CreatedAt, game.UpdatedAt); err != nil {
		t.Fatalf("insert existing game: %v", err)
	}

	start := time.Date(2024, 6, 7, 12, 0, 0, 0, time.Local)
	end := start.Add(45 * time.Minute)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "existing-session", game.ID, start, end, 2700, start); err != nil {
		t.Fatalf("insert existing session: %v", err)
	}

	newStart := start.Add(24 * time.Hour)
	newEnd := newStart.Add(30 * time.Minute)
	_, sessionsImported, err := importService.commitImportedItems([]importItem{
		{
			Game: game,
			Sessions: []models.PlaySession{
				{
					ID:        "duplicate-existing",
					GameID:    game.ID,
					StartTime: start,
					EndTime:   end,
					Duration:  2700,
				},
				{
					ID:        "new-session-a",
					GameID:    game.ID,
					StartTime: newStart,
					EndTime:   newEnd,
					Duration:  1800,
				},
				{
					ID:        "new-session-b",
					GameID:    game.ID,
					StartTime: newStart,
					EndTime:   newEnd,
					Duration:  1800,
				},
			},
			Action: importer.ImportActionUpdateExisting,
		},
	})
	if err != nil {
		t.Fatalf("commitImportedItems returned error: %v", err)
	}
	if sessionsImported != 1 {
		t.Fatalf("expected only one new session to be imported, got %d", sessionsImported)
	}

	var sessionCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM play_sessions WHERE game_id = ?`, game.ID).Scan(&sessionCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 2 {
		t.Fatalf("expected existing + one deduplicated imported session, got %d", sessionCount)
	}
}
