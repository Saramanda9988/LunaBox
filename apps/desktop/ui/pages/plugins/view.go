package plugins

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

const (
	pagePadding = 32   // p-8
	listMaxW    = 1024 // max-w-5xl
)

// View 构建插件管理页：页头标题与操作栏、已安装/开发插件列表、开发目录与安装/卸载确认弹窗。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	gutter := t.Gutter()
	contentW := p.env.MainWidth - 2*pagePadding - gutter
	if contentW < 320 {
		contentW = 320
	}

	var list []plugin.PluginInfo
	var devPaths []string
	if p.env != nil && p.env.Svc != nil && p.env.Svc.Plugin != nil {
		list = p.env.Svc.Plugin.List()
		devPaths = p.env.Svc.Plugin.DevPaths()
	}

	components.ScrollArea(c, t, "plugins-scroll", &p.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(pagePadding, pagePadding+gutter, pagePadding, pagePadding)
		sc.Children(func() {
			ui.Column(c.Key("page")).Width(contentW).Gap(24).Children(func() {
				p.header(c, t, list)

				bodyW := min(listMaxW, contentW)
				ui.Column(c.Key("body")).Width(bodyW).Margin(0, ui.Auto).Gap(24).Children(func() {
					if len(list) == 0 {
						p.emptyState(c, t, bodyW)
					} else {
						ui.Column(c.Key("list")).FillWidth().Gap(12).Children(func() {
							for idx, info := range list {
								p.pluginRow(c, t, list, idx, info)
							}
						})
					}

					if len(devPaths) > 0 {
						p.devDirsSection(c, t, devPaths)
					}
				})
			})
		})
	})

	p.installConfirmModal(c, t)
	p.uninstallConfirmModal(c, t)
}

func (p *Page) header(c *ui.Context, t *theme.Theme, list []plugin.PluginInfo) {
	active := 0
	for _, info := range list {
		if info.Status == plugin.StatusActive {
			active++
		}
	}
	sub := locales.NativePluginsSubtitle()
	if len(list) > 0 {
		sub = locales.NativePluginsCount(strconv.Itoa(active), strconv.Itoa(len(list)))
	}

	ui.Row(c.Key("header")).FillWidth().AlignItems(ui.End).Justify(ui.SpaceBetween).Wrap().GapX(16).GapY(12).Children(func() {
		ui.Column(c).Children(func() {
			components.Text(c, locales.NativeSettingsSectionsPlugins(), theme.Text4XL, theme.WeightBold, t.TextStrong)
			components.TextMargin(c, sub, theme.TextSM, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
		})

		if p.env == nil || p.env.Svc == nil || p.env.Svc.Plugin == nil {
			return
		}
		svc := p.env.Svc.Plugin
		ui.Row(c.Key("toolbar")).Wrap().GapX(8).GapY(8).AlignItems(ui.Center).Children(func() {
			better.Button(c, t, "install-zip", better.ButtonProps{
				Label:    locales.NativePluginsInstallZip(),
				Icon:     "package-variant",
				Variant:  better.ButtonPrimary,
				Size:     better.ButtonSM,
				Disabled: p.busy,
				OnClick: func() {
					p.busy = true
					p.env.Async(func() func() {
						zipPath, err := svc.SelectPluginZip()
						if err != nil || zipPath == "" {
							return func() {
								p.busy = false
								if err != nil {
									p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
								}
							}
						}
						preview, perr := svc.InspectZip(zipPath)
						return func() {
							p.busy = false
							if perr != nil {
								p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(perr.Error())))
								return
							}
							p.installPreview = &preview
						}
					})
				},
			})
			better.Button(c, t, "add-dev", better.ButtonProps{
				Label:    locales.NativePluginsAddDevDir(),
				Icon:     "folder-plus-outline",
				Variant:  better.ButtonSecondary,
				Size:     better.ButtonSM,
				Disabled: p.busy,
				OnClick: func() {
					p.busy = true
					p.env.Async(func() func() {
						dir, err := svc.SelectDevDir()
						if err != nil || dir == "" {
							return func() { p.busy = false }
						}
						aerr := svc.AddDevPath(dir)
						return func() {
							p.busy = false
							if aerr != nil {
								p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(aerr.Error())))
							}
						}
					})
				},
			})
			better.Button(c, t, "open-dir", better.ButtonProps{
				Label:   locales.NativePluginsOpenDir(),
				Icon:    "folder-open-outline",
				Variant: better.ButtonGhost,
				Size:    better.ButtonSM,
				OnClick: func() {
					p.env.Async(func() func() {
						err := svc.OpenPluginDir("")
						return func() {
							if err != nil {
								p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
							}
						}
					})
				},
			})
		})
	})
}

