// Package portable 对应原版 components/panel/PortableSetupPanel.tsx（设置页“本地集成设置”分区，便携版与 AppImage 才有）：
// lunabox:// 协议处理器与 lunacli 的状态，注册、取消注册。注册写的是当前用户的注册表与 PATH，指向当前运行的可执行文件。
package portable

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// monoFont 是 code（font-mono）在 Windows 的 Chromium 里实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// setupAPI 是面板用到的 PortableSetupService 方法（测试里替换会写注册表的调用）。
type setupAPI interface {
	GetStatus() (service.PortableSetupStatus, error)
	RegisterProtocol() (service.PortableSetupStatus, error)
	UnregisterProtocol() (service.PortableSetupStatus, error)
	RegisterCLIPath() (service.PortableSetupStatus, error)
	UnregisterCLIPath() (service.PortableSetupStatus, error)
}

// actionKey 对应原版的 ActionKey。
type actionKey string

const (
	actionRegisterProtocol   actionKey = "registerProtocol"
	actionUnregisterProtocol actionKey = "unregisterProtocol"
	actionRegisterCLI        actionKey = "registerCli"
	actionUnregisterCLI      actionKey = "unregisterCli"
)

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	status  *service.PortableSetupStatus
	busy    actionKey
	loading bool
	gen     int // 最近一次读取状态的编号，丢弃过期结果

	toasts []env.Notice // 后台结果里待显示的提示（apply 拿不到 Context）

	api setupAPI // 为空时用 Env.Svc.Portable
}

func (s *State) setup(p panel.Settings) setupAPI {
	if s.api != nil {
		return s.api
	}
	return p.Env.Svc.Portable
}

