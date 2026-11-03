// Package launch 对应原版 components/panel/GameLaunchPanel.tsx（详情页“启动配置”标签页），它在 routes/game.tsx 里
// 接收的回调（默认启动方式、Steam 设置、进程选择与 ProcessSelectModal、导出快捷方式），
// 以及详情页的启动游戏（handleStartGame、Steam 导入流程与 SteamImportModal，store 的 startGame、gameRuntimes）。
// 界面随平台（GetGOOS）变化：Windows 有管理员启动、导出快捷方式与启动增强工具；macOS 有兼容层启动（Wine、CrossOver）；
// Linux 有 Wine / Proton、Steam 的 Proton 设置与兼容层快捷工具。
package launch

import (
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// State 是面板跨帧保留的状态，由详情页持有；进入另一个游戏时清零。
type State struct {
	gameID string // 后台结果回来时游戏已换就丢弃

	optionsDraft    *string // steamLaunchOptionsDraft：nil 时显示游戏的 steam_launch_options
	toolDraft       *string // pendingCompatibilityTool：nil 表示没有在这里选过 Proton，保存时不改 Steam 里的选择
	saving          bool    // isSteamLaunchOptionsSaving 与 isSteamCompatibilitySaving（总是一起设置）
	applyChecking   bool    // isSteamApplyChecking
	applyConfirm    bool    // isSteamApplyConfirmOpen
	steamRestarting bool    // isSteamRestarting

	// 以下只在 Linux 上读取。key 是上次读取时的依赖（原版 useEffect 的依赖数组），seq 丢弃过时的结果。
	compat        *service.SteamCompatibilityInfo // steamCompatibility
	compatLoading bool
	compatErr     string
	compatKey     string
	compatSeq     int
	prefixOpening bool // isSteamProtonPrefixOpening

	protonTools  []service.LocalProtonTool // localProtonTools
	protonLoaded bool

	tools        *service.GameCompatibilityToolsInfo // gameCompatibilityTools
	toolsLoading bool
	toolsErr     string
	toolsKey     string
	toolsSeq     int
	opening      string // openingCompatibilityAction

	process processModal // ProcessSelectModal（原版由详情页持有）
}

// launchModeOptions 对应面板的 launchModeOptions：普通启动，Windows 加管理员启动，macOS 加兼容层启动，
// 支持 Steam 启动时加 Steam。
func (l *Launcher) launchModeOptions(g *models.Game) []better.SelectOption {
	opts := []better.SelectOption{{Label: locales.GameLaunchLaunchModeNormal(), Value: string(enums.LaunchModeNormal)}}
	if l.SupportsAdmin() {
		opts = append(opts, better.SelectOption{Label: locales.GameLaunchLaunchModeAdmin(), Value: string(enums.LaunchModeAdmin)})
	}
	if l.SupportsCompatibility() {
		opts = append(opts, better.SelectOption{Label: locales.GameLaunchLaunchModeCompatibility(), Value: string(enums.LaunchModeCompatibility)})
	}
	if l.SupportsSteam(g) {
		opts = append(opts, better.SelectOption{Label: locales.GameLaunchLaunchModeSteam(), Value: string(enums.LaunchModeSteam)})
	}
	return opts
}

// launchPreset 是启动参数的预设按钮；文字是 gameLaunch.steamLaunchOptionsPresets.<key>。
type launchPreset struct{ key, value string }

// localeLaunchOptionPresets 对应 localeLaunchOptionPresets（也是 wineLaunchOptionPresets）。
var localeLaunchOptionPresets = []launchPreset{
	{"chineseLocale", "LANG=zh_CN.UTF-8 %command%"},
	{"fullChineseLocale", "LC_ALL=zh_CN.UTF-8 LANG=zh_CN.UTF-8 %command%"},
}

// steamLaunchOptionPresets 对应 steamLaunchOptionPresets（Windows 上也显示，与原版一致）。
var steamLaunchOptionPresets = append(localeLaunchOptionPresets[:len(localeLaunchOptionPresets):len(localeLaunchOptionPresets)],
	launchPreset{"protonLog", "PROTON_LOG=1 %command%"})

// compatibilityToolActions 对应 compatibilityToolActions：兼容层快捷工具的按钮。
var compatibilityToolActions = []struct{ action, icon string }{
	{"prefix_dir", "folder-outline"},
	{"drive_c", "folder-open-outline"},
	{"regedit", "database-cog-outline"},
	{"winecfg", "tune-variant"},
	{"explorer", "file-tree-outline"},
	{"winecmd", "console"},
}

var windowsExecutable = regexp.MustCompile(`(?i)\.(?:exe|bat)$`)

// isWindowsExecutablePath 对应 isWindowsExecutablePath。
func isWindowsExecutablePath(path string) bool {
	return windowsExecutable.MatchString(strings.TrimSpace(path))
}

// isProtonRunner 对应 isProtonRunnerValue：proton（自动选择）或 proton:<本机 Proton 的 ID>。
func isProtonRunner(v string) bool {
	r := strings.ToLower(strings.TrimSpace(v))
	return r == "proton" || strings.HasPrefix(r, "proton:")
}

// toolLabel 是兼容工具的显示名：display_name 与 name 不同时是“display_name (name)”。
func toolLabel(name, display string) string {
	switch {
	case display != "" && display != name:
		return display + " (" + name + ")"
	case display != "":
		return display
	}
	return name
}

// steamToolOptions 对应 steamCompatibilityOptions（面板）与 compatibilityToolOptions（SteamImportModal）：
// 第一项是 Steam 的默认工具，后面是 Steam 里装的兼容工具。
func steamToolOptions(info *service.SteamCompatibilityInfo) []better.SelectOption {
	def := locales.GameLaunchSteamProtonAuto()
	if info != nil && info.DefaultTool != "" {
		def = info.DefaultTool
	}
	opts := []better.SelectOption{{Value: "", Label: locales.GameLaunchSteamProtonDefault(def)}}
	if info != nil {
		for _, tool := range info.Tools {
			opts = append(opts, better.SelectOption{Value: tool.Name, Label: toolLabel(tool.Name, tool.DisplayName)})
		}
	}
	return opts
}

// fileName 对应 path.split(/[\\/]/).pop()。
func fileName(path string) string {
	if i := strings.LastIndexAny(path, `\/`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// View 构建面板：space-y-6 的卡片——进程监控、兼容层启动器（Steam 启动或 Wine / Proton 启动时）、
// 启动增强工具（只有 Windows）。
func View(c *ui.Context, t *theme.Theme, s *State, p panel.Game) {
	if p.Game == nil {
		return
	}
	if s.gameID != p.Game.ID {
		*s = State{gameID: p.Game.ID}
	}
	l := For(p.Env)
	l.latest = p
	h := &handlers{s: s, p: p, l: l}
	h.effects()

	g := h.p.Game
	mode := l.EffectiveMode(g.LaunchMode, g)
	steam := mode == enums.LaunchModeSteam
	showSteam := steam && l.SupportsSteam(g)
	showWine := l.SupportsWine() && !steam && (l.goos == "linux" || mode == enums.LaunchModeCompatibility)

	ui.Column(c.Key("launch-panel")).Gap(24).Children(func() {
		h.processCard(c, t)
		if showSteam || showWine {
			h.compatCard(c, t, showSteam, showWine)
		}
		if l.SupportsAdmin() {
			h.enhancementCard(c, t, steam)
		}
	})

	components.ConfirmModal(c, t, "steam-apply", components.ConfirmProps{
		Open:        &s.applyConfirm,
		Title:       locales.GameLaunchSteamApplyConfirmTitle(),
		Message:     locales.GameLaunchSteamApplyConfirmMessage(),
		ConfirmText: locales.GameLaunchSteamApplyConfirmAction(),
		TopOffset:   panel.TopOffset,
		OnConfirm:   func() { h.applySteam(c, true) },
	})
	s.process.view(c, t, l, p)
}

// handlers 是面板与详情页传给它的回调（handleDefaultLaunchModeChange 等）。
type handlers struct {
	s *State
	p panel.Game
	l *Launcher
}

func (h *handlers) toast(c *ui.Context, n env.Notice) { h.p.Env.Toast(c, n) }

// current 报告后台结果回来时面板是否仍是同一个游戏。
func (h *handlers) current(id string) bool { return h.s.gameID == id }

// edit 在页面当前的游戏上做改动（onGameChange），这一遍构建里随后读到的就是新值。
func (h *handlers) edit(fn func(g *models.Game)) {
	id := h.p.Game.ID
	h.l.change(id, fn)
	if g := h.l.latest.Game; g != nil && g.ID == id {
		h.p.Game = g
	}
}

func (h *handlers) linux() bool { return h.l.goos == "linux" }

// wineState 是 Wine / Proton 启动的派生值（面板里的 selectedWineRunner 等）。
type wineState struct {
	defaultsToSystem bool   // defaultsToSystemWineRunner：macOS，或 Linux 上的 Windows 可执行程序
	runner           string // selectedWineRunner
	proton           bool   // isProtonRunner
	prefix           string // effectiveWinePrefixPath
}

func (h *handlers) wine() wineState {
	g, l := h.p.Game, h.l
	darwin, linux := l.goos == "darwin", l.goos == "linux"
	w := wineState{defaultsToSystem: darwin || (linux && isWindowsExecutablePath(g.Path))}
	w.runner = g.WineRunner
	if linux && w.runner == "crossover" { // Linux 没有 CrossOver
		w.runner = ""
	}
	if w.runner == "" && w.defaultsToSystem {
		w.runner = "system"
	}
	w.proton = isProtonRunner(w.runner)
	if w.runner != "crossover" {
		w.prefix = g.WinePrefix
		if cfg := h.p.Env.Config(); w.prefix == "" && cfg != nil {
			w.prefix = cfg.WinePrefix
		}
	}
	return w
}

// shouldLoadTools 对应 shouldLoadCompatibilityTools：Linux 上 Steam 启动或选了兼容层时读取快捷工具。
func (h *handlers) shouldLoadTools() bool {
	g := h.p.Game
	return h.linux() && (h.l.EffectiveMode(g.LaunchMode, g) == enums.LaunchModeSteam || h.wine().runner != "")
}

// effects 对应面板的 useEffect：依赖变化时（重新）读取 Steam 兼容工具、本机 Proton 与快捷工具；Linux 上把
// CrossOver（只有 macOS 有）改回默认兼容层。
func (h *handlers) effects() {
	s, l := h.s, h.l
	if h.linux() {
		g := h.p.Game
		if g.WineRunner == "crossover" {
			runner := ""
			if h.wine().defaultsToSystem {
				runner = "system"
			}
			h.edit(func(pg *models.Game) { pg.WineRunner, pg.WinePrefix = runner, "" })
			g = h.p.Game
		}
		if key := strings.Join([]string{g.ID, g.SourceID, g.SteamLaunchID, g.SteamLaunchKind}, "\x00"); key != s.compatKey {
			s.compatKey = key
			h.loadCompat()
		}
		if !s.protonLoaded {
			s.protonLoaded = true
			id, e := g.ID, h.p.Env
			e.Async(func() func() {
				tools := e.Svc.Integration.GetLocalProtonTools()
				return func() {
					if h.current(id) {
						s.protonTools = tools
					}
				}
			})
		}
	}
	if !h.shouldLoadTools() {
		s.tools, s.toolsErr, s.toolsLoading, s.toolsKey = nil, "", false, ""
		return
	}
	g, cfg := h.p.Game, l.env.Config()
	parts := []string{g.ID, string(g.LaunchMode), g.Path, g.SourceID, g.SteamLaunchID, g.SteamLaunchKind, g.WinePrefix, g.WineRunner}
	if cfg != nil {
		parts = append(parts, cfg.ProtontricksPath, cfg.WinePrefix, cfg.WineRunnerPath, cfg.WinetricksPath)
	}
	if key := strings.Join(parts, "\x00"); key != s.toolsKey {
		s.toolsKey = key
		h.loadTools()
	}
}

// loadCompat 读取游戏在 Steam 里的兼容工具设置（GetGameSteamCompatibility），读取期间显示加载中。
func (h *handlers) loadCompat() {
	s, id, e := h.s, h.p.Game.ID, h.p.Env
	s.compatSeq++
	seq := s.compatSeq
	s.compatLoading, s.compatErr = true, ""
	e.Async(func() func() {
		info, err := e.Svc.Integration.GetGameSteamCompatibility(id)
		return func() {
			if !h.current(id) || s.compatSeq != seq {
				return
			}
			s.compatLoading = false
			if err != nil {
				s.compat, s.compatErr = nil, err.Error()
				return
			}
			s.compat = &info
		}
	})
}

// loadTools 读取兼容层快捷工具（GetGameCompatibilityTools）。
func (h *handlers) loadTools() {
	s, id, e := h.s, h.p.Game.ID, h.p.Env
	s.toolsSeq++
	seq := s.toolsSeq
	s.toolsLoading, s.toolsErr = true, ""
	e.Async(func() func() {
		info, err := e.Svc.Integration.GetGameCompatibilityTools(id)
		return func() {
			if !h.current(id) || s.toolsSeq != seq {
				return
			}
			s.toolsLoading = false
			if err != nil {
				s.tools, s.toolsErr = nil, err.Error()
				return
			}
			s.tools = &info
		}
	})
}

// processCard：进程监控卡片（space-y-6）：默认启动方式、可执行程序、实际游戏进程、导出快捷启动方式（只有 Windows）。
// macOS / Linux 上进程由兼容层托管，进程名只读，没有选择按钮。
func (h *handlers) processCard(c *ui.Context, t *theme.Theme) {
	g, l := h.p.Game, h.l
	wine := l.SupportsWine()
	winW, _ := c.Size()
	card(c, t, "process-card", 24).Children(func() {
		// 标题下的空 <p className="mt-1"> 没有高度，外边距与 space-y-6 合并，不占位置。
		cardTitle(c, t, locales.GameLaunchProcessMonitor(), 0)

		ui.Column(c).Children(func() {
			fieldLabel(c, t, locales.GameLaunchLaunchMode(), 4)
			better.Select(c, t, "launch-mode", better.SelectProps{
				Value:    string(l.EffectiveMode(g.LaunchMode, g)),
				Options:  l.launchModeOptions(g),
				OnChange: func(v string) { h.launchModeChange(c, enums.LaunchMode(v)) },
			})
		})

		// glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-brand-50
		// dark:bg-brand-700 text-brand-900 dark:text-white font-mono break-all text-sm；提示 mt-1 text-xs text-brand-500。
		ui.Column(c).Children(func() {
			fieldLabel(c, t, locales.GameLaunchExecutable(), 4)
			name := locales.GameLaunchNoPathSet()
			if g.Path != "" {
				name = fileName(g.Path)
			}
			bg, border := t.GlassInput(t.Pick(theme.Brand50, theme.Brand700), t.Pick(theme.Brand300, theme.Brand600))
			ui.Column(c.Key("executable")).Padding(8, 12).Radius(6).Border(t.HL, border).
				Background(bg).Children(func() {
				monoText(c, name, theme.TextSM, theme.WeightNormal, t.TextStrong)
			})
			components.TextMargin(c, locales.GameLaunchExecutableHint(),
				theme.TextXS, theme.WeightNormal, theme.Brand500, 4, 0, 0, 0)
		})

		// flex flex-col gap-2 sm:flex-row：BetterActionInput（flex-1）与“选择运行进程”。
		ui.Column(c).Children(func() {
			fieldLabel(c, t, locales.GameLaunchActualProcess(), 4)
			wide := winW >= 640
			var row ui.Element
			if wide {
				row = ui.Row(c.Key("process-row")).Gap(8).AlignItems(ui.Start)
			} else {
				row = ui.Column(c.Key("process-row")).Gap(8)
			}
			row.Children(func() {
				value := g.ProcessName
				placeholder := ""
				var actions []inputAction
				if wine {
					placeholder = locales.GameLaunchProcessPlaceholderDarwin()
				} else {
					actions = append(actions, inputAction{icon: "file-search-outline", label: locales.GameLaunchSelectProcessFile(), onClick: func() { h.selectProcessExecutable(c) }})
				}
				in := actionInput(c, t, "process-name", &value, locales.GameLaunchActualProcess(), placeholder, func(r ui.Element) {
					if wide {
						r.Grow(1).Basis(0)
					} else {
						r.FillWidth()
					}
					if wine { // readOnly disabled（BetterActionInput disabled:opacity-60）
						components.Disable(r, 0.6)
					}
				}, actions...)
				if in.Changed() {
					h.edit(func(pg *models.Game) { pg.ProcessName = value })
				}
				if !wine {
					emptyIconButton(c, t, "select-running-process", locales.GameLaunchSelectRunningProcess()).OnClick(func() { h.s.process.show(c, h.p) })
				}
			})
			hint := locales.GameLaunchProcessHint()
			if wine {
				hint = locales.GameLaunchProcessHintDarwin()
			}
			components.TextMargin(c, hint, theme.TextXS, theme.WeightNormal, theme.Brand500, 4, 0, 0, 0)
		})

		if l.SupportsAdmin() {
			// glass-panel 里 flex flex-col gap-3 md:flex-row md:items-center md:justify-between。
			glassPanel(c, t, "export-shortcut").Children(func() {
				panelRow(c, winW, func(text ui.Element) {
					text.Children(func() {
						components.Text(c, locales.GameLaunchExportShortcut(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200))
						components.TextMargin(c, locales.GameLaunchExportShortcutHint(),
							textXSRelax, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
					})
				}, func() {
					better.Button(c, t, "export-shortcut-button", better.ButtonProps{
						Variant: better.ButtonPrimary, Icon: "link-variant", Label: locales.GameLaunchExportShortcut(),
					}).OnClick(func() { h.exportShortcut(c) })
				})
			})
		}
	})
}

// panelRow 是 glass-panel 里的 flex flex-col gap-3 md:flex-row md:items-center md:justify-between：左侧 min-w-0 的文字，
// 右侧按钮。
func panelRow(c *ui.Context, winW float32, text func(ui.Element), button func()) {
	wide := winW >= 768
	var row ui.Element
	if wide {
		row = ui.Row(c).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween)
	} else {
		row = ui.Column(c).Gap(12)
	}
	row.Children(func() {
		col := ui.Column(c).MinWidth(0)
		if wide {
			col.Grow(1).Basis(0)
		}
		text(col)
		button()
	})
}

// compatCard：兼容层启动器卡片（space-y-5）。标题行 pb-2 flex items-center justify-between gap-3，Steam 启动时右侧是
// ghost sm 的刷新按钮；说明只在 Windows 上显示。下面是 Steam 启动配置或 Wine / Proton 启动配置，Linux 上还有快捷工具。
func (h *handlers) compatCard(c *ui.Context, t *theme.Theme, showSteam, showWine bool) {
	s, linux := h.s, h.linux()
	card(c, t, "compat-card", 20).Children(func() {
		ui.Row(c).Padding(0, 0, 8, 0).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				title := locales.GameLaunchCompatibilityTools()
				if showSteam && !linux {
					title = locales.GameLaunchSteamTools()
				}
				cardTitle(c, t, title, 0)
				if !h.l.SupportsWine() { // Windows 上只有 Steam 启动配置
					components.TextMargin(c, locales.GameLaunchSteamToolsHint(), theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
				}
			})
			if showSteam {
				better.Button(c, t, "steam-refresh", better.ButtonProps{
					Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: "refresh", Loading: s.compatLoading,
					Disabled: s.saving || s.steamRestarting || s.applyChecking,
				}).Label(locales.GameLaunchSteamProtonRefresh()).OnClick(func() { h.refreshSteamSettings() })
			}
		})
		if showSteam {
			h.steamConfig(c, t)
		}
		if showWine {
			h.wineConfig(c, t)
		}
		if linux && h.shouldLoadTools() {
			h.quickTools(c, t)
		}
	})
}

