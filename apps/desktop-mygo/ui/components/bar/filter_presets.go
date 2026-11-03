package bar

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/tagname"
	"lunabox/desktop-mygo/ui/theme"
)

// presetMenu 是 GameFilterPresetMenu 的状态：读取、新建表单与删除。
type presetMenu struct {
	loaded   bool   // loading 为假：第一次 ListGameFilterPresets 已返回
	saving   bool   // CreateGameFilterPreset 进行中
	deleting string // deletingPresetId
	formOpen bool   // isCreateFormOpen
	name     string // draftName
	draft    vo.SaveGameFilterPresetRequest
	helpOpen bool // 没有预设时的说明弹出层
}

// currentPreset 对应 currentFilters：当前的标签、状态、来源与排序（不含名称）。
func (f *Filters) currentPreset() vo.SaveGameFilterPresetRequest {
	r := vo.SaveGameFilterPresetRequest{
		Tags:            slices.Clone(f.Tags),
		ExcludeTags:     len(f.Tags) > 0 && f.TagsInverted,
		Status:          f.Status,
		ExcludeStatus:   f.Status != "" && f.StatusInverted,
		MetadataSource:  f.Source,
		SortBy:          enums.GameListSortBy(f.SortBy),
		SortOrder:       f.SortOrder,
		SecondarySortBy: enums.GameListSortBy(f.SecondarySortBy),
	}
	if r.SecondarySortBy != "" {
		r.SecondarySortOrder = f.SecondarySortOrder
	}
	return r
}

// presetHasFilters 对应 hasCurrentFilters 与 savePreset 里的 filterRequired 检查。
func presetHasFilters(r vo.SaveGameFilterPresetRequest) bool {
	return len(r.Tags) > 0 || r.Status != "" || r.MetadataSource != "" || (r.SortBy != "" && r.SortOrder != "")
}

func presetRequest(p models.GameFilterPreset) vo.SaveGameFilterPresetRequest {
	return vo.SaveGameFilterPresetRequest{
		Name: p.Name, Tags: p.Tags, ExcludeTags: p.ExcludeTags, Status: p.Status, ExcludeStatus: p.ExcludeStatus,
		MetadataSource: p.MetadataSource, SortBy: p.SortBy, SortOrder: p.SortOrder,
		SecondarySortBy: p.SecondarySortBy, SecondarySortOrder: p.SecondarySortOrder,
	}
}

// describePreset 对应 describeFilters：各条件的摘要，用“ · ”连接。
func describePreset(r vo.SaveGameFilterPresetRequest, translate bool) string {
	var parts []string
	if len(r.Tags) > 0 {
		names := make([]string, len(r.Tags))
		for i, tag := range r.Tags {
			names[i] = tagname.Display(tag, translate)
		}
		tags := strings.Join(names, ", ")
		if r.ExcludeTags {
			parts = append(parts, locales.FilterPresetsExcludeTagsSummary(tags))
		} else {
			parts = append(parts, locales.FilterPresetsIncludeTagsSummary(tags))
		}
	}
	if r.Status != "" {
		if r.ExcludeStatus {
			parts = append(parts, locales.FilterPresetsExcludeStatusSummary(StatusLabel(r.Status)))
		} else {
			parts = append(parts, locales.FilterPresetsIncludeStatusSummary(StatusLabel(r.Status)))
		}
	}
	if r.MetadataSource != "" {
		parts = append(parts, locales.FilterPresetsMetadataSourceSummary(sourceLabel(r.MetadataSource)))
	}
	if r.SortBy != "" && r.SortOrder != "" {
		parts = append(parts, locales.FilterPresetsSortSummary(sortLabel(r.SortBy), orderLabel(r.SortOrder)))
	}
	if r.SecondarySortBy != "" && r.SecondarySortOrder != "" {
		parts = append(parts, locales.FilterPresetsSecondarySortSummary(sortLabel(r.SecondarySortBy), orderLabel(r.SecondarySortOrder)))
	}
	return strings.Join(parts, " · ")
}