// View 构建面板：读取中是 flex items-center gap-2 text-sm 的旋转图标与文字，否则 space-y-6 的说明、协议与 lunacli 两组。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil

	root := ui.Column(c.Key("portable")).FillWidth().Gap(24)
	// 原版每次挂载（进入设置页、第一次展开分区）都重新读取状态；分区收起后这里不再构建内容，再次展开时同样重新读取。
	if mounted := ui.Local(root, "mounted", func() bool { return false }); !*mounted {
		*mounted = true
		s.load(p)
	}
	muted := t.Pick(theme.Brand500, theme.Brand400)
	root.Children(func() {
		st := s.status
		if s.loading || st == nil {
			ui.Row(c.Key("loading")).Gap(8).AlignItems(ui.Center).Children(func() {
				components.Spinner(c, 16, muted)
				components.Text(c, locales.SettingsPortableSetupLoading(), theme.TextSM, theme.WeightNormal, muted)
			})
			return
		}
		isMac := st.Platform == "darwin"
		pick := func(win, mac func() string) string {
			if isMac {
				return mac()
			}
			return win()
		}
		components.Text(c, pick(locales.SettingsPortableSetupDescription, locales.SettingsPortableSetupDescriptionMac),
			theme.TextXS, theme.WeightNormal, muted)

		// 协议处理器
		ui.Column(c.Key("protocol")).FillWidth().Gap(8).Children(func() {
			panel.FieldLabel(c, t, locales.SettingsPortableSetupProtocolTitle())
			panel.FieldHint(c, t, pick(locales.SettingsPortableSetupProtocolHint, locales.SettingsPortableSetupProtocolHintMac))
			text, tone := protocolStatus(st.Protocol)
			statusLine(c, t, locales.SettingsPortableSetupStatusLabel(), text, tone)
			if st.Protocol.Registered && st.Protocol.RegisteredPath != "" {
				detailLine(c, t, "registered-path", locales.SettingsPortableSetupRegisteredPathLabel(), st.Protocol.RegisteredPath)
			}
			current := st.ExecutablePath
			if current == "" {
				current = st.Protocol.CurrentPath
			}
			if current == "" {
				current = "-"
			}
			detailLine(c, t, "current-path", locales.SettingsPortableSetupCurrentPathLabel(), current)
			if !isMac {
				buttons(c, func() {
					label := locales.SettingsPortableSetupRegisterProtocol()
					if st.Protocol.Registered && !st.Protocol.UpToDate {
						label = locales.SettingsPortableSetupReregisterProtocol()
					}
					better.Button(c, t, "register-protocol", better.ButtonProps{
						Variant: better.ButtonPrimary, Icon: "link-variant", Loading: s.busy == actionRegisterProtocol, Label: label,
					}).OnClick(func() {
						s.run(p, actionRegisterProtocol, s.setup(p).RegisterProtocol,
							locales.SettingsPortableSetupToastProtocolRegistered(), locales.SettingsPortableSetupToastProtocolRegisterFailed)
					})
					if st.Protocol.Registered {
						better.Button(c, t, "unregister-protocol", better.ButtonProps{
							Variant: better.ButtonSecondary, Icon: "link-variant-off", Loading: s.busy == actionUnregisterProtocol,
							Label: locales.SettingsPortableSetupUnregisterProtocol(),
						}).OnClick(func() {
							s.run(p, actionUnregisterProtocol, s.setup(p).UnregisterProtocol,
								locales.SettingsPortableSetupToastProtocolUnregistered(), locales.SettingsPortableSetupToastProtocolUnregisterFailed)
						})
					}
				})
			}
		})

		// lunacli
		ui.Column(c.Key("cli")).FillWidth().Gap(8).Children(func() {
			panel.FieldLabel(c, t, locales.SettingsPortableSetupCliTitle())
			panel.FieldHint(c, t, pick(locales.SettingsPortableSetupCliHint, locales.SettingsPortableSetupCliHintMac))
			text, tone := cliStatus(st.CLI)
			statusLine(c, t, locales.SettingsPortableSetupStatusLabel(), text, tone)
			cliPath := st.CLI.CLIPath
			if cliPath == "" {
				cliPath = "-"
			}
			detailLine(c, t, "cli-path", locales.SettingsPortableSetupCliPathLabel(), cliPath)
			if st.CLI.InstallPath != "" {
				detailLine(c, t, "install-path", locales.SettingsPortableSetupCliInstallPathLabel(), st.CLI.InstallPath)
			}
			if !st.CLI.Available {
				// text-xs text-amber-600 dark:text-amber-400
				components.Text(c, pick(locales.SettingsPortableSetupCliMissingHint, locales.SettingsPortableSetupCliMissingHintMac),
					theme.TextXS, theme.WeightNormal, t.Pick(theme.Warning600, theme.Warning400))
			}
			buttons(c, func() {
				label := locales.SettingsPortableSetupRegisterCli()
				if st.CLI.Registered {
					label = locales.SettingsPortableSetupReregisterCli()
				}
				better.Button(c, t, "register-cli", better.ButtonProps{
					Variant: better.ButtonPrimary, Icon: "console-line", Loading: s.busy == actionRegisterCLI,
					Disabled: !st.CLI.Available, Label: label,
				}).OnClick(func() {
					s.run(p, actionRegisterCLI, s.setup(p).RegisterCLIPath,
						locales.SettingsPortableSetupToastCliRegistered(), locales.SettingsPortableSetupToastCliRegisterFailed)
				})
				if st.CLI.Registered {
					better.Button(c, t, "unregister-cli", better.ButtonProps{
						Variant: better.ButtonSecondary, Icon: "console-line", Loading: s.busy == actionUnregisterCLI,
						Label: locales.SettingsPortableSetupUnregisterCli(),
					}).OnClick(func() {
						s.run(p, actionUnregisterCLI, s.setup(p).UnregisterCLIPath,
							locales.SettingsPortableSetupToastCliUnregistered(), locales.SettingsPortableSetupToastCliUnregisterFailed)
					})
				}
			})
			panel.FieldHint(c, t, pick(locales.SettingsPortableSetupPathReopenHint, locales.SettingsPortableSetupPathReopenHintMac))
		})
	})
}

// load 对应挂载时的 loadStatus：读取期间显示“正在读取状态...”，失败时提示并一直停在读取中（同原版）。
func (s *State) load(p panel.Settings) {
	s.gen++
	s.loading = true
	gen, api := s.gen, s.setup(p)
	p.Env.Async(func() func() {
		next, err := api.GetStatus()
		return func() {
			if gen != s.gen {
				return
			}
			s.loading = false
			if err != nil {
				s.toasts = append(s.toasts, env.Error(locales.SettingsPortableSetupToastLoadFailed(err.Error())))
				return
			}
			s.status = &next
		}
	})
}