// steamOptionsValue 是输入框里的 Steam 启动参数（steamLaunchOptionsValue）。
func (h *handlers) steamOptionsValue() string {
	if h.s.optionsDraft != nil {
		return *h.s.optionsDraft
	}
	return h.p.Game.SteamLaunchOptions
}

// steamConfig：space-y-3 的 Steam 启动参数（带保存按钮）、提示与预设按钮（flex-wrap gap-2）；Linux 上还有 Proton
// 选择与 Proton Prefix。
func (h *handlers) steamConfig(c *ui.Context, t *theme.Theme) {
	s := h.s
	ui.Column(c.Key("steam-options")).Gap(12).Children(func() {
		fieldLabel(c, t, locales.GameLaunchSteamLaunchOptions(), 0)
		value := h.steamOptionsValue()
		in := actionInput(c, t, "steam-launch-options", &value, locales.GameLaunchSteamLaunchOptions(), locales.GameLaunchSteamLaunchOptionsPlaceholder(), func(r ui.Element) { r.FillWidth() },
			inputAction{
				icon: "content-save-outline", label: locales.GameLaunchSteamLaunchOptionsSave(), loading: s.saving,
				disabled: s.applyChecking || (h.linux() && s.compat == nil), onClick: func() { h.saveSteamSettings(c) },
			})
		if in.Changed() {
			s.optionsDraft = &value
		}
		components.Text(c, locales.GameLaunchSteamLaunchOptionsHint(), textXSRelax, theme.WeightNormal, t.TextMuted)
		ui.Row(c).Wrap().GapX(8).GapY(8).Children(func() {
			for _, preset := range steamLaunchOptionPresets {
				better.Button(c, t, "preset-"+preset.key, better.ButtonProps{
					Size: better.ButtonSM, Icon: "plus-circle-outline", Label: locales.Key("gameLaunch.steamLaunchOptionsPresets." + preset.key),
				}).OnClick(func() {
					v := preset.value
					s.optionsDraft = &v
				})
			}
		})
	})
	if h.linux() {
		h.steamProton(c, t)
		h.steamPrefix(c, t)
	}
}

