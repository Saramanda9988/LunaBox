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
	s := appconf.MetadataStrategy{Base: base, Enabled: true}
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
	if StrategyActive(appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true}, enabled) {
		t.Fatal("no overrides should be inactive")
	}
	if StrategyActive(strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{enums.MetadataUpdateFieldName: enums.VNDB}), enabled) {
		t.Fatal("override equal to base should be inactive")
	}
	s := strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{enums.MetadataUpdateFieldName: enums.Bangumi})
	if !StrategyActive(s, enabled) {
		t.Fatal("expected active")
	}
	off := s.Clone()
	off.Enabled = false
	if StrategyActive(off, enabled) {
		t.Fatal("switched off strategy should be inactive")
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

	// 以用户选的来源为基底：与必选数据源无关，指定的来源就是基底时不算。
	noBase := s.Clone()
	noBase.Base = ""
	if !StrategyActiveFor(noBase, enums.VNDB, enabled) || StrategyActiveFor(noBase, enums.Bangumi, enabled) {
		t.Fatal("StrategyActiveFor should depend on the chosen base only")
	}
	if StrategyActiveFor(off, enums.VNDB, enabled) {
		t.Fatal("switched off strategy should be inactive for any base")
	}
}

func TestRefreshChain(t *testing.T) {
	s := strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{
		enums.MetadataUpdateFieldName:    enums.Bangumi,
		enums.MetadataUpdateFieldAliases: appconf.MetadataStrategyAllSources,
	})
	if got := RefreshChain(s, enums.MetadataUpdateFieldName, enums.TouchGal); !slices.Equal(got, []enums.SourceType{enums.Bangumi, enums.TouchGal}) {
		t.Fatalf("name chain = %v", got)
	}
	if got := RefreshChain(s, enums.MetadataUpdateFieldName, enums.Bangumi); !slices.Equal(got, []enums.SourceType{enums.Bangumi}) {
		t.Fatalf("name chain on its own source = %v", got)
	}
	if got := RefreshChain(s, enums.MetadataUpdateFieldSummary, enums.TouchGal); !slices.Equal(got, []enums.SourceType{enums.TouchGal}) {
		t.Fatalf("summary chain = %v", got)
	}
	if got := RefreshChain(s, enums.MetadataUpdateFieldAliases, enums.TouchGal); !slices.Equal(got, []enums.SourceType{enums.TouchGal}) {
		t.Fatalf("aliases chain = %v", got)
	}
	if got := RefreshChain(appconf.MetadataStrategy{}, enums.MetadataUpdateFieldName, enums.TouchGal); !slices.Equal(got, []enums.SourceType{enums.TouchGal}) {
		t.Fatalf("empty strategy chain = %v", got)
	}
}

// 更新时每个字段按 字段指定的来源 → 游戏默认来源 取第一个有数据的（必选数据源不参与）；都为空的字段列为 missing。
func TestComposeRefreshFallsBackAlongChain(t *testing.T) {
	s := strategyOf(enums.VNDB, map[enums.MetadataUpdateField]enums.SourceType{
		enums.MetadataUpdateFieldName:        enums.VNDB,    // 有 VNDB 的结果
		enums.MetadataUpdateFieldTags:        enums.VNDB,    // 有 VNDB 的结果
		enums.MetadataUpdateFieldReleaseDate: enums.Bangumi, // 没有 Bangumi 的结果：回退到默认来源
		enums.MetadataUpdateFieldCompany:     enums.Bangumi, // 哪里都没有
	})
	base := vo.GameMetadataFromWebVO{Source: enums.TouchGal, Game: models.Game{Name: "TG", ReleaseDate: "2020-01-01", Summary: "tg"}}
	vndb := vo.GameMetadataFromWebVO{Source: enums.VNDB, Game: models.Game{Name: "VN", Summary: "vn"},
		Tags: []metadata.TagItem{{Name: "tag"}}}
	got, sources, missing := ComposeRefresh(s, base, []vo.GameMetadataFromWebVO{vndb})
	if got.Game.Name != "VN" || sources[enums.MetadataUpdateFieldName] != enums.VNDB {
		t.Fatalf("name = %q from %q", got.Game.Name, sources[enums.MetadataUpdateFieldName])
	}
	if got.Game.ReleaseDate != "2020-01-01" || sources[enums.MetadataUpdateFieldReleaseDate] != enums.TouchGal {
		t.Fatalf("release date = %q from %q", got.Game.ReleaseDate, sources[enums.MetadataUpdateFieldReleaseDate])
	}
	if got.Game.Summary != "tg" || sources[enums.MetadataUpdateFieldSummary] != enums.TouchGal { // 没有指定：默认来源，不看必选数据源
		t.Fatalf("summary = %q from %q", got.Game.Summary, sources[enums.MetadataUpdateFieldSummary])
	}
	if len(got.Tags) != 1 || got.Tags[0].Source != string(enums.VNDB) {
		t.Fatalf("tags = %+v", got.Tags)
	}
	if got.Source != enums.TouchGal || !slices.Contains(got.Game.Aliases, "TG") {
		t.Fatalf("source %q aliases %v", got.Source, got.Game.Aliases)
	}
	want := []enums.MetadataUpdateField{enums.MetadataUpdateFieldCover, enums.MetadataUpdateFieldCompany, enums.MetadataUpdateFieldRating}
	if !slices.Equal(missing, want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
}
