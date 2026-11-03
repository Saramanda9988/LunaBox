package dbbackup

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// card：glass-card bg-white dark:bg-brand-800 p-6 rounded-lg shadow-sm。
func card(c *ui.Context, t *theme.Theme, key string) ui.Element {
	bg, border := t.GlassCard(t.Pick(theme.White, theme.Brand800), ui.Transparent)
	e := ui.Column(c.Key(key)).FillWidth().Padding(24).Radius(8).Background(bg)
	if t.Glass {
		return e.Border(t.HL, border) // glass-card 的 shadow-none
	}
	return e.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
}

// heading：h3 text-lg font-semibold text-brand-900 dark:text-white。
func heading(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return components.Text(c, s, theme.TextLG, theme.WeightSemibold, t.TextStrong)
}

// actionsCard 是备份操作区：flex items-center justify-between mb-4 的标题、说明（text-sm mt-1）与立即备份按钮，
// 下面是 text-xs 的上次备份时间。
func (s *State) actionsCard(c *ui.Context, t *theme.Theme, p panel.Settings, cfg *appconf.AppConfig, cloudEnabled bool) {
	muted := t.Pick(theme.Brand500, theme.Brand400)
	card(c, t, "actions").Children(func() {
		ui.Row(c).FillWidth().Margin(0, 0, 16, 0).Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				heading(c, t, locales.SettingsDbBackupSectionTitle())
				components.TextMargin(c, locales.SettingsDbBackupSectionHint(), theme.TextSM, theme.WeightNormal, muted, 4, 0, 0, 0)
			})
			backupButton(c, t, s.backingUp, s.disabled()).OnClick(func() { s.createBackup(p, cfg, cloudEnabled) })
		})
		if s.status != nil && s.status.LastBackupTime != "" {
			// {t("settings.dbBackup.lastBackup")}{" "}{formatLocalDateTime(…)}
			when := formatTime(s.status.LastBackupTime, timefmt.Location(cfg))
			components.Text(c, locales.SettingsDbBackupLastBackup()+" "+when, theme.TextXS, theme.WeightNormal, muted)
		}
	})
}

// backupButton：glass-btn-neutral px-4 py-2 bg-neutral-600 text-white rounded-md hover:bg-neutral-700
// disabled:opacity-50 flex items-center gap-2，备份中前面是 i-mdi-loading animate-spin（1em）。
func backupButton(c *ui.Context, t *theme.Theme, busy, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key("backup-now")).Padding(8, 16).Radius(6).Gap(8).AlignItems(ui.Center).Shrink(0).FocusRing(false)
	g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{Bg: theme.Neutral600, Hover: theme.Neutral700, Text: theme.White})
	bg := g.Bg
	if btn.Hovered() && !disabled {
		bg = g.Hover
	}
	if t.Glass {
		btn.Border(t.HL, g.Border)
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.5)
	}
	label := locales.SettingsDbBackupBackupBtn()
	if busy {
		label = locales.SettingsDbBackupBackingUp()
	}
	btn.Children(func() {
		if busy {
			components.Spinner(c, 16, g.Text)
		}
		components.Text(c, label, theme.TextBase, theme.WeightNormal, g.Text).NoWrap()
	})
	return btn
}

// localCard 是本地备份列表：标题行（flex items-center gap-2 mb-4，自动备份开启时带徽标），加载中是 py-8 的旋转图标，
// 空时是 py-8 的提示，否则 space-y-3 的备份行。
func (s *State) localCard(c *ui.Context, t *theme.Theme, p panel.Settings, cfg *appconf.AppConfig, cloudEnabled bool) {
	card(c, t, "local").Children(func() {
		ui.Row(c).Margin(0, 0, 16, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			heading(c, t, locales.SettingsDbBackupLocalBackups())
			if cfg != nil && cfg.AutoBackupDB {
				autoBackupBadge(c, t)
			}
		})
		switch {
		case s.loadingLocal:
			loadingBlock(c)
		case s.status == nil || len(s.status.Backups) == 0:
			emptyBlock(c, locales.SettingsDbBackupNoLocalBackups())
		default:
			loc := timefmt.Location(cfg)
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				for _, b := range s.status.Backups {
					s.backupRow(c.Key(b.Path), t, p, b, loc, cloudEnabled)
				}
			})
		}
	})
}

