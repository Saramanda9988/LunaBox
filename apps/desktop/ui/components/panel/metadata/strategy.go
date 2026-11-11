package metadata

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/service/gamehelper"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/metasource"
	"lunabox/desktop/ui/theme"
)

// strategyFields 是弹窗里可以单独指定来源的字段，从上到下。
var strategyFields = []enums.MetadataUpdateField{
	enums.MetadataUpdateFieldCover,
	enums.MetadataUpdateFieldName,
	enums.MetadataUpdateFieldAliases,
	enums.MetadataUpdateFieldCompany,
	enums.MetadataUpdateFieldReleaseDate,
	enums.MetadataUpdateFieldRating,
	enums.MetadataUpdateFieldTags,
	enums.MetadataUpdateFieldSummary,
}

// StrategyModal 是“高级组合策略”弹窗：设置页的元数据分区、批量导入、详情页的更新字段与重新搜索共用。
// 弹窗里改的是草稿（组合策略与“更新元数据时使用”的开关），保存时才写进配置。
type StrategyModal struct {
	open    bool
	draft   appconf.MetadataStrategy
	refresh bool // 更新元数据时使用组合策略（AppConfig.MetadataRefreshUseStrategy）
}

// StrategyModalProps 是弹窗的属性。
type StrategyModalProps struct {
	Enabled   []enums.SourceType // 已启用的元数据来源（可选的来源）
	TopOffset float32
	// OnSave 在保存时调用：apply 把草稿写进配置，调用方用它修改配置（如 PatchConfig(apply)）。
	OnSave func(apply func(cfg *appconf.AppConfig))
}

// Open 用配置里的组合策略与两个开关打开弹窗。
func (m *StrategyModal) Open(cfg *appconf.AppConfig) {
	*m = StrategyModal{open: true, draft: cfg.Strategy(), refresh: cfg != nil && cfg.MetadataRefreshUseStrategy}
}

// IsOpen 报告弹窗是否打开。
func (m *StrategyModal) IsOpen() bool { return m.open }

func (m *StrategyModal) close() { m.open = false }

// StrategyLabel 是来源在组合策略里的名称；“合并所有来源”与未知来源也有名称。
func StrategyLabel(s enums.SourceType) string {
	if s == appconf.MetadataStrategyAllSources {
		return locales.NativeMetadataStrategyMergeAll()
	}
	return metasource.Label(s)
}

// fieldLabel 是字段的名称（metadataUpdateFields.*.label）。
func fieldLabel(f enums.MetadataUpdateField) string {
	return locales.Key("metadataUpdateFields." + string(f) + ".label")
}

// StrategySummary 是组合策略的文字说明：第一句说基底，第二句说各字段的来源；策略不生效时为空。
// 改用了未启用的来源时标出来（这些字段实际会用基底的数据）。
func StrategySummary(s appconf.MetadataStrategy, enabled []enums.SourceType) []string {
	if !s.Active() {
		return nil
	}
	base := sourceOptionLabel(s.Base, enabled)
	first := locales.NativeMetadataStrategySummaryBase(base)
	var order []enums.SourceType
	groups := map[enums.SourceType][]string{}
	mergeAliases := false
	for _, f := range strategyFields {
		src := s.FieldSource(f)
		switch {
		case src == "" || src == s.Base:
			continue
		case src == appconf.MetadataStrategyAllSources:
			mergeAliases = true
			continue
		}
		if _, ok := groups[src]; !ok {
			order = append(order, src)
		}
		groups[src] = append(groups[src], fieldLabel(f))
	}
	var clauses []string
	for _, src := range order {
		clauses = append(clauses, locales.NativeMetadataStrategySummaryField(
			strings.Join(groups[src], locales.NativeMetadataStrategyListSeparator()), sourceOptionLabel(src, enabled)))
	}
	if mergeAliases {
		clauses = append(clauses, locales.NativeMetadataStrategySummaryAliasesAll())
	}
	if len(clauses) == 0 {
		return []string{first, locales.NativeMetadataStrategySummaryAllBase(base)}
	}
	clauses = append(clauses, locales.NativeMetadataStrategySummaryRest(base))
	return []string{first, strings.Join(clauses, locales.NativeMetadataStrategyClauseSeparator())}
}

