// Package better 对应原版 frontend/src/components/ui/better：Better* 系列基础控件。
package better

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// ButtonVariant 对应 BetterButton 的 variant。
type ButtonVariant int

const (
	ButtonSecondary ButtonVariant = iota // 中性（默认）
	ButtonPrimary                        // 深色强调
	ButtonDanger                         // 危险红色
	ButtonGhost                          // 透明 / 文字
)

// ButtonSize 对应 BetterButton 的 size。
type ButtonSize int

const (
	ButtonMD ButtonSize = iota // h-10 px-4 text-sm gap-1.5（默认）
	ButtonSM                   // h-8 px-3 text-xs gap-1
	ButtonLG                   // h-11 px-5 text-sm gap-2
)

// ButtonProps 是 BetterButton 的属性。Label 与 Children 都为空且有 Icon 时是方形图标按钮。
type ButtonProps struct {
	Variant  ButtonVariant
	Size     ButtonSize
	Icon     string // 文字左侧的 i-mdi-* 图标
	Loading  bool   // 禁用并显示旋转图标
	Disabled bool
	Label    string
	Children func(color ui.Color) // 自定义内容（图片 + 文字等），收到当前文字颜色
}

// ButtonStyle 是 BetterButton 在某个状态下的颜色，调用方可以在 Button 返回后按 className 覆盖。
type ButtonStyle struct {
	Bg, Border, Text ui.Color
}

// ButtonColors 返回 variant 在当前主题与悬停状态下的颜色（variantClasses），玻璃模式下是追加的 glass-btn-*
// （secondary、ghost → neutral，primary → primary，danger → error）。
func ButtonColors(t *theme.Theme, v ButtonVariant, hovered bool) ButtonStyle {
	if t.Glass {
		kind := theme.GlassNeutral
		switch v {
		case ButtonPrimary:
			kind = theme.GlassPrimary
		case ButtonDanger:
			kind = theme.GlassError
		}
		g := t.GlassButton(kind, theme.BtnColors{})
		if hovered {
			g.Bg = g.Hover
		}
		return ButtonStyle{g.Bg, g.Border, g.Text}
	}
	switch v {
	case ButtonPrimary:
		s := ButtonStyle{theme.Neutral600, theme.Neutral600, theme.White}
		if hovered {
			s.Bg = theme.Neutral700
		}
		return s
	case ButtonDanger:
		s := ButtonStyle{t.Pick(theme.Error500, theme.Error600), t.Pick(theme.Error500, theme.Error600), theme.White}
		if hovered {
			s.Bg = t.Pick(theme.Error600, theme.Error700)
		}
		return s
	case ButtonGhost:
		s := ButtonStyle{ui.Transparent, ui.Transparent, t.Pick(theme.Brand600, theme.Brand400)}
		if hovered {
			s.Bg, s.Text = t.Pick(theme.Brand100, theme.Brand800), t.Pick(theme.Brand900, theme.Brand200)
		}
		return s
	}
	s := ButtonStyle{t.ToolBtnBg, t.ToolBtnBorder, t.Pick(theme.Brand700, theme.Brand300)}
	if hovered {
		s.Bg = t.ToolBtnHover
	}
	return s
}

// Button 对应 BetterButton：inline-flex rounded-lg font-medium leading-none，transition-all duration-200 active:scale-95。
// 返回按钮元素，调用方可以再设置宽度、圆角、背景等（相当于传 className）。
func Button(c *ui.Context, t *theme.Theme, key string, p ButtonProps) ui.Element {
	h, padX, gap, f, iconSize := float32(40), float32(16), float32(6), theme.FontStep{Size: 14, Line: 14}, float32(18)
	switch p.Size {
	case ButtonSM:
		h, padX, gap, f, iconSize = 32, 12, 4, theme.FontStep{Size: 12, Line: 12}, 16
	case ButtonLG:
		h, padX, gap, f, iconSize = 44, 20, 8, theme.FontStep{Size: 14, Line: 14}, 20
	}
	disabled := p.Disabled || p.Loading
	btn := ui.ButtonBase(c.Key(key)).Height(h).Radius(8).Gap(gap).Center().Shrink(0).FocusRing(false)
	if p.Label == "" && p.Children == nil {
		btn.Width(h)
	} else {
		btn.Padding(0, padX)
	}
	style := ButtonColors(t, p.Variant, btn.Hovered() && !disabled)
	btn.Background(style.Bg).Border(t.HL, style.Border).Transition(theme.ColorsTransition(200 * time.Millisecond))
	scale := float32(1)
	if btn.Pressed() && !disabled {
		scale = 0.95
	}
	s := btn.AnimateWith("press", scale, 200*time.Millisecond, theme.EaseTailwind)
	btn.Scale(s, s)
	if disabled {
		components.Disable(btn, 0.5)
	}
	btn.Children(func() {
		switch {
		case p.Loading:
			components.Spinner(c, iconSize, style.Text)
		case p.Icon != "":
			components.Icon(c, p.Icon, iconSize, style.Text)
		}
		if p.Children != nil {
			p.Children(style.Text)
		} else if p.Label != "" {
			components.Text(c, p.Label, f, theme.WeightMedium, style.Text).NoWrap() // 按钮按文字定宽；SingleLine 会因取整截成“重…”
		}
	})
	return btn
}

// ToolButton 对应工具栏里的 glass-btn-neutral 按钮：px-3 py-2 text-sm border rounded-lg，active 时加深。
// 玻璃模式下 data-glass: 的底色盖过 active 的底色。内容由调用方构建，颜色用 ToolButtonColor。
func ToolButton(c *ui.Context, t *theme.Theme, key string, active bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(8, 12).Radius(8).Gap(8).AlignItems(ui.Center).FocusRing(false)
	bg, border := t.ToolBtnBg, t.ToolBtnBorder
	if active {
		bg, border = t.ToolBtnActiveBg, t.ToolBtnActiveBorder
	}
	if btn.Hovered() {
		bg = t.ToolBtnHover
	}
	if t.Glass {
		g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{})
		bg, border = g.Bg, g.Border
		if btn.Hovered() {
			bg = g.Hover
		}
	}
	btn.Background(bg).Border(t.HL, border)
	return btn
}

// ToolButtonColor 是 ToolButton 内图标与文字的颜色。
func ToolButtonColor(t *theme.Theme, active bool) ui.Color {
	if t.Glass {
		return t.GlassButton(theme.GlassNeutral, theme.BtnColors{}).Text
	}
	if active {
		return t.ToolBtnActiveText
	}
	return t.ToolBtnIcon
}

// IconButton：rounded-xl p-2 / p-2.5 的纯图标按钮，hover:bg-brand-100 dark:hover:bg-brand-700（侧边栏），
// 玻璃模式下 data-glass:hover:bg-white/10 dark:bg-black/10，[&.active]:bg-white/20 dark:bg-black/20。
func IconButton(c *ui.Context, t *theme.Theme, key, icon string, pad float32, active, enabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Padding(pad).Radius(12).Center().FocusRing(false)
	color := t.NavText
	switch {
	case active:
		btn.Background(t.IfGlass(t.NavActiveBg, theme.White.Alpha(0.2), theme.Black.Alpha(0.2)))
		color = t.NavActive
	case enabled && btn.Hovered():
		btn.Background(t.IfGlass(t.NavHover, theme.White.Alpha(0.1), theme.Black.Alpha(0.1)))
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	btn.Children(func() { components.Icon(c, icon, 20, color) })
	return btn
}
