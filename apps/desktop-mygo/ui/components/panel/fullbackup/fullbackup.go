// Package fullbackup 对应原版 components/panel/FullDataBackupPanel.tsx（设置页“全量数据备份”分区）：导出全量备份到
// 用户选择的 ZIP，或选择一个 ZIP 安排在下次启动时恢复（确认后提示并在 1.5 秒后退出）。
package fullbackup

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// quitDelay 是安排恢复之后到退出的时间（setTimeout(() => SafeQuit(), 1500)）。
const quitDelay = 1500 * time.Millisecond

// backupAPI 是面板用到的 BackupService 方法（测试里替换文件对话框）。
type backupAPI interface {
	SelectBackupSavePath() (string, error)
	SelectBackupRestorePath() (string, error)
	CreateFullDataBackup(savePath string) error
	ScheduleFullDataRestore(backupPath string) error
}

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	exporting bool // isFullBackingUp
	restoring bool // isRestoring
	picking   bool // 文件对话框打开中（原版的对话框是模态的，不会被再次点击）

	confirm confirmState
	toasts  []env.Notice // 后台结果里待显示的提示（apply 拿不到 Context）

	quitPending bool      // 已安排恢复，等待退出
	quitAt      time.Time // 退出的时间；为零时下一次构建取 c.Now()

	api  backupAPI // 为空时用 Env.Svc.Backup
	quit func()    // 为空时用 Env.Svc.AppConfig.SafeQuit
}

// confirmState 对应原版的 confirmConfig（这里只有恢复一种）。
type confirmState struct {
	open bool
	path string
}

func (s *State) backup(p panel.Settings) backupAPI {
	if s.api != nil {
		return s.api
	}
	return p.Env.Svc.Backup
}

func (s *State) toast(n env.Notice) { s.toasts = append(s.toasts, n) }

// View 构建面板：space-y-6 里的一个 rounded-lg 块（说明、两个按钮、上次备份时间），最后是确认弹窗。
// 原版读 store 的 config（生效的配置），这里同样读 Env.Config()。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	Background(c, s, p)

	cfg := p.Env.Config()
	disabled := s.restoring || s.exporting
	ui.Column(c.Key("full-backup")).FillWidth().Children(func() {
		// mb-6 > space-y-2 text-sm
		ui.Column(c).FillWidth().Margin(0, 0, 24, 0).Gap(8).Children(func() {
			components.Text(c, locales.SettingsFullDataBackupHint(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand300))
			// text-error-600 dark:text-error-400，前面是 font-medium 的“注意：”
			warn := t.Pick(theme.Error600, theme.Error400)
			d := components.LineOverflow(ui.Span{
				Text:   locales.SettingsFullDataBackupWarningNote() + locales.SettingsFullDataBackupWarningHint(),
				Font:   theme.UIFont,
				Size:   theme.TextSM.Size,
				Weight: theme.WeightNormal,
			}, theme.TextSM.Line)
			rt := ui.RichText(c,
				ui.Span{Text: locales.SettingsFullDataBackupWarningNote(), Weight: theme.WeightMedium},
				ui.Span{Text: locales.SettingsFullDataBackupWarningHint()},
			).FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line).FontWeight(theme.WeightNormal).TextColor(warn).Margin(-d, 0, -d, 0)
			if theme.UIFont != "" {
				rt.Font(theme.UIFont)
			}
		})
		// flex flex-col sm:flex-row gap-3（窗口最窄 960，总是 sm）
		ui.Row(c).Gap(12).Children(func() {
			actionButton(c, t, "export", "export", locales.SettingsFullDataBackupExportBtn(), locales.SettingsFullDataBackupExporting(),
				s.exporting, disabled, theme.Brand600, theme.Brand700).OnClick(func() { s.export(p) })
			actionButton(c, t, "import", "import", locales.SettingsFullDataBackupImportBtn(), locales.SettingsFullDataBackupImporting(),
				s.restoring, disabled, theme.Warning600, theme.Warning700).OnClick(func() { s.pickRestore(p) })
		})
		if cfg != nil && cfg.LastFullBackupTime != "" {
			when := cfg.LastFullBackupTime
			if tm, err := time.Parse(time.RFC3339, when); err == nil {
				when = timefmt.DateTime(tm, timefmt.Location(cfg))
			}
			components.TextMargin(c, locales.SettingsFullDataBackupLastBackup()+when, theme.TextXS, theme.WeightNormal,
				t.Pick(theme.Brand500, theme.Brand400), 16, 0, 0, 0)
		}
	})

	path := s.confirm.path
	components.ConfirmModal(c, t, "full-backup-confirm", components.ConfirmProps{
		Open:      &s.confirm.open,
		Title:     locales.SettingsFullDataBackupModalRestoreTitle(),
		Message:   locales.SettingsFullDataBackupModalRestoreMsg(path),
		Danger:    true,
		TopOffset: panel.TopOffset,
		OnConfirm: func() { s.restore(p, path) },
	})
}

