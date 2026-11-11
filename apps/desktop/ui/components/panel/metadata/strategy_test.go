package metadata

import (
	"strings"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/locales"
)

// 设置页的入口在策略生效时显示“已启用”；弹窗里编辑草稿：必选数据源不受开关影响，打开开关时必须指定，
// 重置让字段跟随基底，保存后才写进配置。
func TestStrategyModal(t *testing.T) {
	h := newHarness(t)
	h.cfg.SetStrategy(appconf.MetadataStrategy{})
	h.frames(2)
	h.expect(t, locales.NativeMetadataStrategySettingsLabel())
	h.expectNot(t, locales.NativeMetadataStrategyActive())

	h.click(t, locales.NativeMetadataStrategySettingsButton())
	h.expect(t, locales.NativeMetadataStrategyBase(), locales.NativeMetadataStrategyEnable(), locales.NativeMetadataRefreshUseStrategy(),
		fieldLabel(enums.MetadataUpdateFieldCover))
	h.expectNot(t, locales.CommonCancel(), locales.NativeMetadataStrategyBaseRequired())

	// 打开开关却没有必选数据源：红字提示，不能保存。
	h.state.strategy.draft.Enabled = true
	h.frames(2)
	h.expect(t, locales.NativeMetadataStrategyBaseRequired())
	h.click(t, locales.CommonSave())
	if !h.state.strategy.open {
		t.Fatal("save should be disabled without a required source")
	}

	// 关闭开关时可以只设必选数据源，保存后策略不生效。
	h.state.strategy.draft = appconf.MetadataStrategy{Base: enums.VNDB}
	h.click(t, locales.CommonSave())
	if got := h.cfg.Strategy(); got.Base != enums.VNDB || got.Enabled {
		t.Fatalf("saved strategy = %+v", got)
	}
	h.frames(2)
	h.expectNot(t, locales.NativeMetadataStrategyActive())
	h.click(t, locales.NativeMetadataStrategySettingsButton())

	h.state.strategy.draft = appconf.MetadataStrategy{Base: enums.VNDB, Enabled: true}.
		WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi).
		WithFieldSource(enums.MetadataUpdateFieldSummary, enums.Hikarinagi) // 默认没有启用 Hikarinagi
	h.frames(2)
	name, summary := fieldLabel(enums.MetadataUpdateFieldName), fieldLabel(enums.MetadataUpdateFieldSummary)
	sep := locales.NativeMetadataStrategyClauseSeparator()
	h.expect(t, locales.NativeMetadataStrategySummaryBase("VNDB"), strings.Join([]string{
		locales.NativeMetadataStrategySummaryField(name, "Bangumi"),
		locales.NativeMetadataStrategySummaryField(summary, locales.NativeMetadataStrategySourceDisabled("Hikarinagi")),
		locales.NativeMetadataStrategySummaryRest("VNDB"),
	}, sep))
	if h.cfg.Strategy().Enabled {
		t.Fatal("draft should not be saved before clicking save")
	}

	// 重置：字段跟随基底，基底不变；再改回来以便保存。
	h.click(t, locales.NativeMetadataStrategyReset())
	if d := h.state.strategy.draft; d.Base != enums.VNDB || !d.Enabled || len(d.Fields) != 0 {
		t.Fatalf("draft after reset = %+v", d)
	}
	h.state.strategy.draft = h.state.strategy.draft.
		WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi).
		WithFieldSource(enums.MetadataUpdateFieldSummary, enums.Hikarinagi)
	h.frames(2)

	h.click(t, locales.CommonSave())
	if h.state.strategy.open {
		t.Fatal("modal should close after saving")
	}
	got := h.cfg.Strategy()
	if got.Base != enums.VNDB || got.FieldSource(enums.MetadataUpdateFieldName) != enums.Bangumi || got.FieldSource(enums.MetadataUpdateFieldSummary) != enums.Hikarinagi {
		t.Fatalf("saved strategy = %+v", got)
	}
	h.expect(t, locales.NativeMetadataStrategyActive())
	if h.cfg.MetadataRefreshUseStrategy {
		t.Fatal("refresh switch should stay off")
	}

	// 更新元数据时使用的开关与刮削的开关分开保存；只打开它时入口也显示“已启用”。
	h.click(t, locales.NativeMetadataStrategySettingsButton())
	if h.state.strategy.refresh {
		t.Fatal("refresh draft should start from the config")
	}
	h.state.strategy.draft.Enabled, h.state.strategy.refresh = false, true
	h.frames(2)
	h.click(t, locales.CommonSave())
	if !h.cfg.MetadataRefreshUseStrategy || h.cfg.Strategy().Enabled {
		t.Fatalf("saved: refresh %v, strategy %+v", h.cfg.MetadataRefreshUseStrategy, h.cfg.Strategy())
	}
	h.expect(t, locales.NativeMetadataStrategyActive())
}

