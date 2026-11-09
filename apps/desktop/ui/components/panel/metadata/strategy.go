package metadata

import (
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/service/gamehelper"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/metasource"
	"lunabox/desktop/ui/theme"
)

// strategyFields 是游戏卡片示意图右侧的字段，从上到下；封面在左侧单独画。
var strategyFields = []enums.MetadataUpdateField{
	enums.MetadataUpdateFieldName,
	enums.MetadataUpdateFieldAliases,
	enums.MetadataUpdateFieldCompany,
	enums.MetadataUpdateFieldReleaseDate,
	enums.MetadataUpdateFieldRating,
	enums.MetadataUpdateFieldTags,
	enums.MetadataUpdateFieldSummary,
}

// StrategyModal 是“高级组合策略”弹窗：设置页的元数据分区与批量导入共用。弹窗里改的是草稿，保存时才写进配置。
type StrategyModal struct {
	open  bool
	draft appconf.MetadataStrategy

	trialName    string
	trialReq     env.Request
	trialLoading bool
	trialError   string
	trial        *strategyTrial
}

// strategyTrial 是一次试刮的组合结果。
type strategyTrial struct {
	result  vo.GameMetadataFromWebVO
	sources gamehelper.FieldSources
}

// StrategyModalProps 是弹窗的属性。
type StrategyModalProps struct {
	Env       *env.Env
	Enabled   []enums.SourceType // 已启用的元数据来源（可选的来源）
	TopOffset float32
	OnSave    func(strategy appconf.MetadataStrategy)
}

// Open 用配置里的组合策略打开弹窗。
func (m *StrategyModal) Open(cfg *appconf.AppConfig) {
	m.trialReq.Cancel()
	*m = StrategyModal{open: true, draft: cfg.Strategy(), trialName: m.trialName}
}

func (m *StrategyModal) close() {
	m.trialReq.Cancel()
	m.open, m.trialLoading = false, false
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

// StrategySummary 是组合策略的文字说明：第一句说基底，第二句说各字段的来源（没有基底时只有一句）。
// 改用了未启用的来源时标出来（这些字段实际会用基底的数据）。
func StrategySummary(s appconf.MetadataStrategy, enabled []enums.SourceType) []string {
	if s.Base == "" {
		return []string{locales.NativeMetadataStrategySummaryNoBase()}
	}
	base := sourceOptionLabel(s.Base, enabled)
	first := locales.NativeMetadataStrategySummaryBase(base)
	var order []enums.SourceType
	groups := map[enums.SourceType][]string{}
	mergeAliases := false
	for _, f := range append([]enums.MetadataUpdateField{enums.MetadataUpdateFieldCover}, strategyFields...) {
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

// StrategyRefreshNotes 是更新元数据时组合策略的说明与各字段改用的来源（给 MetadataFieldSelectModal 的 Notice、FieldNotes）。
// base 是单个游戏的默认来源；为空时是全量更新（每个游戏以自己的默认来源为基底，标注按策略的基底算）。
// 策略没有基底、或者对这次更新没有影响时都为空。
func StrategyRefreshNotes(cfg *appconf.AppConfig, base enums.SourceType) (string, map[enums.MetadataUpdateField]string) {
	strategy := cfg.Strategy()
	if strategy.Base == "" || (base != "" && !metasource.Known(base)) {
		return "", nil
	}
	notice := locales.NativeMetadataStrategyRefreshNoticeAll()
	ref := strategy.Base
	if base != "" {
		notice, ref = locales.NativeMetadataStrategyRefreshNoticeSingle(StrategyLabel(base)), base
	}
	fields := gamehelper.StrategyOverridesFor(strategy, ref, EnabledSources(cfg))
	if len(fields) == 0 {
		return "", nil
	}
	notes := make(map[enums.MetadataUpdateField]string, len(fields))
	for _, f := range fields {
		src := strategy.FieldSource(f)
		notes[f] = locales.NativeMetadataStrategyFieldFrom(StrategyLabel(src))
		if src == appconf.MetadataStrategyAllSources {
			notes[f] = StrategyLabel(src)
		}
	}
	return notice, notes
}

// sourceOptionLabel 是来源的名称，没有启用时加上“（未启用）”。
func sourceOptionLabel(s enums.SourceType, enabled []enums.SourceType) string {
	if s != appconf.MetadataStrategyAllSources && !slices.Contains(enabled, s) {
		return locales.NativeMetadataStrategySourceDisabled(StrategyLabel(s))
	}
	return StrategyLabel(s)
}

// baseOptions 是基底来源的选项：不指定与已启用的来源（当前值没有启用时也列出）。
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

// View 构建弹窗：标题与说明、基底来源、游戏卡片示意图（每块区域选自己的来源）、文字说明、试刮，底部重置、取消与保存。
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
						m.diagram(c, t, p.Enabled)
						m.summary(c, t, p.Enabled)
						m.trialSection(c, t, p)
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
			components.Text(c, locales.NativeMetadataStrategyDescription(), theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
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

// baseRow：基底来源的下拉框与说明。
func (m *StrategyModal) baseRow(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	ui.Row(c.Key("base")).Gap(16).Padding(12).Radius(8).AlignItems(ui.Center).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4))).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
			components.Text(c, locales.NativeMetadataStrategyBase(), theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200))
			components.Text(c, locales.NativeMetadataStrategyBaseHint(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		ui.Column(c).Width(200).Shrink(0).Children(func() {
			better.Select(c, t, "select", better.SelectProps{
				Value: string(m.draft.Base), Options: m.baseOptions(enabled), Font: theme.TextSM,
				OnChange: func(v string) { m.draft.Base = enums.SourceType(v); m.trial, m.trialError = nil, "" },
			})
		})
	})
}

