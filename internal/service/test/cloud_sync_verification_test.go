package test

import (
	"context"
	"testing"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/service"

	_ "github.com/duckdb/duckdb-go/v2"
)

func TestCategoryServiceNormalizesSystemFavorites(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	legacyID := "legacy-favorites"
	if _, err := db.Exec(`INSERT INTO categories (id, name, emoji, is_system, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		legacyID, "最喜欢的游戏", "❤️", true, now, now); err != nil {
		t.Fatalf("failed to insert legacy system category: %v", err)
	}

	categoryService := service.NewCategoryService()
	categoryService.Init(context.Background(), db, &appconf.AppConfig{})

	var normalizedID string
	err := db.QueryRow(`SELECT id FROM categories WHERE name = ? AND is_system = true LIMIT 1`, "最喜欢的游戏").Scan(&normalizedID)
	if err != nil {
		t.Fatalf("failed to query normalized category: %v", err)
	}
	if normalizedID != "system:favorites" {
		t.Fatalf("expected normalized system favorites id to be 'system:favorites', got %s", normalizedID)
	}
}

func TestGameServiceDeleteGameCreatesSyncTombstones(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	gameID := "game-1"
	categoryID := "cat-1"
	sessionID := "session-1"

	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, gameID, "Game One", now, now); err != nil {
		t.Fatalf("failed to insert game: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO categories (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, categoryID, "Test", now, now); err != nil {
		t.Fatalf("failed to insert category: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO game_categories (game_id, category_id, updated_at) VALUES (?, ?, ?)`, gameID, categoryID, now); err != nil {
		t.Fatalf("failed to insert relation: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO play_sessions (id, game_id, start_time, end_time, duration, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, gameID, now, now.Add(time.Hour), int(time.Hour.Seconds()), now); err != nil {
		t.Fatalf("failed to insert play session: %v", err)
	}

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})
	if err := gameService.DeleteGame(gameID); err != nil {
		t.Fatalf("DeleteGame failed: %v", err)
	}

	rows, err := db.Query(`SELECT entity_type, entity_id FROM sync_tombstones`)
	if err != nil {
		t.Fatalf("failed to query tombstones: %v", err)
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	for rows.Next() {
		var entityType, entityID string
		if err := rows.Scan(&entityType, &entityID); err != nil {
			t.Fatalf("failed to scan tombstone: %v", err)
		}
		seen[entityType+"::"+entityID] = struct{}{}
	}

	expected := []string{
		"game::" + gameID,
		"play_session::" + sessionID,
		"game_category::" + gameID + "::" + categoryID,
	}
	for _, key := range expected {
		if _, ok := seen[key]; !ok {
			t.Errorf("expected tombstone %s in sync_tombstones", key)
		}
	}
}

func TestSessionServiceBatchAddClearsTombstoneAndRespectsUpdatedAt(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	gameID := "game-2"
	if _, err := db.Exec(`INSERT INTO games (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, gameID, "Game Two", now, now); err != nil {
		t.Fatalf("failed to insert game: %v", err)
	}

	session := models.PlaySession{
		ID:        "session-2",
		GameID:    gameID,
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
		Duration:  int(time.Hour.Seconds()),
		UpdatedAt: now,
	}

	if _, err := db.Exec(`INSERT INTO sync_tombstones (entity_type, entity_id, parent_id, secondary_id, deleted_at) VALUES (?, ?, '', '', ?)`,
		"play_session", session.ID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("failed to insert tombstone: %v", err)
	}

	sessionService := service.NewSessionService()
	sessionService.Init(context.Background(), db, &appconf.AppConfig{})
	if err := sessionService.BatchAddPlaySessions([]models.PlaySession{session}); err != nil {
		t.Fatalf("BatchAddPlaySessions failed: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sync_tombstones WHERE entity_type = ? AND entity_id = ?`, "play_session", session.ID).Scan(&count); err != nil {
		t.Fatalf("failed to count tombstones: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no tombstone for session %s after insert, found %d", session.ID, count)
	}

	var updatedAt time.Time
	if err := db.QueryRow(`SELECT updated_at FROM play_sessions WHERE id = ?`, session.ID).Scan(&updatedAt); err != nil {
		t.Fatalf("failed to query inserted session: %v", err)
	}
	delta := updatedAt.Sub(session.UpdatedAt.UTC())
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Millisecond {
		t.Fatalf("expected updated_at to match inserted value within tolerance, got %s vs %s", updatedAt, session.UpdatedAt)
	}
}
