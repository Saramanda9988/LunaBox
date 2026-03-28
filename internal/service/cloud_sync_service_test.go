package service

import (
	"testing"
	"time"
)

func TestCloudSyncMergeSnapshotsPrefersRemoteWhenNewerOrTied(t *testing.T) {
	svc := NewCloudSyncService()
	base := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)

	merged := svc.mergeSnapshots(
		cloudSyncSnapshot{
			Games: []cloudSyncGame{
				{ID: "game-newer", Name: "local", UpdatedAt: base.Add(-2 * time.Hour)},
				{ID: "game-tied", Name: "local-tied", UpdatedAt: base},
			},
		},
		cloudSyncSnapshot{
			Games: []cloudSyncGame{
				{ID: "game-newer", Name: "remote", UpdatedAt: base},
				{ID: "game-tied", Name: "remote-tied", UpdatedAt: base},
			},
		},
		true,
	)

	got := mapGames(merged.Games)
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
	svc := NewCloudSyncService()
	base := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)

	merged := svc.mergeSnapshots(
		cloudSyncSnapshot{
			Games: []cloudSyncGame{
				{ID: "game-1", Name: "local", UpdatedAt: base},
			},
			PlaySessions: []cloudSyncPlaySession{
				{ID: "session-1", GameID: "game-1", UpdatedAt: base},
			},
		},
		cloudSyncSnapshot{
			Games: []cloudSyncGame{
				{ID: "game-1", Name: "remote", UpdatedAt: base},
			},
			Tombstones: []cloudSyncTombstone{
				{EntityType: cloudSyncEntityPlaySession, EntityID: "session-1", DeletedAt: base.Add(time.Hour)},
			},
		},
		true,
	)

	if len(merged.PlaySessions) != 0 {
		t.Fatalf("expected play session to be removed by newer tombstone, got %d sessions", len(merged.PlaySessions))
	}

	tombstones := mapTombstones(merged.Tombstones, cloudSyncEntityPlaySession)
	if deletedAt := tombstones["session-1"]; deletedAt.IsZero() {
		t.Fatal("expected merged tombstones to retain the play session deletion")
	}
}

func TestCloudSyncMergeCoversDropsStaleRemoteCoverAfterLocalRemoval(t *testing.T) {
	svc := NewCloudSyncService()
	base := time.Date(2026, 3, 28, 8, 0, 0, 0, time.UTC)

	merged := svc.mergeSnapshots(
		cloudSyncSnapshot{
			Games: []cloudSyncGame{
				{ID: "game-1", Name: "local", UpdatedAt: base.Add(2 * time.Hour)},
			},
		},
		cloudSyncSnapshot{
			Games: []cloudSyncGame{
				{ID: "game-1", Name: "remote", UpdatedAt: base},
			},
			Covers: []cloudSyncCoverAsset{
				{GameID: "game-1", Ext: ".jpg", UpdatedAt: base.Add(time.Hour)},
			},
		},
		true,
	)

	if len(merged.Covers) != 0 {
		t.Fatalf("expected cover manifest to drop stale remote cover after newer local removal, got %d covers", len(merged.Covers))
	}
}