// diagram 是游戏卡片示意图：左边封面，右边名称、别名、开发商等区域。改用其他来源的区域用天蓝色标出。
func (m *StrategyModal) diagram(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	winW, _ := c.Size()
	card := ui.Row(c.Key("diagram")).Gap(16).Padding(16).Radius(12).AlignItems(ui.Start).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Background(t.Pick(theme.White, theme.Brand800))
	if winW < 640 {
		card.Column().AlignItems(ui.Stretch)
	}
	card.Children(func() {
		m.coverRegion(c, t, enabled)
		ui.Column(c.Key("fields")).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			for _, f := range strategyFields {
				m.fieldRegion(c.Key(string(f)), t, f, enabled)
			}
		})
	})
}

// regionColors 是区域的背景与边框：改用其他来源时是天蓝色，跟随基底时是中性色。
func (m *StrategyModal) regionColors(t *theme.Theme, f enums.MetadataUpdateField) (bg, border ui.Color) {
	if m.overridden(f) {
		return t.Pick(theme.Sky50, theme.Sky900.Alpha(0.3)), t.Pick(theme.Sky300, theme.Sky700)
	}
	return t.Pick(theme.Brand50, theme.Brand700.Alpha(0.3)), t.Pick(theme.Brand200, theme.Brand700)
}

// fieldSelect 是区域里的字段来源下拉框（紧凑样式）。
func (m *StrategyModal) fieldSelect(c *ui.Context, t *theme.Theme, f enums.MetadataUpdateField, enabled []enums.SourceType) {
	better.Select(c, t, "select", better.SelectProps{
		Value: string(m.draft.FieldSource(f)), Options: m.fieldOptions(f, enabled), Font: theme.TextXS,
		OnChange: func(v string) {
			m.draft = m.draft.WithFieldSource(f, enums.SourceType(v))
			m.trial, m.trialError = nil, ""
		},
		Style: func(btn ui.Element) { btn.Height(30).Padding(0, 32, 0, 10).AlignItems(ui.Center) },
	})
}

func (m *StrategyModal) coverRegion(c *ui.Context, t *theme.Theme, enabled []enums.SourceType) {
	f := enums.MetadataUpdateFieldCover
	bg, border := m.regionColors(t, f)
	ui.Column(c.Key("cover")).Width(168).Shrink(0).Gap(8).Padding(10).Radius(8).Background(bg).Border(t.HL, border).Children(func() {
		components.Text(c, fieldLabel(f), theme.TextXS, theme.WeightSemibold, t.Pick(theme.Brand700, theme.Brand200))
		ui.Box(c).FillWidth().AspectRatio(3.0/4).Radius(6).Center().Border(t.HL, border).BorderStyle(ui.BorderDashed).Children(func() {
			components.Icon(c, "image-filter-hdr", 36, t.Pick(theme.Brand300, theme.Brand500))
		})
		m.fieldSelect(c, t, f, enabled)
	})
}

// fieldRegion 是一个字段的区域：左边字段名与示意的内容，右边来源下拉框。
func (m *StrategyModal) fieldRegion(c *ui.Context, t *theme.Theme, f enums.MetadataUpdateField, enabled []enums.SourceType) {
	bg, border := m.regionColors(t, f)
	ui.Row(c).Gap(12).Padding(8, 10).Radius(8).AlignItems(ui.Center).Background(bg).Border(t.HL, border).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(6).Children(func() {
			components.Text(c, fieldLabel(f), theme.TextXS, theme.WeightSemibold, t.Pick(theme.Brand700, theme.Brand200)).SingleLine()
			skeleton(c, t, f)
		})
		ui.Column(c).Width(176).Shrink(0).Children(func() { m.fieldSelect(c, t, f, enabled) })
	})
}

