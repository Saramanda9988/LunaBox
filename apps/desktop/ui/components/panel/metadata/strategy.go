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

// StrategyFocus 是打开弹窗的场景：对应的开关用天蓝色标出。
type StrategyFocus int

const (
	FocusNone    StrategyFocus = iota
	FocusScrape                // 刮削：批量导入、重新搜索（更新元数据）
	FocusRefresh               // 更新元数据：选择更新字段的弹窗
)

// StrategyModal 是“高级组合策略”弹窗：设置页的元数据分区、批量导入、详情页的更新字段与重新搜索共用。
// 弹窗里只有各字段指定的来源与刮削、更新元数据两个开关；基底随场景而定（批量导入是必选数据源，在批量导入里设置；
// 单个添加与重新搜索是选中的来源；更新元数据是游戏的默认来源）。每次修改立即写进配置（没有保存按钮）。
type StrategyModal struct {
	open    bool
	focus   StrategyFocus
	draft   appconf.MetadataStrategy
	refresh bool // 更新元数据时使用组合策略（AppConfig.MetadataRefreshUseStrategy）
	onSave  func(apply func(cfg *appconf.AppConfig))
}

// StrategyModalProps 是弹窗的属性。
type StrategyModalProps struct {
	Enabled   []enums.SourceType // 已启用的元数据来源（可选的来源）
	TopOffset float32
	// OnSave 在每次修改后调用：apply 把弹窗里的策略与开关写进配置，调用方用它修改配置（如 PatchConfig(apply)）。
	OnSave func(apply func(cfg *appconf.AppConfig))
}

// Open 用配置里的组合策略与两个开关打开弹窗，不标出哪个开关。
func (m *StrategyModal) Open(cfg *appconf.AppConfig) { m.OpenFor(cfg, FocusNone) }

// OpenFor 同 Open，focus 对应的开关用天蓝色标出。
func (m *StrategyModal) OpenFor(cfg *appconf.AppConfig, focus StrategyFocus) {
	*m = StrategyModal{open: true, focus: focus, draft: cfg.Strategy(), refresh: cfg != nil && cfg.MetadataRefreshUseStrategy}
}

// IsOpen 报告弹窗是否打开。
func (m *StrategyModal) IsOpen() bool { return m.open }

func (m *StrategyModal) close() { m.open = false }

// commit 把当前的策略与开关写进配置（每次修改后调用）。
func (m *StrategyModal) commit() {
	if m.onSave == nil {
		return
	}
	strategy, refresh := m.draft.Clone(), m.refresh
	m.onSave(func(cfg *appconf.AppConfig) {
		cfg.SetStrategy(strategy)
		cfg.MetadataRefreshUseStrategy = refresh
	})
}

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

// StrategySummary 是组合策略的文字说明：先按打开的开关各说一句基底是谁，最后一句说各字段的来源。
// 两个开关都关闭时为空。改用了未启用的来源时标出来（这些字段实际会用基底的数据）。
func StrategySummary(s appconf.MetadataStrategy, refresh bool, enabled []enums.SourceType) []string {
	var lines []string
	if s.Enabled {
		lines = append(lines, locales.NativeMetadataStrategySummaryScrape())
	}
	if refresh {
		lines = append(lines, locales.NativeMetadataStrategySummaryRefresh())
	}
	if len(lines) == 0 {
		return nil
	}
	var order []enums.SourceType
	groups := map[enums.SourceType][]string{}
	mergeAliases := false
	for _, f := range strategyFields {
		switch src := s.FieldSource(f); src {
		case "":
			continue
		case appconf.MetadataStrategyAllSources:
			mergeAliases = true
		default:
			if _, ok := groups[src]; !ok {
				order = append(order, src)
			}
			groups[src] = append(groups[src], fieldLabel(f))
		}
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
		return append(lines, locales.NativeMetadataStrategySummaryAllFromBase())
	}
	clauses = append(clauses, locales.NativeMetadataStrategySummaryRestBase())
	return append(lines, strings.Join(clauses, locales.NativeMetadataStrategyClauseSeparator()))
}

// strategyFieldNote 是“将从 X 更新”；别名合并所有来源时是“合并所有来源”。
func strategyFieldNote(src enums.SourceType) better.FieldNote {
	if src == appconf.MetadataStrategyAllSources {
		return better.FieldNote{Text: StrategyLabel(src)}
	}
	return better.FieldNote{Text: locales.NativeMetadataStrategyFieldFrom(StrategyLabel(src))}
}

