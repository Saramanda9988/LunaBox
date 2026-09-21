package service

import (
	"context"
	"sync"
	"testing"

	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/service/gamehelper/idmapper"
)

func TestGameIDMapperLoadsOnDemand(t *testing.T) {
	svc := NewGameService()
	if svc.idMapperLoaded || svc.idMapper != nil {
		t.Fatal("constructor eagerly loaded the mapping database")
	}
	svc.Init(context.Background(), setupImportServiceTestDB(t), &appconf.AppConfig{})
	preview, err := svc.PreviewLegacyGameMetadataSourceIDs()
	if err != nil || len(preview.Items) != 0 {
		t.Fatalf("empty library preview = %+v, %v", preview, err)
	}
	if svc.idMapperLoaded {
		t.Fatal("empty library loaded the mapping database")
	}

	var callers sync.WaitGroup
	results := make(chan *idmapper.Mapper, 16)
	for range 16 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			mapper, loadErr := svc.getGameIDMapper()
			if loadErr != nil {
				t.Errorf("load mapper: %v", loadErr)
			}
			results <- mapper
		}()
	}
	callers.Wait()
	close(results)
	for mapper := range results {
		if mapper == nil || mapper != svc.idMapper {
			t.Fatal("concurrent callers did not share the same mapper")
		}
	}
	fixture := idmapper.New([]idmapper.IDs{{VNDBID: 1}})
	svc.SetGameIDMapper(fixture)
	if got, err := svc.getGameIDMapper(); err != nil || got != fixture {
		t.Fatalf("injected mapper = %p, %v; want %p", got, err, fixture)
	}
}

func TestPreviewAndEnrichLegacyGameMetadataSourceIDs(t *testing.T) {
	db := setupImportServiceTestDB(t)
	ctx := context.Background()
	gameService := NewGameService()
	gameService.Init(ctx, db, &appconf.AppConfig{})
	gameService.SetGameIDMapper(idmapper.New([]idmapper.IDs{
		{VNDBID: 10, BangumiID: 20, SteamID: 30, HikarinagiID: 40},
		{VNDBID: 50, BangumiID: 40, SteamID: 60, HikarinagiID: 70},
	}))

	for _, game := range []struct {
		id         string
		sourceType enums.SourceType
		sourceID   string
	}{
		{id: "from-vndb", sourceType: enums.VNDB, sourceID: "v10"},
		{id: "from-bangumi", sourceType: enums.Bangumi, sourceID: "40"},
		{id: "unmatched", sourceType: enums.Steam, sourceID: "70"},
		{id: "other-default", sourceType: enums.Ymgal, sourceID: "ga80"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO games (id, name, source_type, source_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, game.id, game.id, string(game.sourceType), game.sourceID); err != nil {
			t.Fatalf("insert game %s: %v", game.id, err)
		}
	}

	for _, source := range []struct {
		gameID   string
		source   enums.SourceType
		sourceID string
	}{
		{gameID: "from-vndb", source: enums.VNDB, sourceID: "v10"},
		{gameID: "from-bangumi", source: enums.Bangumi, sourceID: "40"},
		{gameID: "from-bangumi", source: enums.VNDB, sourceID: "v999"},
		{gameID: "unmatched", source: enums.Steam, sourceID: "70"},
		{gameID: "other-default", source: enums.VNDB, sourceID: "v10"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO game_metadata_sources (game_id, source_type, source_id)
			VALUES (?, ?, ?)
		`, source.gameID, string(source.source), source.sourceID); err != nil {
			t.Fatalf("insert metadata source for %s: %v", source.gameID, err)
		}
	}

	preview, err := gameService.PreviewLegacyGameMetadataSourceIDs()
	if err != nil {
		t.Fatalf("preview game ID enrichment: %v", err)
	}
	if preview.ScannedGames != 3 || preview.EnrichableGames != 2 ||
		preview.UnchangedGames != 1 || preview.AddedSources != 5 || len(preview.Items) != 3 {
		t.Fatalf("unexpected enrichment preview: %+v", preview)
	}
	vndbPreview := findEnrichmentPreviewItem(t, preview.Items, "from-vndb")
	if !vndbPreview.CanEnrich || len(vndbPreview.AddedSources) != 3 {
		t.Fatalf("unexpected VNDB enrichment preview: %+v", vndbPreview)
	}
	unmatchedPreview := findEnrichmentPreviewItem(t, preview.Items, "unmatched")
	if unmatchedPreview.CanEnrich || unmatchedPreview.Reason != "no_mapping" {
		t.Fatalf("unexpected unmatched enrichment preview: %+v", unmatchedPreview)
	}

	result, err := gameService.EnrichLegacyGameMetadataSourceIDs()
	if err != nil {
		t.Fatalf("enrich game IDs: %v", err)
	}
	if result.UpdatedGames != 2 || result.AddedSources != 5 || result.UnmatchedGames != 1 {
		t.Fatalf("unexpected enrichment result: %+v", result)
	}
}

func findEnrichmentPreviewItem(
	t *testing.T,
	items []GameIDEnrichmentPreviewItem,
	gameID string,
) GameIDEnrichmentPreviewItem {
	t.Helper()
	for _, item := range items {
		if item.GameID == gameID {
			return item
		}
	}
	t.Fatalf("missing enrichment preview for %s", gameID)
	return GameIDEnrichmentPreviewItem{}
}
