package service

import (
	"context"
	"testing"

	"lunabox/internal/appconf"
	"lunabox/internal/utils/metadata"
)

func TestUpsertScrapedTagsKeepsOtherProviderTags(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewTagService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	if err := service.upsertScrapedTagsForSource("game-1", "bangumi", []metadata.TagItem{{Name: "ADV"}}); err != nil {
		t.Fatalf("insert Bangumi tags: %v", err)
	}
	if err := service.upsertScrapedTagsForSource("game-1", "hikarinagi", []metadata.TagItem{{Name: "Drama"}}); err != nil {
		t.Fatalf("insert Hikarinagi tags: %v", err)
	}
	if err := service.upsertScrapedTagsForSource("game-1", "bangumi", []metadata.TagItem{{Name: "Visual Novel"}}); err != nil {
		t.Fatalf("replace Bangumi tags: %v", err)
	}

	rows, err := db.Query(`SELECT source, name FROM game_tags WHERE game_id = 'game-1' ORDER BY source, name`)
	if err != nil {
		t.Fatalf("query provider tags: %v", err)
	}
	defer rows.Close()
	got := make([]string, 0, 2)
	for rows.Next() {
		var source string
		var name string
		if err := rows.Scan(&source, &name); err != nil {
			t.Fatalf("scan provider tag: %v", err)
		}
		got = append(got, source+":"+name)
	}
	want := []string{"bangumi:Visual Novel", "hikarinagi:Drama"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected provider tags: got %v want %v", got, want)
	}

	if err := service.upsertScrapedTagsForSource("game-1", "bangumi", nil); err != nil {
		t.Fatalf("clear Bangumi tags: %v", err)
	}
	var hikariCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM game_tags WHERE game_id = 'game-1' AND source = 'hikarinagi'`).Scan(&hikariCount); err != nil {
		t.Fatalf("count Hikarinagi tags: %v", err)
	}
	if hikariCount != 1 {
		t.Fatalf("expected Hikarinagi tag to remain, got %d", hikariCount)
	}
}