// actionButton：glass-btn-neutral px-6 py-3 text-white rounded-md transition-colors disabled:opacity-50 flex items-center
// justify-center gap-2，进行中最前面是 i-mdi-loading animate-spin（1em），然后是 text-xl 的图标与文字（继承 16px/24px）。
func actionButton(c *ui.Context, t *theme.Theme, key, icon, label, busyLabel string, busy, disabled bool, bg, hover ui.Color) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(12, 24).Radius(6).Gap(8).Center().FocusRing(false)
	g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{Bg: bg, Hover: hover, Text: theme.White})
	bg = g.Bg
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
	if busy {
		label = busyLabel
	}
	btn.Children(func() {
		if busy {
			components.Spinner(c, 16, g.Text)
		}
		components.Icon(c, icon, 20, g.Text)
		components.Text(c, label, theme.TextBase, theme.WeightNormal, g.Text).NoWrap()
	})
	return btn
}

// Background 显示后台结果的提示，并在安排恢复 1.5 秒后退出。原版的面板第一次展开后一直挂载，提示是全局的，
// setTimeout 也不随面板卸载取消；这里的面板在分区收起、离开设置页后不再构建，所以设置页每一帧都调用它（View 也调用）。
func Background(c *ui.Context, s *State, p panel.Settings) {
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil
	s.tickQuit(c, p)
}

// tickQuit 在安排恢复 1.5 秒后退出（帧时钟：安排时记下标记，下一次构建取 c.Now()）。
func (s *State) tickQuit(c *ui.Context, p panel.Settings) {
	if !s.quitPending {
		return
	}
	now := c.Now()
	if s.quitAt.IsZero() {
		s.quitAt = now.Add(quitDelay)
	}
	if wait := s.quitAt.Sub(now); wait > 0 {
		c.After(wait)
		return
	}
	s.quitPending, s.quitAt = false, time.Time{}
	if s.quit != nil {
		s.quit()
	} else {
		p.Env.Svc.AppConfig.SafeQuit()
	}
}

// export 对应 handleCreateFullBackup：选择保存位置（取消时什么都不做），再打包。两步都在后台执行。
func (s *State) export(p panel.Settings) {
	if s.exporting || s.picking {
		return
	}
	s.picking = true
	api := s.backup(p)
	p.Env.Async(func() func() {
		path, err := api.SelectBackupSavePath()
		return func() {
			s.picking = false
			if err != nil {
				s.toast(env.Error(locales.SettingsFullDataBackupToastExportFailed(err.Error())))
				return
			}
			if path == "" {
				return
			}
			s.exporting = true
			p.Env.Async(func() func() {
				err := api.CreateFullDataBackup(path)
				return func() {
					s.exporting = false
					if err != nil {
						s.toast(env.Error(locales.SettingsFullDataBackupToastExportFailed(err.Error())))
						return
					}
					s.toast(env.Success(locales.SettingsFullDataBackupToastExportSuccess(path)))
				}
			})
		}
	})
}

// pickRestore 对应 handleRestoreFullBackup：选择备份文件后弹出确认。
func (s *State) pickRestore(p panel.Settings) {
	if s.restoring || s.picking {
		return
	}
	s.picking = true
	api := s.backup(p)
	p.Env.Async(func() func() {
		path, err := api.SelectBackupRestorePath()
		return func() {
			s.picking = false
			if err != nil {
				s.toast(env.Error(locales.SettingsFullDataBackupToastSelectFileFailed(err.Error())))
				return
			}
			if path == "" {
				return
			}
			s.confirm = confirmState{open: true, path: path}
		}
	})
}

// restore 是确认后的 onConfirm：ScheduleFullDataRestore 只检查文件并改配置，在主线程调用。
func (s *State) restore(p panel.Settings, path string) {
	s.restoring = true
	if err := s.backup(p).ScheduleFullDataRestore(path); err != nil {
		s.toast(env.Error(locales.SettingsFullDataBackupToastRestoreScheduleFailed(err.Error())))
		s.restoring = false
		return
	}
	s.toast(env.Success(locales.SettingsFullDataBackupToastRestoreScheduled()))
	s.quitPending, s.quitAt = true, time.Time{}
}
