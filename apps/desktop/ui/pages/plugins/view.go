package plugins

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/assets"
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

// View 构建插件列表页：页头标题与操作栏、插件条目（图标 + 名称/简介 + 开关，点击进入详情页）、开发目录与确认弹窗。
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
				p.header(c, t)

				bodyW := min(listMaxW, contentW)
				ui.Column(c.Key("body")).Width(bodyW).Margin(0, ui.Auto).Gap(24).Children(func() {
					if len(list) == 0 {
						p.emptyState(c, t, bodyW)
					} else {
						ui.Column(c.Key("list")).FillWidth().Gap(10).Children(func() {
							for _, info := range list {
								p.pluginRow(c, t, info)
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

func (p *Page) header(c *ui.Context, t *theme.Theme) {
	ui.Row(c.Key("header")).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Wrap().GapX(16).GapY(12).Children(func() {
		components.Text(c, locales.NativeSideBarPlugins(), theme.Text4XL, theme.WeightBold, t.TextStrong)

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

func (p *Page) pluginRow(c *ui.Context, t *theme.Theme, info plugin.PluginInfo) {
	svc := p.env.Svc.Plugin
	bg, border := t.GlassCard(t.CardBg, t.CardBorder)

	card := ui.Row(c.Key("plugin-"+info.ID)).FillWidth().Padding(12, 16).Gap(14).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).Radius(12).
		Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))

	if card.Hovered() {
		hoverBg, hoverBorder := t.GlassCard(
			t.Pick(theme.Brand50, theme.Brand800.Alpha(0.75)),
			t.CardBorderHover,
		)
		card.Background(hoverBg).Border(t.HL, hoverBorder)
	} else {
		card.Background(bg)
	}

	card.Children(func() {
		// 左侧可点击区域：图标块 + 标题 + 简介（点击进入插件详情页）
		openBtn := ui.ButtonBase(c.Key("open-"+info.ID)).Grow(1).Basis(0).MinWidth(0).
			AlignItems(ui.Center).Justify(ui.Start).Gap(14).FocusRing(false).Label(info.Name)
		openBtn.OnClick(func() {
			if p.env != nil && p.env.Router != nil {
				p.env.Router.Push(env.PluginDetailPath(info.ID))
			}
		}).Children(func() {
			p.pluginIcon(c, t, info, 44, 12)

			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(3).Children(func() {
				components.Text(c, info.Name, theme.TextBase, theme.WeightSemibold, t.TextStrong).SingleLine()
				desc := info.Description
				if desc == "" {
					desc = info.ID
				}
				components.Text(c, desc, theme.TextXS, theme.WeightNormal, t.TextMuted).SingleLine()
				if info.LastError != "" {
					errFg := t.Pick(theme.Error600, theme.Error400)
					components.Text(c, info.LastError, theme.TextXS, theme.WeightNormal, errFg).SingleLine()
				}
			})
		})

		// 右侧启用开关（与左侧点击区并列，避免冒泡误入详情页）
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
}

// pluginIcon 渲染插件左侧圆角图标块：支持清单中的本地图片/SVG 文件、内置 MDI 图标名与默认回退图标。
func (p *Page) pluginIcon(c *ui.Context, t *theme.Theme, info plugin.PluginInfo, size, radius float32) {
	tileBg := t.Pick(theme.Brand100.Alpha(0.75), theme.Brand800.Alpha(0.85))
	if t.Glass {
		tileBg = t.Pick(theme.White.Alpha(0.55), theme.Black.Alpha(0.32))
	}
	if raw := strings.TrimSpace(info.IconBg); len(raw) == 7 && raw[0] == '#' {
		tileBg = ui.Hex(raw)
	}
	tileBorder := t.Pick(theme.Brand200.Alpha(0.9), theme.Brand700.Alpha(0.8))
	if t.Glass {
		tileBorder = t.Pick(theme.White.Alpha(0.35), theme.White.Alpha(0.14))
	}

	ui.Box(c.Key("icon-"+info.ID)).Size(size, size).Radius(radius).Center().Shrink(0).Clip().
		Background(tileBg).Border(t.HL, tileBorder).Children(func() {
		if info.IconPath != "" && p.plugins != nil {
			if src := p.plugins.LoadImageSource(info.IconPath); src != nil {
				imgSize := size * 0.62
				ui.Image(c, src).Size(imgSize, imgSize).Fit(ui.Contain)
				return
			}
		}
		iconName := assets.SafeIcon(info.Icon, "application-brackets-outline")
		iconSize := size * 0.52
		components.Icon(c, iconName, iconSize, t.TextStrong)
	})
}

func (p *Page) devDirsSection(c *ui.Context, t *theme.Theme, devPaths []string) {
	svc := p.env.Svc.Plugin
	heading := t.IfGlass(t.Pick(theme.Brand600, theme.Brand300), theme.Brand700, theme.Brand200)
	ui.Column(c.Key("dev-dirs")).FillWidth().Gap(8).Children(func() {
		components.TextMargin(c, locales.NativePluginsDevDirs(), theme.TextSM, theme.WeightMedium, heading, 0, 4, 4, 4)
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
		OnClose: func() {
			p.uninstallTarget = nil
			p.uninstallBackToList = false
		},
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
						OnClick: func() {
							p.uninstallTarget = nil
							p.uninstallBackToList = false
						},
					})
					better.Button(c, t, "confirm-uninstall", better.ButtonProps{
						Label:   locales.NativePluginsUninstall(),
						Variant: better.ButtonDanger,
						OnClick: func() {
							pid := target.ID
							delData := p.deleteData
							goBack := p.uninstallBackToList
							p.uninstallTarget = nil
							p.uninstallBackToList = false
							p.env.Async(func() func() {
								err := p.env.Svc.Plugin.Uninstall(pid, delData)
								return func() {
									if err != nil {
										p.env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
									} else {
										p.env.Toast(c, env.Success(locales.NativePluginsUninstallSuccess()))
										if goBack && p.env != nil && p.env.Router != nil {
											p.env.Router.Push(env.PathPlugins)
										}
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
