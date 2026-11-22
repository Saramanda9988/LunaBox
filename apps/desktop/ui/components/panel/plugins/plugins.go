// Package plugins 实现设置页内置的“插件管理”原生分区（见 docs/plugin-system.md §11）。
package plugins

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/panel"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// State 保存插件管理分区的弹窗与操作状态（由设置页持有）。
type State struct {
	busy bool

	// zip 安装确认弹窗
	installPreview *plugin.ZipInstallPreview
	installing     bool

	// 卸载确认弹窗
	uninstallTarget *plugin.PluginInfo
	deleteData      bool
}

// View 绘制插件管理分区内容与确认弹窗。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	if p.Env == nil || p.Env.Svc == nil || p.Env.Svc.Plugin == nil {
		return
	}
	svc := p.Env.Svc.Plugin
	list := svc.List()
	devPaths := svc.DevPaths()

	ui.Column(c.Key("plugins-panel")).FillWidth().Gap(16).Children(func() {
		// 顶部工具栏：安装 .zip、添加开发目录、打开插件目录
		ui.Row(c.Key("toolbar")).FillWidth().Wrap().GapX(8).GapY(8).AlignItems(ui.Center).Children(func() {
			better.Button(c, t, "install-zip", better.ButtonProps{
				Label:    locales.NativePluginsInstallZip(),
				Icon:     "package-variant",
				Variant:  better.ButtonPrimary,
				Size:     better.ButtonSM,
				Disabled: s.busy,
				OnClick: func() {
					s.busy = true
					p.Env.Async(func() func() {
						zipPath, err := svc.SelectPluginZip()
						if err != nil || zipPath == "" {
							return func() {
								s.busy = false
								if err != nil {
									p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
								}
							}
						}
						preview, perr := svc.InspectZip(zipPath)
						return func() {
							s.busy = false
							if perr != nil {
								p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(perr.Error())))
								return
							}
							s.installPreview = &preview
						}
					})
				},
			})
			better.Button(c, t, "add-dev", better.ButtonProps{
				Label:    locales.NativePluginsAddDevDir(),
				Icon:     "folder-plus-outline",
				Variant:  better.ButtonSecondary,
				Size:     better.ButtonSM,
				Disabled: s.busy,
				OnClick: func() {
					s.busy = true
					p.Env.Async(func() func() {
						dir, err := svc.SelectDevDir()
						if err != nil || dir == "" {
							return func() { s.busy = false }
						}
						aerr := svc.AddDevPath(dir)
						return func() {
							s.busy = false
							if aerr != nil {
								p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(aerr.Error())))
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
					p.Env.Async(func() func() {
						err := svc.OpenPluginDir("")
						return func() {
							if err != nil {
								p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
							}
						}
					})
				},
			})
		})

		// 已发现/已安装插件列表
		if len(list) == 0 {
			bg, border := t.GlassCard(t.CardBg, t.CardBorder)
			ui.Column(c.Key("empty")).FillWidth().Padding(24).Gap(6).Center().Radius(10).
				Background(bg).Border(t.HL, border).Children(func() {
				components.Icon(c, "application-brackets-outline", 32, t.TextFaint)
				components.Text(c, locales.NativePluginsEmpty(), theme.TextSM, theme.WeightMedium, t.TextMuted)
				components.Text(c, locales.NativePluginsEmptyHint(), theme.TextXS, theme.WeightNormal, t.TextFaint)
			})
		} else {
			ui.Column(c.Key("list")).FillWidth().Gap(10).Children(func() {
				for idx, info := range list {
					pluginRow(c, t, s, p, list, idx, info)
				}
			})
		}

		// 开发目录列表
		if len(devPaths) > 0 {
			ui.Column(c.Key("dev-dirs")).FillWidth().Gap(8).Padding(8, 0, 0, 0).Children(func() {
				panel.GroupTitle(c, t, locales.NativePluginsDevDirs())
				for _, dir := range devPaths {
					d := dir
					bg, border := t.GlassCard(t.CardBg, t.CardBorder)
					ui.Row(c.Key("dev-"+d)).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).
						Justify(ui.SpaceBetween).Radius(8).Background(bg).Border(t.HL, border).Children(func() {
						components.Text(c, d, theme.TextXS, theme.WeightNormal, t.TextMuted).SingleLine().Grow(1)
						btn := better.IconButton(c, t, "rm-dev-"+d, "close", 4, false, true)
						btn.Label(locales.CommonDelete()).OnClick(func() {
							p.Env.Async(func() func() {
								err := svc.RemoveDevPath(d)
								return func() {
									if err != nil {
										p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
									}
								}
							})
						})
					})
				}
			})
		}
	})

	installConfirmModal(c, t, s, p)
	uninstallConfirmModal(c, t, s, p)
}

