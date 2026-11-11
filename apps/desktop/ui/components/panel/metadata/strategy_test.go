package metadata

import (
	"strings"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/locales"
)

// 设置页的入口按打开的开关分别显示“刮削时启用”“更新时启用”。弹窗里没有必选数据源，只有两个开关与各字段的来源；
// 文字说明按打开的开关说基底是谁。修改立即写进配置（没有保存按钮，必选数据源原样保留）；重置让字段跟随基底。
func TestStrategyModal(t *testing.T) {
	h := newHarness(t)
	h.cfg.SetStrategy(appconf.MetadataStrategy{Base: enums.VNDB})
	h.frames(2)
	h.expect(t, locales.NativeMetadataStrategySettingsLabel())
	h.expectNot(t, locales.NativeMetadataStrategyActiveScrape(), locales.NativeMetadataStrategyActiveRefresh())

	h.click(t, locales.NativeMetadataStrategySettingsButton())
	h.expect(t, locales.NativeMetadataStrategyEnable(), locales.NativeMetadataRefreshUseStrategy(), fieldLabel(enums.MetadataUpdateFieldCover),
		locales.NativeMetadataStrategyReset())
	h.expectNot(t, locales.CommonCancel(), locales.CommonSave(), locales.NativeMetadataStrategyBase(), locales.NativeMetadataStrategySummaryScrape())

	m := &h.state.strategy
	m.draft = m.draft.
		WithFieldSource(enums.MetadataUpdateFieldName, enums.Bangumi).
		WithFieldSource(enums.MetadataUpdateFieldSummary, enums.Hikarinagi) // 默认没有启用 Hikarinagi
	m.draft.Enabled = true
	m.commit()
	h.frames(2)
	name, summary := fieldLabel(enums.MetadataUpdateFieldName), fieldLabel(enums.MetadataUpdateFieldSummary)
	sep := locales.NativeMetadataStrategyClauseSeparator()
	h.expect(t, locales.NativeMetadataStrategySummaryScrape(), strings.Join([]string{
		locales.NativeMetadataStrategySummaryField(name, "Bangumi"),
		locales.NativeMetadataStrategySummaryField(summary, locales.NativeMetadataStrategySourceDisabled("Hikarinagi")),
		locales.NativeMetadataStrategySummaryRestBase(),
	}, sep))
	h.expectNot(t, locales.NativeMetadataStrategySummaryRefresh())
	if got := h.cfg.Strategy(); got.Base != enums.VNDB || !got.Enabled || got.FieldSource(enums.MetadataUpdateFieldName) != enums.Bangumi {
		t.Fatalf("saved strategy = %+v", got)
	}

	// 重置：字段跟随基底，开关不变，立即保存。
	h.click(t, locales.NativeMetadataStrategyReset())
	if got := h.cfg.Strategy(); !got.Enabled || len(got.Fields) != 0 || got.Base != enums.VNDB {
		t.Fatalf("saved after reset = %+v", got)
	}
	h.expect(t, locales.NativeMetadataStrategySummaryAllFromBase())

	m.refresh = true
	m.commit()
	h.frames(2)
	h.expect(t, locales.NativeMetadataStrategySummaryScrape(), locales.NativeMetadataStrategySummaryRefresh())
	if !h.cfg.MetadataRefreshUseStrategy {
		t.Fatal("refresh switch should be saved")
	}
	m.close()
	h.frames(2)
	h.expect(t, locales.NativeMetadataStrategyActiveScrape(), locales.NativeMetadataStrategyActiveRefresh())

	// 只打开更新时的开关：入口只标“更新时启用”。
	h.click(t, locales.NativeMetadataStrategySettingsButton())
	if !m.refresh || m.focus != FocusNone {
		t.Fatalf("draft should start from the config: refresh %v, focus %v", m.refresh, m.focus)
	}
	m.draft.Enabled = false
	m.commit()
	m.close()
	h.frames(2)
	h.expect(t, locales.NativeMetadataStrategyActiveRefresh())
	h.expectNot(t, locales.NativeMetadataStrategyActiveScrape())
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
	// 全量更新：只标指定了来源的字段；没有指定的用各游戏自己的默认来源（必选数据源 VNDB 不参与）。
	batch := BatchRefreshNotes(cfg)
	for f, want := range map[enums.MetadataUpdateField]better.FieldNote{
		enums.MetadataUpdateFieldCover:   {Text: from("Bangumi")},
		enums.MetadataUpdateFieldName:    {},
		enums.MetadataUpdateFieldAliases: {Text: locales.NativeMetadataStrategyMergeAll()},
		enums.MetadataUpdateFieldSummary: {Text: locales.NativeMetadataStrategyFieldFromDefault(), Warn: true, Reason: ymgal},
	} {
		if batch[f] != want {
			t.Errorf("batch %s = %+v, want %+v", f, batch[f], want)
		}
	}

	// 单个游戏：字段指定的来源 → 默认来源 Bangumi。名称指定了 VNDB（没有 ID）。
	cfg.SetStrategy(cfg.Strategy().WithFieldSource(enums.MetadataUpdateFieldName, enums.VNDB))
	game := GameRefreshNotes(cfg, enums.Bangumi, map[enums.SourceType]string{enums.Bangumi: "1"}) // VNDB 没有 ID
	for f, want := range map[enums.MetadataUpdateField]better.FieldNote{
		enums.MetadataUpdateFieldCover:   {Text: from("Bangumi")},
		enums.MetadataUpdateFieldName:    {Text: from("Bangumi"), Warn: true, Reason: noVNDB},
		enums.MetadataUpdateFieldSummary: {Text: from("Bangumi"), Warn: true, Reason: ymgal},
		enums.MetadataUpdateFieldCompany: {Text: from("Bangumi")},
	} {
		if game[f] != want {
			t.Errorf("game %s = %+v, want %+v", f, game[f], want)
		}
	}
	if got := GameRefreshNotes(cfg, enums.Bangumi, nil)[enums.MetadataUpdateFieldName]; got != (better.FieldNote{Text: from("VNDB")}) {
		t.Errorf("ids not loaded: name note %+v", got)
	}
}