func (p *Page) emptyState(c *ui.Context, t *theme.Theme, width float32) {
	fg := t.Pick(theme.Brand400, theme.Brand500)
	bg, border := t.GlassPanel(ui.Transparent, t.Pick(theme.Brand300, theme.Brand700))
	ui.Column(c.Key("empty")).Width(width).MinHeight(320).Gap(10).Center().Radius(16).Background(bg).
		Border(t.HL, border).BorderStyle(ui.BorderDashed).Children(func() {
		components.Icon(c, "application-brackets-outline", 48, fg)
		components.Text(c, locales.NativePluginsEmpty(), theme.TextBase, theme.WeightMedium, fg)
		components.Text(c, locales.NativePluginsEmptyHint(), theme.TextSM, theme.WeightNormal, t.TextFaint)
	})
}

func (p *Page) pluginRow(c *ui.Context, t *theme.Theme, all []plugin.PluginInfo, idx int, info plugin.PluginInfo) {
	svc := p.env.Svc.Plugin
	bg, border := t.GlassCard(t.CardBg, t.CardBorder)
	ui.Column(c.Key("plugin-"+info.ID)).FillWidth().Padding(14, 16).Gap(8).Radius(12).
		Background(bg).Border(t.HL, border).Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			// 左侧信息：名称 + 版本 + 状态徽章 + 描述
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
				ui.Row(c).Wrap().GapX(8).GapY(4).AlignItems(ui.Center).Children(func() {
					components.Text(c, info.Name, theme.TextBase, theme.WeightSemibold, t.TextStrong)
					if info.Version != "" {
						components.Text(c, "v"+info.Version, theme.TextXS, theme.WeightNormal, t.TextMuted)
					}
					statusBadge(c, t, info.Status)
					if info.IsDev {
						ui.Box(c).Padding(2, 6).Radius(999).Background(t.Pick(theme.Primary100, theme.Primary900)).Children(func() {
							components.Text(c, locales.NativePluginsDevBadge(), theme.TextXS, theme.WeightMedium, t.Pick(theme.Primary700, theme.Primary300))
						})
					}
				})
				if info.Description != "" {
					components.Text(c, info.Description, theme.TextXS, theme.WeightNormal, t.TextMuted)
				}
			})

			// 右侧动作：排序、重载、打开目录、卸载、启用开关
			ui.Row(c).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
				if idx > 0 {
					up := better.IconButton(c, t, "up-"+info.ID, "chevron-up", 6, false, true)
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
					dn := better.IconButton(c, t, "dn-"+info.ID, "chevron-down", 6, false, true)
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
				rl := better.IconButton(c, t, "reload-"+info.ID, "refresh", 6, false, true)
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

				od := better.IconButton(c, t, "dir-"+info.ID, "folder-open-outline", 6, false, true)
				od.Label(locales.NativePluginsOpenDir()).OnClick(func() {
					pid := info.ID
					p.env.Async(func() func() {
						_ = svc.OpenPluginDir(pid)
						return func() {}
					})
				})
				better.Tooltip(c, t, od, locales.NativePluginsOpenDir())

				un := better.IconButton(c, t, "uninstall-"+info.ID, "delete-outline", 6, false, true)
				un.Label(locales.NativePluginsUninstall()).OnClick(func() {
					cp := info
					p.uninstallTarget = &cp
					p.deleteData = false
				})
				better.Tooltip(c, t, un, locales.NativePluginsUninstall())

				pid := info.ID
				better.Switch(c, t, "enable-"+info.ID, info.Enabled, info.Status == plugin.StatusLoading, func(next bool) {
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

		if info.LastError != "" {
			errBg := t.Pick(theme.Error50, theme.Error900.Alpha(0.25))
			errFg := t.Pick(theme.Error600, theme.Error400)
			ui.Box(c).FillWidth().Padding(8, 10).Radius(6).Background(errBg).Children(func() {
				components.Text(c, info.LastError, theme.TextXS, theme.WeightNormal, errFg)
			})
		}
	})
}

