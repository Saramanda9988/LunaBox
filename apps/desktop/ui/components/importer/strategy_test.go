package importer

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	metadatapanel "lunabox/desktop/ui/components/panel/metadata"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// 后端按高级组合策略组合好结果时，匹配用组合的结果（来源是基底）。
func TestMatchUsesComposedResult(t *testing.T) {
	composed := meta(enums.VNDB, "组合名称", "v1")
	fake := &fakeImport{match: func(name string) (vo.BatchImportMetadataMatchResult, error) {
		return vo.BatchImportMetadataMatchResult{
			PreferredMatched: true,
			Matches:          []vo.GameMetadataFromWebVO{meta(enums.VNDB, "v-"+name, "v1"), meta(enums.Bangumi, "b-"+name, "9")},
			Composed:         &composed,
		}, nil
	}}
	f := flowWith(fake, "sakura")
	runMatch(t, f, matchSettings{preferred: enums.VNDB, all: []enums.SourceType{enums.Bangumi, enums.VNDB}})
	c := f.Candidates[0]
	if c.MatchStatus != MatchMatched || c.MatchSource != enums.VNDB || c.MatchedGame.Name != "组合名称" || len(c.AllMatches) != 2 {
		t.Fatalf("candidate = %+v", c)
	}
}

// 单个添加：刮削时使用组合策略时，搜索结果最前面是组合卡片，默认以必选数据源为基底；选它保存组合后的游戏，
// 默认来源是基底，各候选的 ID 都关联上。
func TestAddGameStrategyLeadCard(t *testing.T) {
	var a *AddGame
	h := newHarness(t)
	h.env.Svc.Config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true}.WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi))
	vndb := meta(enums.VNDB, "Original Title", "v17")
	vndb.Game.Summary = "from vndb"
	h.env.Svc.Game = &fakeMetadataGame{GameService: h.env.Svc.Game, results: []vo.GameMetadataFromWebVO{meta(enums.Bangumi, "组合后的名称", "20"), vndb}}
	added := 0
	a = NewAddGame(h.env)
	h.run(func(h *harness, c *ui.Context, th *theme.Theme) { a.View(c, th, func() { added++ }) })
	a.Open()
	h.frames(2)
	h.click(t, locales.AddGameModalRemoteGame())
	a.name = "Original"
	h.frames(1)
	h.click(t, locales.AddGameModalSearchMeta())
	h.expect(t, locales.NativeMetadataStrategyCardBase(), "VNDB")

	h.click(t, locales.NativeMetadataStrategyCardHint())
	h.frames(2)
	if added != 1 {
		t.Fatalf("game not added; texts %q", h.tt.Texts())
	}
	var name, source, sourceID, summary string
	if err := h.svc.DB.QueryRow(`SELECT name, source_type, source_id, summary FROM games`).Scan(&name, &source, &sourceID, &summary); err != nil {
		t.Fatal(err)
	}
	if name != "组合后的名称" || source != "vndb" || sourceID != "v17" || summary != "from vndb" {
		t.Fatalf("saved %q %q %q %q", name, source, sourceID, summary)
	}
	var linked int
	if err := h.svc.DB.QueryRow(`SELECT COUNT(*) FROM game_metadata_sources`).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked != 2 {
		t.Fatalf("linked sources = %d", linked)
	}
}

// 策略没有生效（没有改用其他来源）时不显示组合卡片。
func TestAddGameNoLeadCardWithoutOverrides(t *testing.T) {
	a := NewAddGame(newHarness(t).env)
	a.env.Svc.Config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true})
	a.results = []vo.GameMetadataFromWebVO{meta(enums.VNDB, "A", "v1"), meta(enums.Bangumi, "B", "2")}
	if a.leadResult(nil, nil) != nil {
		t.Fatal("lead card shown without overrides")
	}
}

