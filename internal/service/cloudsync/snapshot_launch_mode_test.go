package cloudsync

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/migrations"

	_ "github.com/duckdb/duckdb-go/v2"
)

func setupCloudSyncLaunchModeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.InitSchema(db); err != nil {
		t.Fatalf("init test schema: %v", err)
	}
	return db
}

func TestBuildLocalStateOmitsLaunchModeAndOldJSONStillDecodes(t *testing.T) {
	db := setupCloudSyncLaunchModeTestDB(t)
	now := time.Now().Truncate(time.Second)
	if _, err := db.Exec(`
		INSERT INTO games (id, name, status, source_type, cached_at, source_id, created_at, updated_at, launch_mode)
		VALUES ('game-1', 'Steam Game', 'not_started', 'steam', ?, '123456', ?, ?, 'steam')
	`, now, now, now); err != nil {
		t.Fatalf("insert game: %v", err)
	}

	helper := NewHelper(context.Background(), db, &appconf.AppConfig{})
	state, err := helper.BuildLocalState()
	if err != nil {
		t.Fatalf("BuildLocalState() error = %v", err)
	}
	raw, err := json.Marshal(state.Snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if strings.Contains(string(raw), `"launch_mode"`) {
		t.Fatalf("cloud snapshot still contains launch_mode: %s", raw)
	}

	var oldSnapshot Snapshot
	if err := json.Unmarshal([]byte(`{"schema_version":1,"games":[{"id":"old-game","name":"Old Game","launch_mode":"steam"}]}`), &oldSnapshot); err != nil {
		t.Fatalf("decode old snapshot containing launch_mode: %v", err)
	}
	if len(oldSnapshot.Games) != 1 || oldSnapshot.Games[0].ID != "old-game" {
		t.Fatalf("unexpected decoded old snapshot: %+v", oldSnapshot)
	}
}

func TestApplyMergedSnapshotPreservesExistingLaunchModeAndDefaultsNewGame(t *testing.T) {
	db := setupCloudSyncLaunchModeTestDB(t)
	now := time.Now().Truncate(time.Second)
	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, path, game_directory, status, source_type, cached_at, source_id,
			launch_mode, steam_launch_id, steam_launch_kind, created_at, updated_at
		) VALUES ('existing', 'Existing', '/local/game', '/local/game', 'not_started', 'steam', ?, '123456', 'steam', '123456', 'native', ?, ?)
	`, now, now, now); err != nil {
		t.Fatalf("insert existing game: %v", err)
	}

	snapshot := Snapshot{Games: []Game{
		{
			ID:         "existing",
			Name:       "Existing Synced",
			Status:     "playing",
			SourceType: "steam",
			SourceID:   "123456",
			CreatedAt:  now,
			UpdatedAt:  now.Add(time.Minute),
		},
		{
			ID:         "new-game",
			Name:       "New Synced",
			Status:     "not_started",
			SourceType: "steam",
			SourceID:   "654321",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}}
	helper := NewHelper(context.Background(), db, &appconf.AppConfig{})
	if err := helper.ApplyMergedSnapshot(snapshot, nil); err != nil {
		t.Fatalf("ApplyMergedSnapshot() error = %v", err)
	}

	var launchMode, path, steamLaunchID, steamLaunchKind string
	if err := db.QueryRow(`
		SELECT launch_mode, path, steam_launch_id, steam_launch_kind
		FROM games WHERE id = 'existing'
	`).Scan(&launchMode, &path, &steamLaunchID, &steamLaunchKind); err != nil {
		t.Fatalf("query existing game: %v", err)
	}
	if launchMode != "steam" || path != "/local/game" || steamLaunchID != "123456" || steamLaunchKind != "native" {
		t.Fatalf("existing local launch fields changed: mode=%q path=%q id=%q kind=%q", launchMode, path, steamLaunchID, steamLaunchKind)
	}

	if err := db.QueryRow(`SELECT launch_mode FROM games WHERE id = 'new-game'`).Scan(&launchMode); err != nil {
		t.Fatalf("query new game: %v", err)
	}
	if launchMode != "normal" {
		t.Fatalf("new synced game launch_mode = %q, want normal", launchMode)
	}
}