// skeleton 是区域里示意内容的灰色条：名称一条粗的，标签几个小块，简介两行。
func skeleton(c *ui.Context, t *theme.Theme, f enums.MetadataUpdateField) {
	color := t.Pick(theme.Brand200, theme.Brand600)
	bar := func(key string, w, h float32) { ui.Box(c.Key(key)).Width(w).Height(h).Radius(h / 2).Background(color) }
	switch f {
	case enums.MetadataUpdateFieldName:
		bar("name", 160, 10)
	case enums.MetadataUpdateFieldTags:
		ui.Row(c.Key("tags")).Gap(6).Children(func() {
			for i, w := range []float32{40, 56, 32} {
				ui.Box(c.Key(i)).Width(w).Height(12).Radius(6).Background(color)
			}
		})
	case enums.MetadataUpdateFieldSummary:
		ui.Column(c.Key("summary")).Gap(5).FillWidth().Children(func() {
			ui.Box(c.Key(0)).FillWidth().Height(6).Radius(3).Background(color)
			bar("1", 120, 6)
		})
	}
}

// summary 是自动生成的文字说明与更新元数据时的规则。
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
			components.Text(c, locales.NativeMetadataStrategyRefreshHint(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
	})
}

// trialSection：输入游戏名试刮一次，结果按字段列出，每项标出来源。
func (m *StrategyModal) trialSection(c *ui.Context, t *theme.Theme, p StrategyModalProps) {
	ui.Column(c.Key("trial")).Gap(10).Children(func() {
		components.Text(c, locales.NativeMetadataStrategyTrialTitle(), theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand200))
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				in := better.Input(c, t, "name", &m.trialName, better.InputProps{Placeholder: locales.NativeMetadataStrategyTrialPlaceholder()})
				if in.Submitted() {
					m.runTrial(p)
				}
			})
			better.Button(c, t, "run", better.ButtonProps{
				Icon: "magnify", Label: locales.NativeMetadataStrategyTrialButton(), Loading: m.trialLoading,
				Disabled: strings.TrimSpace(m.trialName) == "" || m.draft.Base == "",
				OnClick:  func() { m.runTrial(p) },
			})
		})
		switch {
		case m.draft.Base == "":
			components.Text(c, locales.NativeMetadataStrategyTrialNeedBase(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		case m.trialError != "":
			components.Text(c, m.trialError, theme.TextXS, theme.WeightNormal, t.Pick(theme.Error600, theme.Error400))
		case m.trial != nil:
			m.trialResult(c, t, p)
		}
	})
}

// runTrial 按基底来源搜索，用草稿里的策略组合结果。
func (m *StrategyModal) runTrial(p StrategyModalProps) {
	name := strings.TrimSpace(m.trialName)
	if name == "" || m.draft.Base == "" || m.trialLoading || p.Env == nil || p.Env.Svc == nil {
		return
	}
	m.trialLoading, m.trialError, m.trial = true, "", nil
	strategy, svc, ctx := m.draft.Clone(), p.Env.Svc.Import, m.trialReq.Start()
	p.Env.Async(func() func() {
		r, err := svc.FetchMetadataForCandidateWithPreferenceContext(ctx, name, strategy.Base)
		return func() {
			if !m.trialReq.Finish(ctx) {
				return
			}
			m.trialLoading = false
			switch {
			case err != nil:
				log.Printf("metadata strategy: trial %q: %v", name, err)
				m.trialError = locales.NativeMetadataStrategyTrialFailed(err.Error())
			case r.PreferredNoResult || (r.PreferredMatched && len(r.Matches) == 0):
				m.trialError = locales.NativeMetadataStrategyTrialNotFound(StrategyLabel(strategy.Base))
			case !r.PreferredMatched:
				m.trialError = locales.NativeMetadataStrategyTrialFailed(r.PreferredError)
			default:
				result, sources := gamehelper.ComposeMetadata(strategy, r.Matches[0], r.Matches[1:])
				m.trial = &strategyTrial{result: result, sources: sources}
			}
		}
	})
}

