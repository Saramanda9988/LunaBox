package modal

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/components/panel/basic"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// defaultTimeZone 是检测不到系统时区时的默认选项（原版 TimezoneSelectModal 的 browserTimezone || "Asia/Shanghai"）。
const defaultTimeZone = "Asia/Shanghai"

// 向导的步骤：语言放在最前面，选好后后面几步就用这种语言显示。
const (
	setupStepLanguage = iota
	setupStepTheme
	setupStepTimezone
	setupStepLibrary
	setupStepCount
)

// setupSelectDirectory 打开原生的文件夹对话框，会阻塞，在 Env.Async 里调用。测试里替换掉，不弹对话框。
var setupSelectDirectory = func(e *env.Env, title string) (string, error) {
	return e.Svc.AppConfig.SelectDirectory(title)
}

// SetupWizard 是初始设置向导的状态，由根布局持有。取代原版第一次打开时只弹出的 TimezoneSelectModal。
type SetupWizard struct {
	step     int
	started  bool // 已从配置取出时区、游戏库目录的初始值
	timezone string
	library  string
	busy     bool // 正在选择文件夹或保存
}

// Step 是当前步骤（从 0 开始，测试用）。
func (m *SetupWizard) Step() int { return m.step }

// SetupWizardProps 是初始设置向导的属性。
type SetupWizardProps struct {
	Env       *env.Env
	Open      bool   // 配置里还没有时区（第一次打开）
	Detected  string // 系统的 IANA 时区，不知道时为空
	TopOffset float32
	OnDone    func() // 完成并保存之后调用
}