// strategyFieldNote 是“将从 X 更新”；别名合并所有来源时是“合并所有来源”。
func strategyFieldNote(src enums.SourceType) better.FieldNote {
	if src == appconf.MetadataStrategyAllSources {
		return better.FieldNote{Text: StrategyLabel(src)}
	}
	return better.FieldNote{Text: locales.NativeMetadataStrategyFieldFrom(StrategyLabel(src))}
}

// refreshStrategy 是更新元数据时用的组合策略：设置里没有打开“更新时使用高级组合策略”时为空（只用游戏的默认来源）。
func refreshStrategy(cfg *appconf.AppConfig) (appconf.MetadataStrategy, bool) {
	if cfg == nil || !cfg.MetadataRefreshUseStrategy {
		return appconf.MetadataStrategy{}, false
	}
	return cfg.Strategy(), true
}

// chainNote 沿着来源链找第一个可用的来源（skip 返回不可用的原因）：第一个就可用时是天蓝色的“将从 X 更新”，
// 回退到后面的来源时是琥珀色并带上原因；都不可用时 ok 为 false，原因照样返回。
func chainNote(chain []enums.SourceType, skip func(enums.SourceType) string) (note better.FieldNote, ok bool) {
	var reasons []string
	for _, src := range chain {
		if r := skip(src); r != "" {
			reasons = append(reasons, r)
			continue
		}
		note = strategyFieldNote(src)
		if len(reasons) > 0 {
			note.Warn, note.Reason = true, strings.Join(reasons, locales.NativeMetadataStrategyClauseSeparator())
		}
		return note, true
	}
	return better.FieldNote{Reason: strings.Join(reasons, locales.NativeMetadataStrategyClauseSeparator())}, false
}

// GameRefreshNotes 是更新单个游戏时每个字段将从哪个来源更新（给 FieldNotes）。base 是游戏的默认来源；
// 打开了“更新时使用高级组合策略”时按 字段指定的来源 → 必选数据源 → base 找第一个可用的来源；否则都是 base，
// 不标来源，只在 base 不可用时标“暂无元数据 ID”。
// ids 是游戏在各来源可用的元数据 ID（GameService.ResolveMetadataSourceIDs，含 ID 对照表查到的），没有 ID 或者
// 没有启用的来源跳过；ids 为 nil 表示还没查到，先不按 ID 判断。字段为空时的回退要拿到数据才知道，这里标不出来。
// 游戏没有默认来源（本地游戏）或者都不可用时标“暂无元数据 ID”。
func GameRefreshNotes(cfg *appconf.AppConfig, base enums.SourceType, ids map[enums.SourceType]string) map[enums.MetadataUpdateField]better.FieldNote {
	notes := make(map[enums.MetadataUpdateField]better.FieldNote, len(enums.DefaultMetadataUpdateFields))
	if base == "" || base == enums.Local {
		for _, f := range enums.DefaultMetadataUpdateFields {
			notes[f] = better.FieldNote{Text: locales.NativeMetadataStrategyNoMetadataID(), Warn: true}
		}
		return notes
	}
	enabled := EnabledSources(cfg)
	skip := func(src enums.SourceType) string {
		if !slices.Contains(enabled, src) {
			return locales.NativeMetadataStrategyReasonDisabled(StrategyLabel(src))
		}
		if ids != nil && strings.TrimSpace(ids[src]) == "" {
			return locales.NativeMetadataStrategyNoSourceID(StrategyLabel(src))
		}
		return ""
	}
	strategy, on := refreshStrategy(cfg)
	for _, f := range enums.DefaultMetadataUpdateFields {
		if on && strategy.FieldSource(f) == appconf.MetadataStrategyAllSources {
			notes[f] = strategyFieldNote(appconf.MetadataStrategyAllSources)
			continue
		}
		note, ok := chainNote(gamehelper.RefreshChain(strategy, f, base), skip)
		if !ok {
			note.Text, note.Warn = locales.NativeMetadataStrategyNoMetadataID(), true
		} else if !on {
			continue // 没有打开组合策略时都从默认来源更新，不标来源。
		}
		notes[f] = note
	}
	return notes
}

