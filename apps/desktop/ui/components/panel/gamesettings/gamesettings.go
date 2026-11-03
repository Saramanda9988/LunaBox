// Package gamesettings 对应原版 components/panel/GameSettingsPanel.tsx（设置页的“游戏设置”分区）：
// 只记录活跃时长、后台静音、进程检测超时，以及启动工具（Windows 是 Locale Emulator 与 Magpie，
// macOS / Linux 是 Wine、CrossOver 与 winetricks / protontricks）。
package gamesettings

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// processDetectionTimeoutSeconds 对应 PROCESS_DETECTION_TIMEOUT_SECONDS。
var processDetectionTimeoutSeconds = []int{60, 120, 180, 300, 600}

// monoFont 是 font-mono 在 Windows 上实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"

// State 是面板跨帧保留的状态，由设置页持有。
type State struct {
	// goos 与 muteSupported 对应 store 的 platformGOOS、backgroundProcessMuteSupported（GetGOOS、
	// SupportsBackgroundProcessMute），第一次构建时读一次。
	probed        bool
	goos          string
	muteSupported bool

	toasts []env.Notice // 文件选择失败的提示（后台结果拿不到 Context），下一次构建时显示
}

// pickFile 对应 SelectGameExecutable（wine 为 false）与 SelectWineRunnerExecutable：打开原生的文件对话框，
// 会阻塞，在 Env.Async 里调用。测试里替换掉，不弹对话框。
var pickFile = func(e *env.Env, wine bool, current string) (string, error) {
	if wine {
		return e.Svc.Game.SelectWineRunnerExecutable(current)
	}
	return e.Svc.Game.SelectGameExecutable(current)
}

// pathField 是可以用对话框选择的路径字段（locale_emulator_path、wine_runner_path 等）。
type pathField struct {
	get     func(cfg *appconf.AppConfig) string
	set     func(cfg *appconf.AppConfig, v string)
	wine    bool          // 用 SelectWineRunnerExecutable
	failed  func() string // 选择失败的提示
	mono    bool          // className="font-mono"
	label   func() string
	holder  func() string
	hint    func() string
	extraFn func(c *ui.Context, t *theme.Theme, p panel.Settings) // 提示之后的内容（默认启用的开关）
}

func (s *State) probe(p panel.Settings) {
	if s.probed {
		return
	}
	s.probed = true
	if p.Env != nil && p.Env.Svc != nil {
		s.goos = p.Env.Svc.Version.GetGOOS()
		s.muteSupported = p.Env.Svc.Version.SupportsBackgroundProcessMute()
	}
}

