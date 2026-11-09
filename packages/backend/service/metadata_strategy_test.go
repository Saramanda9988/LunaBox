package service

import (
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service/gamehelper/idmapper"
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
	config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB}.WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi))
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