// BatchRefreshNotes 是全量更新时各字段将从哪个来源更新（给 FieldNotes）：打开了“更新时使用高级组合策略”时按
// 字段指定的来源 → 必选数据源 找第一个启用的来源；都没有时用各游戏自己的默认来源，不标。没有打开时为空。
// 各游戏有没有那个来源的 ID 要逐个看，这里不标。
func BatchRefreshNotes(cfg *appconf.AppConfig) map[enums.MetadataUpdateField]better.FieldNote {
	strategy, on := refreshStrategy(cfg)
	if !on {
		return nil
	}
	enabled := EnabledSources(cfg)
	skip := func(src enums.SourceType) string {
		if !slices.Contains(enabled, src) {
			return locales.NativeMetadataStrategyReasonDisabled(StrategyLabel(src))
		}
		return ""
	}
	notes := map[enums.MetadataUpdateField]better.FieldNote{}
	for _, f := range enums.DefaultMetadataUpdateFields {
		if strategy.FieldSource(f) == appconf.MetadataStrategyAllSources {
			notes[f] = strategyFieldNote(appconf.MetadataStrategyAllSources)
		} else if note, ok := chainNote(gamehelper.RefreshChain(strategy, f, ""), skip); ok {
			notes[f] = note
		}
	}
	return notes
}

// RefreshUsesStrategy 报告更新以 base 为默认来源的游戏时组合策略有没有影响：打开了“更新时使用高级组合策略”，
// 并且有字段会先去取 base 以外的来源。
func RefreshUsesStrategy(cfg *appconf.AppConfig, base enums.SourceType) bool {
	strategy, on := refreshStrategy(cfg)
	if !on {
		return false
	}
	for _, f := range enums.DefaultMetadataUpdateFields {
		if strategy.FieldSource(f) == appconf.MetadataStrategyAllSources {
			return true
		}
		if chain := gamehelper.RefreshChain(strategy, f, base); len(chain) > 0 && chain[0] != base {
			return true
		}
	}
	return false
}

// sourceOptionLabel 是来源的名称，没有启用时加上“（未启用）”。
func sourceOptionLabel(s enums.SourceType, enabled []enums.SourceType) string {
	if s != appconf.MetadataStrategyAllSources && !slices.Contains(enabled, s) {
		return locales.NativeMetadataStrategySourceDisabled(StrategyLabel(s))
	}
	return StrategyLabel(s)
}

// baseOptions 是必选数据源的选项：不指定与已启用的来源（当前值没有启用时也列出）。
func (m *StrategyModal) baseOptions(enabled []enums.SourceType) []better.SelectOption {
	opts := []better.SelectOption{{Label: locales.BatchImportModalPreferredSourceNone(), Value: ""}}
	for _, s := range enabled {
		opts = append(opts, better.SelectOption{Label: StrategyLabel(s), Value: string(s)})
	}
	if m.draft.Base != "" && !slices.Contains(enabled, m.draft.Base) {
		opts = append(opts, better.SelectOption{Label: sourceOptionLabel(m.draft.Base, enabled), Value: string(m.draft.Base)})
	}
	return opts
}

// missingBase 报告打开了组合策略却没有指定必选数据源（不能保存）。
func (m *StrategyModal) missingBase() bool { return m.draft.Enabled && m.draft.Base == "" }

// fieldOptions 是字段来源的选项：跟随基底、（别名）合并所有来源、基底以外已启用的来源（当前值没有启用时也列出）。
func (m *StrategyModal) fieldOptions(f enums.MetadataUpdateField, enabled []enums.SourceType) []better.SelectOption {
	follow := locales.NativeMetadataStrategyFollowBase()
	if m.draft.Base != "" {
		follow = locales.NativeMetadataStrategyFollowBaseWith(StrategyLabel(m.draft.Base))
	}
	opts := []better.SelectOption{{Label: follow, Value: ""}}
	if f == enums.MetadataUpdateFieldAliases {
		opts = append(opts, better.SelectOption{Label: locales.NativeMetadataStrategyMergeAll(), Value: string(appconf.MetadataStrategyAllSources)})
	}
	for _, s := range enabled {
		if s != m.draft.Base {
			opts = append(opts, better.SelectOption{Label: StrategyLabel(s), Value: string(s)})
		}
	}
	cur := m.draft.FieldSource(f)
	if cur != "" && cur != m.draft.Base && !slices.ContainsFunc(opts, func(o better.SelectOption) bool { return o.Value == string(cur) }) {
		opts = append(opts, better.SelectOption{Label: sourceOptionLabel(cur, enabled), Value: string(cur)})
	}
	return opts
}

