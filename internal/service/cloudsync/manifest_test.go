package cloudsync

import (
	"testing"
	"time"
)

func buildSampleLocal(t *testing.T) (map[string]map[string]*BucketContent, Manifest) {
	t.Helper()
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	snapshot := Snapshot{
		Games: []Game{
			{ID: "3aaa", Name: "G1", UpdatedAt: now, CreatedAt: now},
			{ID: "9bbb", Name: "G2", UpdatedAt: now, CreatedAt: now},
		},
		PlaySessions: []PlaySession{
			{ID: "s1", GameID: "3aaa", StartTime: now, EndTime: now, Duration: 60, UpdatedAt: now},
		},
		Categories: []Category{
			{ID: "cat1", Name: "X", CreatedAt: now, UpdatedAt: now},
		},
	}
	buckets := Bucketize(snapshot)
	m, err := BuildManifestFromBuckets(buckets, snapshot.Categories, nil, nil, "test-device", "rev-local", now)
	if err != nil {
		t.Fatalf("build manifest: %v", err)
	}
	return buckets, m
}

func TestDiffBucketsNoChangesProducesEmptyDiff(t *testing.T) {
	_, local := buildSampleLocal(t)

	cached := make(map[string]SyncStateRow, 0)
	for entityKey, byBucket := range local.Buckets {
		for ch, ref := range byBucket {
			cached[BucketKey(entityKey, ch)] = SyncStateRow{
				BucketKey:  BucketKey(entityKey, ch),
				LocalHash:  ref.Hash,
				RemoteHash: ref.Hash,
			}
		}
	}
	for name, ref := range local.Singletons {
		cached[SingletonStateKey(name)] = SyncStateRow{
			BucketKey:  SingletonStateKey(name),
			LocalHash:  ref.Hash,
			RemoteHash: ref.Hash,
		}
	}

	diff := DiffBuckets(local, cached, local, true)
	if diff.HasWork() {
		t.Errorf("expected empty diff, got %+v", diff)
	}
}

func TestDiffBucketsLocalOnlyChange(t *testing.T) {
	_, local := buildSampleLocal(t)

	// 远端等于本地，但 cached.LocalHash 标记为陈旧 → LocalChanged 非空，ToPull 空
	cached := make(map[string]SyncStateRow, 0)
	for entityKey, byBucket := range local.Buckets {
		for ch, ref := range byBucket {
			cached[BucketKey(entityKey, ch)] = SyncStateRow{
				BucketKey:  BucketKey(entityKey, ch),
				LocalHash:  "stale",
				RemoteHash: ref.Hash,
			}
		}
	}
	for name, ref := range local.Singletons {
		cached[SingletonStateKey(name)] = SyncStateRow{
			BucketKey:  SingletonStateKey(name),
			LocalHash:  "stale",
			RemoteHash: ref.Hash,
		}
	}

	diff := DiffBuckets(local, cached, local, true)
	if len(diff.ToPull) != 0 {
		t.Errorf("expected ToPull empty, got %v", diff.ToPull)
	}
	if len(diff.LocalChanged) == 0 {
		t.Errorf("expected LocalChanged non-empty")
	}
}

func TestDiffBucketsRemoteOnlyChange(t *testing.T) {
	_, local := buildSampleLocal(t)

	// 远端 manifest 与本地内容相同但 hash 字符串伪造为不同；cached 等于"本地版本一致"
	remote := local
	remote.Buckets = make(map[string]map[string]BucketRef, len(local.Buckets))
	for entityKey, byBucket := range local.Buckets {
		remote.Buckets[entityKey] = make(map[string]BucketRef, len(byBucket))
		for ch, ref := range byBucket {
			fake := ref
			if ref.Count > 0 {
				fake.Hash = "remote-changed"
			}
			remote.Buckets[entityKey][ch] = fake
		}
	}

	cached := make(map[string]SyncStateRow, 0)
	for entityKey, byBucket := range local.Buckets {
		for ch, ref := range byBucket {
			cached[BucketKey(entityKey, ch)] = SyncStateRow{
				BucketKey:  BucketKey(entityKey, ch),
				LocalHash:  ref.Hash,
				RemoteHash: ref.Hash,
			}
		}
	}
	for name, ref := range local.Singletons {
		cached[SingletonStateKey(name)] = SyncStateRow{
			BucketKey:  SingletonStateKey(name),
			LocalHash:  ref.Hash,
			RemoteHash: ref.Hash,
		}
	}

	diff := DiffBuckets(local, cached, remote, true)
	if len(diff.LocalChanged) != 0 {
		t.Errorf("expected LocalChanged empty, got %v", diff.LocalChanged)
	}
	if len(diff.ToPull) == 0 {
		t.Errorf("expected ToPull non-empty")
	}
}