// compatPending 对应 isSteamCompatibilityPending：还没有读到兼容工具设置，也没有出错。
func (h *handlers) compatPending() bool { return h.s.compat == nil && h.s.compatErr == "" }

// steamProton：space-y-2 的 Proton 选择（flex flex-col gap-2 md:flex-row：BetterSelect min-w-0 flex-1、保存与重启 Steam），
// 未保存提示、读取状态（出错时 text-error-500，深色主题下 dark:text-brand-400 生效）与 Steam 安装目录。
func (h *handlers) steamProton(c *ui.Context, t *theme.Theme) {
	s, info := h.s, h.s.compat
	current := ""
	if info != nil {
		current = info.CurrentTool
	}
	value := current
	if s.toolDraft != nil {
		value = *s.toolDraft
	}
	hasDraft := s.toolDraft != nil && *s.toolDraft != current
	busy := s.compatLoading || h.compatPending() || s.saving || s.applyChecking || s.compatErr != "" ||
		info == nil || !info.Supported || !info.SteamInstalled
	disabled := busy || info.AppID == ""
	restartDisabled := busy || s.steamRestarting

	var notice string
	switch {
	case s.compatLoading || h.compatPending():
		notice = locales.GameLaunchSteamProtonLoading()
	case s.compatErr != "":
		notice = locales.GameLaunchSteamProtonError(s.compatErr)
	case !info.SteamInstalled:
		notice = locales.GameLaunchSteamProtonNotInstalled()
	case info.AppID == "":
		notice = locales.GameLaunchSteamProtonNotAssociated()
	}

	winW, _ := c.Size()
	ui.Column(c.Key("steam-proton")).Gap(8).Children(func() {
		fieldLabel(c, t, locales.GameLaunchSteamProton(), 0)
		wide := winW >= 768
		var row ui.Element
		if wide {
			row = ui.Row(c.Key("row")).Gap(8).AlignItems(ui.Center)
		} else {
			row = ui.Column(c.Key("row")).Gap(8)
		}
		row.Children(func() {
			box := ui.Box(c).MinWidth(0)
			if wide {
				box.Grow(1).Basis(0)
			} else {
				box.FillWidth()
			}
			box.Children(func() {
				better.Select(c, t, "select", better.SelectProps{
					Value: value, Options: steamToolOptions(info), Disabled: disabled,
					OnChange: func(v string) { s.toolDraft = &v },
				})
			})
			better.Button(c, t, "save", better.ButtonProps{
				Icon: "content-save-outline", Loading: s.saving, Disabled: !hasDraft || disabled,
			}).Label(locales.GameLaunchSteamProtonSave()).OnClick(func() { h.saveSteamSettings(c) })
			better.Button(c, t, "restart", better.ButtonProps{
				Icon: "restart", Label: locales.GameLaunchSteamRestart(), Loading: s.steamRestarting, Disabled: restartDisabled,
			}).OnClick(func() { h.restartSteam(c) })
		})
		if hasDraft {
			components.Text(c, locales.GameLaunchSteamProtonUnsaved(), theme.TextXS, theme.WeightNormal, t.TextMuted)
		}
		if notice != "" {
			color := t.Pick(theme.Brand500, theme.Brand400)
			if s.compatErr != "" {
				color = t.Pick(theme.Error500, theme.Brand400)
			}
			components.Text(c, notice, theme.TextXS, theme.WeightNormal, color)
		}
		if info != nil && info.SteamRoot != "" {
			monoText(c, info.SteamRoot, theme.TextXS, theme.WeightNormal, t.TextFaint)
		}
	})
}

