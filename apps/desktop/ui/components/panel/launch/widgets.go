package launch

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/theme"
)

// monoFont 是 font-mono 在 Windows 的 Chromium 里实际生效的字体：字体栈 ui-monospace, SFMono-Regular, Menlo, Monaco,
// Consolas… 里前几个都不存在，落到 Consolas。
const monoFont = "Consolas"

// 原版用到、theme 里没有的颜色（uno.config.ts 的 primary、warning、neutral-900）。
var (
	primary100 = ui.Hex("#ede9fe")
	primary300 = ui.Hex("#c4b5fd")
	primary400 = ui.Hex("#a78bfa")
	primary600 = ui.Hex("#6d5dd3")
	primary700 = ui.Hex("#5b4cb8")
	primary900 = ui.Hex("#2e2660")
	primary200 = ui.Hex("#ddd6fe")
	warning100 = ui.Hex("#fef3c7")
	warning400 = ui.Hex("#fbbf24")
	warning600 = ui.Hex("#d97706")
	warning900 = ui.Hex("#78350f")
	neutral900 = ui.Hex("#0f172a")
)

var (
	textLG       = theme.FontStep{Size: 18, Line: 28}    // text-lg
	textXSRelax  = theme.FontStep{Size: 12, Line: 19.5}  // text-xs leading-relaxed
	textSMRelax  = theme.FontStep{Size: 14, Line: 22.75} // text-sm leading-relaxed
	buttonTextMD = theme.FontStep{Size: 14, Line: 14}    // BetterButton md：text-sm leading-none
)

// monoText 是 font-mono 的文字。Consolas 的升部+降部小于 CSS 行高，不需要 components.Text 的负外边距。
func monoText(c *ui.Context, s string, f theme.FontStep, weight int, color ui.Color) ui.Element {
	return ui.Text(c, s).Font(monoFont).FontSize(f.Size).FixedLineHeight(f.Line).FontWeight(weight).TextColor(color)
}

// cardTitle：h3 text-lg font-semibold text-brand-900 dark:text-white，bottom 是外层 div 的 pb-*。
func cardTitle(c *ui.Context, t *theme.Theme, s string, bottom float32) ui.Element {
	return components.TextMargin(c, s, textLG, theme.WeightSemibold, t.TextStrong, 0, 0, bottom, 0)
}

// card：glass-card bg-white dark:bg-brand-800 p-6 rounded-lg，子元素 space-y-*。
func card(c *ui.Context, t *theme.Theme, key string, gap float32) ui.Element {
	bg, border := t.GlassCard(t.Pick(theme.White, theme.Brand800), ui.Transparent)
	e := ui.Column(c.Key(key)).Padding(24).Radius(8).Gap(gap).Background(bg)
	if t.Glass {
		e.Border(t.HL, border)
	}
	return e
}

// fieldLabel：block text-sm font-medium text-brand-700 dark:text-brand-300，bottom 是 mb-*。
func fieldLabel(c *ui.Context, t *theme.Theme, s string, bottom float32) ui.Element {
	return components.TextMargin(c, s, theme.TextSM, theme.WeightMedium, t.SectionLabel, 0, 0, bottom, 0)
}

// glassPanel：glass-panel rounded-xl border border-brand-200/80 bg-brand-50/70 p-4 dark:border-brand-700 dark:bg-brand-900/30。
func glassPanel(c *ui.Context, t *theme.Theme, key string) ui.Element {
	bg, border := t.GlassPanel(t.Pick(theme.Brand50.Alpha(0.7), theme.Brand900.Alpha(0.3)), t.Pick(theme.Brand200.Alpha(0.8), theme.Brand700))
	return ui.Column(c.Key(key)).Padding(16).Radius(12).Border(t.HL, border).Background(bg)
}