// sourceLabel 是来源在 SourceOptions 里的名称（local 是“无元数据来源”）。
func sourceLabel(s enums.SourceType) string {
	for _, o := range SourceOptions {
		if o.Value == s {
			return o.Label()
		}
	}
	return string(s)
}

func sortLabel(s enums.GameListSortBy) string {
	for _, o := range GameSortOptions() {
		if o.Value == string(s) {
			return o.Label
		}
	}
	return string(s)
}

func orderLabel(o enums.SortOrder) string {
	if o == enums.SortOrderAsc {
		return locales.FilterBarSortAsc()
	}
	return locales.FilterBarSortDesc()
}

// savePreset 对应 savePreset：检查名称与条件，CreateGameFilterPreset 后加到列表末尾。
func (f *Filters) savePreset(c *ui.Context, e *env.Env) {
	m := &f.presetUI
	name := strings.TrimSpace(m.name)
	if name == "" {
		e.Toast(c, env.Error(locales.FilterPresetsNameRequired()))
		return
	}
	if !presetHasFilters(m.draft) {
		e.Toast(c, env.Error(locales.FilterPresetsFilterRequired()))
		return
	}
	if m.saving || e.Svc == nil || e.Svc.Presets == nil {
		return
	}
	req := m.draft
	req.Name = name
	m.saving = true
	svc := e.Svc.Presets
	e.Async(func() func() {
		created, err := svc.CreateGameFilterPreset(req)
		return func() {
			m.saving = false
			if err != nil {
				log.Printf("filter presets: create: %v", err)
				e.Toast(c, env.Error(locales.FilterPresetsCreateFailed()))
				return
			}
			f.Presets = append(f.Presets, created)
			e.Toast(c, env.Success(locales.FilterPresetsCreateSuccess()))
			m.formOpen, m.name = false, ""
		}
	})
}

// deletePreset 对应 deletePreset：删除后从列表里移除（失败时保留，原版也不提示）。
func (f *Filters) deletePreset(e *env.Env, id string) {
	m := &f.presetUI
	if m.deleting != "" || e.Svc == nil || e.Svc.Presets == nil {
		return
	}
	m.deleting = id
	svc := e.Svc.Presets
	e.Async(func() func() {
		err := svc.DeleteGameFilterPreset(id)
		return func() {
			m.deleting = ""
			if err != nil {
				log.Printf("filter presets: delete %s: %v", id, err)
				return
			}
			f.Presets = slices.DeleteFunc(f.Presets, func(p models.GameFilterPreset) bool { return p.ID == id })
		}
	})
}

// presetSection 对应 GameFilterPresetMenu（space-y-2）：标题与“保存当前”，新建表单，已保存的预设。
func presetSection(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	f := p.Filters
	m := &f.presetUI
	translate := tagname.Enabled(p.Env.Config())
	ui.Column(c.Key("presets")).Padding(6, 8).Gap(8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			components.SectionLabel(c, t, locales.FilterPresetsTitle())
			current := f.currentPreset()
			enabled := presetHasFilters(current) && !m.saving
			if smallActionButton(c, t, "save", "bookmark-plus-outline", locales.FilterPresetsSaveCurrent(), "", enabled, false).Clicked() && enabled {
				m.name, m.draft, m.formOpen = "", current, true // openCreateForm
			}
		})
		if m.formOpen {
			presetForm(c, t, p, translate)
		}
		switch {
		case !m.loaded:
			// flex items-center justify-center gap-2 py-4 text-xs text-brand-400 dark:text-brand-500
			ui.Row(c.Key("loading")).Padding(16, 0).Gap(8).Center().Children(func() {
				components.Spinner(c, 16, t.TextFaint)
				components.Text(c, locales.CommonLoading(), theme.TextXS, theme.WeightNormal, t.TextFaint)
			})
		case len(f.Presets) == 0:
			presetEmpty(c, t, m)
		default:
			ui.Column(c.Key("list")).Gap(6).Children(func() {
				for _, preset := range f.Presets {
					presetRow(c.Key(preset.ID), t, p, preset, translate)
				}
			})
		}
	})
}

