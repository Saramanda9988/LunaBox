package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// SplitOption 是 BetterSplitButton 菜单里的一项。
type SplitOption struct {
	Key, Label, Description string
	Icon                    string // i-mdi-* 图标，默认 play
}

// SplitButtonProps 是 BetterSplitButton 的属性。
type SplitButtonProps struct {
	Label       string
	Icon        string
	Options     []SplitOption
	SelectedKey string
	MenuTitle   string
	Disabled    bool
	Loading     bool // 禁用并以旋转图标代替 Icon
	OnClick     func()
	OnSelect    func(key string)
}

// SplitButton 对应 BetterSplitButton（variant="primary" size="sm"，菜单在下方左对齐）：
// 左侧 rounded-l-lg px-2.5 text-xs gap-1 的主按钮，右侧 rounded-r-lg px-1.5 的下拉按钮，两者 h-8。
func SplitButton(c *ui.Context, t *theme.Theme, key string, p SplitButtonProps) ui.Element {
	disabled := p.Disabled || p.Loading
	row := ui.Row(c.Key(key)).AlignItems(ui.Stretch).Shrink(0)
	open := ui.Local(row, "menu-open", func() bool { return false })
	if disabled {
		*open = false
	}
	var menuBtn ui.Element
	row.Children(func() {
		main := splitPart(c, t, "main", disabled).Radius(8, 0, 0, 8).Padding(0, 10).Gap(4)
		main.Children(func() {
			if p.Loading {
				components.Spinner(c, 16, theme.White)
			} else if p.Icon != "" {
				components.Icon(c, p.Icon, 16, theme.White)
			}
			components.Text(c, p.Label, theme.TextXS, theme.WeightMedium, theme.White).NoWrap()
		})
		if p.OnClick != nil {
			main.OnClick(p.OnClick)
		}

		menuBtn = splitPart(c, t, "menu", disabled).Radius(0, 8, 8, 0).Padding(0, 6).Label(p.MenuTitle)
		menuBtn.OnClick(func() { *open = !*open }).Children(func() {
			components.Icon(c, "chevron-down", 16, theme.White)
			// border-l border-white/20 dark:border-neutral-900/15，叠在 neutral-600 边框上。
			ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(t.HL).PassThrough().
				Background(t.Pick(theme.White.Alpha(0.2), ui.Hex("#0f172a").Alpha(0.15)))
		})
	})

	minW := row.Bounds().W
	ui.PopoverBase(c, menuBtn, open, func(panel ui.Element) {
		// absolute left-0 top-full mt-1.5 min-w-full w-max max-w-[22rem] rounded-xl border p-1.5
		panel.AttachTo(row, ui.AnchorBottomLeft, ui.AnchorTopLeft).Top(6).MinWidth(minW).MaxWidth(352).
			Padding(6).Radius(12).Background(t.PanelBg).Border(t.HL, t.PanelBorder)
		panel.Children(func() {
			if p.MenuTitle != "" {
				components.Text(c, p.MenuTitle, theme.TextXS, theme.WeightMedium, t.TextFaint).Padding(2, 8, 4, 8)
			}
			for _, o := range p.Options {
				splitOption(c, t, o, o.Key == p.SelectedKey, func() {
					*open = false
					if p.OnSelect != nil {
						p.OnSelect(o.Key)
					}
				})
			}
		})
	})
	return row
}

// splitPart 是两半共用的样式：bg-neutral-600 text-white hover:bg-neutral-700 border border-neutral-600，
// transition-all duration-200 active:scale-95，disabled:opacity-50；玻璃模式下是 glass-btn-primary。
func splitPart(c *ui.Context, t *theme.Theme, key string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Height(32).AlignItems(ui.Center).Justify(ui.Center).FocusRing(false)
	g := t.GlassButton(theme.GlassPrimary, theme.BtnColors{Bg: theme.Neutral600, Hover: theme.Neutral700, Border: theme.Neutral600})
	bg := g.Bg
	if btn.Hovered() && !disabled {
		bg = g.Hover
	}
	btn.Background(bg).Border(t.HL, g.Border).Transition(theme.ColorsTransition(200 * time.Millisecond))
	scale := float32(1)
	if btn.Pressed() && !disabled {
		scale = 0.95
	}
	s := btn.AnimateWith("press", scale, 200*time.Millisecond, theme.EaseTailwind)
	btn.Scale(s, s)
	if disabled {
		components.Disable(btn, 0.5)
	}
	return btn
}

// splitOption：flex w-full items-center rounded-lg px-3 py-2.5 text-sm，聚焦或选中时 bg-brand-100 dark:bg-brand-700。
func splitOption(c *ui.Context, t *theme.Theme, o SplitOption, selected bool, choose func()) {
	row := ui.ButtonBase(c.Key(o.Key)).FillWidth().Padding(10, 12).Radius(8).AlignItems(ui.Center).
		Justify(ui.Start).FocusRing(false)
	if selected || row.Hovered() {
		row.Background(t.Pick(theme.Brand100, theme.Brand700))
	}
	row.Transition(theme.ColorsTransition(150 * time.Millisecond))
	icon := o.Icon
	if icon == "" {
		icon = "play"
	}
	row.OnClick(choose).Children(func() {
		components.Icon(c, icon, 20, t.TextFaint).Margin(0, 12, 0, 0)
		ui.Column(c).Grow(1).Children(func() {
			text := t.Pick(theme.Brand700, theme.Brand200)
			components.Text(c, o.Label, theme.FontStep{Size: 14, Line: 17.5}, theme.WeightMedium, text).NoWrap()
			if o.Description != "" {
				components.TextMargin(c, o.Description, theme.FontStep{Size: 12, Line: 15}, theme.WeightNormal, t.TextFaint, 2, 0, 0, 0).NoWrap()
			}
		})
		if selected {
			components.Icon(c, "check", 18, t.Pick(theme.Neutral600, theme.Neutral300)).Margin(0, 0, 0, 12)
		}
	})
}
