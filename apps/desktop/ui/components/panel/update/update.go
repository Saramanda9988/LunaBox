// Package update 对应原版 components/panel/UpdateSettingsPanel.tsx（设置页“软件更新”分区）：启动时自动检查的开关、
// 手动检查更新（结果、错误与新版本提示）以及有更新时弹出的 UpdateDialog（components.UpdateDialog）。
package update

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/service"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/panel"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// uno.config.ts 的 accent（星光蓝），theme 包里没有。
var (
	accent50  = ui.Hex("#eff6ff")
	accent100 = ui.Hex("#dbeafe")
	accent200 = ui.Hex("#bfdbfe")
	accent300 = ui.Hex("#93c5fd")
	accent400 = ui.Hex("#60a5fa")
	accent500 = ui.Hex("#3b82f6")
	accent600 = ui.Hex("#2563eb")
	accent700 = ui.Hex("#1d4ed8")
	accent900 = ui.Hex("#1e3a8a")
)

// monoFont 是 font-mono 在 Windows 的 Chromium 里实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// updateAPI 是面板用到的 UpdateService 方法（测试里替换联网的检查）。
type updateAPI interface {
	CheckForUpdates() (*service.UpdateCheckResult, error)
	SkipVersion(ver string) error
	DownloadAndApplyUpdate(manifestURL string) (*service.UpdateApplyResult, error)
}

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	checking   bool                       // isChecking
	info       *service.UpdateCheckResult // updateInfo
	showDialog bool
	err        string // error
	gen        int    // 最近一次检查的编号，丢弃过期结果

	dialog components.UpdateDialogState
	toasts []env.Notice // 待显示的提示

	api updateAPI // 为空时用 Env.Svc.Update
}

func (s *State) update(p panel.Settings) updateAPI {
	if s.api != nil {
		return s.api
	}
	return p.Env.Svc.Update
}

// View 构建面板：space-y-4 的开关行、pt-2 的手动检查按钮、错误、新版本或已是最新的提示，最后是更新对话框。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil

	ui.Column(c.Key("update")).FillWidth().Gap(16).Children(func() {
		// space-y-2 里的一行，结构同 SettingSwitchRow（label htmlFor：点击标题切换）。
		better.SettingSwitchRow(c, t, "check-on-startup", better.SettingSwitchRowProps{
			Label:   locales.SettingsUpdateAutoCheckLabel(),
			Hint:    locales.SettingsUpdateAutoCheckHint(),
			Checked: p.Config.CheckUpdateOnStartup,
			OnChange: func(v bool) {
				p.Change(func(cfg *appconf.AppConfig) { cfg.CheckUpdateOnStartup = v })
			},
		})
		ui.Column(c.Key("check")).FillWidth().Padding(8, 0, 0, 0).Children(func() {
			label := locales.SettingsUpdateManualCheckBtn()
			if s.checking {
				label = locales.SettingsUpdateChecking()
			}
			better.Button(c, t, "check-update", better.ButtonProps{
				Variant: better.ButtonPrimary, Size: better.ButtonLG, Icon: "update", Loading: s.checking, Label: label,
			}).FillWidth().OnClick(func() { s.check(p) })
		})
		if s.err != "" {
			errorBox(c, t, s.err)
		}
		if s.info != nil && s.info.HasUpdate && !s.showDialog && s.err == "" {
			newVersionHint(c, t, s.info.LatestVer).OnClick(func() { s.openDialog() })
		}
		if s.info != nil && !s.info.HasUpdate && s.err == "" {
			upToDate(c, t, s.info.CurrentVer)
		}
	})

	if s.showDialog && s.info != nil {
		info := s.info
		components.UpdateDialog(c, t, "update-dialog", &s.dialog, components.UpdateDialogProps{
			Info:      info,
			TopOffset: panel.TopOffset,
			OnClose:   func() { s.showDialog = false },
			OnSkip:    func(ver string) { s.skip(p, ver) },
			OnInAppUpdate: func(manifestURL string) {
				api := s.update(p)
				s.dialog.StartInAppUpdate(p.Env.Async, func() (*service.UpdateApplyResult, error) {
					return api.DownloadAndApplyUpdate(manifestURL)
				})
			},
		})
	}
}

func (s *State) openDialog() {
	s.showDialog = true
	s.dialog = components.UpdateDialogState{}
}

