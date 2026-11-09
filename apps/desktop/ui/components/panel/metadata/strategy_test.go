package metadata

import (
	"strings"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
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
	h.expect(t, locales.NativeMetadataStrategyBase(), locales.NativeMetadataStrategyEnable(), fieldLabel(enums.MetadataUpdateFieldCover))
	h.expectNot(t, locales.CommonCancel(), locales.NativeMetadataStrategyRefreshHint(), locales.NativeMetadataStrategyBaseRequired())

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
}
