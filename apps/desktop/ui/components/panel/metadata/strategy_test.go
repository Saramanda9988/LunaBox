package metadata

import (
	"context"
	"strings"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop/api"
	"lunabox/desktop/ui/locales"
)

type fakeStrategyImport struct {
	api.ImportService
	base enums.SourceType
}

func (f *fakeStrategyImport) FetchMetadataForCandidateWithPreferenceContext(_ context.Context, name string, base enums.SourceType) (vo.BatchImportMetadataMatchResult, error) {
	f.base = base
	return vo.BatchImportMetadataMatchResult{PreferredMatched: true, Matches: []vo.GameMetadataFromWebVO{
		{Source: enums.VNDB, Game: models.Game{Name: name, SourceID: "v1", Summary: "VNDB summary", Company: "Studio"}},
		{Source: enums.Bangumi, Game: models.Game{Name: "试刮标题", SourceID: "20"}},
	}}, nil
}

// 设置页的入口显示当前策略的说明；弹窗里编辑草稿、试刮，保存后才写进配置。
func TestStrategyModal(t *testing.T) {
	h := newHarness(t)
	fake := &fakeStrategyImport{ImportService: h.env.Svc.Import}
	h.env.Svc.Import = fake
	h.cfg.SetStrategy(appconf.MetadataStrategy{})
	h.frames(2)
	h.expect(t, locales.NativeMetadataStrategySettingsLabel(), locales.NativeMetadataStrategySummaryNoBase())

	h.click(t, locales.NativeMetadataStrategySettingsButton())
	h.expect(t, locales.NativeMetadataStrategyBase(), locales.NativeMetadataStrategyBaseHint(), locales.NativeMetadataStrategyTrialNeedBase(),
		fieldLabel(enums.MetadataUpdateFieldCover), locales.NativeMetadataStrategyRefreshHint())

	h.state.strategy.draft = appconf.MetadataStrategy{Base: enums.VNDB}.
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
	if h.cfg.Strategy().Base != "" {
		t.Fatal("draft should not be saved before clicking save")
	}

	// 试刮：按基底搜索，用草稿组合；名称来自 Bangumi，简介（Hikarinagi 没有结果）回退到 VNDB。
	h.state.strategy.trialName = "Trial Game"
	h.frames(1)
	h.click(t, locales.NativeMetadataStrategyTrialButton())
	if fake.base != enums.VNDB {
		t.Fatalf("trial searched base %q", fake.base)
	}
	h.expect(t, "试刮标题", "VNDB summary", "Studio")

	h.click(t, locales.CommonSave())
	if h.state.strategy.open {
		t.Fatal("modal should close after saving")
	}
	got := h.cfg.Strategy()
	if got.Base != enums.VNDB || got.FieldSource(enums.MetadataUpdateFieldName) != enums.Bangumi || got.FieldSource(enums.MetadataUpdateFieldSummary) != enums.Hikarinagi {
		t.Fatalf("saved strategy = %+v", got)
	}
}