// steamPrefix：Proton Prefix 的 glass-panel：标题、路径（font-mono break-all）或未找到、提示与“打开”按钮（secondary sm）。
func (h *handlers) steamPrefix(c *ui.Context, t *theme.Theme) {
	s := h.s
	path := ""
	if s.compat != nil {
		path = s.compat.ProtonPrefix
	}
	disabled := s.compatLoading || h.compatPending() || s.prefixOpening || path == "" || s.compatErr != ""
	winW, _ := c.Size()
	glassPanel(c, t, "steam-prefix").Children(func() {
		panelRow(c, winW, func(text ui.Element) {
			text.Children(func() {
				components.Text(c, locales.GameLaunchSteamProtonPrefix(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200))
				if path != "" {
					monoText(c, path, theme.TextXS, theme.WeightNormal, t.TextMuted).Margin(4, 0, 0, 0)
					components.TextMargin(c, locales.GameLaunchSteamProtonPrefixHint(), textXSRelax, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
				} else {
					components.TextMargin(c, locales.GameLaunchSteamProtonPrefixNotFound(), theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
				}
			})
		}, func() {
			better.Button(c, t, "steam-prefix-open", better.ButtonProps{
				Size: better.ButtonSM, Icon: "folder-open-outline", Label: locales.GameLaunchSteamProtonPrefixOpen(),
				Loading: s.prefixOpening, Disabled: disabled,
			}).OnClick(func() { h.openSteamPrefix(c) })
		})
	})
}

// wineRunnerOptions 对应 wineRunnerOptions：Linux 上非 Windows 程序可以不用兼容层；系统 Wine；macOS 加 CrossOver；
// Linux 加自定义 Wine、自动选择的 Proton、本机的 Proton（“名称 · 来源”），以及已选但本机找不到的 Proton。
func (h *handlers) wineRunnerOptions(w wineState) []better.SelectOption {
	darwin, linux := h.l.goos == "darwin", h.linux()
	var opts []better.SelectOption
	if linux && !w.defaultsToSystem {
		opts = append(opts, better.SelectOption{Value: "", Label: locales.GameLaunchWineRunnerNone()})
	}
	opts = append(opts, better.SelectOption{Value: "system", Label: locales.GameLaunchWineRunnerSystem()})
	if darwin {
		opts = append(opts, better.SelectOption{Value: "crossover", Label: locales.GameLaunchWineRunnerCrossover()})
	}
	if linux {
		opts = append(opts,
			better.SelectOption{Value: "custom", Label: locales.GameLaunchWineRunnerCustom()},
			better.SelectOption{Value: "proton", Label: locales.GameLaunchWineRunnerProtonAuto()})
		found := false
		for _, tool := range h.s.protonTools {
			v := "proton:" + tool.ID
			found = found || v == w.runner
			opts = append(opts, better.SelectOption{Value: v, Label: toolLabel(tool.Name, tool.DisplayName) + " · " + tool.Source})
		}
		if w.proton && w.runner != "proton" && !found {
			opts = append(opts, better.SelectOption{Value: w.runner, Label: w.runner})
		}
	}
	return opts
}

// wineConfig：兼容引擎（space-y-2 的 BetterSelect 与提示）；选了兼容层时再有 Wine 启动参数（BetterInput font-mono、
// ghost sm 的预设按钮）与 WINEPREFIX / CrossOver bottle / Proton compatdata（BetterActionInput，右侧在文件管理器中打开）。
// 原版 GetLocalProtonTools 失败时的 localProtonToolsError 这里没有：服务方法不返回错误。
func (h *handlers) wineConfig(c *ui.Context, t *theme.Theme) {
	g, w := h.p.Game, h.wine()
	ui.Column(c.Key("wine-runner")).Gap(8).Children(func() {
		fieldLabel(c, t, locales.GameLaunchCompatibilityEngine(), 0)
		better.Select(c, t, "select", better.SelectProps{
			Value: w.runner, Options: h.wineRunnerOptions(w),
			OnChange: func(v string) {
				h.edit(func(pg *models.Game) {
					keep := v == pg.WineRunner || (isProtonRunner(v) && isProtonRunner(pg.WineRunner))
					pg.WineRunner = v
					if !keep {
						pg.WinePrefix = ""
					}
				})
			},
		})
		hint := locales.GameLaunchWineRunnerHint()
		if w.proton {
			hint = locales.GameLaunchWineRunnerProtonHint()
		}
		components.Text(c, hint, theme.TextXS, theme.WeightNormal, t.TextMuted)
	})
	if w.runner == "" {
		return
	}

	ui.Column(c.Key("wine-args")).Gap(8).Children(func() {
		fieldLabel(c, t, locales.GameLaunchWineArgs(), 0)
		v := g.WineArgs
		if better.Input(c, t, "input", &v, better.InputProps{Placeholder: locales.GameLaunchWineArgsPlaceholder()}).Font(monoFont).Changed() {
			h.edit(func(pg *models.Game) { pg.WineArgs = v })
		}
		presets := localeLaunchOptionPresets
		if w.proton {
			presets = steamLaunchOptionPresets
		}
		ui.Row(c).Wrap().GapX(8).GapY(8).Children(func() {
			for _, preset := range presets {
				better.Button(c, t, "preset-"+preset.key, better.ButtonProps{
					Variant: better.ButtonGhost, Size: better.ButtonSM, Icon: "plus-circle-outline",
					Label: locales.Key("gameLaunch.steamLaunchOptionsPresets." + preset.key),
				}).OnClick(func() { h.edit(func(pg *models.Game) { pg.WineArgs = preset.value }) })
			}
		})
		components.Text(c, locales.GameLaunchWineArgsHint(), textXSRelax, theme.WeightNormal, t.TextMuted)
	})

	label, placeholder, hint := locales.GameLaunchWinePrefix(), locales.GameLaunchWinePrefixPlaceholder(), locales.GameLaunchWinePrefixHint()
	switch {
	case w.runner == "crossover":
		label, placeholder, hint = locales.GameLaunchCrossoverBottle(), locales.GameLaunchCrossoverBottlePlaceholder(), locales.GameLaunchCrossoverBottleHint()
	case w.proton:
		label, placeholder, hint = locales.GameLaunchProtonCompatDataPath(), locales.GameLaunchProtonCompatDataPathPlaceholder(), locales.GameLaunchProtonCompatDataPathHint()
	}
	ui.Column(c.Key("wine-prefix")).Gap(8).Children(func() {
		fieldLabel(c, t, label, 0)
		var actions []inputAction
		if w.runner != "crossover" {
			prefix := w.prefix
			actions = append(actions, inputAction{
				icon: "folder-open-outline", label: locales.GameEditOpenInExplorer(), disabled: prefix == "",
				onClick: func() { h.openWinePrefix(c, prefix) },
			})
		}
		v := g.WinePrefix
		if actionInput(c, t, "input", &v, label, placeholder, func(r ui.Element) { r.FillWidth() }, actions...).Changed() {
			h.edit(func(pg *models.Game) { pg.WinePrefix = v })
		}
		components.Text(c, hint, theme.TextXS, theme.WeightNormal, t.TextMuted)
	})
}

// compatActionLabel 对应 compatibilityActionLabel：Proton 的 winecfg 叫 protoncfg。
func (h *handlers) compatActionLabel(action string) string {
	if tools := h.s.tools; action == "winecfg" && tools != nil && (tools.RunnerKind == "proton" || tools.RunnerKind == "steam-proton") {
		return locales.GameLaunchCompatibilityActionProtoncfg()
	}
	switch action {
	case "prefix_dir":
		return locales.GameLaunchCompatibilityActionPrefix()
	case "drive_c":
		return locales.GameLaunchCompatibilityActionDriveC()
	case "regedit":
		return locales.GameLaunchCompatibilityActionRegedit()
	case "winecfg":
		return locales.GameLaunchCompatibilityActionWinecfg()
	case "explorer":
		return locales.GameLaunchCompatibilityActionExplorer()
	case "winecmd":
		return locales.GameLaunchCompatibilityActionWinecmd()
	}
	return action
}

// quickTools：兼容层快捷工具的 glass-panel（space-y-3）：标题、状态或 prefix 路径（break-all，出错或不支持时
// text-error-500 dark:text-error-400）、secondary sm 的工具按钮（flex-wrap gap-2）与兼容层类型。
func (h *handlers) quickTools(c *ui.Context, t *theme.Theme) {
	s, tools := h.s, h.s.tools
	notice := ""
	switch {
	case s.toolsLoading:
		notice = locales.GameLaunchCompatibilityQuickToolsLoading()
	case s.toolsErr != "":
		notice = locales.GameLaunchCompatibilityQuickToolsError(s.toolsErr)
	case tools != nil:
		notice = tools.Message
	}
	if notice == "" && tools != nil {
		notice = tools.PrefixPath
	}
	if notice == "" {
		notice = locales.GameLaunchCompatibilityQuickToolsReady()
	}
	color := t.TextMuted
	if s.toolsErr != "" || (tools != nil && !tools.Supported) {
		color = t.Pick(theme.Error500, theme.Error400)
	}
	glassPanel(c, t, "quick-tools").Gap(12).Children(func() {
		ui.Column(c).MinWidth(0).Children(func() {
			components.Text(c, locales.GameLaunchCompatibilityQuickTools(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand800, theme.Brand200))
			components.TextMargin(c, notice, theme.TextXS, theme.WeightNormal, color, 4, 0, 0, 0)
		})
		ui.Row(c).Wrap().GapX(8).GapY(8).Children(func() {
			for _, item := range compatibilityToolActions {
				available := tools != nil && containsString(tools.Actions, item.action)
				better.Button(c, t, "tool-"+item.action, better.ButtonProps{
					Size: better.ButtonSM, Icon: item.icon, Label: h.compatActionLabel(item.action),
					Loading:  s.opening == item.action,
					Disabled: s.toolsLoading || !available || (s.opening != "" && s.opening != item.action),
				}).OnClick(func() { h.openCompatTool(c, item.action) })
			}
		})
		if tools != nil && tools.RunnerKind != "" {
			components.Text(c, locales.GameLaunchCompatibilityQuickToolsRunner(tools.RunnerKind), theme.TextXS, theme.WeightNormal, t.TextFaint)
		}
	})
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// enhancementCard：启动增强工具卡片（space-y-6，只有 Windows），Steam 启动时没有 Locale Emulator。
func (h *handlers) enhancementCard(c *ui.Context, t *theme.Theme, steam bool) {
	g := h.p.Game
	cfg := h.p.Env.Config()
	hasLE := cfg != nil && cfg.LocaleEmulatorPath != ""
	hasMagpie := cfg != nil && cfg.MagpiePath != ""
	card(c, t, "enhancement-card", 24).Children(func() {
		cardTitle(c, t, locales.GameLaunchEnhancementTools(), 8)
		if !steam {
			h.toolRow(c, t, "use_locale_emulator", "Locale Emulator", locales.GameLaunchLeDesc(),
				locales.GameLaunchLeNotConfigured(), hasLE, g.UseLocaleEmulator, func(on bool) {
					if on && !hasLE {
						h.toast(c, env.Error(locales.GameLaunchToastLePathRequired()))
						return
					}
					h.edit(func(pg *models.Game) { pg.UseLocaleEmulator = on })
				})
		}
		h.toolRow(c, t, "use_magpie", "Magpie", locales.GameLaunchMagpieDesc(),
			locales.GameLaunchMagpieNotConfigured(), hasMagpie, g.UseMagpie, func(on bool) {
				if on && !hasMagpie {
					h.toast(c, env.Error(locales.GameLaunchToastMagpiePathRequired()))
					return
				}
				h.edit(func(pg *models.Game) { pg.UseMagpie = on })
			})
	})
}

// toolRow：flex items-center justify-between；左侧 min-w-0 pr-4 的名称（text-sm font-medium）、说明（mt-1 text-xs）
// 与未配置提示（mt-1 flex items-center gap-1 text-xs text-error-500，i-mdi-alert-circle text-sm），右侧 BetterSwitch。
func (h *handlers) toolRow(c *ui.Context, t *theme.Theme, key, name, desc, missing string, configured, checked bool, onChange func(bool)) {
	ui.Row(c.Key(key)).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Padding(0, 16, 0, 0).Children(func() {
			components.Text(c, name, theme.TextSM, theme.WeightMedium, t.SectionLabel)
			components.TextMargin(c, desc, theme.TextXS, theme.WeightNormal, t.TextMuted, 4, 0, 0, 0)
			if !configured {
				ui.Row(c).Margin(4, 0, 0, 0).Gap(4).AlignItems(ui.Center).Children(func() {
					components.Icon(c, "alert-circle", 14, theme.Error500)
					components.Text(c, missing, theme.TextXS, theme.WeightNormal, theme.Error500)
				})
			}
		})
		ui.Box(c).Shrink(0).Children(func() {
			better.Switch(c, t, "switch", checked, !configured, onChange)
		})
	})
}

// launchModeChange 对应 handleDefaultLaunchModeChange：Steam 要先确认 Steam 关联，其余直接改游戏；改成兼容层启动时
// 兼容层取 CrossOver（已经是）或系统 Wine。
func (h *handlers) launchModeChange(c *ui.Context, mode enums.LaunchMode) {
	g := h.p.Game
	if mode != enums.LaunchModeSteam {
		h.edit(func(pg *models.Game) {
			pg.LaunchMode = mode
			if mode == enums.LaunchModeCompatibility && pg.WineRunner != "crossover" {
				pg.WineRunner = "system"
			}
		})
		return
	}
	h.l.resolveSteam(c, steamSaveDefault, *g, func(sg models.Game) {
		h.l.steam.action = steamNone
		h.l.saveSteamAsDefault(sg)
	})
}

// selectProcessExecutable 对应 handleSelectProcessExecutable：选择文件，取文件名作为进程名。
func (h *handlers) selectProcessExecutable(c *ui.Context) {
	id, path, e := h.p.Game.ID, h.p.Game.Path, h.p.Env
	e.Async(func() func() {
		selection, err := e.Svc.Game.SelectGameExecutable(path)
		return func() {
			if err != nil {
				h.toast(c, env.Error(locales.GameToastSelectFileFailed()))
				return
			}
			if name := fileName(selection); selection != "" && name != "" {
				h.l.change(id, func(pg *models.Game) { pg.ProcessName = name })
			}
		}
	})
}

// exportShortcut 对应 handleExportLaunchShortcut：保存对话框取消时什么也不提示。
func (h *handlers) exportShortcut(c *ui.Context) {
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		path, err := e.Svc.Game.ExportLaunchShortcut(id)
		return func() {
			switch {
			case err != nil:
				h.toast(c, env.Error(locales.GameLaunchToastShortcutExportFailed(err.Error())))
			case path != "":
				h.toast(c, env.Success(locales.GameLaunchToastShortcutExportSuccess(path)))
			}
		}
	})
}