// check 对应 handleCheckUpdate：CheckForUpdates 要联网，在后台执行；有更新时弹出对话框。
func (s *State) check(p panel.Settings) {
	if s.checking {
		return
	}
	s.checking, s.err, s.info = true, "", nil
	s.gen++
	gen, api := s.gen, s.update(p)
	p.Env.Async(func() func() {
		result, err := api.CheckForUpdates()
		return func() {
			if gen != s.gen {
				return
			}
			s.checking = false
			if err != nil {
				s.err = err.Error()
				if s.err == "" {
					s.err = locales.SettingsUpdateCheckFailed()
				}
				return
			}
			if result != nil {
				s.info = result
				if result.HasUpdate {
					s.openDialog()
				}
			}
		}
	})
}

// skip 对应 handleSkipVersion：SkipVersion 只是保存配置，在主线程调用（同设置页保存草稿）。
func (s *State) skip(p panel.Settings, ver string) {
	if err := s.update(p).SkipVersion(ver); err != nil {
		s.toasts = append(s.toasts, env.Error(locales.SettingsUpdateSkipFailed(err.Error())))
		return
	}
	if s.info != nil {
		info := *s.info
		info.HasUpdate = false
		s.info = &info
	}
	s.showDialog = false
}

// errorBox：p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 rounded-lg，flex items-start gap-2 的
// i-mdi-alert-circle（text-red-600 dark:text-red-400 text-lg mt-0.5）与 text-xs text-red-700 dark:text-red-300 的标题
// （font-medium）、错误（mt-1）。
func errorBox(c *ui.Context, t *theme.Theme, msg string) {
	fg := t.Pick(theme.Error700, theme.Error300)
	ui.Row(c.Key("error")).FillWidth().Padding(12).Radius(8).Gap(8).AlignItems(ui.Start).
		Background(t.Pick(theme.Error50, theme.Error900.Alpha(0.2))).Border(t.HL, t.Pick(theme.Error200, theme.Error700)).Children(func() {
		components.Icon(c, "alert-circle", 18, t.Pick(theme.Error600, theme.Error400)).Margin(2, 0, 0, 0)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, locales.SettingsUpdateCheckFailedTitle(), theme.TextXS, theme.WeightMedium, fg)
			components.TextMargin(c, msg, theme.TextXS, theme.WeightNormal, fg, 4, 0, 0, 0)
		})
	})
}

// newVersionHint 是对话框关闭后仍有更新时的提示按钮：w-full p-3 bg-accent-50 dark:bg-accent-900/20 border border-accent-200
// dark:border-accent-700 rounded-lg hover:bg-accent-100 dark:hover:bg-accent-900/30 transition-colors text-left；
// i-mdi-update text-xl，flex-1 text-sm 的“发现新版本”（font-medium）与 ml-2 font-mono font-semibold 的版本号，
// 右侧 i-mdi-chevron-right text-lg。
func newVersionHint(c *ui.Context, t *theme.Theme, latest string) ui.Element {
	btn := ui.ButtonBase(c.Key("new-version")).FillWidth().Padding(12).Radius(8).Gap(8).AlignItems(ui.Center).FocusRing(false).
		Border(t.HL, t.Pick(accent200, accent700))
	bg := t.Pick(accent50, accent900.Alpha(0.2))
	if btn.Hovered() {
		bg = t.Pick(accent100, accent900.Alpha(0.3))
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	fg := t.Pick(accent700, accent300)
	btn.Children(func() {
		components.Icon(c, "update", 20, t.Pick(accent600, accent400))
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			components.Text(c, locales.SettingsUpdateNewVersion(), theme.TextSM, theme.WeightMedium, fg).NoWrap()
			components.Text(c, "v"+latest, theme.TextSM, theme.WeightSemibold, fg).Font(monoFont).NoWrap()
		})
		components.Icon(c, "chevron-right", 18, t.Pick(accent500, accent400))
	})
	return btn
}

// upToDate：p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-700 rounded-lg，flex items-center
// gap-2 的 i-mdi-check-circle（text-xl text-green-600 dark:text-green-400）与 text-sm text-green-700 dark:text-green-300 的
// “已是最新版本”（font-medium）、ml-2 font-mono 的当前版本。
func upToDate(c *ui.Context, t *theme.Theme, current string) {
	fg := t.Pick(theme.Green700, theme.Green300)
	ui.Row(c.Key("up-to-date")).FillWidth().Padding(12).Radius(8).Gap(8).AlignItems(ui.Center).
		Background(t.Pick(theme.Green50, theme.Green900.Alpha(0.2))).Border(t.HL, t.Pick(theme.Green200, theme.Green700)).Children(func() {
		components.Icon(c, "check-circle", 20, t.Pick(theme.Green600, theme.Green400))
		ui.Row(c).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			components.Text(c, locales.SettingsUpdateUpToDate(), theme.TextSM, theme.WeightMedium, fg).NoWrap()
			components.Text(c, current, theme.TextSM, theme.WeightNormal, fg).Font(monoFont).NoWrap()
		})
	})
}
