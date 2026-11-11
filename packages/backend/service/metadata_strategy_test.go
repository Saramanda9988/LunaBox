package service

import (
	"context"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service/gamehelper"
	"lunabox/backend/service/gamehelper/idmapper"
	"lunabox/backend/utils/metadata"
)

func TestSelectImportMetadataMatchByID(t *testing.T) {
	matches := []vo.GameMetadataFromWebVO{
		{Source: enums.VNDB, Game: models.Game{Name: "A", SourceID: "v1"}},
		{Source: enums.VNDB, Game: models.Game{Name: "B", SourceID: "V2"}},
	}
	if got, ok := selectImportMetadataMatchByID(matches, "v2"); !ok || got.Game.Name != "B" {
		t.Fatalf("got %+v, %v", got, ok)
	}
	if _, ok := selectImportMetadataMatchByID(matches, ""); ok {
		t.Fatal("empty ID should not match")
	}
}

func TestMappedMetadataSourceIDs(t *testing.T) {
	s := &GameService{}
	s.SetGameIDMapper(idmapper.New([]idmapper.IDs{{VNDBID: 10, BangumiID: 20, HikarinagiID: 40}}))
	got := s.mappedMetadataSourceIDs(map[enums.SourceType]string{enums.Bangumi: "20"})
	if got[enums.VNDB] != "v10" || got[enums.Hikarinagi] != "40" || got[enums.Bangumi] != "" || len(got) != 2 {
		t.Fatalf("mapped = %v", got)
	}
	var nilService *GameService
	if nilService.mappedMetadataSourceIDs(map[enums.SourceType]string{enums.Bangumi: "20"}) != nil {
		t.Fatal("nil service should map nothing")
	}
}

func TestComposeImportMatchesFollowsStrategyBase(t *testing.T) {
	config := &appconf.AppConfig{}
	config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true}.WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi))
	s := &ImportService{config: config}
	matches := []vo.GameMetadataFromWebVO{
		{Source: enums.VNDB, Game: models.Game{Name: "Title", SourceID: "v1"}},
		{Source: enums.Bangumi, Game: models.Game{Name: "标题", SourceID: "20"}},
	}
	composed, sources := s.composeImportMatches(enums.VNDB, matches)
	if composed == nil || composed.Game.Name != "标题" || composed.Source != enums.VNDB || sources[enums.MetadataUpdateFieldName] != enums.Bangumi {
		t.Fatalf("composed = %+v, sources = %v", composed, sources)
	}
	if composed, _ := s.composeImportMatches(enums.Bangumi, matches); composed != nil {
		t.Fatal("strategy for another base should not apply")
	}
}

func TestResolveMetadataSourceIDs(t *testing.T) {
	s := &GameService{}
	s.SetGameIDMapper(idmapper.New([]idmapper.IDs{{VNDBID: 10, BangumiID: 20, HikarinagiID: 40}}))
	game := models.Game{
		SourceType: enums.Bangumi, SourceID: "20",
		MetadataSources: []models.GameMetadataSource{
			{SourceType: enums.Bangumi, SourceID: "20"},
			{SourceType: enums.Hikarinagi, SourceID: "99"}, // 已关联的 ID 优先于对照表
			{SourceType: enums.Ymgal, SourceID: " "},
		},
	}
	got := s.ResolveMetadataSourceIDs(game)
	if got[enums.Bangumi] != "20" || got[enums.VNDB] != "v10" || got[enums.Hikarinagi] != "99" || got[enums.Ymgal] != "" || len(got) != 3 {
		t.Fatalf("ids = %v", got)
	}
	if got := s.ResolveMetadataSourceIDs(models.Game{SourceType: enums.Local}); len(got) != 0 {
		t.Fatalf("local game ids = %v", got)
	}
}

// 默认的更新是否套用组合策略由 MetadataRefreshUseStrategy 单独控制（与刮削的开关无关）；套用时所有来源都为空的字段不更新，保留本地内容。
func TestComposeRefreshResultUsesRefreshSwitch(t *testing.T) {
	config := &appconf.AppConfig{MetadataSources: []string{"bangumi", "vndb"}}
	config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB}.WithFieldSource(enums.MetadataUpdateFieldName, enums.VNDB)) // 导入开关关闭
	s := &GameService{config: config}
	s.SetGameIDMapper(idmapper.New(nil)) // VNDB 没有 ID，不会请求网络
	game := models.Game{Name: "本地", SourceType: enums.Bangumi, SourceID: "20",
		MetadataSources: []models.GameMetadataSource{{SourceType: enums.Bangumi, SourceID: "20"}}}
	base := metadata.MetadataResult{Game: models.Game{Name: "BGM", SourceType: enums.Bangumi}}
	fields := gamehelper.NormalizeMetadataUpdateFields(nil)

	if s.refreshUsesStrategy() {
		t.Fatal("refresh switch off but refreshUsesStrategy is true")
	}

	config.MetadataRefreshUseStrategy = true
	if !s.refreshUsesStrategy() {
		t.Fatal("refresh switch on but refreshUsesStrategy is false")
	}
	got, _, gotFields := s.composeRefreshResult(context.Background(), game, game.SourceType, base, fields)
	if got.Game.Name != "BGM" || !gotFields.Has(enums.MetadataUpdateFieldName) || !gotFields.Has(enums.MetadataUpdateFieldAliases) {
		t.Fatalf("switch on: name %q, fields %v", got.Game.Name, gotFields)
	}
	if gotFields.Has(enums.MetadataUpdateFieldSummary) || gotFields.Has(enums.MetadataUpdateFieldTags) {
		t.Fatalf("empty fields should be kept local: %v", gotFields)
	}
}
