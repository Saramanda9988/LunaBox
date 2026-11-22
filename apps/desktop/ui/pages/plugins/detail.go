package plugins

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/pluginui"
	"lunabox/desktop/ui/theme"
)

const detailMaxW = 900

// ViewDetail 构建插件详情页（路由 /plugins/{id}）：
// 返回按钮、插件头图与信息、启用开关与操作、插件设置、已注册扩展以及来源信息。
func (p *Page) ViewDetail(c *ui.Context, id string) {
	t := p.env.Theme
	gutter := t.Gutter()
	contentW := p.env.MainWidth - 2*pagePadding - gutter
	if contentW < 320 {
		contentW = 320
	}
	bodyW := min(detailMaxW, contentW)

	all, idx, info, found := p.findPlugin(id)

	components.ScrollArea(c, t, "plugin-detail-scroll", &p.detailScroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			ui.Column(c.Key("detail-page-"+id)).Width(bodyW).Margin(0, ui.Auto).Gap(24).Children(func() {
				p.detailBackButton(c, t)

				if !found {
					p.detailNotFound(c, t)
					return
				}

				p.detailHeader(c, t, info)
				ui.Box(c).FillWidth().Height(t.HL).Background(t.Divider)
				p.detailSettingsSection(c, t, info)
				p.detailExtensionsSection(c, t, info)
				p.detailSourceInfoSection(c, t, all, idx, info)
			})
		})
	})

	p.uninstallConfirmModal(c, t)
}

func (p *Page) detailBackButton(c *ui.Context, t *theme.Theme) {
	btn := ui.ButtonBase(c.Key("detail-back")).AlignSelf(ui.Start).AlignItems(ui.Center).
		Radius(6).FocusRing(false).Label(locales.NativeSideBarPlugins())
	color := t.Pick(theme.Brand600, theme.Brand400)
	if btn.Hovered() {
		color = t.Pick(theme.Brand900, theme.Brand200)
	}
	color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	btn.OnClick(func() {
		if p.env != nil && p.env.Router != nil {
			p.env.Router.Push(env.PathPlugins)
		}
	}).Children(func() {
		components.Icon(c, "chevron-left", 20, color).Margin(0, 2, 0, 0)
		components.Text(c, locales.NativeSideBarPlugins(), theme.TextSM, theme.WeightMedium, color).NoWrap()
	})
}

func (p *Page) detailNotFound(c *ui.Context, t *theme.Theme) {
	fg := t.Pick(theme.Brand400, theme.Brand500)
	ui.Column(c.Key("detail-not-found")).FillWidth().MinHeight(240).Center().Gap(10).Children(func() {
		components.Icon(c, "application-brackets-outline", 44, fg)
		components.Text(c, locales.NativePluginsDetailNotFound(), theme.TextBase, theme.WeightMedium, fg)
	})
}

func (p *Page) detailHeader(c *ui.Context, t *theme.Theme, info plugin.PluginInfo) {
	svc := p.env.Svc.Plugin
	ui.Column(c.Key("detail-hero")).FillWidth().Gap(14).Children(func() {
		// 顶行：左侧图标块 + 右侧操作按钮与总开关
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Children(func() {
			p.pluginIcon(c, t, info, 56, 14)

			ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
				rl := better.IconButton(c, t, "detail-reload-"+info.ID, "refresh", 8, false, true)
				rl.Label(locales.NativePluginsReload()).OnClick(func() {
					pid := info.ID
					p.env.Async(func() func() {
						err := svc.Reload(pid)
						return func() {
							if err != nil {
								p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
							} else {
								p.env.Toast(c, env.Success(locales.NativePluginsReloadSuccess()))
							}
						}
					})
				})
				better.Tooltip(c, t, rl, locales.NativePluginsReload())

				od := better.IconButton(c, t, "detail-dir-"+info.ID, "folder-open-outline", 8, false, true)
				od.Label(locales.NativePluginsOpenDir()).OnClick(func() {
					pid := info.ID
					p.env.Async(func() func() {
						_ = svc.OpenPluginDir(pid)
						return func() {}
					})
				})
				better.Tooltip(c, t, od, locales.NativePluginsOpenDir())

				un := better.IconButton(c, t, "detail-uninstall-"+info.ID, "delete-outline", 8, false, true)
				un.Label(locales.NativePluginsUninstall()).OnClick(func() {
					cp := info
					p.uninstallTarget = &cp
					p.deleteData = false
					p.uninstallBackToList = true
				})
				better.Tooltip(c, t, un, locales.NativePluginsUninstall())

				ui.Box(c).Width(4)
				pid := info.ID
				better.Switch(c, t, "detail-enable-"+info.ID, info.Enabled, info.Status == plugin.StatusLoading, func(next bool) {
					p.env.Async(func() func() {
						var err error
						if next {
							err = svc.Enable(pid)
						} else {
							err = svc.Disable(pid)
						}
						return func() {
							if err != nil {
								p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
							}
						}
					})
				})
			})
		})

		// 名称 + 版本号 + ID + 完整简介
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			ui.Row(c).Wrap().GapX(10).GapY(6).AlignItems(ui.Center).Children(func() {
				components.Text(c, info.Name, theme.Text3XL, theme.WeightBold, t.TextStrong)
				if info.Version != "" {
					ui.Box(c).Padding(2, 10).Radius(999).Background(t.ChipBg).Children(func() {
						components.Text(c, "v"+info.Version, theme.TextXS, theme.WeightMedium, t.ChipText).NoWrap()
					})
				}
			})
			components.Text(c, info.ID, theme.TextXS, theme.WeightNormal, t.TextFaint)
			if info.Description != "" {
				components.TextMargin(c, info.Description, theme.TextSM, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
			}
		})

		if info.LastError != "" {
			errBg := t.Pick(theme.Error50, theme.Error900.Alpha(0.25))
			errBorder := t.Pick(theme.Error200, theme.Error700)
			errFg := t.Pick(theme.Error600, theme.Error400)
			ui.Box(c).FillWidth().Padding(10, 12).Radius(8).Background(errBg).Border(t.HL, errBorder).Children(func() {
				components.Text(c, info.LastError, theme.TextXS, theme.WeightNormal, errFg)
			})
		}
	})
}