// refreshStrategy 是更新元数据时用的组合策略：设置里没有打开“更新元数据时使用组合策略”时为空（只用游戏的默认来源）。
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
// 打开了“更新元数据时使用组合策略”时按 字段指定的来源 → base 找第一个可用的来源；否则都是 base，
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

// BatchRefreshNotes 是全量更新时各字段将从哪个来源更新（给 FieldNotes）：打开了“更新元数据时使用组合策略”时，
// 指定了来源的字段标“将从 X 更新”，指定的来源没有启用时是琥珀色的“将从默认来源更新”；没有指定的字段用各游戏自己的
// 默认来源，不标。没有打开时为空。各游戏有没有那个来源的 ID 要逐个看，这里不标。
func BatchRefreshNotes(cfg *appconf.AppConfig) map[enums.MetadataUpdateField]better.FieldNote {
	strategy, on := refreshStrategy(cfg)
	if !on {
		return nil
	}
	enabled := EnabledSources(cfg)
	notes := map[enums.MetadataUpdateField]better.FieldNote{}
	for _, f := range enums.DefaultMetadataUpdateFields {
		switch src := strategy.FieldSource(f); {
		case src == "":
		case src == appconf.MetadataStrategyAllSources || slices.Contains(enabled, src):
			notes[f] = strategyFieldNote(src)
		default:
			notes[f] = better.FieldNote{Text: locales.NativeMetadataStrategyFieldFromDefault(), Warn: true,
				Reason: locales.NativeMetadataStrategyReasonDisabled(StrategyLabel(src))}
		}
	}
	return notes
}

// RefreshUsesStrategy 报告更新以 base 为默认来源的游戏时组合策略有没有影响：打开了“更新元数据时使用组合策略”，
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

// BaseOptions 是批量导入的必选数据源的选项：不指定与已启用的来源（当前值 base 没有启用时也列出）。
func BaseOptions(base enums.SourceType, enabled []enums.SourceType) []better.SelectOption {
	opts := []better.SelectOption{{Label: locales.BatchImportModalPreferredSourceNone(), Value: ""}}
	for _, s := range enabled {
		opts = append(opts, better.SelectOption{Label: StrategyLabel(s), Value: string(s)})
	}
	if base != "" && !slices.Contains(enabled, base) {
		opts = append(opts, better.SelectOption{Label: sourceOptionLabel(base, enabled), Value: string(base)})
	}
	return opts
}

// fieldOptions 是字段来源的选项：跟随基底、（别名）合并所有来源、已启用的来源（当前值没有启用时也列出）。
func (m *StrategyModal) fieldOptions(f enums.MetadataUpdateField, enabled []enums.SourceType) []better.SelectOption {
	opts := []better.SelectOption{{Label: locales.NativeMetadataStrategyFollowBase(), Value: ""}}
	if f == enums.MetadataUpdateFieldAliases {
		opts = append(opts, better.SelectOption{Label: locales.NativeMetadataStrategyMergeAll(), Value: string(appconf.MetadataStrategyAllSources)})
	}
	for _, s := range enabled {
		opts = append(opts, better.SelectOption{Label: StrategyLabel(s), Value: string(s)})
	}
	cur := m.draft.FieldSource(f)
	if cur != "" && !slices.ContainsFunc(opts, func(o better.SelectOption) bool { return o.Value == string(cur) }) {
		opts = append(opts, better.SelectOption{Label: sourceOptionLabel(cur, enabled), Value: string(cur)})
	}
	return opts
}

// View 构建弹窗：标题、刮削与更新元数据两个开关、各字段的来源、文字说明，底部右边是重置。
// 各字段的来源不受开关影响；两个开关都关闭时没有文字说明。修改立即保存。
func (m *StrategyModal) View(c *ui.Context, t *theme.Theme, p StrategyModalProps) {
	m.onSave = p.OnSave
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
						m.switchRows(c, t)
						m.fieldRows(c, t, p.Enabled)
						if lines := StrategySummary(m.draft, m.refresh, p.Enabled); len(lines) > 0 {
							m.summary(c, t, lines)
						}
					})
				})
				m.footer(c, t)
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

