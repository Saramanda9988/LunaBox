package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/enums"
	"lunabox/internal/migrations"
	"lunabox/internal/models"
	"lunabox/internal/service/importer"
	"lunabox/internal/utils/metadata"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

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

func TestCommitImportedItemsUpdateExistingMergesMetadataTagsAndSessions(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()

	gameService := NewGameService()
	gameService.Init(ctx, db, &appconf.AppConfig{})
	importService := NewImportService()
	importService.Init(ctx, db, &appconf.AppConfig{}, gameService)

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

func TestCommitImportedItemsDeduplicatesImportedSessions(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()

	gameService := NewGameService()
	gameService.Init(ctx, db, &appconf.AppConfig{})
	importService := NewImportService()
	importService.Init(ctx, db, &appconf.AppConfig{}, gameService)

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
