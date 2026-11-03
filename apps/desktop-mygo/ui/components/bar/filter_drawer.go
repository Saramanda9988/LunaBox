package bar

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/tagname"
	"lunabox/desktop-mygo/ui/theme"
)

// FilterDrawer 是 FilterBar 里的 BetterDrawer：标签（TagFilterMenu）、状态、来源、预设（GameFilterPresetMenu）、
// 排序。要在页面最后构建，浮层才会盖住页面内容。
func FilterDrawer(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	f := p.Filters
	better.Drawer(c, t, "filter-drawer", better.DrawerProps{
		Open:       &f.DrawerOpen,
		Title:      locales.FilterBarFilters(),
		CloseLabel: locales.CommonCancel(),
		TopOffset:  env.TopbarHeight,
		HeaderAction: func() {
			btn := ui.ButtonBase(c.Key("clear-all")).Size(32, 32).Radius(8).Center().FocusRing(false).Label(locales.FilterBarClearFiltersAndSort())
			if !f.canClear(p) {
				components.Disable(btn, 0.35)
			}
			btn.OnClick(func() {
				f.clear(p)
				p.changed()
			}).Children(func() {
				components.Icon(c, "filter-off-outline", 18, t.TextMuted)
			})
		},
		Body: func() { drawerBody(c, t, p) },
	})
}

func drawerBody(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	f := p.Filters
	ui.Column(c).Children(func() {
		if p.GameFilters {
			tagFilterSection(c, t, p)
			chipSection(c, t, "status", locales.FilterBarStatus(), locales.FilterBarInvertStatusFilter(), f.Status != "", f.StatusInverted, func() {
				f.StatusInverted = !f.StatusInverted
				p.changed()
			}, func() {
				for _, o := range StatusOptions {
					if chip(c, t, string(o.Value), o.Label(), f.Status == o.Value).Clicked() {
						f.Status = o.Value
						if o.Value == "" {
							f.StatusInverted = false
						}
						p.changed()
					}
				}
			})
			chipSection(c, t, "source", locales.FilterBarMetadataSource(), locales.FilterBarInvertMetadataSourceFilter(), f.Source != "", f.SourceInverted, func() {
				f.SourceInverted = !f.SourceInverted
				p.changed()
			}, func() {
				for _, o := range SourceOptions {
					if chip(c, t, string(o.Value), o.Label(), f.Source == o.Value).Clicked() {
						f.Source = o.Value
						if o.Value == "" {
							f.SourceInverted = false
						}
						p.changed()
					}
				}
			})
			components.Divider(c, t)
			presetSection(c, t, p)
			components.Divider(c, t)
		}

		// 主排序（没有次排序时标题是“排序字段”）
		sortTitle := locales.FilterBarSortBy()
		if p.SecondarySort {
			sortTitle = locales.FilterBarPrimarySortBy()
		}
		ui.Column(c).Padding(6, 8).Children(func() {
			ui.Box(c).Margin(0, 0, 6, 0).Children(func() { components.SectionLabel(c, t, sortTitle) })
			better.Select(c, t, "sort", better.SelectProps{
				Value:   f.SortBy,
				Options: p.SortOptions,
				OnChange: func(v string) {
					f.SortBy = v
					if f.SecondarySortBy == f.SortBy {
						f.SecondarySortBy = ""
					}
					p.changed()
				},
			})
		})
		orderButtons(c, t, "order", f.SortOrder, true, func(o enums.SortOrder) { f.SortOrder = o; p.changed() })

		if p.SecondarySort {
			components.Divider(c, t)
			secondarySort(c, t, p)
		}

		if p.GameFilters {
			// 封面展示排序字段
			ui.Row(c).Padding(2, 8, 6, 8).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
				components.SectionLabel(c, t, locales.FilterBarShowSortField())
				better.Switch(c, t, "show-sort-field", f.ShowSortField, false, func(on bool) { f.ShowSortField = on })
			})
		}
	})
}

