package edit

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// metadataDrawer 是“元数据来源”抽屉：BetterDrawer className="!w-[min(92vw,42rem)]" topOffset={TOPBAR_HEIGHT}，内容 space-y-4。
func (e *editor) metadataDrawer(c *ui.Context) {
	better.Drawer(c, e.t, "metadata-drawer", better.DrawerProps{
		Open:       &e.s.drawerOpen,
		Title:      locales.GameEditMetadataSources(),
		CloseLabel: locales.CommonCancel(),
		TopOffset:  panel.TopOffset,
		Width:      min(e.winW*0.92, 672),
		Body: func() {
			ui.Column(c.Key("drawer-body")).Gap(16).Children(func() {
				e.sourceList(c)
				e.addSourceForm(c)
				e.orDivider(c)
				e.searchButtons(c)
			})
		},
	})
}

// sourceList：divide-y divide-brand-200 dark:divide-brand-700 的来源列表，每项 space-y-3 py-4 first:pt-0 last:pb-0；
// 没有来源时是 rounded-lg border border-dashed px-3 py-5 text-center text-xs 的提示。
func (e *editor) sourceList(c *ui.Context) {
	t := e.t
	sources := e.g.MetadataSources
	if len(sources) == 0 {
		ui.Column(c.Key("no-sources")).Padding(20, 12).Radius(8).AlignItems(ui.Center).
			Border(t.HL, t.Pick(theme.Brand300, theme.Brand600)).BorderStyle(ui.BorderDashed).Children(func() {
			components.Text(c, locales.GameEditNoMetadataSources(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400))
		})
		return
	}
	ui.Column(c.Key("sources")).Children(func() {
		for i, src := range sources {
			item := ui.Column(c.Key(string(src.SourceType))).Gap(12)
			top, bottom := float32(16), float32(16)
			if i == 0 {
				top = 0
			} else {
				item.BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Divider)
			}
			if i == len(sources)-1 {
				bottom = 0
			}
			item.Padding(top, 0, bottom, 0).Children(func() { e.sourceItem(c, src) })
		}
	})
}

// sourceItem 是一个来源：标题行（logo h-[22px] 与 text-sm font-semibold 的名称，右侧设为默认与删除的 ghost 按钮），
// 下面是可编辑的来源 ID（输入停止 500ms 后自动保存，清空后失去焦点时恢复原值），右侧打开数据源网页。
func (e *editor) sourceItem(c *ui.Context, src models.GameMetadataSource) {
	t, s := e.t, e.s
	source := src.SourceType
	sourceID, edited := s.sourceEdits[source]
	if !edited {
		sourceID = src.SourceID
	}
	isDefault := e.g.SourceType == source
	busy := strings.HasPrefix(s.busySource, string(source)+":")
	label := sourceLabel(source)

	ui.Row(c.Key("header")).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Row(c).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
			if l := sourceLogo(source); l != nil {
				logoImage(c, t, l, 22, 96)
			}
			components.Text(c, label, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand100)).NoWrap()
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			defaultLabel := locales.GameEditSetDefaultSource()
			if isDefault {
				defaultLabel = locales.GameEditDefaultSource()
			}
			star := better.ButtonProps{
				Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: "star-outline",
				Loading: s.busySource == string(source)+":default", Disabled: busy,
			}
			if isDefault && !star.Loading {
				// className="!text-warning-500 dark:!text-warning-400"
				warning := t.Pick(ui.Hex("#f59e0b"), ui.Hex("#fbbf24"))
				star.Icon = ""
				star.Children = func(ui.Color) { components.Icon(c, "star", 16, warning) }
			}
			btn := better.Button(c, t, "default", star).Label(defaultLabel)
			if star.Children != nil {
				btn.Width(32).Padding(0)
			}
			btn.OnClick(func() {
				if !isDefault {
					e.setDefaultSource(c, source)
				}
			})
			better.Button(c, t, "delete", better.ButtonProps{
				Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: "delete-outline",
				Loading: s.busySource == string(source)+":delete", Disabled: busy,
			}).Label(locales.GameEditDeleteSource()).OnClick(func() { e.deleteSource(c, source) })
		})
	})

	url := sourceURL(source, sourceID, erogameScapeBase(e))
	v := sourceID
	in := actionInput(c, t, "source-id", &v, actionInputProps{Placeholder: locales.GameEditSourceIdPlaceholder(), Label: label + " ID"},
		inputAction{Icon: "open-in-new", Label: locales.GameEditOpenSourcePage(), Disabled: url == "", OnClick: func() { c.OpenURL(url) }},
	)
	if in.Changed() {
		s.sourceEdits[source] = v
		e.scheduleSourceSave(c, source, v)
	}
	if blurred(in) && strings.TrimSpace(v) == "" {
		s.sourceEdits[source] = src.SourceID
	}
}