// refreshSteamSettings 对应面板的 handleRefreshSteamSettings：同时重新读取 Steam 兼容工具设置（只在 Linux 上）与游戏
// （详情页的 handleRefreshSteamSettings，只合并 Steam 相关字段）。
func (h *handlers) refreshSteamSettings() {
	s, id, e, linux := h.s, h.p.Game.ID, h.p.Env, h.linux()
	if linux {
		s.compatLoading = true
	}
	s.compatErr = ""
	s.compatSeq++
	seq := s.compatSeq
	e.Async(func() func() {
		var info service.SteamCompatibilityInfo
		var infoErr error
		if linux {
			info, infoErr = e.Svc.Integration.GetGameSteamCompatibility(id)
		}
		refreshed, err := e.Svc.Game.GetGameByID(id)
		return func() {
			if !h.current(id) {
				return
			}
			if err == nil {
				h.l.change(id, func(pg *models.Game) { mergeSteamSettings(pg, refreshed) })
			} else {
				infoErr = err
			}
			if s.compatSeq != seq {
				return
			}
			s.compatLoading = false
			if infoErr != nil {
				s.compat, s.compatErr = nil, infoErr.Error()
				return
			}
			if linux {
				s.compat = &info
			}
		}
	})
}

// saveSteamSettings 对应 handleSaveSteamSettings：Linux 上先重新读取 Steam 状态，Steam 正在运行时确认后再保存
// （保存会关闭并重启 Steam）；其他平台直接保存。
func (h *handlers) saveSteamSettings(c *ui.Context) {
	s := h.s
	if s.saving || s.applyChecking {
		return
	}
	if !h.linux() {
		h.applySteam(c, false)
		return
	}
	s.applyChecking = true
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		info, err := e.Svc.Integration.GetGameSteamCompatibility(id)
		return func() {
			if !h.current(id) {
				return
			}
			s.applyChecking = false
			if err == nil {
				s.compat = &info
			}
			if s.compat != nil && s.compat.SteamRunning {
				s.applyConfirm = true
				return
			}
			h.applySteam(c, false)
		}
	})
}