// autoBackupBadge：px-2 py-0.5 text-xs font-medium bg-success-100 text-success-700 dark:bg-success-900/30
// dark:text-success-400 rounded-full flex items-center gap-1，i-mdi-shield-check text-sm。
func autoBackupBadge(c *ui.Context, t *theme.Theme) {
	fg := t.Pick(theme.Success700, theme.Success400)
	ui.Row(c.Key("auto-badge")).Padding(2, 8).Gap(4).Radius(999).AlignItems(ui.Center).
		Background(t.Pick(theme.Success100, theme.Success900.Alpha(0.3))).Children(func() {
		components.Icon(c, "shield-check", 14, fg)
		components.Text(c, locales.SettingsDbBackupAutoBackupEnabled(), theme.TextXS, theme.WeightMedium, fg).NoWrap()
	})
}

// loadingBlock：flex justify-center py-8，i-mdi-loading animate-spin text-2xl text-brand-500。
func loadingBlock(c *ui.Context) {
	ui.Row(c.Key("loading")).FillWidth().Padding(32, 0).Justify(ui.Center).Children(func() {
		components.Spinner(c, 24, theme.Brand500)
	})
}

// emptyBlock：text-center py-8 text-brand-500。
func emptyBlock(c *ui.Context, s string) {
	ui.Column(c.Key("empty")).FillWidth().Padding(32, 0).AlignItems(ui.Center).Children(func() {
		components.Text(c, s, theme.TextBase, theme.WeightNormal, theme.Brand500)
	})
}

// backupRow：flex items-center justify-between p-4 bg-brand-50 dark:bg-brand-700 rounded-lg；左侧 gap-4 的
// i-mdi-database text-2xl text-brand-500 与时间（font-medium）、大小（text-sm），右侧 gap-2 的上传（云备份可用时）、
// 恢复、删除按钮，正在上传或恢复的那一个换成旋转图标。
func (s *State) backupRow(c *ui.Context, t *theme.Theme, p panel.Settings, b vo.DBBackupInfo, loc *time.Location, cloudEnabled bool) {
	disabled := s.disabled()
	ui.Row(c).FillWidth().Padding(16).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(16).
		Background(t.IfGlass(t.Pick(theme.Brand50, theme.Brand700), theme.White.Alpha(0.01), theme.Black.Alpha(0.01))).Children(func() { // data-glass:bg-white/1 dark:bg-black/1
		ui.Row(c).Gap(16).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
			components.Icon(c, "database", 24, theme.Brand500)
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				components.Text(c, timefmt.DateTime(b.CreatedAt, loc), theme.TextBase, theme.WeightMedium, t.TextStrong)
				components.Text(c, locales.SettingsDbBackupSizeLabel()+components.FormatFileSize(float64(b.Size)),
					theme.TextSM, theme.WeightNormal, theme.Brand500)
			})
		})
		path := b.Path
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			if cloudEnabled {
				iconButton(c, t, "upload", locales.NativeBackupUploadToCloud(), "cloud-upload", s.uploading == path,
					theme.Neutral600, t.Pick(theme.Neutral100, theme.Neutral900), disabled).
					OnClick(func() { s.upload(p, path) })
			}
			iconButton(c, t, "restore", locales.SettingsDbBackupModalRestoreTitle(), "backup-restore", s.restoring == path,
				theme.Success600, t.Pick(theme.Success100, theme.Success900), disabled).
				OnClick(func() { s.askRestore(p, path) })
			iconButton(c, t, "delete", locales.SettingsDbBackupModalDeleteTitle(), "delete", false,
				theme.Error600, t.Pick(theme.Error100, theme.Error900), disabled).
				OnClick(func() { s.askDelete(p, path) })
		})
	})
}

// iconButton：p-2 rounded transition-colors 的图标按钮，i-mdi-* text-xl（进行中是 i-mdi-loading animate-spin），
// 悬停换背景，disabled:opacity-50。原版的按钮没有文字，无障碍名称用对应弹窗的标题。
func iconButton(c *ui.Context, t *theme.Theme, key, label, icon string, busy bool, color, hover ui.Color, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8).Radius(4).FocusRing(false).Label(label)
	if btn.Hovered() && !disabled {
		btn.Background(hover)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() {
		if busy {
			components.Spinner(c, 20, color)
		} else {
			components.Icon(c, icon, 20, color)
		}
	})
	return btn
}

