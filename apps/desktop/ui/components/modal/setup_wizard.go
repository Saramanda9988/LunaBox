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

var (
	setupTextWelcome = theme.FontStep{Size: 28, Line: 36} // text-[28px] leading-9
	setupTextTitle   = theme.FontStep{Size: 23, Line: 32} // 同启动窗：text-[23px] leading-8
	setupTextDetail  = theme.FontStep{Size: 13, Line: 20} // 同启动窗：text-[13px] leading-5
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

// SetupWizardView 是第一次打开时的初始设置向导：盖住顶栏下方的侧边栏与内容区，左边是启动窗那样的图标区，右边依次设置
// 语言、主题、时区与游戏库目录。语言与主题选了立即保存并生效，向导本身随之换语言与配色；时区与游戏库目录在最后一步
// 一起保存，时区保存后数据库立即改用它，不用重启。没有关闭方式，只能走完。要在页面最后构建，浮层才会盖住页面内容。
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
	// 盖住顶栏下方的整个区域（侧边栏与内容区），排版同启动窗：左边是图标区，右边是当前这一步。
	components.Overlay(c, t, func() {
		host := ui.Row(c.Key("setup-wizard")).Absolute().Top(p.TopOffset).Left(0).Right(0).Bottom(0).AlignItems(ui.Stretch).
			Modal().Background(t.Pick(theme.White, theme.Brand800))
		host.Children(func() {
			winW, _ := c.Size()
			components.BrandAside(c, t, min(600, max(340, winW*0.36)), true) // 约窗口宽度的 36%
			// flex min-w-0 flex-1 items-center justify-center px-12 py-10：欢迎语、步骤条、当前这一步与按钮是一组（max-w-xl），
			// 整组在右侧居中。每一步的内容高度不同，留出 min-h 让按钮不随步骤上下跳动。
			ui.Column(c.Key("main")).Grow(1).Basis(0).MinWidth(0).Padding(40, 48).Center().Children(func() {
				ui.Column(c.Key("content")).FillWidth().MaxWidth(576).Children(func() {
					components.Text(c, locales.NativeSetupWizardTitle(), setupTextWelcome, theme.WeightBold, t.Pick(theme.Brand900, theme.White))
					components.TextMargin(c, locales.NativeSetupWizardSubtitle(), theme.TextSM, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 6, 0, 28, 0)
					setupProgress(c, t, m.step)
					ui.Column(c.Key("step-"+strconv.Itoa(m.step))).FillWidth().MinHeight(220).Padding(40, 0, 0, 0).
						Children(func() { m.stepContent(c, t, p) })
					m.actions(c, t, p)
				})
			})
		})
	})
}

// stepContent 是当前这一步的标题、说明与输入。
func (m *SetupWizard) stepContent(c *ui.Context, t *theme.Theme, p SetupWizardProps) {
	e := p.Env
	cfg := e.Config()
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
		// 检测到的时区是 font-semibold，原版 TimezoneSelectModal 是 primary 色，这里同主色调 text-neutral-600 dark:text-neutral-400
		ui.Row(c.Key("detected")).Wrap().Gap(4).Margin(0, 0, 8, 0).Children(func() {
			components.Text(c, locales.TimezoneModalDetectedLabel(), theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand300))
			detected := p.Detected
			if detected == "" {
				detected = locales.TimezoneModalUnknown()
			}
			components.Text(c, detected, theme.TextSM, theme.WeightSemibold, t.Pick(theme.Neutral600, theme.Neutral400))
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
}

// actions 是底部的按钮：左边“返回”（第一步没有），右边“下一步”或“开始使用”。
func (m *SetupWizard) actions(c *ui.Context, t *theme.Theme, p SetupWizardProps) {
	ui.Row(c.Key("actions")).FillWidth().Shrink(0).Gap(12).AlignItems(ui.Center).Children(func() {
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
		setupPrimaryButton(c, t, "finish", locales.NativeSetupWizardFinish(), m.busy).OnClick(func() { m.finish(p.Env, p.OnDone) })
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

// setupProgress 是步骤条：每步一段 h-1 rounded-full，已到达的是 neutral-600 dark:neutral-400，下面是“第 n 步，共 4 步”。
func setupProgress(c *ui.Context, t *theme.Theme, step int) {
	ui.Column(c.Key("progress")).FillWidth().Gap(8).Children(func() {
		ui.Row(c.Key("bars")).FillWidth().Gap(6).Children(func() {
			for i := range setupStepCount {
				color := t.Pick(theme.Brand200, theme.Brand700)
				if i <= step {
					color = t.Pick(theme.Neutral600, theme.Neutral400)
				}
				bar := ui.Box(c.Key("bar-" + strconv.Itoa(i))).Height(4).Grow(1).Basis(0).Radius(999)
				bar.Background(bar.AnimateColor("bg", color, theme.Tween(200*time.Millisecond)))
			}
		})
		components.Text(c, locales.NativeSetupWizardStep(strconv.Itoa(step+1), strconv.Itoa(setupStepCount)),
			theme.TextXS, theme.WeightMedium, t.TextFaint)
	})
}

// setupStepHeader 是每一步的标题，排版同启动窗的 header：flex items-start gap-3.5，text-[28px] 的图标，
// text-[23px] font-semibold 的标题与 text-[13px] 的说明；下面 mb-6 是这一步的输入。
func setupStepHeader(c *ui.Context, t *theme.Theme, icon, title, desc string) {
	ui.Row(c.Key("header")).FillWidth().AlignItems(ui.Start).Gap(14).Margin(0, 0, 24, 0).Children(func() {
		components.Icon(c, icon, 28, t.Pick(theme.Neutral500, theme.Neutral400)).Margin(2, 0, 0, 0).Shrink(0)
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, title, setupTextTitle, theme.WeightSemibold, t.Pick(theme.Brand900, theme.White))
			components.TextMargin(c, desc, setupTextDetail, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 0)
		})
	})
}

// setupPrimaryButton 是原版 TimezoneSelectModal 确认按钮的尺寸（px-6 py-2.5 text-sm font-medium rounded-lg），颜色同主按钮
// （BetterButton primary）：bg-neutral-600 hover:bg-neutral-700 shadow-sm shadow-neutral-200 dark:shadow-none。
// busy 时禁用并显示旋转图标。
func setupPrimaryButton(c *ui.Context, t *theme.Theme, key, label string, busy bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(10, 24).Radius(8).Gap(8).AlignItems(ui.Center).FocusRing(false)
	bg := theme.Neutral600
	if btn.Hovered() && !busy {
		bg = theme.Neutral700
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if !t.Dark {
		btn.Shadow(0, 1, 2, 0, theme.Neutral200)
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