func (p *Page) devDirsSection(c *ui.Context, t *theme.Theme, devPaths []string) {
	svc := p.env.Svc.Plugin
	heading := t.IfGlass(t.Pick(theme.Brand600, theme.Brand300), theme.Brand700, theme.Brand200)
	ui.Column(c.Key("dev-dirs")).FillWidth().Gap(8).Children(func() {
		components.TextMargin(c, locales.NativePluginsDevDirs(), theme.TextBase, theme.WeightMedium, heading, 0, 4, 4, 4)
		for _, dir := range devPaths {
			d := dir
			bg, border := t.GlassCard(t.CardBg, t.CardBorder)
			ui.Row(c.Key("dev-"+d)).FillWidth().Padding(10, 14).Gap(8).AlignItems(ui.Center).
				Justify(ui.SpaceBetween).Radius(10).Background(bg).Border(t.HL, border).Children(func() {
				components.Text(c, d, theme.TextXS, theme.WeightNormal, t.TextMuted).SingleLine().Grow(1)
				btn := better.IconButton(c, t, "rm-dev-"+d, "close", 4, false, true)
				btn.Label(locales.CommonDelete()).OnClick(func() {
					p.env.Async(func() func() {
						err := svc.RemoveDevPath(d)
						return func() {
							if err != nil {
								p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
							}
						}
					})
				})
			})
		}
	})
}

func statusBadge(c *ui.Context, t *theme.Theme, st plugin.PluginStatus) {
	var lbl string
	var bg, fg ui.Color
	switch st {
	case plugin.StatusActive:
		lbl = locales.NativePluginsStatusActive()
		bg, fg = t.Pick(theme.Success100, theme.Success900), t.Pick(theme.Success700, theme.Success300)
	case plugin.StatusLoading:
		lbl = locales.NativePluginsStatusLoading()
		bg, fg = t.Pick(theme.Info100, theme.Info900), t.Pick(theme.Info700, theme.Info300)
	case plugin.StatusError:
		lbl = locales.NativePluginsStatusError()
		bg, fg = t.Pick(theme.Error100, theme.Error900), t.Pick(theme.Error700, theme.Error300)
	case plugin.StatusDisposing:
		lbl = locales.NativePluginsStatusDisposing()
		bg, fg = t.Pick(theme.Warning100, theme.Warning900), t.Pick(theme.Warning700, theme.Warning300)
	case plugin.StatusDiscovered:
		lbl = locales.NativePluginsStatusDiscovered()
		bg, fg = t.ChipBg, t.ChipText
	default:
		lbl = locales.NativePluginsStatusDisabled()
		bg, fg = t.ChipBg, t.ChipText
	}
	ui.Box(c).Padding(2, 8).Radius(999).Background(bg).Children(func() {
		components.Text(c, lbl, theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
}

func (p *Page) installConfirmModal(c *ui.Context, t *theme.Theme) {
	if p.installPreview == nil {
		return
	}
	prev := p.installPreview
	title := locales.NativePluginsInstallTitle()
	if prev.IsUpgrade {
		title = locales.NativePluginsUpgradeTitle()
	}
	components.Modal(c, t, "plugin-install-confirm", components.ModalProps{
		Open:      true,
		MaxWidth:  480,
		TopOffset: env.TopbarHeight,
		OnClose: func() {
			if !p.installing {
				p.installPreview = nil
			}
		},
	}, func(panel ui.Element) {
		components.ModalPanel(t, panel).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() {
				components.Text(c, title, theme.TextXL, theme.WeightBold, t.TextStrong)
				ui.Column(c).Gap(4).Children(func() {
					components.Text(c, prev.DisplayName+" (v"+prev.Manifest.Version+")", theme.TextBase, theme.WeightSemibold, t.TextStrong)
					if prev.Description != "" {
						components.Text(c, prev.Description, theme.TextSM, theme.WeightNormal, t.TextMuted)
					}
					if prev.IsUpgrade {
						components.Text(c, locales.NativePluginsUpgradeFrom(prev.ExistingVersion, prev.Manifest.Version), theme.TextXS, theme.WeightMedium, theme.Primary500)
					}
				})
				// 明确的安全风险提示（docs/plugin-system.md §11 MUST）
				warnBg := t.Pick(theme.Warning50, theme.Warning900.Alpha(0.3))
				warnBorder := t.Pick(theme.Warning300, theme.Warning700)
				warnFg := t.Pick(theme.Warning800, theme.Warning300)
				ui.Row(c).FillWidth().Padding(10, 12).Gap(8).Radius(8).Background(warnBg).Border(t.HL, warnBorder).Children(func() {
					components.Icon(c, "alert-outline", 20, warnFg).Shrink(0)
					components.Text(c, locales.NativePluginsTrustWarning(), theme.TextXS, theme.WeightMedium, warnFg).Grow(1)
				})
				ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
					better.Button(c, t, "cancel-install", better.ButtonProps{
						Label:    locales.CommonCancel(),
						Variant:  better.ButtonSecondary,
						Disabled: p.installing,
						OnClick:  func() { p.installPreview = nil },
					})
					better.Button(c, t, "confirm-install", better.ButtonProps{
						Label:   locales.NativePluginsConfirmInstall(),
						Variant: better.ButtonPrimary,
						Loading: p.installing,
						OnClick: func() {
							zipPath := prev.ZipPath
							name := prev.DisplayName
							p.installing = true
							p.env.Async(func() func() {
								_, err := p.env.Svc.Plugin.InstallFromZip(zipPath)
								return func() {
									p.installing = false
									p.installPreview = nil
									if err != nil {
										p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
									} else {
										p.env.Toast(c, env.Success(locales.NativePluginsInstallSuccess(name)))
									}
								}
							})
						},
					})
				})
			})
		})
	})
}