// View 构建面板。原版的根是片段，各项直接放进分区内容的 space-y-4 列：原版各组的 mt-6 被 space-y-4 盖掉，
// 组与组之间是 16px，组内是 border-t pt-6。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Settings) {
	s.probe(p)
	for _, msg := range s.toasts {
		p.Toast(c, msg)
	}
	s.toasts = nil

	cfg := p.Config
	isDarwin, isLinux := s.goos == "darwin", s.goos == "linux"

	// space-y-2 > flex items-center justify-between gap-4：只记录活跃时长。
	ui.Column(c.Key("record-active")).FillWidth().Gap(8).Children(func() {
		switchRow(c, t, "row", locales.SettingsGameRecordActiveOnly(), locales.SettingsGameRecordActiveOnlyHint(),
			cfg.RecordActiveTimeOnly, func(v bool) {
				p.Change(func(cfg *appconf.AppConfig) { cfg.RecordActiveTimeOnly = v })
			})
	})

	if s.muteSupported {
		// mt-6 border-t border-brand-200 dark:border-brand-700 pt-6：后台静音。
		panel.Divided(c, t, "mute", 24, func() {
			hint := locales.SettingsGameMuteInBackgroundHint()
			if isDarwin {
				hint = locales.SettingsGameMuteInBackgroundHintMacOS()
			}
			switchRow(c, t, "row", locales.SettingsGameMuteInBackground(), hint, cfg.MuteGameInBackground, func(v bool) {
				p.Change(func(cfg *appconf.AppConfig) { cfg.MuteGameInBackground = v })
			})
		})
	}

	panel.Divided(c, t, "process-timeout", 24, func() {
		timeout := cfg.ProcessDetectionTimeoutSec
		if timeout == 0 {
			timeout = 60
		}
		panel.Field(c, t, "field", locales.SettingsGameProcessDetectionTimeout(), locales.SettingsGameProcessDetectionTimeoutHint(), func() {
			options := make([]better.SelectOption, 0, len(processDetectionTimeoutSeconds))
			for _, sec := range processDetectionTimeoutSeconds {
				options = append(options, better.SelectOption{
					Value: strconv.Itoa(sec),
					Label: locales.SettingsGameProcessDetectionTimeoutMinutes(strconv.Itoa(sec / 60)),
				})
			}
			better.Select(c, t, "select", better.SelectProps{
				Value:   strconv.Itoa(timeout),
				Options: options,
				OnChange: func(v string) {
					n, err := strconv.Atoi(v)
					if err != nil {
						return
					}
					p.Change(func(cfg *appconf.AppConfig) { cfg.ProcessDetectionTimeoutSec = n })
				},
			})
		})
		warningBox(c, t)
	})

	// 启动工具：mb-4 block text-sm font-semibold 的标题，下面 space-y-4。
	panel.Divided(c, t, "launch-tools", 24, func() {
		ui.Column(c.Key("title")).Padding(0, 0, 16, 0).Children(func() { // mb-4（Text 的外边距用来压行高，不能覆盖）
			panel.GroupTitle(c, t, locales.SettingsGameLaunchTools())
		})
		ui.Column(c.Key("tools")).FillWidth().Gap(16).Children(func() {
			if isDarwin || isLinux {
				s.wineTools(c, t, p, isDarwin, isLinux)
			} else {
				s.windowsTools(c, t, p)
			}
		})
	})
}

// warningBox 对应 mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-700 dark:bg-amber-900/20：
// flex items-start gap-2，i-mdi-alert mt-0.5 text-lg text-amber-600 dark:text-amber-400，右侧 text-xs
// text-amber-700 dark:text-amber-300 的标题（mb-1 font-medium）与说明。
func warningBox(c *ui.Context, t *theme.Theme) {
	ui.Row(c.Key("warning")).FillWidth().Margin(16, 0, 0, 0).Padding(12).Gap(8).AlignItems(ui.Start).Radius(8).
		Background(t.Pick(theme.Warning50, theme.Warning900.Alpha(0.2))).
		Border(t.HL, t.Pick(theme.Warning200, theme.Warning700)).Children(func() {
		components.Icon(c, "alert", 18, t.Pick(theme.Warning600, theme.Warning400)).Margin(2, 0, 0, 0)
		color := t.Pick(theme.Warning700, theme.Warning300)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.TextMargin(c, locales.SettingsGameWarningTitle(), theme.TextXS, theme.WeightMedium, color, 0, 0, 4, 0)
			components.Text(c, locales.SettingsGameProcessDetectionTimeoutWarning(), theme.TextXS, theme.WeightNormal, color)
		})
	})
}

// windowsTools 是 Windows 的启动工具：Locale Emulator 与 Magpie，各自的提示下面是 pt-2 的“新游戏默认启用”开关。
func (s *State) windowsTools(c *ui.Context, t *theme.Theme, p panel.Settings) {
	cfg := p.Config
	s.pathInput(c, t, p, "le", pathField{
		get:    func(cfg *appconf.AppConfig) string { return cfg.LocaleEmulatorPath },
		set:    func(cfg *appconf.AppConfig, v string) { cfg.LocaleEmulatorPath = v },
		failed: locales.SettingsGameToastLeSelectFailed,
		label:  locales.SettingsGameLePath,
		holder: locales.SettingsGameLePathPlaceholder,
		hint:   locales.SettingsGameLePathHint,
		extraFn: func(c *ui.Context, t *theme.Theme, p panel.Settings) {
			defaultSwitch(c, t, "default-le", locales.SettingsGameDefaultUseLe(), cfg.DefaultUseLocaleEmulator, func(v bool) {
				p.Change(func(cfg *appconf.AppConfig) { cfg.DefaultUseLocaleEmulator = v })
			})
		},
	})
	s.pathInput(c, t, p, "magpie", pathField{
		get:    func(cfg *appconf.AppConfig) string { return cfg.MagpiePath },
		set:    func(cfg *appconf.AppConfig, v string) { cfg.MagpiePath = v },
		failed: locales.SettingsGameToastMagpieSelectFailed,
		label:  locales.SettingsGameMagpiePath,
		holder: locales.SettingsGameMagpiePathPlaceholder,
		hint:   locales.SettingsGameMagpiePathHint,
		extraFn: func(c *ui.Context, t *theme.Theme, p panel.Settings) {
			defaultSwitch(c, t, "default-magpie", locales.SettingsGameDefaultUseMagpie(), cfg.DefaultUseMagpie, func(v bool) {
				p.Change(func(cfg *appconf.AppConfig) { cfg.DefaultUseMagpie = v })
			})
		},
	})
}

