package importer

import (
	"image/png"
	"os"
	"path/filepath"
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

// 单个添加：策略生效时，搜索结果最前面是组合卡片；选它保存组合后的游戏，默认来源是基底，各候选的 ID 都关联上。
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
	h.expect(t, locales.NativeMetadataStrategyCardSource("VNDB"))

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

// 批量导入的预览里，原来的必选数据源换成了组合策略的入口：显示基底来源，策略生效时加上“组合”，点它打开组合策略弹窗。
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
	mixed := locales.NativeMetadataStrategyButtonMixed("VNDB")
	h.expect(t, locales.NativeMetadataStrategyBase(), mixed)

	// 关掉组合策略时只显示必选数据源。
	off := h.svc.Config.Strategy()
	off.Enabled = false
	h.svc.Config.SetStrategy(off)
	h.frames(2)
	h.expect(t, "VNDB")
	if h.tt.HasText(mixed) {
		t.Fatal("switched off strategy should not show the mixed label")
	}
	h.svc.Config.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true}.WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi))
	h.frames(2)

	h.click(t, mixed)
	h.expect(t, locales.NativeMetadataStrategyTitle(), locales.NativeMetadataStrategySummaryBase("VNDB"))
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