// applySteam 对应 applySteamSettings 与详情页的 handleApplySteamSettings：ApplyGameSteamSettings 写入启动参数与这里选的
// Proton（没选时保留 Steam 里的设置），再合并重新读到的 Steam 字段并清掉草稿。
func (h *handlers) applySteam(c *ui.Context, steamRunning bool) {
	s := h.s
	if s.saving {
		return
	}
	s.saving = true
	id, e, linux := h.p.Game.ID, h.p.Env, h.linux()
	options := h.steamOptionsValue()
	var tool *string
	if s.toolDraft != nil {
		v := *s.toolDraft
		tool = &v
	}
	e.Async(func() func() {
		err := e.Svc.Integration.ApplyGameSteamSettings(id, tool, options)
		var refreshed models.Game
		if err == nil {
			refreshed, err = e.Svc.Game.GetGameByID(id)
		}
		var info service.SteamCompatibilityInfo
		infoErr := err
		if err == nil && linux {
			info, infoErr = e.Svc.Integration.GetGameSteamCompatibility(id)
		}
		return func() {
			if !h.current(id) {
				return
			}
			s.saving = false
			if err != nil {
				h.toast(c, env.Error(locales.GameLaunchToastSteamSettingsSaveFailed(err.Error())))
				return
			}
			h.l.change(id, func(pg *models.Game) { mergeSteamSettings(pg, refreshed) })
			s.optionsDraft, s.toolDraft = nil, nil
			if steamRunning {
				h.toast(c, env.Success(locales.GameLaunchToastSteamSettingsSavedRestarted()))
			} else {
				h.toast(c, env.Success(locales.GameLaunchToastSteamSettingsSaved()))
			}
			if linux && infoErr == nil {
				s.compat = &info
			}
		}
	})
}