// presetForm：space-y-2 rounded-lg border border-brand-200 bg-brand-50/70 p-2.5 dark:border-brand-700 dark:bg-brand-900/40。
func presetForm(c *ui.Context, t *theme.Theme, p FilterBarProps, translate bool) {
	f := p.Filters
	m := &f.presetUI
	closeForm := func() { m.formOpen, m.name = false, "" }
	ui.Column(c.Key("form")).Gap(8).Padding(10).Radius(8).Border(t.HL, t.PanelBorder).
		Background(t.Pick(theme.Brand50.Alpha(0.7), theme.Brand900.Alpha(0.4))).Children(func() {
		components.Text(c, locales.FilterPresetsName(), theme.FontStep{Size: 11, Line: 16.5}, theme.WeightMedium, t.TextMuted)
		// glass-input w-full rounded-lg border px-2.5 py-2 text-xs placeholder:text-brand-400，focus:border-neutral-500
		in := ui.TextInputBase(c.Key("name"), &m.name).FillWidth().Padding(8, 10).Radius(8).AutoFocus().
			FontSize(theme.TextXS.Size).FixedLineHeight(theme.TextXS.Line).TextColor(t.TextStrong).
			Placeholder(locales.FilterPresetsNamePlaceholder()).PlaceholderColor(theme.Brand400).Label(locales.FilterPresetsName())
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		bg, border := t.GlassInput(t.Pick(theme.White, theme.Brand900.Alpha(0.7)), t.PanelBorder)
		if in.Focused() {
			border = theme.Neutral500
		}
		in.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(150*time.Millisecond)).
			OnShortcut(0, ui.KeyEscape, closeForm)
		if in.Submitted() {
			f.savePreset(c, p.Env)
		}
		// rounded-md bg-white/70 px-2 py-1.5 text-[11px] leading-relaxed text-brand-500 dark:bg-brand-800/60 dark:text-brand-400
		components.Text(c, describePreset(m.draft, translate), theme.FontStep{Size: 11, Line: 17.875}, theme.WeightNormal, t.TextMuted).
			Padding(6, 8).Radius(6).Background(t.Pick(theme.White.Alpha(0.7), theme.Brand800.Alpha(0.6)))
		ui.Row(c).Justify(ui.End).Gap(6).Children(func() {
			better.Button(c, t, "cancel", better.ButtonProps{Variant: better.ButtonGhost, Size: better.ButtonSM, Label: locales.CommonCancel(), Disabled: m.saving}).
				OnClick(closeForm)
			better.Button(c, t, "create", better.ButtonProps{Variant: better.ButtonPrimary, Size: better.ButtonSM, Icon: "plus", Label: locales.FilterPresetsCreate(), Loading: m.saving}).
				OnClick(func() { f.savePreset(c, p.Env) })
		})
	})
}

