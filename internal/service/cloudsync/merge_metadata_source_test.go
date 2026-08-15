package cloudsync

import (
	"testing"
	"time"
)

func TestMergeSnapshotsKeepsMetadataSourcesAddedOnDifferentDevices(t *testing.T) {
	now := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	game := Game{ID: "a-game", Name: "Game", CreatedAt: now, UpdatedAt: now}
	helper := &Helper{}
	merged := helper.MergeSnapshots(
		Snapshot{
			Games: []Game{game},
			MetadataSources: []MetadataSource{
				{GameID: game.ID, SourceType: "bangumi", SourceID: "42", CreatedAt: now, UpdatedAt: now},
			},
		},
		Snapshot{
			Games: []Game{game},
			MetadataSources: []MetadataSource{
				{GameID: game.ID, SourceType: "hikarinagi", SourceID: "84", CreatedAt: now, UpdatedAt: now},
			},
		},
		true,
	)

	if len(merged.MetadataSources) != 2 {
		t.Fatalf("expected two merged metadata sources, got %+v", merged.MetadataSources)
	}
	if merged.MetadataSources[0].SourceType != "bangumi" || merged.MetadataSources[1].SourceType != "hikarinagi" {
		t.Fatalf("unexpected merged metadata source order: %+v", merged.MetadataSources)
	}
}

func TestMergeSnapshotsConvertsLegacyGameSource(t *testing.T) {
	now := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	helper := &Helper{}
	merged := helper.MergeSnapshots(Snapshot{}, Snapshot{
		Games: []Game{{
			ID:         "b-game",
			Name:       "Legacy",
			SourceType: "bangumi",
			SourceID:   "101",
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
	}, true)

	if len(merged.MetadataSources) != 1 {
		t.Fatalf("expected a converted metadata source, got %+v", merged.MetadataSources)
	}
	if merged.MetadataSources[0].SourceType != "bangumi" || merged.MetadataSources[0].SourceID != "101" {
		t.Fatalf("unexpected converted metadata source: %+v", merged.MetadataSources[0])
	}
}