func secondarySort(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	f := p.Filters
	ui.Column(c).Padding(6, 8).Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Margin(0, 0, 6, 0).Children(func() {
			components.SectionLabel(c, t, locales.FilterBarSecondarySortBy())
			help := ui.ButtonBase(c.Key("secondary-help")).Size(20, 20).Radius(999).Center().FocusRing(false).
				Label(locales.FilterBarSecondarySortHelp())
			help.Children(func() { components.Icon(c, "help-circle-outline", 14, t.TextMuted) })
			better.Tooltip(c, t, help, locales.FilterBarSecondarySortHelp())
		})
		opts := []better.SelectOption{{Label: locales.FilterBarDefaultSecondarySort(), Value: ""}}
		for _, o := range p.SortOptions {
			if o.Value != f.SortBy {
				opts = append(opts, o)
			}
		}
		better.Select(c, t, "secondary", better.SelectProps{
			Value:   f.SecondarySortBy,
			Options: opts,
			OnChange: func(v string) {
				f.SecondarySortBy = v
				p.changed()
			},
		})
	})
	orderButtons(c, t, "secondary-order", f.SecondarySortOrder, f.SecondarySortBy != "", func(o enums.SortOrder) {
		f.SecondarySortOrder = o
		p.changed()
	})
}

// chip 对应抽屉中 rounded-full px-2.5 py-1 text-xs font-medium 的选项，选中时 ring-1。
func chip(c *ui.Context, t *theme.Theme, key, label string, active bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(4, 10).Radius(999).FocusRing(false)
	bg, fg := t.ChipBg, t.ChipText
	switch {
	case active:
		bg, fg = t.ChipActiveBg, t.ChipActiveText
		btn.Shadow(0, 0, 0, t.HL, t.ChipRing)
	case btn.Hovered():
		bg = t.ChipHover
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Text(c, label, theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
	return btn
}

// smallActionButton 对应 h-7 px-1.5 rounded-md text-sm font-medium 的“反选”“保存筛选与排序”。
// aria 是原版的 aria-label，为空时用按钮上的文字。
func smallActionButton(c *ui.Context, t *theme.Theme, key, icon, label, aria string, enabled, on bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Height(28).Padding(0, 6).Radius(6).Gap(4).AlignItems(ui.Center).FocusRing(false)
	if aria != "" {
		btn.Label(aria)
	}
	color := t.SectionLabel
	if on {
		color = t.Pick(theme.Neutral600, theme.Brand200)
	}
	if !enabled {
		components.Disable(btn, 0.45)
	} else if btn.Hovered() && !on {
		btn.Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.6)))
		color = t.Pick(theme.Brand900, theme.White)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		components.Icon(c, icon, 14, color)
		components.Text(c, label, theme.TextSM, theme.WeightMedium, color).NoWrap()
	})
	return btn
}

// chipSection：px-2 py-1.5，标题行 mb-1.5（标题 + 反选），下方 flex-wrap gap-1.5 的胶囊。
func chipSection(c *ui.Context, t *theme.Theme, key, title, invertLabel string, canInvert, inverted bool, onInvert func(), chips func()) {
	ui.Column(c.Key(key)).Padding(6, 8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Margin(0, 0, 6, 0).Children(func() {
			components.SectionLabel(c, t, title)
			if smallActionButton(c, t, "invert", "swap-horizontal", locales.FilterBarInvert(), invertLabel, canInvert, canInvert && inverted).Clicked() && canInvert {
				onInvert()
			}
		})
		ui.Row(c).Wrap().GapX(6).GapY(6).Children(chips)
	})
}