func TestDiffBucketsRemoteAbsentTreatsLocalNonEmptyAsChanged(t *testing.T) {
	_, local := buildSampleLocal(t)

	diff := DiffBuckets(local, map[string]SyncStateRow{}, Manifest{}, false)
	// 远端不存在 → ToPull 空（无 manifest 可拉），所有本地非空桶 + singleton 都视为 LocalChanged
	if len(diff.ToPull) != 0 {
		t.Errorf("expected ToPull empty when remote absent")
	}
	if len(diff.LocalChanged) == 0 {
		t.Errorf("expected at least one LocalChanged bucket")
	}
}

func TestDiffBucketsCoverOnlyChangeHasWork(t *testing.T) {
	_, local := buildSampleLocal(t)
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	local.Covers = []CoverRef{{GameID: "3aaa", Ext: ".webp", UpdatedAt: now, Hash: "local-cover"}}
	remote := local
	remote.Covers = []CoverRef{{GameID: "3aaa", Ext: ".webp", UpdatedAt: now.Add(-time.Minute), Hash: "remote-cover"}}

	cached := make(map[string]SyncStateRow)
	for entityKey, byBucket := range local.Buckets {
		for ch, ref := range byBucket {
			cached[BucketKey(entityKey, ch)] = SyncStateRow{BucketKey: BucketKey(entityKey, ch), LocalHash: ref.Hash, RemoteHash: ref.Hash}
		}
	}
	for name, ref := range local.Singletons {
		cached[SingletonStateKey(name)] = SyncStateRow{BucketKey: SingletonStateKey(name), LocalHash: ref.Hash, RemoteHash: ref.Hash}
	}

	diff := DiffBuckets(local, cached, remote, true)
	if !diff.HasWork() {
		t.Fatal("expected cover-only difference to trigger sync")
	}
	if len(diff.CoversChanged) != 1 || diff.CoversChanged[0] != "3aaa" {
		t.Fatalf("unexpected changed covers: %v", diff.CoversChanged)
	}
}

func TestAssembleFinalSnapshotKeepsMergedRemoteCover(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	game := Game{ID: "3aaa", Name: "G1", CreatedAt: now, UpdatedAt: now}
	remoteCover := CoverAsset{GameID: game.ID, Ext: ".webp", UpdatedAt: now}
	changed := map[string]struct{}{BucketKey(EntityKeyGames, BucketKeyOfGame(game.ID)): {}}
	helper := &Helper{}
	merged := helper.MergeSnapshots(
		Snapshot{},
		Snapshot{Games: []Game{game}, Covers: []CoverAsset{remoteCover}},
		true,
	)

	got := assembleFinalSnapshot(
		Bucketize(Snapshot{}),
		Bucketize(Snapshot{Games: []Game{game}}),
		changed,
		merged,
		Snapshot{},
	)

	if len(got.Covers) != 1 || got.Covers[0].GameID != game.ID {
		t.Fatalf("expected merged remote cover to survive assembly, got %+v", got.Covers)
	}
}

func TestEncodeManifestDeterministic(t *testing.T) {
	_, local := buildSampleLocal(t)
	b1, err := EncodeManifest(local)
	if err != nil {
		t.Fatalf("encode 1: %v", err)
	}
	b2, err := EncodeManifest(local)
	if err != nil {
		t.Fatalf("encode 2: %v", err)
	}
	if string(b1) != string(b2) {
		t.Errorf("manifest encoding should be deterministic")
	}
}

func TestBuildManifestExposesCurrentSchema(t *testing.T) {
	_, local := buildSampleLocal(t)
	if local.SchemaVersion != SchemaVersion {
		t.Errorf("expected schema v%d, got %d", SchemaVersion, local.SchemaVersion)
	}
}
