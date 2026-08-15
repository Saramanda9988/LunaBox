package cloudsync

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBucketKeyOfGame(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"3a2e1f00-0000-0000-0000-000000000000", "3"},
		{"deadbeef", "d"},
		{"ABCDEF12-0000-0000-0000-000000000000", "a"}, // 大写 → 小写
		{"", "0"},
		{"not-hex-prefix", "0"},
		{"5", "5"},
		{"system:favorites", "0"}, // 非 hex 落到桶 0
	}
	for _, c := range cases {
		got := BucketKeyOfGame(c.in)
		if got != c.want {
			t.Errorf("BucketKeyOfGame(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBucketizeUnbucketizeRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	snapshot := Snapshot{
		SchemaVersion: SchemaVersion,
		Games: []Game{
			{ID: "3a2e", Name: "G1", UpdatedAt: now, CreatedAt: now},
			{ID: "9bbb", Name: "G2", UpdatedAt: now, CreatedAt: now},
		},
		PlaySessions: []PlaySession{
			{ID: "s1", GameID: "3a2e", StartTime: now, EndTime: now, Duration: 60, UpdatedAt: now},
		},
		GameProgresses: []GameProgress{
			{ID: "p1", GameID: "9bbb", UpdatedAt: now},
		},
		GameReviews: []GameReview{
			{GameID: "3a2e", Rating: intPtr(9), Content: "great", IsSpoiler: true, UpdatedAt: now, CreatedAt: now},
		},
		GameTags: []GameTag{
			{ID: "t1", GameID: "3a2e", Name: "fav", Source: "user", Weight: 1.0, UpdatedAt: now, CreatedAt: now},
		},
		MetadataSources: []MetadataSource{
			{GameID: "3a2e", SourceType: "bangumi", SourceID: "42", CachedAt: now, UpdatedAt: now, CreatedAt: now},
			{GameID: "3a2e", SourceType: "hikarinagi", SourceID: "84", CachedAt: now, UpdatedAt: now, CreatedAt: now},
		},
		GameCategories: []Relation{
			{GameID: "9bbb", CategoryID: "cat1", UpdatedAt: now},
		},
		Categories: []Category{
			{ID: "cat1", Name: "X", IsSystem: false, CreatedAt: now, UpdatedAt: now},
		},
		Tombstones: []Tombstone{
			{EntityType: entityGame, EntityID: "old", DeletedAt: now},
		},
	}

	buckets := Bucketize(snapshot)

	// 3a2e 应当在桶 "3"，9bbb 应当在桶 "9"
	if len(buckets[EntityKeyGames]["3"].Games) != 1 || buckets[EntityKeyGames]["3"].Games[0].ID != "3a2e" {
		t.Errorf("game 3a2e missing from bucket 3, got %+v", buckets[EntityKeyGames]["3"])
	}
	if len(buckets[EntityKeyGames]["9"].Games) != 1 || buckets[EntityKeyGames]["9"].Games[0].ID != "9bbb" {
		t.Errorf("game 9bbb missing from bucket 9")
	}
	if len(buckets[EntityKeyPlaySessions]["3"].PlaySessions) != 1 {
		t.Errorf("play session for 3a2e missing from bucket 3")
	}

	// 拼回 → 与原 snapshot 等价（忽略 SchemaVersion）
	got := Unbucketize(buckets, snapshot.Categories, snapshot.Tombstones)
	if !reflect.DeepEqual(got.Games, snapshot.Games) {
		t.Errorf("Games mismatch after round-trip:\n got: %+v\nwant: %+v", got.Games, snapshot.Games)
	}
	if !reflect.DeepEqual(got.PlaySessions, snapshot.PlaySessions) {
		t.Errorf("PlaySessions mismatch after round-trip")
	}
	if !reflect.DeepEqual(got.MetadataSources, snapshot.MetadataSources) {
		t.Errorf("MetadataSources mismatch after round-trip: got %+v want %+v", got.MetadataSources, snapshot.MetadataSources)
	}
	if !reflect.DeepEqual(got.GameReviews, snapshot.GameReviews) {
		t.Errorf("GameReviews mismatch after round-trip: got %+v want %+v", got.GameReviews, snapshot.GameReviews)
	}
}

func intPtr(value int) *int {
	return &value
}

func TestBucketHashIsDeterministic(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	games := []Game{
		{ID: "a", Name: "Alpha", UpdatedAt: now, CreatedAt: now},
		{ID: "b", Name: "Beta", UpdatedAt: now, CreatedAt: now},
	}
	h1, err := BucketHash(games)
	if err != nil {
		t.Fatalf("first hash failed: %v", err)
	}
	h2, err := BucketHash(games)
	if err != nil {
		t.Fatalf("second hash failed: %v", err)
	}
	if h1 != h2 {
		t.Errorf("hash should be deterministic, got %s vs %s", h1, h2)
	}
	if len(h1) != 32 {
		t.Errorf("hash should be 32 hex chars, got %d (%s)", len(h1), h1)
	}
}

func TestBucketHashTimePrecisionInsensitive(t *testing.T) {
	base := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	withNanos := base.Add(123 * time.Microsecond) // 同一秒内的不同微秒
	g1 := []Game{{ID: "a", Name: "Alpha", UpdatedAt: base, CreatedAt: base}}
	g2 := []Game{{ID: "a", Name: "Alpha", UpdatedAt: withNanos, CreatedAt: withNanos}}

	h1, err := BucketHash(g1)
	if err != nil {
		t.Fatalf("hash g1 failed: %v", err)
	}
	h2, err := BucketHash(g2)
	if err != nil {
		t.Fatalf("hash g2 failed: %v", err)
	}
	if h1 != h2 {
		t.Errorf("hash should ignore sub-second time precision, got %s vs %s", h1, h2)
	}
}

func TestBucketHashTimezoneInsensitive(t *testing.T) {
	utc := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	cst := utc.In(time.FixedZone("CST", 8*3600))
	g1 := []Game{{ID: "a", Name: "Alpha", UpdatedAt: utc, CreatedAt: utc}}
	g2 := []Game{{ID: "a", Name: "Alpha", UpdatedAt: cst, CreatedAt: cst}}

	h1, _ := BucketHash(g1)
	h2, _ := BucketHash(g2)
	if h1 != h2 {
		t.Errorf("hash should be timezone-independent (same instant), got %s vs %s", h1, h2)
	}
}

func TestBucketHashChangesWithData(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	g1 := []Game{{ID: "a", Name: "Alpha", UpdatedAt: now, CreatedAt: now}}
	g2 := []Game{{ID: "a", Name: "Alpha edited", UpdatedAt: now, CreatedAt: now}}

	h1, _ := BucketHash(g1)
	h2, _ := BucketHash(g2)
	if h1 == h2 {
		t.Errorf("hash should change when content changes")
	}
}

func TestBucketHashEmptyIsStable(t *testing.T) {
	var zeroSlice []Game
	emptySlice := []Game{}
	nilAny, err := BucketHash(zeroSlice)
	if err != nil {
		t.Fatalf("nil slice hash failed: %v", err)
	}
	emptyAny, err := BucketHash(emptySlice)
	if err != nil {
		t.Fatalf("empty slice hash failed: %v", err)
	}
	if nilAny != emptyAny {
		t.Errorf("nil vs empty slice should hash identically, got %s vs %s", nilAny, emptyAny)
	}
}

func TestMarshalUnmarshalBucketFileRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	bc := &BucketContent{
		Games: []Game{
			{ID: "3aaa", Name: "Test", UpdatedAt: now, CreatedAt: now},
		},
	}
	raw, err := MarshalBucketFile(EntityKeyGames, "3", bc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(raw), `"games/3"`) {
		t.Errorf("bucket_key not in payload: %s", string(raw))
	}

	entity, ch, got, err := UnmarshalBucketFile(raw)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if entity != EntityKeyGames || ch != "3" {
		t.Errorf("entity/ch parse mismatch: %s/%s", entity, ch)
	}
	if len(got.Games) != 1 || got.Games[0].ID != "3aaa" {
		t.Errorf("round-trip lost game content: %+v", got)
	}
}