func (p *Page) uninstallConfirmModal(c *ui.Context, t *theme.Theme) {
	if p.uninstallTarget == nil {
		return
	}
	target := p.uninstallTarget
	components.Modal(c, t, "plugin-uninstall-confirm", components.ModalProps{
		Open:      true,
		MaxWidth:  440,
		TopOffset: env.TopbarHeight,
		OnClose:   func() { p.uninstallTarget = nil },
	}, func(panel ui.Element) {
		components.ModalPanel(t, panel).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() {
				components.Text(c, locales.NativePluginsUninstallTitle(), theme.TextXL, theme.WeightBold, t.TextStrong)
				components.Text(c, locales.NativePluginsUninstallMessage(target.Name), theme.TextSM, theme.WeightNormal, t.Text)
				if !target.IsDev {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						components.Checkbox(c, "delete-plugin-data", components.CheckboxProps{
							Checked:  p.deleteData,
							Label:    locales.NativePluginsUninstallDeleteData(),
							OnChange: func(v bool) { p.deleteData = v },
						})
						components.Text(c, locales.NativePluginsUninstallDeleteData(), theme.TextXS, theme.WeightNormal, t.TextMuted)
					})
				}
				ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
					better.Button(c, t, "cancel-uninstall", better.ButtonProps{
						Label:   locales.CommonCancel(),
						Variant: better.ButtonSecondary,
						OnClick: func() { p.uninstallTarget = nil },
					})
					better.Button(c, t, "confirm-uninstall", better.ButtonProps{
						Label:   locales.NativePluginsUninstall(),
						Variant: better.ButtonDanger,
						OnClick: func() {
							pid := target.ID
							delData := p.deleteData
							p.uninstallTarget = nil
							p.env.Async(func() func() {
								err := p.env.Svc.Plugin.Uninstall(pid, delData)
								return func() {
									if err != nil {
										p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
									} else {
										p.env.Toast(c, env.Success(locales.NativePluginsUninstallSuccess()))
									}
								}
							})
						},
					})
				})
			})
		})
	})
}