func erogameScapeBase(e *editor) string {
	if cfg := e.p.Env.Config(); cfg != nil {
		return cfg.ErogameScapeBaseURL
	}
	return appconf.DefaultErogameScapeBaseURL
}

// addSourceForm：space-y-2 border-t pt-4，“添加来源”（text-sm font-semibold），下面 grid-cols-[9rem_minmax(0,1fr)_auto] gap-2 的
// 来源下拉框、ID 输入框与添加按钮。下拉框只列出还没配置的来源。
func (e *editor) addSourceForm(c *ui.Context) {
	t, s := e.t, e.s
	g := &e.g
	ui.Column(c.Key("add-source")).Gap(8).Padding(16, 0, 0, 0).BorderWidth(t.HL, 0, 0, 0).BorderColor(t.Divider).Children(func() {
		components.Text(c, locales.GameEditAddSource(), theme.TextSM, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand100))
		ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
			var options []better.SelectOption
			for _, source := range metadataSourceTypes {
				if !configured(g, source) {
					options = append(options, better.SelectOption{Label: sourceLabel(source), Value: string(source)})
				}
			}
			ui.Box(c).Width(144).Shrink(0).Children(func() {
				better.Select(c, t, "source-type", better.SelectProps{
					Value: string(s.sourceDraftType), Options: options,
					OnChange: func(v string) { s.sourceDraftType = enums.SourceType(v) },
				})
			})
			ui.Box(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				better.Input(c, t, "source-id", &s.sourceDraftID, better.InputProps{Placeholder: locales.GameEditSourceIdPlaceholder()})
			})
			better.Button(c, t, "add", better.ButtonProps{
				Variant: better.ButtonSecondary, Icon: "plus", Label: locales.GameEditAddSource(),
				Loading:  s.busySource == "add",
				Disabled: strings.TrimSpace(s.sourceDraftID) == "" || configured(g, s.sourceDraftType),
			}).OnClick(func() { e.addSource(c) })
		})
	})
}

// orDivider：flex items-center gap-3，两侧 h-px flex-1 bg-brand-200 dark:bg-brand-700，中间 text-xs 的“或”。
func (e *editor) orDivider(c *ui.Context) {
	t := e.t
	ui.Row(c.Key("or")).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Height(1).Grow(1).Background(t.Divider)
		components.Text(c, locales.GameEditOr(), theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
		ui.Box(c).Height(1).Grow(1).Background(t.Divider)
	})
}

// searchButtons：flex flex-col gap-3 的两个 w-full 按钮，按当前名称重新搜索（只更新关联 / 更新元数据）。
func (e *editor) searchButtons(c *ui.Context) {
	t, s := e.t, e.s
	ui.Column(c.Key("search")).Gap(12).Children(func() {
		label := func(normal string) string {
			if s.searching {
				return locales.CommonSearching()
			}
			return normal
		}
		better.Button(c, t, "search-relations", better.ButtonProps{
			Variant: better.ButtonSecondary, Icon: "database-search-outline", Loading: s.searching,
			Label: label(locales.GameEditSearchMetadataByCurrentName()),
		}).FillWidth().OnClick(func() { e.searchByName(c, searchSourceRelations) })
		better.Button(c, t, "search-update", better.ButtonProps{
			Variant: better.ButtonPrimary, Icon: "database-refresh-outline", Loading: s.searching,
			Label: label(locales.GameEditSearchMetadataAndUpdateByCurrentName()),
		}).FillWidth().OnClick(func() { e.searchByName(c, searchUpdateMetadata) })
	})
}