// SetupWizardView 是第一次打开时的初始设置向导：依次设置语言、主题、时区与游戏库目录。语言与主题选了立即保存并生效，
// 向导本身随之换语言与配色；时区与游戏库目录在最后一步一起保存，时区保存后数据库立即改用它，不用重启。
// 没有关闭方式，只能走完。要在页面最后构建，浮层才会盖住页面内容。
func SetupWizardView(c *ui.Context, t *theme.Theme, m *SetupWizard, p SetupWizardProps) {
	e := p.Env
	cfg := e.Config()
	if !p.Open || cfg == nil || e.Svc == nil {
		return
	}
	if !m.started {
		m.started = true
		m.timezone = p.Detected
		if m.timezone == "" {
			m.timezone = defaultTimeZone
		}
		m.library = cfg.GameLibraryPath
	}
	components.Modal(c, t, "setup-wizard", components.ModalProps{Open: true, TopOffset: p.TopOffset, MaxWidth: 512}, func(panel ui.Element) {
		components.ModalPanel(t, panel).Children(func() {
			components.TextMargin(c, locales.NativeSetupWizardTitle(), theme.TextXL, theme.WeightBold, t.TextStrong, 0, 0, 4, 0)
			components.TextMargin(c, locales.NativeSetupWizardSubtitle(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400), 0, 0, 20, 0)
			setupProgress(c, t, m.step)
			ui.Column(c.Key("step-"+strconv.Itoa(m.step))).FillWidth().Margin(24, 0, 0, 0).Children(func() {
				switch m.step {
				case setupStepLanguage:
					setupStepHeader(c, t, "web", locales.SettingsBasicLanguageLabel(), locales.NativeSetupWizardLanguageDesc())
					better.Select(c, t, "language", better.SelectProps{
						Value:    cfg.Language,
						Options:  basic.LanguageOptions(),
						OnChange: func(v string) { setupPatch(e, func(cfg *appconf.AppConfig) { cfg.Language = v }) },
					})
				case setupStepTheme:
					setupStepHeader(c, t, "palette", locales.SettingsBasicThemeLabel(), locales.NativeSetupWizardThemeDesc())
					better.Select(c, t, "theme", better.SelectProps{
						Value:    cfg.Theme,
						Options:  basic.ThemeOptions(),
						OnChange: func(v string) { setupPatch(e, func(cfg *appconf.AppConfig) { cfg.Theme = v }) },
					})
				case setupStepTimezone:
					setupStepHeader(c, t, "earth", locales.SettingsBasicTimezoneLabel(), locales.NativeSetupWizardTimezoneDesc())
					// 检测到的时区是 font-semibold text-primary-600 dark:text-primary-400（同原版 TimezoneSelectModal）
					ui.Row(c.Key("detected")).Wrap().Gap(4).Margin(0, 0, 8, 0).Children(func() {
						components.Text(c, locales.TimezoneModalDetectedLabel(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
						detected := p.Detected
						if detected == "" {
							detected = locales.TimezoneModalUnknown()
						}
						components.Text(c, detected, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Primary600, theme.Primary400))
					})
					better.Select(c, t, "timezone", better.SelectProps{
						Value:       m.timezone,
						Options:     basic.TimezoneOptions(p.Detected),
						Placeholder: locales.TimezoneModalPlaceholder(),
						OnChange:    func(v string) { m.timezone = v },
					})
				case setupStepLibrary:
					setupStepHeader(c, t, "folder-open-outline", locales.SettingsBasicGameLibraryPath(), locales.SettingsBasicGameLibraryPathHint())
					m.libraryInput(c, t, e, p.OnDone)
					components.TextMargin(c, locales.NativeSetupWizardLibraryOptional(), theme.TextXS, theme.WeightNormal, t.TextFaint, 8, 0, 0, 0)
				}
			})
			ui.Row(c.Key("actions")).FillWidth().Margin(32, 0, 0, 0).Gap(12).AlignItems(ui.Center).Children(func() {
				if m.step > 0 {
					back := components.ModalTextButton(c, t, "back", locales.CommonBack())
					if m.busy {
						components.Disable(back, 0.5)
					} else {
						back.OnClick(func() { m.step-- })
					}
				}
				ui.Box(c).Grow(1)
				if m.step < setupStepCount-1 {
					setupPrimaryButton(c, t, "next", locales.NativeSetupWizardNext(), false).OnClick(func() { m.step++ })
					return
				}
				setupPrimaryButton(c, t, "finish", locales.NativeSetupWizardFinish(), m.busy).OnClick(func() { m.finish(e, p.OnDone) })
			})
		})
	})
}

// libraryInput 是游戏库目录的输入框：右侧选择文件夹。只在完成时保存。
func (m *SetupWizard) libraryInput(c *ui.Context, t *theme.Theme, e *env.Env, onDone func()) {
	title := locales.SettingsBasicSelectGameLibraryTitle()
	in := better.ActionInput(c, t, "library", &m.library, better.InputProps{
		Placeholder: locales.SettingsBasicGameLibraryPathPlaceholder(),
		Disabled:    m.busy,
	}, better.InputAction{Icon: "folder-open-outline", Label: title, OnClick: func() {
		m.busy = true
		e.Async(func() func() {
			path, err := setupSelectDirectory(e, title)
			return func() {
				m.busy = false
				if err != nil {
					log.Printf("setup wizard: failed to select game library path: %v", err)
					e.Notify(env.Error(locales.SettingsBasicSelectGameLibraryFailed()))
					return
				}
				if path != "" {
					m.library = path
				}
			}
		})
	}})
	in.FontSize(theme.TextSM.Size).FixedLineHeight(theme.TextSM.Line)
	if in.Submitted() {
		m.finish(e, onDone)
	}
}

// finish 保存游戏库目录（填了且有变化时，不改写已有游戏的路径）与时区，成功后调用 onDone。保存时区后配置里有了
// time_zone，向导随之关闭。
func (m *SetupWizard) finish(e *env.Env, onDone func()) {
	if m.busy {
		return
	}
	m.busy = true
	tz, library := m.timezone, strings.TrimSpace(m.library)
	current := ""
	if cfg := e.Config(); cfg != nil {
		current = strings.TrimSpace(cfg.GameLibraryPath)
	}
	e.Async(func() func() {
		var err error
		if library != "" && library != current {
			_, err = e.Svc.AppConfig.ApplyGameLibraryPathChange(library, false)
		}
		return func() {
			m.busy = false
			if err != nil {
				log.Printf("setup wizard: failed to apply game library path: %v", err)
				e.Notify(env.Error(locales.SettingsBasicLibraryChangeApplyFailed()))
				return
			}
			if err := e.PatchConfig(func(cfg *appconf.AppConfig) { cfg.TimeZone = tz }); err != nil {
				log.Printf("setup wizard: failed to save time zone: %v", err)
				e.Notify(env.Error(locales.NativeSettingsSaveFailed()))
				return
			}
			*m = SetupWizard{}
			e.Notify(env.Success(locales.NativeSetupWizardDone()))
			if onDone != nil {
				onDone()
			}
		}
	})
}

// setupPatch 立即保存一项设置（语言、主题），失败时提示。
func setupPatch(e *env.Env, fn func(cfg *appconf.AppConfig)) {
	if err := e.PatchConfig(fn); err != nil {
		log.Printf("setup wizard: failed to save config: %v", err)
		e.Notify(env.Error(locales.NativeSettingsSaveFailed()))
	}
}

// setupProgress 是步骤条：每步一段 h-1 rounded-full，已到达的是 primary-600，下面是“第 n 步，共 4 步”。
func setupProgress(c *ui.Context, t *theme.Theme, step int) {
	ui.Column(c.Key("progress")).FillWidth().Gap(8).Children(func() {
		ui.Row(c.Key("bars")).FillWidth().Gap(6).Children(func() {
			for i := range setupStepCount {
				color := t.Pick(theme.Brand200, theme.Brand700)
				if i <= step {
					color = t.Pick(theme.Primary600, theme.Primary400)
				}
				bar := ui.Box(c.Key("bar-" + strconv.Itoa(i))).Height(4).Grow(1).Basis(0).Radius(999)
				bar.Background(bar.AnimateColor("bg", color, theme.Tween(200*time.Millisecond)))
			}
		})
		components.Text(c, locales.NativeSetupWizardStep(strconv.Itoa(step+1), strconv.Itoa(setupStepCount)),
			theme.TextXS, theme.WeightMedium, t.TextFaint)
	})
}

// setupStepHeader 是每一步的标题：左侧 p-2 rounded-full bg-primary-100 的图标（同原版 TimezoneSelectModal 的地球），
// 右侧 text-lg font-semibold 的标题与 text-sm 的说明。
func setupStepHeader(c *ui.Context, t *theme.Theme, icon, title, desc string) {
	ui.Row(c.Key("header")).FillWidth().AlignItems(ui.Start).Gap(16).Margin(0, 0, 16, 0).Children(func() {
		ui.Box(c).Padding(8).Radius(999).Shrink(0).Background(t.Pick(theme.Primary100, theme.Primary900.Alpha(0.3))).Children(func() {
			components.Icon(c, icon, 24, t.Pick(theme.Primary600, theme.Primary400))
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.TextMargin(c, title, theme.TextLG, theme.WeightSemibold, t.TextStrong, 0, 0, 4, 0)
			components.Text(c, desc, theme.FontStep{Size: 14, Line: 22.75}, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400))
		})
	})
}

// setupPrimaryButton 同原版 TimezoneSelectModal 的确认按钮：px-6 py-2.5 text-sm font-medium text-white bg-primary-600
// hover:bg-primary-700 rounded-lg shadow-sm shadow-primary-200 dark:shadow-none。busy 时禁用并显示旋转图标。
func setupPrimaryButton(c *ui.Context, t *theme.Theme, key, label string, busy bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(10, 24).Radius(8).Gap(8).AlignItems(ui.Center).FocusRing(false)
	bg := theme.Primary600
	if btn.Hovered() && !busy {
		bg = theme.Primary700
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if !t.Dark {
		btn.Shadow(0, 1, 2, 0, theme.Primary200)
	}
	if busy {
		components.Disable(btn, 0.6)
	}
	btn.Children(func() {
		if busy {
			components.Spinner(c, 16, theme.White)
		}
		components.Text(c, label, theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}