// overridden 判断字段是否改用了基底以外的来源。
func (m *StrategyModal) overridden(f enums.MetadataUpdateField) bool {
	src := m.draft.FieldSource(f)
	return src != "" && src != m.draft.Base
}

// View 构建弹窗：标题、必选数据源、刮削与更新元数据两个开关、各字段的来源、文字说明，底部重置与保存。
// 必选数据源与各字段的来源都不受开关影响；刮削的开关关闭时没有文字说明（说明讲的是刮削时怎么识别与组合）。
func (m *StrategyModal) View(c *ui.Context, t *theme.Theme, p StrategyModalProps) {
	components.Modal(c, t, "metadata-strategy", components.ModalProps{
		Open: m.open, TopOffset: p.TopOffset, MaxWidth: 760, OnClose: m.close, Label: locales.NativeMetadataStrategyTitle(),
	}, func(pnl ui.Element) {
		_, winH := c.Size()
		// 窗口减去弹窗的上下边距、内边距、标题与底部按钮，至少留 240；内容超出时在中间滚动。
		bodyH := max(240, winH-p.TopOffset-260)
		components.ModalPanel(t, pnl).Children(func() {
			ui.Column(c.Key("strategy")).Gap(20).Children(func() {
				m.header(c, t)
				// 滚动区伸进面板右侧的内边距，滚动条落在那里。
				ui.Scroll(c.Key("body")).MaxHeight(bodyH).Margin(0, -14, 0, 0).Children(func() {
					ui.Column(c).Padding(2, 14, 2, 2).Gap(20).Children(func() {
						m.baseRow(c, t, p.Enabled)
						m.switchRows(c, t)
						m.fieldRows(c, t, p.Enabled)
						if m.draft.Active() {
							m.summary(c, t, p.Enabled)
						}
					})
				})
				m.footer(c, t, p)
			})
		})
	})
}

func (m *StrategyModal) header(c *ui.Context, t *theme.Theme) {
	ui.Row(c.Key("header")).Gap(16).AlignItems(ui.Start).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Icon(c, "layers-triple-outline", 22, t.Pick(theme.Sky600, theme.Sky400))
				components.Text(c, locales.NativeMetadataStrategyTitle(), theme.TextXL, theme.WeightBold, t.TextStrong)
			})
		})
		btn := ui.ButtonBase(c.Key("close")).Padding(8).Radius(8).Center().Shrink(0).FocusRing(false).Label(locales.CommonClose())
		color := t.Pick(theme.Brand500, theme.Brand400)
		if btn.Hovered() {
			btn.Background(t.Pick(theme.Brand100, theme.Brand700))
			color = t.Pick(theme.Brand800, theme.Brand100)
		}
		btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
		btn.OnClick(m.close).Children(func() { components.Icon(c, "close", 20, color) })
	})
}

// settingRow 是必选数据源与开关共用的行：左边名称与说明（errText 不为空时在下面用红字提示），右边控件。
func settingRow(c *ui.Context, t *theme.Theme, key, label, hint, errText string, control func()) {
	border := t.Pick(theme.Brand200, theme.Brand700)
	if errText != "" {
		border = t.Pick(theme.Error400, theme.Error600)
	}
	ui.Row(c.Key(key)).Gap(16).Padding(12).Radius(8).AlignItems(ui.Center).
		Border(t.HL, border).Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4))).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			components.Text(c, label, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200))
			components.Text(c, hint, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
			if errText != "" {
				components.Text(c, errText, theme.TextXS, theme.WeightMedium, t.Pick(theme.Error600, theme.Error400))
			}
		})
		control()
	})
}

// baseRow：必选数据源（组合策略的基底）的下拉框与说明，不受开关影响。打开了组合策略却选了“不指定”时红字提示。
func (m *StrategyModal) baseRow(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	var errText string
	if m.missingBase() {
		errText = locales.NativeMetadataStrategyBaseRequired()
	}
	settingRow(c, t, "base", locales.NativeMetadataStrategyBase(), locales.NativeMetadataStrategyBaseHint(), errText, func() {
		ui.Column(c).Width(200).Shrink(0).Children(func() {
			better.Select(c, t, "select", better.SelectProps{
				Value: string(m.draft.Base), Options: m.baseOptions(enabled), Font: theme.TextSM,
				OnChange: func(v string) { m.draft.Base = enums.SourceType(v) },
			})
		})
	})
}

