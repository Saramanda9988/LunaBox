package gamehelper

import (
	"slices"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/utils/metadata"
)

func strategyOf(base enums.SourceType, fields map[enums.MetadataUpdateField]enums.SourceType) appconf.MetadataStrategy {
	s := appconf.MetadataStrategy{Base: base}
	for f, src := range fields {
		s = s.WithFieldSource(f, src)
	}
	return s
}

func TestComposeMetadataPicksFieldSourcesAndFallsBackToBase(t *testing.T) {
	base := vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: models.Game{
		Name: "Base Title", Aliases: []string{"BT"}, CoverURL: "vndb.jpg", Summary: "base summary",
		Company: "Base Co", Rating: 8, ReleaseDate: "2020-01-01", SourceType: enums.VNDB, SourceID: "v1", IsNSFW: true,
	}, Tags: []metadata.TagItem{{Name: "vn-tag", Source: "vndb"}}}
	bgm := vo.GameMetadataFromWebVO{Source: enums.Bangumi, Game: models.Game{
		Name: "中文名", Aliases: []string{"别名"}, CoverURL: "bgm.jpg", Summary: "", SourceType: enums.Bangumi, SourceID: "10",
	}, Tags: []metadata.TagItem{{Name: "bgm-tag"}}}
	strategy := strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{
		enums.MetadataUpdateFieldName:    enums.Bangumi,
		enums.MetadataUpdateFieldCover:   enums.Bangumi,
		enums.MetadataUpdateFieldSummary: enums.Bangumi,    // bgm 的简介为空：回退基底
		enums.MetadataUpdateFieldCompany: enums.Hikarinagi, // 没有匹配到：回退基底
		enums.MetadataUpdateFieldTags:    enums.Bangumi,
	})

	got, sources := ComposeMetadata(strategy, base, []vo.GameMetadataFromWebVO{bgm})

	if got.Source != enums.VNDB || got.Game.SourceID != "v1" || !got.Game.IsNSFW {
		t.Fatalf("identity should follow base, got %+v", got.Game)
	}
	if got.Game.Name != "中文名" || got.Game.CoverURL != "bgm.jpg" {
		t.Fatalf("name/cover = %q/%q", got.Game.Name, got.Game.CoverURL)
	}
	if got.Game.Summary != "base summary" || sources[enums.MetadataUpdateFieldSummary] != enums.VNDB {
		t.Fatalf("summary should fall back to base, got %q from %s", got.Game.Summary, sources[enums.MetadataUpdateFieldSummary])
	}
	if got.Game.Company != "Base Co" || sources[enums.MetadataUpdateFieldCompany] != enums.VNDB {
		t.Fatalf("company should fall back to base, got %q", got.Game.Company)
	}
	if len(got.Tags) != 1 || got.Tags[0].Name != "bgm-tag" || got.Tags[0].Source != "bangumi" {
		t.Fatalf("tags = %+v", got.Tags)
	}
	// 别名沿用基底，名称换了之后基底的名称留作别名。
	if !slices.Equal(got.Game.Aliases, []string{"BT", "Base Title"}) {
		t.Fatalf("aliases = %v", got.Game.Aliases)
	}
	if sources[enums.MetadataUpdateFieldName] != enums.Bangumi || sources[enums.MetadataUpdateFieldRating] != enums.VNDB {
		t.Fatalf("sources = %v", sources)
	}
	if base.Game.Name != "Base Title" || base.Tags[0].Name != "vn-tag" {
		t.Fatal("base was modified")
	}
}

func TestComposeMetadataMergesAllAliases(t *testing.T) {
	base := vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: models.Game{Name: "Title", Aliases: []string{"T"}}}
	others := []vo.GameMetadataFromWebVO{
		{Source: enums.Bangumi, Game: models.Game{Name: "标题", Aliases: []string{"t"}}},
		{Source: enums.Ymgal, Game: models.Game{Name: "タイトル"}},
	}
	strategy := strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{
		enums.MetadataUpdateFieldAliases: appconf.MetadataStrategyAllSources,
	})

	got, sources := ComposeMetadata(strategy, base, others)

	if !slices.Equal(got.Game.Aliases, []string{"T", "标题", "タイトル"}) {
		t.Fatalf("aliases = %v", got.Game.Aliases)
	}
	if sources[enums.MetadataUpdateFieldAliases] != appconf.MetadataStrategyAllSources {
		t.Fatalf("aliases source = %s", sources[enums.MetadataUpdateFieldAliases])
	}
}

func TestStrategyActive(t *testing.T) {
	enabled := []enums.SourceType{enums.VNDB, enums.Bangumi}
	if StrategyActive(appconf.MetadataStrategy{Base: enums.VNDB}, enabled) {
		t.Fatal("no overrides should be inactive")
	}
	if StrategyActive(strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{enums.MetadataUpdateFieldName: enums.VNDB}), enabled) {
		t.Fatal("override equal to base should be inactive")
	}
	s := strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{enums.MetadataUpdateFieldName: enums.Bangumi})
	if !StrategyActive(s, enabled) {
		t.Fatal("expected active")
	}
	if StrategyActive(s, []enums.SourceType{enums.Bangumi}) {
		t.Fatal("disabled base should be inactive")
	}
	if StrategyActive(s, []enums.SourceType{enums.VNDB}) {
		t.Fatal("override with a disabled source should be inactive")
	}
	aliases := strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{enums.MetadataUpdateFieldAliases: appconf.MetadataStrategyAllSources})
	if !StrategyActive(aliases, []enums.SourceType{enums.VNDB}) {
		t.Fatal("merging aliases should be active")
	}
	if got := StrategySources(s, enums.VNDB); !slices.Equal(got, []enums.SourceType{enums.Bangumi}) {
		t.Fatalf("StrategySources = %v", got)
	}
}