// cloudCard 是云端备份列表：flex items-center justify-between mb-4 的标题（i-mdi-cloud text-xl text-neutral-500，gap-2）
// 与刷新按钮，下面同本地列表。
func (s *State) cloudCard(c *ui.Context, t *theme.Theme, p panel.Settings, cfg *appconf.AppConfig) {
	card(c, t, "cloud").Children(func() {
		ui.Row(c).FillWidth().Margin(0, 0, 16, 0).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Icon(c, "cloud", 20, theme.Neutral500)
				heading(c, t, locales.SettingsDbBackupCloudBackups())
			})
			// p-2 text-brand-600 hover:bg-brand-100 dark:hover:bg-brand-700 rounded disabled:opacity-50
			disabled := s.loadingCloud || s.disabled()
			btn := ui.ButtonBase(c.Key("refresh")).Padding(8).Radius(4).FocusRing(false).Label(locales.NativeBackupRefreshCloud())
			if btn.Hovered() && !disabled {
				btn.Background(t.Pick(theme.Brand100, theme.Brand700))
			}
			btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
			if disabled {
				components.Disable(btn, 0.5)
			}
			btn.OnClick(func() { s.loadCloud(p, nil) }).Children(func() {
				ic := components.Icon(c, "refresh", 20, theme.Brand600)
				if s.loadingCloud {
					ic.Rotate(ic.Loop("spin", time.Second, ui.Linear) * 360)
				}
			})
		})
		switch {
		case s.loadingCloud:
			loadingBlock(c)
		case len(s.cloudBackups) == 0:
			emptyBlock(c, locales.SettingsDbBackupNoCloudBackups())
		default:
			loc := timefmt.Location(cfg)
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				for _, item := range s.cloudBackups {
					s.cloudRow(c.Key(item.Key), t, p, item, loc)
				}
			})
		}
	})
}

// cloudRow：flex items-center justify-between p-4 bg-neutral-50 dark:bg-neutral-900/30 rounded-lg，
// i-mdi-cloud-check text-2xl text-neutral-500，名称（没有时是时间）与时间，右侧从云端恢复按钮。
func (s *State) cloudRow(c *ui.Context, t *theme.Theme, p panel.Settings, item vo.CloudBackupItem, loc *time.Location) {
	when := timefmt.DateTime(item.CreatedAt, loc)
	name := item.Name
	if name == "" {
		name = when
	}
	key := item.Key
	ui.Row(c).FillWidth().Padding(16).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(16).
		Background(t.IfGlass(t.Pick(theme.Neutral50, theme.Neutral900.Alpha(0.3)), theme.White.Alpha(0.01), theme.Black.Alpha(0.01))).Children(func() { // data-glass:bg-white/1 dark:bg-black/1
		ui.Row(c).Gap(16).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
			components.Icon(c, "cloud-check", 24, theme.Neutral500)
			ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
				components.Text(c, name, theme.TextBase, theme.WeightMedium, t.TextStrong)
				components.Text(c, when, theme.TextSM, theme.WeightNormal, theme.Brand500)
			})
		})
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			iconButton(c, t, "restore-cloud", locales.SettingsDbBackupModalRestoreCloudTitle(), "cloud-download", s.restoring == key,
				theme.Success600, t.Pick(theme.Success100, theme.Success900), s.disabled()).
				OnClick(func() { s.askRestoreCloud(p, key) })
		})
	})
}

// cloudHint 是云备份未启用提示：glass-card bg-brand-50 dark:bg-brand-800 p-4 rounded-lg border border-brand-200
// dark:border-brand-700，flex items-center gap-3，i-mdi-cloud-off-outline text-2xl text-brand-400。
func cloudHint(c *ui.Context, t *theme.Theme) {
	bg, border := t.GlassCard(t.Pick(theme.Brand50, theme.Brand800), t.Pick(theme.Brand200, theme.Brand700))
	ui.Row(c.Key("cloud-hint")).FillWidth().Padding(16).Radius(8).Gap(12).AlignItems(ui.Center).
		Background(bg).Border(t.HL, border).Children(func() {
		components.Icon(c, "cloud-off-outline", 24, theme.Brand400)
		ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
			components.Text(c, locales.SettingsDbBackupCloudNotEnabled(), theme.TextBase, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
			components.Text(c, locales.SettingsDbBackupCloudNotEnabledHint(), theme.TextSM, theme.WeightNormal, theme.Brand500)
		})
	})
}