// trialResult：左边封面，右边每个字段一行（字段名、内容、来源）。
func (m *StrategyModal) trialResult(c *ui.Context, t *theme.Theme, p StrategyModalProps) {
	g := m.trial.result.Game
	ui.Row(c.Key("trial-result")).Gap(16).Padding(12).Radius(8).AlignItems(ui.Start).
		Border(t.HL, t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
		ui.Column(c.Key("cover")).Width(96).Shrink(0).Gap(6).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(96, 128).Radius(6).Clip().Center().Background(t.Pick(theme.Brand200, theme.Brand700)).Children(func() {
				switch {
				case g.CoverURL == "" && g.CoverSourceURL == "":
					components.Icon(c, "image-off", 28, theme.Brand400)
				case p.Env != nil && p.Env.Covers != nil:
					if bmp := p.Env.Covers.Get(&g); bmp != nil {
						ui.Image(c, bmp).Fill().Fit(ui.Cover)
					}
				}
			})
			sourceChip(c, t, m.trial.sources[enums.MetadataUpdateFieldCover], m.draft.Base)
		})
		ui.Column(c.Key("fields")).Grow(1).Basis(0).MinWidth(0).Gap(6).Children(func() {
			for _, f := range strategyFields {
				ui.Row(c.Key(string(f))).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Width(64).Shrink(0).Children(func() {
						components.Text(c, fieldLabel(f), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).SingleLine()
					})
					value := trialValue(m.trial.result, f)
					color := t.TextStrong
					if value == "" {
						value, color = locales.NativeMetadataStrategyEmpty(), t.Pick(theme.Brand400, theme.Brand500)
					}
					ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
						components.Text(c, value, theme.TextXS, theme.WeightNormal, color).SingleLine()
					})
					sourceChip(c, t, m.trial.sources[f], m.draft.Base)
				})
			}
		})
	})
}

// trialValue 是试刮结果里字段的文字（单行显示）。
func trialValue(r vo.GameMetadataFromWebVO, f enums.MetadataUpdateField) string {
	g := r.Game
	switch f {
	case enums.MetadataUpdateFieldName:
		return g.Name
	case enums.MetadataUpdateFieldAliases:
		return strings.Join(g.Aliases, locales.NativeMetadataStrategyListSeparator())
	case enums.MetadataUpdateFieldCompany:
		return g.Company
	case enums.MetadataUpdateFieldReleaseDate:
		return g.ReleaseDate
	case enums.MetadataUpdateFieldRating:
		if g.Rating > 0 {
			return strconv.FormatFloat(g.Rating, 'f', 1, 64)
		}
	case enums.MetadataUpdateFieldTags:
		names := make([]string, 0, len(r.Tags))
		for _, tag := range r.Tags {
			names = append(names, tag.Name)
		}
		return strings.Join(names, locales.NativeMetadataStrategyListSeparator())
	case enums.MetadataUpdateFieldSummary:
		return strings.Join(strings.Fields(g.Summary), " ")
	}
	return ""
}

// sourceChip 是来源的小标签：来自基底时是中性色，来自其他来源时是天蓝色。
func sourceChip(c *ui.Context, t *theme.Theme, src, base enums.SourceType) {
	if src == "" {
		return
	}
	bg, fg := t.Pick(theme.Brand100, theme.Brand700), t.Pick(theme.Brand600, theme.Brand300)
	if src != base {
		bg, fg = t.Pick(theme.Sky100, theme.Sky900.Alpha(0.5)), t.Pick(theme.Sky700, theme.Sky300)
	}
	ui.Box(c.Key("chip")).Padding(2, 8).Radius(999).Shrink(0).Background(bg).Children(func() {
		components.Text(c, StrategyLabel(src), theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
}

// footer：左边“全部跟随基底”，右边取消与保存。
func (m *StrategyModal) footer(c *ui.Context, t *theme.Theme, p StrategyModalProps) {
	ui.Row(c.Key("footer")).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		better.Button(c, t, "reset", better.ButtonProps{
			Variant: better.ButtonGhost, Icon: "database-refresh-outline", Label: locales.NativeMetadataStrategyReset(),
			Disabled: len(m.draft.Fields) == 0,
			OnClick: func() {
				m.draft.Fields = nil
				m.trial, m.trialError = nil, ""
			},
		})
		ui.Row(c).Gap(12).Children(func() {
			better.Button(c, t, "cancel", better.ButtonProps{Label: locales.CommonCancel(), OnClick: m.close})
			better.Button(c, t, "save", better.ButtonProps{
				Variant: better.ButtonPrimary, Icon: "check", Label: locales.CommonSave(),
				OnClick: func() {
					if p.OnSave != nil {
						p.OnSave(m.draft.Clone())
					}
					m.close()
				},
			})
		})
	})
}