// restartSteam 对应 handleRestartSteam：RestartSteamClient 后刷新 Steam 设置。
func (h *handlers) restartSteam(c *ui.Context) {
	s := h.s
	if s.steamRestarting {
		return
	}
	s.steamRestarting = true
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		err := e.Svc.Integration.RestartSteamClient()
		return func() {
			if !h.current(id) {
				return
			}
			s.steamRestarting = false
			if err != nil {
				h.toast(c, env.Error(locales.GameLaunchToastSteamRestartFailed(err.Error())))
				return
			}
			h.toast(c, env.Success(locales.GameLaunchToastSteamRestarted()))
			h.refreshSteamSettings()
		}
	})
}

// openSteamPrefix 对应 handleOpenSteamProtonPrefix。
func (h *handlers) openSteamPrefix(c *ui.Context) {
	s := h.s
	if s.compat == nil || s.compat.ProtonPrefix == "" || s.prefixOpening {
		return
	}
	s.prefixOpening = true
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		_, err := e.Svc.Integration.OpenGameSteamProtonPrefix(id)
		return func() {
			if !h.current(id) {
				return
			}
			s.prefixOpening = false
			if err != nil {
				h.toast(c, env.Error(locales.GameLaunchToastSteamProtonPrefixOpenFailed(err.Error())))
			}
		}
	})
}