// 更新字段弹窗的标签：没有打开“使用高级组合策略”时全量更新不标、单个游戏都是默认来源；打开后按
// 字段指定的来源 → 必选数据源 → 默认来源 找第一个可用的，回退时琥珀色并给出原因。
func TestRefreshNotes(t *testing.T) {
	cfg := &appconf.AppConfig{MetadataSources: []string{"bangumi", "vndb"}}
	cfg.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB}.
		WithFieldSource(enums.MetadataUpdateFieldCover, enums.Bangumi).
		WithFieldSource(enums.MetadataUpdateFieldAliases, appconf.MetadataStrategyAllSources).
		WithFieldSource(enums.MetadataUpdateFieldSummary, enums.Ymgal)) // 没有启用
	from := locales.NativeMetadataStrategyFieldFrom
	ymgal := locales.NativeMetadataStrategyReasonDisabled(StrategyLabel(enums.Ymgal))
	noVNDB := locales.NativeMetadataStrategyNoSourceID("VNDB")
	sep := locales.NativeMetadataStrategyClauseSeparator()

	if notes := BatchRefreshNotes(cfg); notes != nil || RefreshUsesStrategy(cfg, enums.Bangumi) {
		t.Fatalf("switch off: notes %v", notes)
	}
	if got := GameRefreshNotes(cfg, enums.Bangumi, nil)[enums.MetadataUpdateFieldName]; got != (better.FieldNote{}) {
		t.Fatalf("switch off: name note %+v", got)
	}
	if got := GameRefreshNotes(cfg, enums.Bangumi, map[enums.SourceType]string{})[enums.MetadataUpdateFieldName]; !got.Warn || got.Text != locales.NativeMetadataStrategyNoMetadataID() {
		t.Fatalf("switch off, no ID: name note %+v", got)
	}

	cfg.MetadataRefreshUseStrategy = true
	if !RefreshUsesStrategy(cfg, enums.Bangumi) {
		t.Fatal("strategy should apply")
	}
	batch := BatchRefreshNotes(cfg)
	for f, want := range map[enums.MetadataUpdateField]better.FieldNote{
		enums.MetadataUpdateFieldCover:   {Text: from("Bangumi")},
		enums.MetadataUpdateFieldName:    {Text: from("VNDB")},
		enums.MetadataUpdateFieldAliases: {Text: locales.NativeMetadataStrategyMergeAll()},
		enums.MetadataUpdateFieldSummary: {Text: from("VNDB"), Warn: true, Reason: ymgal},
	} {
		if batch[f] != want {
			t.Errorf("batch %s = %+v, want %+v", f, batch[f], want)
		}
	}

	game := GameRefreshNotes(cfg, enums.Bangumi, map[enums.SourceType]string{enums.Bangumi: "1"}) // VNDB 没有 ID
	for f, want := range map[enums.MetadataUpdateField]better.FieldNote{
		enums.MetadataUpdateFieldCover:   {Text: from("Bangumi")},
		enums.MetadataUpdateFieldName:    {Text: from("Bangumi"), Warn: true, Reason: noVNDB},
		enums.MetadataUpdateFieldSummary: {Text: from("Bangumi"), Warn: true, Reason: ymgal + sep + noVNDB},
	} {
		if game[f] != want {
			t.Errorf("game %s = %+v, want %+v", f, game[f], want)
		}
	}
	if got := GameRefreshNotes(cfg, enums.Bangumi, nil)[enums.MetadataUpdateFieldName]; got != (better.FieldNote{Text: from("VNDB")}) {
		t.Errorf("ids not loaded: name note %+v", got)
	}
}