// settingRow 是开关的行：左边名称与说明，右边控件。highlight 时（从对应的场景打开）用天蓝色标出。
func settingRow(c *ui.Context, t *theme.Theme, key, label, hint string, highlight bool, control func()) {
	bg, border := t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4)), t.Pick(theme.Brand200, theme.Brand700)
	if highlight {
		bg, border = t.Pick(theme.Sky50, theme.Sky900.Alpha(0.3)), t.Pick(theme.Sky300, theme.Sky700)
	}
	ui.Row(c.Key(key)).Gap(16).Padding(12).Radius(8).AlignItems(ui.Center).
		Border(t.HL, border).Background(bg).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			components.Text(c, label, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200))
			components.Text(c, hint, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		control()
	})
}

// switchRows：组合策略分别用于刮削（批量导入、添加游戏、按名称重新搜索）与更新元数据的两个开关。
func (m *StrategyModal) switchRows(c *ui.Context, t *theme.Theme) {
	ui.Column(c.Key("switches")).Gap(8).Children(func() {
		settingRow(c, t, "switch", locales.NativeMetadataStrategyEnable(), locales.NativeMetadataStrategyEnableHint(), m.focus == FocusScrape, func() {
			better.Switch(c, t, "switch", m.draft.Enabled, false, func(on bool) { m.draft.Enabled = on; m.commit() })
		})
		settingRow(c, t, "refresh-switch", locales.NativeMetadataRefreshUseStrategy(), locales.NativeMetadataRefreshUseStrategyHint(), m.focus == FocusRefresh, func() {
			better.Switch(c, t, "switch", m.refresh, false, func(on bool) { m.refresh = on; m.commit() })
		})
	})
}

// fieldRows 是各字段的来源：每行左边字段名，右边来源下拉框。指定了来源的行用天蓝色标出。
func (m *StrategyModal) fieldRows(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	ui.Column(c.Key("fields")).Gap(8).Children(func() {
		for _, f := range strategyFields {
			value := m.draft.FieldSource(f)
			bg, border := t.Pick(theme.Brand50, theme.Brand700.Alpha(0.3)), t.Pick(theme.Brand200, theme.Brand700)
			if value != "" {
				bg, border = t.Pick(theme.Sky50, theme.Sky900.Alpha(0.3)), t.Pick(theme.Sky300, theme.Sky700)
			}
			ui.Row(c.Key(string(f))).Gap(12).Padding(8, 12).Radius(8).AlignItems(ui.Center).Background(bg).Border(t.HL, border).Children(func() {
				ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.Text(c, fieldLabel(f), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand200)).SingleLine()
				})
				ui.Column(c).Width(200).Shrink(0).Children(func() {
					better.Select(c, t, "select", better.SelectProps{
						Value: string(value), Options: m.fieldOptions(f, enabled), Font: theme.TextSM,
						OnChange: func(v string) { m.draft = m.draft.WithFieldSource(f, enums.SourceType(v)); m.commit() },
						Style:    func(btn ui.Element) { btn.Height(34).Padding(0, 32, 0, 12).AlignItems(ui.Center) },
					})
				})
			})
		}
	})
}

// summary 是自动生成的文字说明。
func (m *StrategyModal) summary(c *ui.Context, t *theme.Theme, lines []string) {
	ui.Row(c.Key("summary")).Gap(10).Padding(12).Radius(8).AlignItems(ui.Start).
		Background(t.Pick(theme.Sky50.Alpha(0.6), theme.Sky900.Alpha(0.2))).Children(func() {
		components.Icon(c, "information-outline", 18, t.Pick(theme.Sky600, theme.Sky400))
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			for i, line := range lines {
				ui.Box(c.Key(i)).Children(func() {
					components.Text(c, line, theme.FontStep{Size: 13, Line: 20}, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand100))
				})
			}
		})
	})
}

// footer：右边重置（所有字段跟随基底，立即保存）。
func (m *StrategyModal) footer(c *ui.Context, t *theme.Theme) {
	ui.Row(c.Key("footer")).Gap(12).AlignItems(ui.Center).Justify(ui.End).Children(func() {
		better.Button(c, t, "reset", better.ButtonProps{
			Label: locales.NativeMetadataStrategyReset(), Disabled: len(m.draft.Fields) == 0,
			OnClick: func() { m.draft.Fields = nil; m.commit() },
		})
	})
}

// StrategyButtonLabel 是各入口打开组合策略弹窗的按钮文字：on（这个场景的开关打开了）时加上“已启用”。
func StrategyButtonLabel(on bool) string {
	if on {
		return locales.NativeMetadataStrategyButtonOn()
	}
	return locales.NativeMetadataStrategySettingsLabel()
}