func pluginRow(c *ui.Context, t *theme.Theme, s *State, p panel.Settings, all []plugin.PluginInfo, idx int, info plugin.PluginInfo) {
	svc := p.Env.Svc.Plugin
	bg, border := t.GlassCard(t.CardBg, t.CardBorder)
	ui.Column(c.Key("plugin-"+info.ID)).FillWidth().Padding(12, 14).Gap(8).Radius(10).
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
						p.Env.Async(func() func() {
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
						p.Env.Async(func() func() {
							_ = svc.Reorder(ids)
							return func() {}
						})
					})
					better.Tooltip(c, t, dn, locales.NativePluginsMoveDown())
				}
				rl := better.IconButton(c, t, "reload-"+info.ID, "refresh", 6, false, true)
				rl.Label(locales.NativePluginsReload()).OnClick(func() {
					pid := info.ID
					p.Env.Async(func() func() {
						err := svc.Reload(pid)
						return func() {
							if err != nil {
								p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
							} else {
								p.Env.Toast(c, env.Success(locales.NativePluginsReloadSuccess()))
							}
						}
					})
				})
				better.Tooltip(c, t, rl, locales.NativePluginsReload())

				od := better.IconButton(c, t, "dir-"+info.ID, "folder-open-outline", 6, false, true)
				od.Label(locales.NativePluginsOpenDir()).OnClick(func() {
					pid := info.ID
					p.Env.Async(func() func() {
						_ = svc.OpenPluginDir(pid)
						return func() {}
					})
				})
				better.Tooltip(c, t, od, locales.NativePluginsOpenDir())

				un := better.IconButton(c, t, "uninstall-"+info.ID, "delete-outline", 6, false, true)
				un.Label(locales.NativePluginsUninstall()).OnClick(func() {
					cp := info
					s.uninstallTarget = &cp
					s.deleteData = false
				})
				better.Tooltip(c, t, un, locales.NativePluginsUninstall())

				pid := info.ID
				better.Switch(c, t, "enable-"+info.ID, info.Enabled, info.Status == plugin.StatusLoading, func(next bool) {
					p.Env.Async(func() func() {
						var err error
						if next {
							err = svc.Enable(pid)
						} else {
							err = svc.Disable(pid)
						}
						return func() {
							if err != nil {
								p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
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

func installConfirmModal(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	if s.installPreview == nil {
		return
	}
	prev := s.installPreview
	title := locales.NativePluginsInstallTitle()
	if prev.IsUpgrade {
		title = locales.NativePluginsUpgradeTitle()
	}
	components.Modal(c, t, "plugin-install-confirm", components.ModalProps{
		Open:      true,
		MaxWidth:  480,
		TopOffset: env.TopbarHeight,
		OnClose: func() {
			if !s.installing {
				s.installPreview = nil
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
						Disabled: s.installing,
						OnClick:  func() { s.installPreview = nil },
					})
					better.Button(c, t, "confirm-install", better.ButtonProps{
						Label:   locales.NativePluginsConfirmInstall(),
						Variant: better.ButtonPrimary,
						Loading: s.installing,
						OnClick: func() {
							zipPath := prev.ZipPath
							name := prev.DisplayName
							s.installing = true
							p.Env.Async(func() func() {
								_, err := p.Env.Svc.Plugin.InstallFromZip(zipPath)
								return func() {
									s.installing = false
									s.installPreview = nil
									if err != nil {
										p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
									} else {
										p.Env.Toast(c, env.Success(locales.NativePluginsInstallSuccess(name)))
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

func uninstallConfirmModal(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	if s.uninstallTarget == nil {
		return
	}
	target := s.uninstallTarget
	components.Modal(c, t, "plugin-uninstall-confirm", components.ModalProps{
		Open:      true,
		MaxWidth:  440,
		TopOffset: env.TopbarHeight,
		OnClose:   func() { s.uninstallTarget = nil },
	}, func(panel ui.Element) {
		components.ModalPanel(t, panel).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() {
				components.Text(c, locales.NativePluginsUninstallTitle(), theme.TextXL, theme.WeightBold, t.TextStrong)
				components.Text(c, locales.NativePluginsUninstallMessage(target.Name), theme.TextSM, theme.WeightNormal, t.Text)
				if !target.IsDev {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						components.Checkbox(c, "delete-plugin-data", components.CheckboxProps{
							Checked:  s.deleteData,
							Label:    locales.NativePluginsUninstallDeleteData(),
							OnChange: func(v bool) { s.deleteData = v },
						})
						components.Text(c, locales.NativePluginsUninstallDeleteData(), theme.TextXS, theme.WeightNormal, t.TextMuted)
					})
				}
				ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
					better.Button(c, t, "cancel-uninstall", better.ButtonProps{
						Label:   locales.CommonCancel(),
						Variant: better.ButtonSecondary,
						OnClick: func() { s.uninstallTarget = nil },
					})
					better.Button(c, t, "confirm-uninstall", better.ButtonProps{
						Label:   locales.NativePluginsUninstall(),
						Variant: better.ButtonDanger,
						OnClick: func() {
							pid := target.ID
							delData := s.deleteData
							s.uninstallTarget = nil
							p.Env.Async(func() func() {
								err := p.Env.Svc.Plugin.Uninstall(pid, delData)
								return func() {
									if err != nil {
										p.Env.Toast(c, env.Error(locales.NativePluginsActionFailed(err.Error())))
									} else {
										p.Env.Toast(c, env.Success(locales.NativePluginsUninstallSuccess()))
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

func extractIDs(list []plugin.PluginInfo) []string {
	ids := make([]string, len(list))
	for i, it := range list {
		ids[i] = it.ID
	}
	return slices.Clone(ids)
}