func (p *Page) detailSettingsSection(c *ui.Context, t *theme.Theme, info plugin.PluginInfo) {
	if p.env == nil || p.env.Svc == nil || p.env.Svc.Plugin == nil {
		return
	}
	var sections []plugin.SlotRegistration
	for _, s := range p.env.Svc.Plugin.Slots(plugin.SlotSettingsSection) {
		if s.PluginID == info.ID {
			sections = append(sections, s)
		}
	}
	if len(sections) == 0 {
		return
	}

	ui.Column(c.Key("detail-settings-"+info.ID)).FillWidth().Gap(12).Children(func() {
		components.Text(c, locales.NativePluginsDetailSettings(), theme.TextBase, theme.WeightSemibold, t.TextStrong)
		for _, slot := range sections {
			s := slot
			bg, border := t.GlassCard(t.CardBg, t.CardBorder)
			ui.Column(c.Key("detail-sec-"+s.PluginID+"-"+s.ID)).FillWidth().Padding(20).Gap(14).Radius(12).
				Background(bg).Border(t.HL, border).Children(func() {
				if len(sections) > 1 && s.Title != "" {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						components.Icon(c, assets.SafeIcon(s.Icon, "application-brackets-outline"), 18, t.TextMuted)
						components.Text(c, s.Title, theme.TextSM, theme.WeightSemibold, t.TextStrong)
					})
				}
				if s.ConfigSchema != nil {
					pluginui.RenderConfigSchemaForm(c, t, p.env, s.PluginID, s.ConfigSchema)
				} else if p.plugins != nil {
					p.plugins.RenderSlot(c, t, plugin.SlotSettingsSection, s.PluginID, s.ID, "plugin-detail-"+info.ID, nil)
				}
			})
		}
	})
}