// 批量导入用组合策略的基底作为必选数据源（基底没有启用时没有），不再读 batch_import_preferred_source。
func TestPreferredSourceIsStrategyBase(t *testing.T) {
	cfg := &appconf.AppConfig{BatchImportPreferredSource: "bangumi"}
	cfg.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB})
	if got := preferredSource(cfg, []enums.SourceType{enums.Bangumi, enums.VNDB}); got != enums.VNDB {
		t.Fatalf("preferred = %q", got)
	}
	if got := preferredSource(cfg, []enums.SourceType{enums.Bangumi}); got != "" {
		t.Fatalf("disabled base should not be preferred, got %q", got)
	}
}

// 组合卡片的基底：默认是 preferred 里第一个在候选里的来源，没有时用第一个候选的来源；卡片上选的优先。
// 以基底组合时没有字段取自其他候选、或者候选只有一个来源时没有组合卡片。
func TestStrategyLeadBase(t *testing.T) {
	cfg := &appconf.AppConfig{MetadataSources: []string{"bangumi", "vndb"}}
	cfg.SetStrategy(appconf.MetadataStrategy{Enabled: true}.WithFieldSource(enums.MetadataUpdateFieldSummary, enums.VNDB))
	bgm, vndb := meta(enums.Bangumi, "BGM", "2"), meta(enums.VNDB, "VN", "v1")
	bgm.Game.Summary, vndb.Game.Summary = "from bgm", "from vndb"
	results := []vo.GameMetadataFromWebVO{bgm, vndb}
	var base enums.SourceType
	var picked vo.GameMetadataFromWebVO
	pick := func(r vo.GameMetadataFromWebVO) { picked = r }

	lead := StrategyLead(cfg, results, &base, []enums.SourceType{enums.Steam}, pick) // Steam 不在候选里：用能作基底的 Bangumi
	if lead == nil || lead.Base != enums.Bangumi || lead.Result.Game.Name != "BGM" || lead.Result.Game.Summary != "from vndb" {
		t.Fatalf("lead = %+v", lead)
	}
	if lead.Fields != locales.NativeMetadataStrategySummaryField(locales.Key("metadataUpdateFields.summary.label"), "VNDB") {
		t.Fatalf("fields = %q", lead.Fields)
	}
	lead.OnSelect()
	if picked.Source != enums.Bangumi || picked.Game.Summary != "from vndb" {
		t.Fatalf("picked %+v", picked)
	}

	// VNDB 作基底时简介就取自基底，没有字段取自其他候选：不能作基底，卡片上不列，选了也回到默认。
	if !slices.Equal(lead.Bases, []enums.SourceType{enums.Bangumi}) {
		t.Fatalf("bases = %v", lead.Bases)
	}
	lead.OnBase(enums.VNDB)
	if lead := StrategyLead(cfg, results, &base, nil, pick); base != enums.VNDB || lead == nil || lead.Base != enums.Bangumi {
		t.Fatalf("a base without overrides should fall back: base %q, lead %+v", base, lead)
	}
	// 选的基底不在候选里时回到默认。
	base = enums.Steam
	if lead := StrategyLead(cfg, results, &base, []enums.SourceType{enums.Bangumi}, pick); lead == nil || lead.Base != enums.Bangumi {
		t.Fatalf("fallback lead = %+v", lead)
	}
	base = ""
	if StrategyLead(cfg, results[:1], &base, nil, pick) != nil {
		t.Fatal("a single source should not have a lead card")
	}
	off := cfg.Strategy()
	off.Enabled = false
	cfg.SetStrategy(off)
	if StrategyLead(cfg, results, &base, nil, pick) != nil {
		t.Fatal("switched off strategy should not have a lead card")
	}
}