// presetEmpty 是没有预设时的说明按钮（PopoverButton）与弹出层（PopoverPanel anchor="bottom" w-80 rounded-xl p-4 shadow-xl）。
func presetEmpty(c *ui.Context, t *theme.Theme, m *presetMenu) {
	btn := ui.ButtonBase(c.Key("empty")).FillWidth().Padding(16, 12).Gap(6).Center().Radius(8).
		BorderStyle(ui.BorderDashed).FocusRing(false).Label(locales.FilterPresetsEmptyHelp())
	border, color, icon := t.PanelBorder, t.TextFaint, t.TextFaint
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand50.Alpha(0.7), theme.Brand900.Alpha(0.4)))
		border, color, icon = t.Pick(theme.Brand300, theme.Brand600), t.Pick(theme.Brand600, theme.Brand300), t.Pick(theme.Brand700, theme.Brand200)
	}
	btn.Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(func() { m.helpOpen = !m.helpOpen }).Children(func() {
		components.Text(c, locales.FilterPresetsEmpty(), theme.TextXS, theme.WeightNormal, color)
		components.Icon(c, "help-circle-outline", 16, icon)
	})
	ui.PopoverBase(c, btn, &m.helpOpen, func(panel ui.Element) {
		bg := t.PanelBg
		if t.Glass {
			bg = t.Pick(theme.White.Alpha(0.9), theme.Brand900.Alpha(0.9))
		}
		panel.AttachTo(btn, ui.AnchorBottom, ui.AnchorTop).Top(8).Width(320).Padding(16).Radius(12).
			Background(bg).Border(t.HL, t.PanelBorder).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		panel.Children(func() {
			components.Text(c, locales.FilterPresetsEmptyHelpTitle(), theme.TextSM, theme.WeightSemibold, t.TextStrong)
			components.TextMargin(c, locales.FilterPresetsEmptyHelpMessage(), theme.FontStep{Size: 14, Line: 24}, theme.WeightNormal,
				t.Pick(theme.Brand600, theme.Brand300), 8, 0, 0, 0)
		})
	})
}

// presetRow：group flex items-center gap-1 rounded-lg border bg-white/70 p-1，悬停 border-brand-300（深色 600）；
// 左边点击应用（名称 truncate text-xs，摘要 line-clamp-3 text-[11px]），右边 ghost sm 的删除按钮。
func presetRow(c *ui.Context, t *theme.Theme, p FilterBarProps, preset models.GameFilterPreset, translate bool) {
	f := p.Filters
	m := &f.presetUI
	row := ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(4).Radius(8).
		Background(t.Pick(theme.White.Alpha(0.7), theme.Brand900.Alpha(0.4)))
	border := t.PanelBorder
	if row.Hovered() {
		border = t.Pick(theme.Brand300, theme.Brand600)
	}
	row.Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		apply := ui.ButtonBase(c.Key("apply")).Grow(1).Basis(0).MinWidth(0).Padding(6, 8).Radius(6).
			Justify(ui.Start).AlignItems(ui.Start).FocusRing(false).Label(preset.Name)
		apply.OnClick(func() {
			f.ApplyPreset(preset)
			p.changed()
		}).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				components.Text(c, preset.Name, theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand200)).SingleLine()
				components.Text(c, describePreset(presetRequest(preset), translate), theme.FontStep{Size: 11, Line: 17.875}, theme.WeightNormal, t.TextFaint).
					MaxLines(3).Margin(2, 0, 0, 0)
			})
		})
		// BetterButton ghost sm 只有图标（h-8 w-8），hover:!bg-error-50 hover:!text-error-600
		// dark:hover:!bg-error-900/30 dark:hover:!text-error-400
		busy := m.deleting != ""
		del := ui.ButtonBase(c.Key("delete")).Size(32, 32).Radius(8).Center().Shrink(0).FocusRing(false).
			Label(locales.FilterPresetsDelete(preset.Name))
		color := t.Pick(theme.Brand600, theme.Brand400)
		if del.Hovered() && !busy {
			del.Background(t.Pick(theme.Error50, theme.Error900.Alpha(0.3)))
			color = t.Pick(theme.Error600, theme.Error400)
		}
		if busy {
			components.Disable(del, 0.5)
		}
		del.Transition(theme.ColorsTransition(200 * time.Millisecond)).Children(func() {
			if m.deleting == preset.ID {
				components.Spinner(c, 16, color)
			} else {
				components.Icon(c, "delete-outline", 16, color)
			}
		})
		del.OnClick(func() { f.deletePreset(p.Env, preset.ID) })
	})
}