// openWinePrefix 对应 handleOpenWinePrefix：在文件管理器中打开实际使用的 prefix（游戏没填时是设置里的默认值）。
func (h *handlers) openWinePrefix(c *ui.Context, prefix string) {
	if prefix == "" {
		return
	}
	e := h.p.Env
	e.Async(func() func() {
		err := e.Svc.Game.OpenLocalPath(prefix)
		return func() {
			if err != nil {
				h.toast(c, env.Error(locales.GameEditOpenPathFailed()))
			}
		}
	})
}

// openCompatTool 对应 handleOpenCompatibilityTool：打开工具后重新读取快捷工具的状态。
func (h *handlers) openCompatTool(c *ui.Context, action string) {
	s := h.s
	if s.opening != "" {
		return
	}
	s.opening = action
	id, e := h.p.Game.ID, h.p.Env
	e.Async(func() func() {
		_, err := e.Svc.Integration.OpenGameCompatibilityTool(id, action)
		var info service.GameCompatibilityToolsInfo
		var infoErr error
		if err == nil {
			info, infoErr = e.Svc.Integration.GetGameCompatibilityTools(id)
		}
		return func() {
			if !h.current(id) {
				return
			}
			s.opening = ""
			if err == nil {
				h.toast(c, env.Success(locales.GameLaunchToastCompatibilityToolOpened()))
				err = infoErr
			}
			if err != nil {
				h.toast(c, env.Error(locales.GameLaunchToastCompatibilityToolOpenFailed(err.Error())))
				return
			}
			s.tools, s.toolsErr = &info, ""
		}
	})
}