// tagFilterSection 对应 TagFilterMenu。
func tagFilterSection(c *ui.Context, t *theme.Theme, p FilterBarProps) {
	f := p.Filters
	translate := tagname.Enabled(p.Env.Config())
	ui.Column(c.Key("tags")).Padding(6, 8).Gap(8).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			components.SectionLabel(c, t, locales.FilterBarTagFilter())
			canInvert := len(f.Tags) > 0
			if smallActionButton(c, t, "invert", "swap-horizontal", locales.FilterBarInvert(), locales.FilterBarInvertTagFilter(), canInvert, canInvert && f.TagsInverted).Clicked() && canInvert {
				f.TagsInverted = !f.TagsInverted
				p.changed()
			}
		})
		hasValue := len(f.Tags) > 0 || f.TagInput != ""
		field := ui.Row(c.Key("tag-field")).Wrap().AlignItems(ui.Center).GapX(6).GapY(6).Padding(6, 8).MinHeight(34).
			Radius(8).Border(t.HL, t.PanelBorder).Background(t.ChipBg)
		if hasValue {
			field.Padding(6, 32, 20, 8)
		}
		field.Children(func() {
			components.Icon(c, "tag-outline", 16, t.TextMuted)
			for _, tag := range f.Tags {
				ui.Row(c.Key("tag-"+tag)).Gap(4).AlignItems(ui.Center).Padding(2, 6).Radius(4).Background(theme.Brand800).Children(func() {
					components.Text(c, tagname.Display(tag, translate), theme.TextXS, theme.WeightNormal, theme.Brand200)
					x := ui.ButtonBase(c.Key("remove")).FocusRing(false).Label(locales.NativeDeleteTag())
					x.OnClick(func() {
						f.removeTag(tag)
						p.changed()
					}).Children(func() { components.Icon(c, "close", 10, theme.Brand200) })
				})
			}
			input := ui.TextInputBase(c.Key("tag-input"), &f.TagInput).Grow(1).Basis(96).MinWidth(96).
				FontSize(theme.TextXS.Size).FixedLineHeight(theme.TextXS.Line).TextColor(t.TextStrong)
			if len(f.Tags) == 0 {
				input.Placeholder(locales.FilterBarTagsPlaceholder()).PlaceholderColor(theme.Brand400) // placeholder:text-brand-400
			}
			if input.Changed() {
				f.searchTags(p.Env, f.TagInput)
			}
			if input.Submitted() && len(f.TagSuggestions) > 0 {
				f.addTag(f.TagSuggestions[0])
				p.changed()
			}
			if hasValue {
				clr := ui.ButtonBase(c.Key("clear-tags")).Absolute().Bottom(4).Right(4).Padding(2).Radius(999).
					Background(theme.Brand900.Alpha(0.7)).FocusRing(false).Label(locales.NativeClearTags())
				clr.OnClick(func() {
					f.Tags, f.TagInput, f.TagSuggestions = nil, "", nil
					f.TagsInverted = false
					p.changed()
				}).Children(func() { components.Icon(c, "close-circle", 14, theme.Brand400) })
			}
		})
		if f.TagInput != "" && len(f.TagSuggestions) > 0 {
			tagSuggestionList(c, t, p, field)
		}
	})
}

func tagSuggestionList(c *ui.Context, t *theme.Theme, p FilterBarProps, field ui.Element) {
	f := p.Filters
	translate := tagname.Enabled(p.Env.Config())
	components.Overlay(c, t, func() {
		list := ui.Column(c.Key("tag-suggestions")).AttachTo(field, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(4).
			Width(field.Bounds().W).MaxHeight(144).Padding(4).Radius(8).
			Background(theme.Brand900.Alpha(0.95)).Border(t.HL, t.PanelBorder).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		list.Children(func() {
			ui.Scroll(c).Grow(1).Children(func() {
				for _, s := range f.TagSuggestions {
					item := ui.ButtonBase(c.Key(s)).FillWidth().Padding(6, 10).Radius(6).Justify(ui.Start).FocusRing(false)
					if item.Hovered() {
						item.Background(theme.Brand700)
					}
					item.OnClick(func() {
						f.addTag(s)
						p.changed()
					}).Children(func() {
						components.Text(c, tagname.Display(s, translate), theme.TextXS, theme.WeightNormal, theme.Brand200)
					})
				}
			})
		})
	})
}

// orderButtons：grid grid-cols-2 gap-1.5，按钮 rounded-lg px-2.5 py-2 text-xs font-medium。
func orderButtons(c *ui.Context, t *theme.Theme, key string, current enums.SortOrder, enabled bool, onChange func(enums.SortOrder)) {
	ui.Row(c.Key(key)).Padding(2, 8, 6, 8).Gap(6).Children(func() {
		for _, o := range []struct {
			value enums.SortOrder
			label func() string
			icon  string
		}{
			{enums.SortOrderAsc, locales.FilterBarSortAsc, "sort-ascending"},
			{enums.SortOrderDesc, locales.FilterBarSortDesc, "sort-descending"},
		} {
			btn := ui.ButtonBase(c.Key(o.value)).Grow(1).Basis(0).Padding(8, 10).Radius(8).Gap(6).Center().FocusRing(false)
			bg, fg := t.ChipBg, t.ChipText
			switch {
			case current == o.value:
				bg, fg = t.ChipActiveBg, t.ChipActiveText
			case enabled && btn.Hovered():
				bg = t.ChipHover
			}
			if !enabled {
				components.Disable(btn, 0.4)
			}
			btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
			btn.OnClick(func() { onChange(o.value) }).Children(func() {
				components.Icon(c, o.icon, 16, fg)
				components.Text(c, o.label(), theme.TextXS, theme.WeightMedium, fg)
			})
		}
	})
}
