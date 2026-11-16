package service

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service/gamehelper"
	"lunabox/backend/utils/metadata"
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

func tagRows(t *testing.T, db interface {
	Query(string, ...any) (*sql.Rows, error)
}, gameID string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT source, name FROM game_tags WHERE game_id = ? ORDER BY source, name`, gameID)
	if err != nil {
		t.Fatalf("query tags: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var source, name string
		if err := rows.Scan(&source, &name); err != nil {
			t.Fatalf("scan tag: %v", err)
		}
		got = append(got, source+":"+name)
	}
	return got
}

// 替换模式删除其他来源自动获取的标签，用户标签保留。
func TestReplaceScrapedTagsDropsOtherProvidersButKeepsUserTags(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewTagService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	if err := service.upsertScrapedTagsForSource("game-1", "bangumi", []metadata.TagItem{{Name: "ADV"}}); err != nil {
		t.Fatalf("insert Bangumi tags: %v", err)
	}
	if err := service.AddUserTag("game-1", "Mine"); err != nil {
		t.Fatalf("add user tag: %v", err)
	}
	if err := service.replaceScrapedTags("game-1", "vndb", []metadata.TagItem{{Name: "Romance"}}); err != nil {
		t.Fatalf("replace with VNDB tags: %v", err)
	}

	want := []string{"user:Mine", "vndb:Romance"}
	if got := tagRows(t, db, "game-1"); !slices.Equal(got, want) {
		t.Fatalf("unexpected tags: got %v want %v", got, want)
	}
}

// 批量删除只删给出的标签，不存在的 ID 忽略。
func TestDeleteTags(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewTagService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	if err := service.upsertScrapedTagsForSource("game-1", "bangumi", []metadata.TagItem{{Name: "ADV"}, {Name: "Drama"}}); err != nil {
		t.Fatalf("insert tags: %v", err)
	}
	if err := service.AddUserTag("game-1", "Mine"); err != nil {
		t.Fatalf("add user tag: %v", err)
	}
	tags, err := service.GetTagsByGame("game-1")
	if err != nil {
		t.Fatalf("get tags: %v", err)
	}
	var ids []string
	for _, tag := range tags {
		if tag.Name != "Drama" {
			ids = append(ids, tag.ID)
		}
	}
	if err := service.DeleteTags(append(ids, "missing")); err != nil {
		t.Fatalf("delete tags: %v", err)
	}
	if got := tagRows(t, db, "game-1"); !slices.Equal(got, []string{"bangumi:Drama"}) {
		t.Fatalf("unexpected tags after delete: %v", got)
	}
}

// 只差大小写的标签名在搜索里算一个，按存在性过滤也不区分大小写。
func TestTagSearchIsCaseInsensitive(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewTagService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	if err := service.upsertScrapedTagsForSource("game-1", "bangumi", []metadata.TagItem{{Name: "ADV"}}); err != nil {
		t.Fatalf("insert tags: %v", err)
	}
	if err := service.upsertScrapedTagsForSource("game-2", "vndb", []metadata.TagItem{{Name: "ADV"}}); err != nil {
		t.Fatalf("insert tags: %v", err)
	}
	if err := service.upsertScrapedTagsForSource("game-1", "kungal", []metadata.TagItem{{Name: "adv"}}); err != nil {
		t.Fatalf("insert tags: %v", err)
	}

	names, err := service.SearchTagsInLibrary("ad")
	if err != nil {
		t.Fatalf("search tags: %v", err)
	}
	if !slices.Equal(names, []string{"ADV"}) {
		t.Fatalf("search = %v, want [ADV]", names)
	}
	existing, err := service.FilterExistingTagNames([]string{"Adv", "Missing"})
	if err != nil {
		t.Fatalf("filter existing: %v", err)
	}
	if !slices.Equal(existing, []string{"Adv"}) {
		t.Fatalf("existing = %v, want [Adv]", existing)
	}
	ids, err := service.GetGameIDsByTag("adv")
	if err != nil {
		t.Fatalf("game ids by tag: %v", err)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"game-1", "game-2"}) {
		t.Fatalf("game ids = %v", ids)
	}
}

// 设置了“更新元数据时替换旧标签”时，更新元数据删除其他来源的标签；本次没有拿到标签时不清空。
func TestApplyMetadataReplaceTagsSetting(t *testing.T) {
	for _, replace := range []bool{false, true} {
		db := setupImportServiceTestDB(t)
		cfg := &appconf.AppConfig{ReplaceTagsOnMetadataUpdate: replace}
		games := NewGameService()
		games.Init(context.Background(), db, cfg)
		tags := NewTagService()
		tags.Init(context.Background(), db, cfg)
		games.SetTagService(tags)

		existing := models.Game{ID: "replace-tags", Name: "Game", SourceType: enums.VNDB, SourceID: "v1"}
		if err := games.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{Game: existing, Tags: []metadata.TagItem{{Name: "ADV", Source: "bangumi"}}}); err != nil {
			t.Fatalf("add game: %v", err)
		}
		if err := tags.AddUserTag(existing.ID, "Mine"); err != nil {
			t.Fatalf("add user tag: %v", err)
		}
		existing, err := games.GetGameByID(existing.ID)
		if err != nil {
			t.Fatalf("get game: %v", err)
		}
		fields := gamehelper.NormalizeMetadataUpdateFields([]enums.MetadataUpdateField{enums.MetadataUpdateFieldTags})

		// 没有拿到标签：两种设置都不动其他来源。
		if _, err := games.applyRemoteMetadataResult(existing, metadata.MetadataResult{Game: models.Game{SourceType: enums.VNDB}}, false, fields); err != nil {
			t.Fatalf("apply empty tags: %v", err)
		}
		if got := tagRows(t, db, existing.ID); !slices.Equal(got, []string{"bangumi:ADV", "user:Mine"}) {
			t.Fatalf("replace=%v empty refresh changed tags: %v", replace, got)
		}

		if _, err := games.applyRemoteMetadataResult(existing, metadata.MetadataResult{
			Game: models.Game{SourceType: enums.VNDB},
			Tags: []metadata.TagItem{{Name: "Romance"}},
		}, false, fields); err != nil {
			t.Fatalf("apply tags: %v", err)
		}
		want := []string{"bangumi:ADV", "user:Mine", "vndb:Romance"}
		if replace {
			want = []string{"user:Mine", "vndb:Romance"}
		}
		if got := tagRows(t, db, existing.ID); !slices.Equal(got, want) {
			t.Fatalf("replace=%v: got %v want %v", replace, got, want)
		}
	}
}