// run 对应原版的 run：同一时间只执行一个操作，成功后换成服务返回的状态。注册表与 PATH 的读写在后台执行。
func (s *State) run(p panel.Settings, key actionKey, action func() (service.PortableSetupStatus, error), success string, failure func(string) string) {
	if s.busy != "" {
		return
	}
	s.busy = key
	p.Env.Async(func() func() {
		next, err := action()
		return func() {
			s.busy = ""
			if err != nil {
				s.toasts = append(s.toasts, env.Error(failure(err.Error())))
				return
			}
			s.status = &next
			s.toasts = append(s.toasts, env.Success(success))
		}
	})
}

// buttons：flex flex-wrap gap-2 pt-1。
func buttons(c *ui.Context, content func()) {
	ui.Row(c).FillWidth().Wrap().Gap(8).Padding(4, 0, 0, 0).Children(content)
}

// tone 对应原版的 Tone。
type tone int

const (
	toneOff tone = iota
	toneOK
	toneWarn
)

// protocolStatus 对应 describeProtocolStatus。
func protocolStatus(st service.PortableProtocolStatus) (string, tone) {
	switch {
	case !st.Registered:
		return locales.SettingsPortableSetupStatusNotRegistered(), toneOff
	case !st.UpToDate:
		return locales.SettingsPortableSetupStatusStalePath(), toneWarn
	}
	return locales.SettingsPortableSetupStatusRegistered(), toneOK
}

// cliStatus 对应 describeCLIStatus。
func cliStatus(st service.PortableCLIStatus) (string, tone) {
	switch {
	case !st.Available:
		return locales.SettingsPortableSetupStatusCliMissing(), toneWarn
	case st.Registered:
		return locales.SettingsPortableSetupStatusRegistered(), toneOK
	}
	return locales.SettingsPortableSetupStatusNotRegistered(), toneOff
}

// statusLine 对应 StatusLine：flex flex-wrap items-center gap-2 text-xs，标签 text-brand-500 dark:text-brand-400，
// 徽标 rounded-full px-2 py-0.5 font-medium：ok 是 emerald、warn 是 amber（深色 bg-*-500/15、text-*-300），off 是 brand。
func statusLine(c *ui.Context, t *theme.Theme, label, value string, tn tone) {
	bg, fg := t.Pick(theme.Brand100, theme.Brand700), t.Pick(theme.Brand600, theme.Brand300)
	switch tn {
	case toneOK:
		bg, fg = t.Pick(theme.Success100, theme.Success500.Alpha(0.15)), t.Pick(theme.Success700, theme.Success300)
	case toneWarn:
		bg, fg = t.Pick(theme.Warning100, theme.Warning500.Alpha(0.15)), t.Pick(theme.Warning700, theme.Warning300)
	}
	ui.Row(c.Key("status")).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		components.Text(c, label, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
		ui.Box(c).Padding(2, 8).Radius(999).Background(bg).Children(func() {
			components.Text(c, value, theme.TextXS, theme.WeightMedium, fg).NoWrap()
		})
	})
}

// detailLine 对应 DetailLine：flex flex-wrap items-baseline gap-2 text-xs，标签 text-brand-500 dark:text-brand-400，
// 值是 code：break-all rounded bg-brand-100 px-1.5 py-0.5 text-brand-800 dark:bg-brand-700 dark:text-brand-200
// （flex 项目是块级，高度是行高加 py-0.5；标签下移 2 与它的文字基线对齐）。
func detailLine(c *ui.Context, t *theme.Theme, key, label, value string) {
	ui.Row(c.Key(key)).FillWidth().Wrap().Gap(8).AlignItems(ui.Start).Children(func() {
		components.TextMargin(c, label, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 2, 0, 0, 0).NoWrap()
		ui.Box(c).Shrink(1).MinWidth(0).Padding(2, 6).Radius(4).Background(t.Pick(theme.Brand100, theme.Brand700)).Children(func() {
			components.Text(c, value, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand800, theme.Brand200)).Font(monoFont)
		})
	})
}
