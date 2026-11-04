// Package gamefield 是编辑游戏信息用到的输入组件：带按钮的输入框、发售日期选择器、别名、评分与标签输入。
// 详情页的编辑标签页与添加游戏的手动填写共用它们。
package gamefield

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/theme"
)

// Label：block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1。
func Label(c *ui.Context, t *theme.Theme, s string) ui.Element {
	return components.TextMargin(c, s, theme.TextSM, theme.WeightMedium, t.SectionLabel, 0, 0, 4, 0)
}

// Hint：mt-1 text-xs text-brand-500；darkLighter 时另有 dark:text-brand-400（原版各处写法不一）。
func Hint(c *ui.Context, t *theme.Theme, s string, darkLighter bool) ui.Element {
	color := theme.Brand500
	if darkLighter {
		color = t.Pick(theme.Brand500, theme.Brand400)
	}
	return components.TextMargin(c, s, theme.TextXS, theme.WeightNormal, color, 4, 0, 0, 0)
}

// Blurred 报告输入框在这一遍构建里失去了焦点（onBlur）。
func Blurred(in ui.Element) bool {
	was := ui.Local(in, "was-focused", func() bool { return false })
	focused := in.Focused()
	lost := *was && !focused
	*was = focused
	return lost
}

// InputAction 是 BetterActionInput 的 actions 之一。
type InputAction struct {
	Icon     string
	Label    string // aria-label，也是提示
	Disabled bool
	Loading  bool // i-mdi-loading animate-spin
	OnClick  func()
}

// ActionInputProps 是 BetterActionInput 的输入框属性。
type ActionInputProps struct {
	Placeholder string
	Label       string                 // 无障碍名称，默认用 Placeholder
	OnPaste     func(text string) bool // onPaste
}

// ActionInput 对应 BetterActionInput（同 better.ActionInput，另支持 onPaste 与加载中的旋转图标）：
// glass-input flex w-full min-w-0 items-center gap-1 rounded-md border border-brand-300 bg-white pr-1
// focus-within:ring-2 focus-within:ring-neutral-500 dark:border-brand-600 dark:bg-brand-700；
// 输入框 min-w-0 flex-1 bg-transparent px-3 py-2；按钮 h-10 w-10 text-xl，hover:text-brand-900 dark:hover:text-brand-100，
// disabled:opacity-50。返回输入框，调用方用 Changed 观察编辑。
func ActionInput(c *ui.Context, t *theme.Theme, key string, value *string, p ActionInputProps, actions ...InputAction) ui.Element {
	row := ui.Row(c.Key(key)).FillWidth().MinWidth(0).Gap(4).Padding(0, 4, 0, 0).AlignItems(ui.Center).Radius(6).
		Background(t.Pick(theme.White, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	var in ui.Element
	row.Children(func() {
		in = ui.TextInputBase(c.Key("input"), value).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).
			FontSize(theme.TextBase.Size).FixedLineHeight(theme.TextBase.Line).TextColor(t.TextStrong).
			Placeholder(p.Placeholder).PlaceholderColor(t.Pick(theme.Brand400, theme.Brand500))
		if theme.UIFont != "" {
			in.Font(theme.UIFont)
		}
		label := p.Label
		if label == "" {
			label = p.Placeholder
		}
		if label != "" {
			in.Label(label)
		}
		if p.OnPaste != nil {
			in.OnPaste(p.OnPaste)
		}
		for _, a := range actions {
			ActionButton(c, t, a)
		}
	})
	if in.Focused() {
		row.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	return in
}

// ActionButton 是 actionInput 右侧的图标按钮（带 BetterTooltip）。元素的 key 用图标（同一个输入框里各不相同），
// 不用会随语言变化的 Label。
func ActionButton(c *ui.Context, t *theme.Theme, a InputAction) {
	btn := ui.ButtonBase(c.Key(a.Icon)).Size(40, 40).Center().Shrink(0).FocusRing(false).Label(a.Label)
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() && !a.Disabled {
		color = t.Pick(theme.Brand900, theme.Brand100)
	}
	color = btn.AnimateColor("text", color, theme.Tween(200*time.Millisecond))
	if a.Disabled {
		components.Disable(btn, 0.5)
	} else if a.OnClick != nil {
		btn.OnClick(a.OnClick)
	}
	btn.Children(func() {
		if a.Loading {
			components.Spinner(c, 20, color)
		} else {
			components.Icon(c, a.Icon, 20, color)
		}
	})
	better.Tooltip(c, t, btn, a.Label)
}