func (p *Page) detailExtensionsSection(c *ui.Context, t *theme.Theme, info plugin.PluginInfo) {
	if p.env == nil || p.env.Svc == nil || p.env.Svc.Plugin == nil {
		return
	}
	type extItem struct {
		key   string
		icon  string
		title string
		sub   string
	}
	var items []extItem
	for _, s := range p.env.Svc.Plugin.Slots("") {
		if s.PluginID != info.ID || s.SlotName == plugin.SlotSettingsSection {
			continue
		}
		title := s.Title
		if title == "" {
			title = s.ID
		}
		items = append(items, extItem{
			key:   "slot-" + s.SlotName + "-" + s.ID,
			icon:  assets.SafeIcon(s.Icon, "application-brackets-outline"),
			title: title,
			sub:   s.SlotName + " · " + s.ID,
		})
	}
	for _, d := range p.env.Svc.Plugin.Decorations() {
		if d.PluginID != info.ID {
			continue
		}
		items = append(items, extItem{
			key:   "deco-" + d.ID,
			icon:  "palette",
			title: d.ID,
			sub:   plugin.SlotAppDecoration + " · " + d.Layer,
		})
	}
	if len(items) == 0 {
		return
	}

	ui.Column(c.Key("detail-exts-"+info.ID)).FillWidth().Gap(10).Children(func() {
		components.Text(c, locales.NativePluginsDetailExtensions(), theme.TextBase, theme.WeightSemibold, t.TextStrong)
		ui.Column(c).FillWidth().Children(func() {
			for i, it := range items {
				if i > 0 {
					ui.Box(c).FillWidth().Height(t.HL).Background(t.Divider)
				}
				ui.Row(c.Key(it.key)).FillWidth().Padding(12, 4).Gap(12).AlignItems(ui.Center).Children(func() {
					tileBg := t.Pick(theme.Brand100.Alpha(0.75), theme.Brand800.Alpha(0.85))
					ui.Box(c).Size(36, 36).Radius(10).Center().Shrink(0).Background(tileBg).Border(t.HL, t.CardBorder).Children(func() {
						components.Icon(c, it.icon, 18, t.TextStrong)
					})
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(2).Children(func() {
						components.Text(c, it.title, theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine()
						components.Text(c, it.sub, theme.TextXS, theme.WeightNormal, t.TextFaint).SingleLine()
					})
				})
			}
		})
	})
}

func (p *Page) detailSourceInfoSection(c *ui.Context, t *theme.Theme, all []plugin.PluginInfo, idx int, info plugin.PluginInfo) {
	svc := p.env.Svc.Plugin
	sourceLabel := locales.NativePluginsDetailSourceInstalled()
	if info.IsDev {
		sourceLabel = locales.NativePluginsDetailSourceDev()
	}

	ui.Column(c.Key("detail-source-"+info.ID)).FillWidth().Gap(8).Children(func() {
		components.Text(c, locales.NativePluginsDetailSourceInfo(), theme.TextBase, theme.WeightSemibold, t.TextStrong)

		ui.Column(c).FillWidth().Children(func() {
			p.infoRow(c, t, locales.NativePluginsDetailSourceType(), sourceLabel, nil)
			if info.Version != "" {
				p.infoRow(c, t, locales.NativePluginsDetailVersion(), info.Version, nil)
			}
			p.infoRow(c, t, locales.NativePluginsDetailPluginId(), info.ID, nil)
			if info.Author != "" {
				p.infoRow(c, t, locales.NativePluginsDetailAuthor(), info.Author, nil)
			}
			if info.Engines != "" {
				p.infoRow(c, t, locales.NativePluginsDetailEngines(), info.Engines, nil)
			}
			if info.License != "" {
				p.infoRow(c, t, locales.NativePluginsDetailLicense(), info.License, nil)
			}
			if info.Homepage != "" {
				url := info.Homepage
				p.infoRow(c, t, locales.NativePluginsDetailHomepage(), url, func() {
					c.OpenURL(url)
				})
			}
			if info.Dir != "" {
				pid := info.ID
				p.infoRow(c, t, locales.NativePluginsDetailDirectory(), info.Dir, func() {
					p.env.Async(func() func() {
						_ = svc.OpenPluginDir(pid)
						return func() {}
					})
				})
			}
			if len(all) > 1 && idx >= 0 {
				p.orderInfoRow(c, t, all, idx, info)
			}
		})
	})
}

func (p *Page) infoRow(c *ui.Context, t *theme.Theme, label, value string, onClick func()) {
	ui.Row(c.Key("info-"+label)).FillWidth().Padding(12, 0).Gap(16).AlignItems(ui.Center).
		Justify(ui.SpaceBetween).BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider).Children(func() {
		components.Text(c, label, theme.TextSM, theme.WeightNormal, t.TextMuted).NoWrap().Shrink(0)
		if onClick != nil {
			btn := ui.ButtonBase(c.Key("info-link-"+label)).FocusRing(false).Label(value).OnClick(onClick)
			fg := t.TextStrong
			if btn.Hovered() {
				fg = theme.Primary500
			}
			btn.Children(func() {
				components.Text(c, value, theme.TextSM, theme.WeightNormal, fg).SingleLine()
			})
		} else {
			components.Text(c, value, theme.TextSM, theme.WeightNormal, t.TextStrong).SingleLine()
		}
	})
}

func (p *Page) orderInfoRow(c *ui.Context, t *theme.Theme, all []plugin.PluginInfo, idx int, info plugin.PluginInfo) {
	svc := p.env.Svc.Plugin
	ui.Row(c.Key("info-order")).FillWidth().Padding(10, 0).Gap(16).AlignItems(ui.Center).
		Justify(ui.SpaceBetween).BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider).Children(func() {
		components.Text(c, locales.NativePluginsDetailOrder(), theme.TextSM, theme.WeightNormal, t.TextMuted).NoWrap().Shrink(0)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			components.Text(c, fmt.Sprintf("#%d / %d", idx+1, len(all)), theme.TextSM, theme.WeightNormal, t.TextStrong).NoWrap()
			if idx > 0 {
				up := better.IconButton(c, t, "detail-up-"+info.ID, "chevron-up", 4, false, true)
				up.Label(locales.NativePluginsMoveUp()).OnClick(func() {
					ids := extractIDs(all)
					ids[idx-1], ids[idx] = ids[idx], ids[idx-1]
					p.env.Async(func() func() {
						_ = svc.Reorder(ids)
						return func() {}
					})
				})
				better.Tooltip(c, t, up, locales.NativePluginsMoveUp())
			}
			if idx < len(all)-1 {
				dn := better.IconButton(c, t, "detail-dn-"+info.ID, "chevron-down", 4, false, true)
				dn.Label(locales.NativePluginsMoveDown()).OnClick(func() {
					ids := extractIDs(all)
					ids[idx+1], ids[idx] = ids[idx], ids[idx+1]
					p.env.Async(func() func() {
						_ = svc.Reorder(ids)
						return func() {}
					})
				})
				better.Tooltip(c, t, dn, locales.NativePluginsMoveDown())
			}
		})
	})
}