// wineTools 是 macOS / Linux 的启动工具：space-y-5 的列，先是 space-y-4 的 Wine 路径与 WINEPREFIX；Linux 再加
// border-t pt-5 的 winetricks / protontricks（标题 mb-1，被 space-y-4 盖掉），macOS 再加 border-t pt-5 的 CrossOver。
func (s *State) wineTools(c *ui.Context, t *theme.Theme, p panel.Settings, isDarwin, isLinux bool) {
	cfg := p.Config
	ui.Column(c.Key("wine")).FillWidth().Gap(20).Children(func() {
		ui.Column(c.Key("runner")).FillWidth().Gap(16).Children(func() {
			s.pathInput(c, t, p, "wine-runner", pathField{
				get:    func(cfg *appconf.AppConfig) string { return cfg.WineRunnerPath },
				set:    func(cfg *appconf.AppConfig, v string) { cfg.WineRunnerPath = v },
				wine:   true,
				failed: locales.SettingsGameToastWineSelectFailed,
				mono:   true,
				label:  locales.SettingsGameWineRunnerPath,
				holder: locales.SettingsGameWineRunnerPathPlaceholder,
				hint:   locales.SettingsGameWineRunnerPathHint,
			})
			textInput(c, t, "wine-prefix", locales.SettingsGameWinePrefix(), locales.SettingsGameWinePrefixPlaceholder(),
				locales.SettingsGameWinePrefixHint(), cfg.WinePrefix, func(v string) {
					p.Change(func(cfg *appconf.AppConfig) { cfg.WinePrefix = v })
				})
		})
		if isLinux {
			panel.Divided(c, t, "tricks", 20, func() {
				ui.Column(c.Key("list")).FillWidth().Gap(16).Children(func() {
					panel.GroupTitle(c, t, locales.SettingsGameTricksTools())
					s.pathInput(c, t, p, "winetricks", pathField{
						get:    func(cfg *appconf.AppConfig) string { return cfg.WinetricksPath },
						set:    func(cfg *appconf.AppConfig, v string) { cfg.WinetricksPath = v },
						wine:   true,
						failed: locales.SettingsGameToastWineSelectFailed,
						mono:   true,
						label:  locales.SettingsGameWinetricksPath,
						holder: locales.SettingsGameWinetricksPathPlaceholder,
						hint:   locales.SettingsGameWinetricksPathHint,
					})
					s.pathInput(c, t, p, "protontricks", pathField{
						get:    func(cfg *appconf.AppConfig) string { return cfg.ProtontricksPath },
						set:    func(cfg *appconf.AppConfig, v string) { cfg.ProtontricksPath = v },
						wine:   true,
						failed: locales.SettingsGameToastWineSelectFailed,
						mono:   true,
						label:  locales.SettingsGameProtontricksPath,
						holder: locales.SettingsGameProtontricksPathPlaceholder,
						hint:   locales.SettingsGameProtontricksPathHint,
					})
				})
			})
		}
		if isDarwin {
			panel.Divided(c, t, "crossover", 20, func() {
				ui.Column(c.Key("list")).FillWidth().Gap(16).Children(func() {
					s.pathInput(c, t, p, "crossover-runner", pathField{
						get:    func(cfg *appconf.AppConfig) string { return cfg.CrossOverRunnerPath },
						set:    func(cfg *appconf.AppConfig, v string) { cfg.CrossOverRunnerPath = v },
						wine:   true,
						failed: locales.SettingsGameToastWineSelectFailed,
						mono:   true,
						label:  locales.SettingsGameCrossoverRunnerPath,
						holder: locales.SettingsGameCrossoverRunnerPathPlaceholder,
						hint:   locales.SettingsGameCrossoverRunnerPathHint,
					})
					textInput(c, t, "crossover-bottle", locales.SettingsGameCrossoverBottle(), locales.SettingsGameCrossoverBottlePlaceholder(),
						locales.SettingsGameCrossoverBottleHint(), cfg.CrossOverBottle, func(v string) {
							p.Change(func(cfg *appconf.AppConfig) { cfg.CrossOverBottle = v })
						})
				})
			})
		}
	})
}

