package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/theme"
)

// InputProps 是 BetterInput 的属性。
type InputProps struct {
	Placeholder string
	Label       string // 无障碍名称，默认用 Placeholder
	Disabled    bool
	Password    bool
}

// Input 对应 BetterInput：w-full rounded-md border border-brand-300 bg-white px-3 py-2 text-base，
// 聚焦时 border-neutral-500 ring-2 ring-neutral-500/30，disabled:opacity-60。返回输入框元素，
// 调用方用 Changed / Submitted 观察编辑。占位文字 placeholder:text-brand-400 dark:placeholder:text-brand-500。
func Input(c *ui.Context, t *theme.Theme, key string, value *string, p InputProps) ui.Element {
	in := ui.TextInputBase(c.Key(key), value).FillWidth().Padding(8, 12).Radius(6).
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
	if p.Password {
		in.Password()
	}
	bg, border := t.GlassInput(t.Pick(theme.White, theme.Brand700), t.Pick(theme.Brand300, theme.Brand600)) // glass-input
	if in.Focused() {
		border = theme.Neutral500
		in.Shadow(0, 0, 0, 2, theme.Neutral500.Alpha(0.3))
	}
	in.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))
	if p.Disabled {
		components.Disable(in, 0.6)
	}
	return in
}

// InputAction 是 ActionInput 右侧的图标按钮。
type InputAction struct {
	Icon     string
	Label    string // aria-label 与提示
	Disabled bool
	Loading  bool // 图标换成 i-mdi-loading animate-spin
	OnClick  func()
}

// ActionInput 对应 BetterActionInput：带边框的行（rounded-md border pr-1 gap-1，focus-within:ring-2 ring-neutral-500），
// 左侧透明输入框 px-3 py-2，右侧 h-10 w-10 的图标按钮（text-xl，带提示）。返回输入框元素。
func ActionInput(c *ui.Context, t *theme.Theme, key string, value *string, p InputProps, actions ...InputAction) ui.Element {
	bg, border := t.GlassInput(t.Pick(theme.White, theme.Brand700), t.Pick(theme.Brand300, theme.Brand600)) // glass-input
	row := ui.Row(c.Key(key)).FillWidth().MinWidth(0).Gap(4).Padding(0, 4, 0, 0).AlignItems(ui.Center).Radius(6).
		Background(bg).Border(t.HL, border)
	if p.Disabled {
		components.Disable(row, 0.6) // 行内的输入框与按钮一起禁用
	}
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
		for _, a := range actions {
			btn := ui.ButtonBase(c.Key(a.Label)).Size(40, 40).Center().Shrink(0).FocusRing(false).Label(a.Label)
			disabled := p.Disabled || a.Disabled
			color := t.Pick(theme.Brand500, theme.Brand400)
			if btn.Hovered() && !disabled {
				color = t.Pick(theme.Brand900, theme.Brand100)
			}
			color = btn.AnimateColor("text", color, theme.Tween(200*time.Millisecond))
			if disabled {
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
			Tooltip(c, t, btn, a.Label)
		}
	})
	if in.Focused() {
		row.Shadow(0, 0, 0, 2, theme.Neutral500)
	}
	return in
}