// emptyIconButton 是 variant="secondary" 的 BetterButton（md）。icon 是 UnoCSS 生成不出样式的图标类（如 i-mdi-application-search-outline，
// @iconify-json/mdi 里没有）时，原版留下一个空的 span，它仍是 flex 子项，文字前多出 gap-1.5。
func emptyIconButton(c *ui.Context, t *theme.Theme, key, label string) ui.Element {
	return better.Button(c, t, key, better.ButtonProps{
		Variant: better.ButtonSecondary,
		Children: func(color ui.Color) {
			ui.Box(c).Size(0, 0).Shrink(0)
			components.Text(c, label, buttonTextMD, theme.WeightMedium, color).SingleLine()
		},
	}).Label(label)
}

// inputAction 是 actionInput 右侧的图标按钮（BetterActionInputAction）。
type inputAction struct {
	icon     string // 也是按钮元素的 key（label 是翻译后的文字，不能当 key）
	label    string // aria-label 与提示
	disabled bool
	loading  bool // 图标换成 i-mdi-loading animate-spin
	onClick  func()
}

// actionInput 对应 className="font-mono" 的 BetterActionInput：glass-input flex w-full min-w-0 items-center gap-1 rounded-md
// border border-brand-300 bg-white pr-1 focus-within:ring-2 focus-within:ring-neutral-500 dark:border-brand-600
// dark:bg-brand-700；输入框 min-w-0 flex-1 bg-transparent px-3 py-2（字号继承 16px），按钮 h-10 w-10 text-xl
// text-brand-500 hover:text-brand-900 dark:text-brand-400 dark:hover:text-brand-100 disabled:opacity-50，带提示。
// 共享的 better.ActionInput 不能换等宽字体，也没有加载中的图标。wrap 设置外框在父元素里的尺寸。返回输入框。
func actionInput(c *ui.Context, t *theme.Theme, key string, value *string, label, placeholder string, wrap func(ui.Element), actions ...inputAction) ui.Element {
	row := ui.Row(c.Key(key)).MinWidth(0).Gap(4).Padding(0, 4, 0, 0).AlignItems(ui.Center).Radius(6).
		Background(t.Pick(theme.White, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	wrap(row)
	var in ui.Element
	row.Children(func() {
		in = ui.TextInputBase(c.Key("input"), value).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).
			Font(monoFont).FontSize(theme.TextBase.Size).FixedLineHeight(theme.TextBase.Line).TextColor(t.TextStrong).
			Placeholder(placeholder).PlaceholderColor(t.Pick(theme.Brand400, theme.Brand500)).Label(label)
		for _, a := range actions {
			btn := ui.ButtonBase(c.Key(a.icon)).Size(40, 40).Center().Shrink(0).FocusRing(false).Label(a.label)
			disabled := a.disabled || a.loading
			color := t.Pick(theme.Brand500, theme.Brand400)
			if btn.Hovered() && !disabled {
				color = t.Pick(theme.Brand900, theme.Brand100)
			}
			color = btn.AnimateColor("text", color, theme.Tween(200*time.Millisecond))
			if disabled {
				components.Disable(btn, 0.5)
			} else if a.onClick != nil {
				btn.OnClick(a.onClick)
			}
			btn.Children(func() {
				if a.loading {
					components.Spinner(c, 20, color)
				} else {
					components.Icon(c, a.icon, 20, color)
				}
			})
			better.Tooltip(c, t, btn, a.label)
		}
	})
	if in.Focused() {
		row.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	return in
}

// primaryModalButton 是 ProcessSelectModal 的确认按钮：px-4 py-2 text-sm font-medium text-white bg-primary-600
// hover:bg-primary-700 rounded-lg shadow-sm shadow-primary-200 dark:shadow-none disabled:opacity-50。
// 与 components.ModalPrimaryButton（bg-neutral-600）不同，这里是月光紫。
func primaryModalButton(c *ui.Context, t *theme.Theme, key, label string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 16).Radius(8).FocusRing(false).Label(label)
	bg := primary600
	if btn.Hovered() && !disabled {
		bg = primary700
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if !t.Dark {
		btn.Shadow(0, 1, 2, 0, primary200)
	}
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() {
		components.Text(c, label, theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
	})
	return btn
}