// pathInput 是 space-y-2 的 label、BetterActionInput（右侧 i-mdi-file-search-outline 的“选择”按钮）与提示。
func (s *State) pathInput(c *ui.Context, t *theme.Theme, p panel.Settings, key string, f pathField) {
	label := f.label()
	ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
		panel.FieldLabel(c, t, label)
		v := f.get(p.Config)
		in := better.ActionInput(c, t, "input", &v, better.InputProps{Placeholder: f.holder()},
			better.InputAction{Icon: "file-search-outline", Label: locales.SettingsGameSelectBtn(), OnClick: func() { s.selectPath(p, f) }})
		if f.mono {
			in.Font(monoFont)
		}
		if in.Changed() {
			p.Change(func(cfg *appconf.AppConfig) { f.set(cfg, v) })
		}
		panel.FieldHint(c, t, f.hint())
		if f.extraFn != nil {
			f.extraFn(c, t, p)
		}
	})
}

// selectPath 对应 handleSelectLocaleEmulatorPath、handleSelectMagpiePath、handleSelectCompatibilityRunnerPath：
// 以当前路径为起点选择文件，选了就写进草稿，失败时提示。
func (s *State) selectPath(p panel.Settings, f pathField) {
	e, current := p.Env, f.get(p.Config)
	e.Async(func() func() {
		path, err := pickFile(e, f.wine, current)
		return func() {
			switch {
			case err != nil:
				s.toasts = append(s.toasts, env.Error(f.failed()))
			case path != "":
				p.Change(func(cfg *appconf.AppConfig) { f.set(cfg, path) })
			}
		}
	})
}

// textInput 是 space-y-2 的 label、BetterInput（font-mono）与提示（WINEPREFIX、CrossOver bottle）。
func textInput(c *ui.Context, t *theme.Theme, key, label, placeholder, hint, value string, onChange func(string)) {
	ui.Column(c.Key(key)).FillWidth().Gap(8).Children(func() {
		panel.FieldLabel(c, t, label)
		v := value
		if better.Input(c, t, "input", &v, better.InputProps{Placeholder: placeholder}).Font(monoFont).Changed() {
			onChange(v)
		}
		panel.FieldHint(c, t, hint)
	})
}

// switchRow 对应 flex items-center justify-between gap-4：左侧 flex-1 space-y-2 的 label 与提示，右侧 BetterSwitch。
// 原版的 label 没有 htmlFor，点文字不切换开关。
func switchRow(c *ui.Context, t *theme.Theme, key, label, hint string, checked bool, onChange func(bool)) {
	ui.Row(c.Key(key)).FillWidth().Gap(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(8).Children(func() {
			panel.FieldLabel(c, t, label)
			if hint != "" {
				panel.FieldHint(c, t, hint)
			}
		})
		better.Switch(c, t, "switch", checked, false, onChange)
	})
}

// defaultSwitch 是路径提示下面 pt-2 的“新游戏默认启用”开关行（左侧只有 label）。
func defaultSwitch(c *ui.Context, t *theme.Theme, key, label string, checked bool, onChange func(bool)) {
	ui.Column(c.Key(key)).FillWidth().Padding(8, 0, 0, 0).Children(func() {
		switchRow(c, t, "row", label, "", checked, onChange)
	})
}
