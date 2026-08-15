package service

import (
	"context"
	"database/sql"
	"testing"

	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"

	_ "github.com/duckdb/duckdb-go/v2"
)

func setupGameFilterPresetServiceTest(t *testing.T) *GameFilterPresetService {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})

	if _, err := db.Exec(`
		CREATE TABLE game_filter_presets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			tags TEXT NOT NULL DEFAULT '[]',
			exclude_tags BOOLEAN NOT NULL DEFAULT FALSE,
			status TEXT NOT NULL DEFAULT '',
			exclude_status BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("create test table: %v", err)
	}

	service := NewGameFilterPresetService()
	service.Init(context.Background(), db, nil)
	return service
}

func TestGameFilterPresetServiceCRUD(t *testing.T) {
	service := setupGameFilterPresetServiceTest(t)

	created, err := service.CreateGameFilterPreset(vo.SaveGameFilterPresetRequest{
		Name:          "  想玩的剧情游戏  ",
		Tags:          []string{"tag1", " tag2 ", "tag1", ""},
		ExcludeTags:   true,
		Status:        enums.StatusWantToPlay,
		ExcludeStatus: true,
	})
	if err != nil {
		t.Fatalf("create preset: %v", err)
	}
	if created.Name != "想玩的剧情游戏" {
		t.Fatalf("unexpected normalized name: %q", created.Name)
	}
	if len(created.Tags) != 2 || created.Tags[0] != "tag1" || created.Tags[1] != "tag2" {
		t.Fatalf("unexpected normalized tags: %#v", created.Tags)
	}
	if !created.ExcludeTags || !created.ExcludeStatus {
		t.Fatalf("expected inverted filters to be preserved: %#v", created)
	}

	presets, err := service.ListGameFilterPresets()
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	if len(presets) != 1 || presets[0].ID != created.ID {
		t.Fatalf("unexpected presets: %#v", presets)
	}

	updated, err := service.UpdateGameFilterPreset(created.ID, vo.SaveGameFilterPresetRequest{
		Name:          "游玩中",
		Tags:          nil,
		ExcludeTags:   true,
		Status:        enums.StatusPlaying,
		ExcludeStatus: false,
	})
	if err != nil {
		t.Fatalf("update preset: %v", err)
	}
	if len(updated.Tags) != 0 || updated.ExcludeTags {
		t.Fatalf("expected empty tag filter to clear inversion: %#v", updated)
	}
	if updated.Status != enums.StatusPlaying || updated.ExcludeStatus {
		t.Fatalf("unexpected updated status filter: %#v", updated)
	}

	if err := service.DeleteGameFilterPreset(created.ID); err != nil {
		t.Fatalf("delete preset: %v", err)
	}
	presets, err = service.ListGameFilterPresets()
	if err != nil {
		t.Fatalf("list presets after delete: %v", err)
	}
	if len(presets) != 0 {
		t.Fatalf("expected no presets after delete: %#v", presets)
	}
}

func TestGameFilterPresetServiceValidation(t *testing.T) {
	service := setupGameFilterPresetServiceTest(t)

	if _, err := service.CreateGameFilterPreset(vo.SaveGameFilterPresetRequest{Name: "empty"}); err == nil {
		t.Fatal("expected empty filters to be rejected")
	}
	if _, err := service.CreateGameFilterPreset(vo.SaveGameFilterPresetRequest{
		Name:   "invalid status",
		Status: enums.GameStatus("invalid"),
	}); err == nil {
		t.Fatal("expected invalid status to be rejected")
	}
	if _, err := service.CreateGameFilterPreset(vo.SaveGameFilterPresetRequest{
		Name: "   ",
		Tags: []string{"tag1"},
	}); err == nil {
		t.Fatal("expected an empty name to be rejected")
	}
}