// 默认基底是策略里手动指定字段最多的来源（必选数据源与候选的顺序都排在后面）；一样多时按 preferred 的顺序。
func TestStrategyLeadDefaultsToMostUsedSource(t *testing.T) {
	cfg := &appconf.AppConfig{MetadataSources: []string{"bangumi", "vndb", "touchgal"}}
	cfg.SetStrategy(appconf.MetadataStrategy{Enabled: true}.
		WithFieldSource(enums.MetadataUpdateFieldName, enums.VNDB).
		WithFieldSource(enums.MetadataUpdateFieldAliases, enums.VNDB).
		WithFieldSource(enums.MetadataUpdateFieldReleaseDate, enums.VNDB).
		WithFieldSource(enums.MetadataUpdateFieldCover, enums.Bangumi).
		WithFieldSource(enums.MetadataUpdateFieldRating, enums.TouchGal))
	results := []vo.GameMetadataFromWebVO{meta(enums.Bangumi, "BGM", "2"), meta(enums.TouchGal, "TG", "t"), meta(enums.VNDB, "VN", "v1")}
	for i := range results {
		results[i].Game.CoverURL, results[i].Game.Rating = "https://example.com/"+results[i].Game.SourceID, 7
	}
	var base enums.SourceType
	pick := func(vo.GameMetadataFromWebVO) {}

	lead := StrategyLead(cfg, results, &base, []enums.SourceType{enums.Bangumi}, pick) // 必选数据源 Bangumi 只指定了 1 个字段
	if lead == nil || lead.Base != enums.VNDB || lead.Result.Game.Name != "VN" || lead.Result.Game.CoverURL != "https://example.com/2" {
		t.Fatalf("lead = %+v", lead)
	}
	if !slices.Equal(lead.Bases, []enums.SourceType{enums.Bangumi, enums.TouchGal, enums.VNDB}) {
		t.Fatalf("bases = %v", lead.Bases)
	}

	// Bangumi 与 TouchGAL 各 1 个：按 preferred；preferred 里都没有时按候选的顺序。
	tie := appconf.MetadataStrategy{Enabled: true}.
		WithFieldSource(enums.MetadataUpdateFieldCover, enums.Bangumi).
		WithFieldSource(enums.MetadataUpdateFieldRating, enums.TouchGal)
	cfg.SetStrategy(tie)
	if lead := StrategyLead(cfg, results, &base, []enums.SourceType{enums.TouchGal}, pick); lead == nil || lead.Base != enums.TouchGal {
		t.Fatalf("tie with preferred: %+v", lead)
	}
	if lead := StrategyLead(cfg, results, &base, nil, pick); lead == nil || lead.Base != enums.Bangumi {
		t.Fatalf("tie in result order: %+v", lead)
	}
}

// 批量导入的预览里是“必选数据源”的下拉框（改它写进配置），旁边是组合策略的入口：打开了刮削的开关时标“已启用”，
// 点它打开组合策略弹窗并标出刮削的开关。
func TestBatchImportStrategyButton(t *testing.T) {
	var b *BatchImport
	h := newHarness(t)
	b = NewBatchImport(h.env)
	h.run(func(h *harness, c *ui.Context, th *theme.Theme) { b.View(c, th, nil) })
	h.svc.Config.GameLibraryPath = gameLibrary(t)
	h.svc.Config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true}.WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi))
	b.Open()
	h.frames(2)
	h.click(t, locales.BatchImportModalBtnScanGameLibrary())
	on := locales.NativeMetadataStrategyButtonOn()
	h.expect(t, locales.NativeMetadataStrategyBase(), "VNDB", on)

	// 关掉刮削的开关时按钮不标“已启用”。
	off := h.svc.Config.Strategy()
	off.Enabled = false
	h.svc.Config.SetStrategy(off)
	h.frames(2)
	if h.tt.HasText(on) {
		t.Fatal("switched off strategy should not show the enabled label")
	}
	h.svc.Config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true}.WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi))
	h.frames(2)

	h.click(t, locales.NativeMetadataStrategySettingsLabel())
	h.expect(t, locales.NativeMetadataStrategyTitle(), locales.NativeMetadataStrategySummaryScrape())
	if dir := os.Getenv("DESKTOP_MYGO_PNG"); dir != "" {
		b.strategy = metadatapanel.StrategyModal{}
		h.frames(2)
		f, err := os.Create(filepath.Join(dir, "batch-import-strategy-toolbar.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}