// switchRows：组合策略分别用于刮削（导入、添加游戏、按名称重新搜索）与更新元数据的两个开关。
func (m *StrategyModal) switchRows(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("switches")).Gap(8).Children(func() {
		settingRow(c, t, "switch", locales.NativeMetadataStrategyEnable(), locales.NativeMetadataStrategyEnableHint(), "", func() {
			better.Switch(c, t, "switch", m.draft.Enabled, false, func(on bool) { m.draft.Enabled = on })
		})
		settingRow(c, t, "refresh-switch", locales.NativeMetadataRefreshUseStrategy(), locales.NativeMetadataRefreshUseStrategyHint(), "", func() {
			better.Switch(c, t, "switch", m.refresh, false, func(on bool) { m.refresh = on })
		})
	})
}

// fieldRows 是各字段的来源：每行左边字段名，右边来源下拉框。改用其他来源的行用天蓝色标出。
func (m *StrategyModal) fieldRows(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	ui.Column(c.Key("fields")).Gap(8).Children(func() {
		for _, f := range strategyFields {
			value := m.draft.FieldSource(f)
			if value == m.draft.Base {
				value = "" // 指定的就是基底来源，与跟随基底相同
			}
			bg, border := t.Pick(theme.Brand50, theme.Brand700.Alpha(0.3)), t.Pick(theme.Brand200, theme.Brand700)
			if m.overridden(f) {
				bg, border = t.Pick(theme.Sky50, theme.Sky900.Alpha(0.3)), t.Pick(theme.Sky300, theme.Sky700)
			}
			ui.Row(c.Key(string(f))).Gap(12).Padding(8, 12).Radius(8).AlignItems(ui.Center).Background(bg).Border(t.HL, border).Children(func() {
				ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.Text(c, fieldLabel(f), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand200)).SingleLine()
				})
				ui.Column(c).Width(200).Shrink(0).Children(func() {
					better.Select(c, t, "select", better.SelectProps{
						Value: string(value), Options: m.fieldOptions(f, enabled), Font: theme.TextSM,
						OnChange: func(v string) { m.draft = m.draft.WithFieldSource(f, enums.SourceType(v)) },
						Style:    func(btn ui.Element) { btn.Height(34).Padding(0, 32, 0, 12).AlignItems(ui.Center) },
					})
				})
			})
		}
	})
}

// summary 是自动生成的文字说明。
func (m *StrategyModal) summary(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	ui.Row(c.Key("summary")).Gap(10).Padding(12).Radius(8).AlignItems(ui.Start).
		Background(t.Pick(theme.Sky50.Alpha(0.6), theme.Sky900.Alpha(0.2))).Children(func() {
		components.Icon(c, "information-outline", 18, t.Pick(theme.Sky600, theme.Sky400))
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			for i, line := range StrategySummary(m.draft, enabled) {
				ui.Box(c.Key(i)).Children(func() {
					components.Text(c, line, theme.FontStep{Size: 13, Line: 20}, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100))
				})
			}
		})
	})
}

// footer：右边重置（所有字段跟随基底）与保存。
func (m *StrategyModal) footer(c *ui.Context, t *theme.Theme, p StrategyModalProps) {
	ui.Row(c.Key("footer")).Gap(12).AlignItems(ui.Center).Justify(ui.End).Children(func() {
		better.Button(c, t, "reset", better.ButtonProps{
			Label: locales.NativeMetadataStrategyReset(), Disabled: len(m.draft.Fields) == 0,
			OnClick: func() { m.draft.Fields = nil },
		})
		better.Button(c, t, "save", better.ButtonProps{
			Variant: better.ButtonPrimary, Icon: "check", Label: locales.CommonSave(), Disabled: m.missingBase(),
			OnClick: func() {
				if p.OnSave != nil {
					strategy, refresh := m.draft.Clone(), m.refresh
					p.OnSave(func(cfg *appconf.AppConfig) {
						cfg.SetStrategy(strategy)
						cfg.MetadataRefreshUseStrategy = refresh
					})
				}
				m.close()
			},
		})
	})
}
