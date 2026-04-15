package test

import (
	"lunabox/internal/service"
	"testing"
	"time"
)

func TestCloudSyncMergeSnapshotsPrefersRemoteWhenNewerOrTied(t *testing.T) {
	svc := service.NewCloudSyncService()
	base := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)

	merged := svc.mergeSnapshots(
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-newer", Name: "local", UpdatedAt: base.Add(-2 * time.Hour)},
				{ID: "game-tied", Name: "local-tied", UpdatedAt: base},
			},
		},
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-newer", Name: "remote", UpdatedAt: base},
				{ID: "game-tied", Name: "remote-tied", UpdatedAt: base},
			},
		},
		true,
	)

	got := service.mapGames(merged.Games)
	if got["game-newer"].Name != "remote" {
		t.Fatalf("expected newer remote game to win, got %q", got["game-newer"].Name)
	}
	if got["game-tied"].Name != "remote-tied" {
		t.Fatalf("expected tied record to prefer remote, got %q", got["game-tied"].Name)
	}
	if merged.RevisionID == "" || merged.DeviceID == "" || merged.ExportedAt.IsZero() {
		t.Fatal("expected merged snapshot metadata to be populated")
	}
}

func TestCloudSyncMergeSnapshotsAppliesNewerSessionTombstone(t *testing.T) {
	svc := service.NewCloudSyncService()
	base := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)

	merged := svc.mergeSnapshots(
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-1", Name: "local", UpdatedAt: base},
			},
			PlaySessions: []service.cloudSyncPlaySession{
				{ID: "session-1", GameID: "game-1", UpdatedAt: base},
			},
		},
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-1", Name: "remote", UpdatedAt: base},
			},
			Tombstones: []service.cloudSyncTombstone{
				{EntityType: service.cloudSyncEntityPlaySession, EntityID: "session-1", DeletedAt: base.Add(time.Hour)},
			},
		},
		true,
	)

	if len(merged.PlaySessions) != 0 {
		t.Fatalf("expected play session to be removed by newer tombstone, got %d sessions", len(merged.PlaySessions))
	}

	tombstones := service.mapTombstones(merged.Tombstones, service.cloudSyncEntityPlaySession)
	if deletedAt := tombstones["session-1"]; deletedAt.IsZero() {
		t.Fatal("expected merged tombstones to retain the play session deletion")
	}
}

func TestCloudSyncMergeCoversDropsStaleRemoteCoverAfterLocalRemoval(t *testing.T) {
	svc := service.NewCloudSyncService()
	base := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)

	merged := svc.mergeSnapshots(
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-1", Name: "local", UpdatedAt: base.Add(2 * time.Hour)},
			},
		},
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-1", Name: "remote", UpdatedAt: base},
			},
			Covers: []service.cloudSyncCoverAsset{
				{GameID: "game-1", Ext: ".jpg", UpdatedAt: base.Add(time.Hour)},
			},
		},
		true,
	)

	if len(merged.Covers) != 0 {
		t.Fatalf("expected cover manifest to drop stale remote cover after newer local removal, got %d covers", len(merged.Covers))
	}
}

func TestCloudSyncMergeSnapshotsAppliesNewerTagTombstone(t *testing.T) {
	svc := service.NewCloudSyncService()
	base := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)

	merged := svc.mergeSnapshots(
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-1", Name: "local", UpdatedAt: base},
			},
			GameTags: []service.cloudSyncGameTag{
				{ID: "tag-local", GameID: "game-1", Name: "mystery", Source: "user", CreatedAt: base, UpdatedAt: base},
			},
		},
		service.cloudSyncSnapshot{
			Games: []service.cloudSyncGame{
				{ID: "game-1", Name: "remote", UpdatedAt: base},
			},
			Tombstones: []service.cloudSyncTombstone{
				{EntityType: service.cloudSyncEntityGameTag, EntityID: service.tagTombstoneID("game-1", "user", "mystery"), DeletedAt: base.Add(time.Hour)},
			},
		},
		true,
	)

	if len(merged.GameTags) != 0 {
		t.Fatalf("expected game tag to be removed by newer tombstone, got %d tags", len(merged.GameTags))
	}

	tombstones := service.mapTombstones(merged.Tombstones, service.cloudSyncEntityGameTag)
	if deletedAt := tombstones[service.tagTombstoneID("game-1", "user", "mystery")]; deletedAt.IsZero() {
		t.Fatal("expected merged tombstones to retain the game tag deletion")
	}
}
