// Package bar 对应原版 frontend/src/components/bar：窗口顶栏与侧边栏。
package bar

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/theme"
)

// TopBarProps 是窗口控制按钮的状态与动作。
type TopBarProps struct {
	Maximised                       bool
	OnMinimise, OnMaximise, OnClose func()
}

// TopBar 对应 TopBar.tsx：h-[28px] 可拖动窗口，标题图片绝对居中，右侧三个窗口按钮。
// 最大化按钮标记为窗口的最大化按钮（MaximizeButton）：Windows 11 上悬停时显示贴靠布局，同系统标题栏。
func TopBar(c *ui.Context, t *theme.Theme, p TopBarProps) {
	bar := ui.Row(c).Height(env.TopbarHeight).Shrink(0).Background(t.TopbarBg).
		BorderWidth(0, 0, t.HL, 0).BorderColor(t.TopbarBorder).DragWindow()
	bar.Children(func() {
		title := assets.Title(t.Dark)
		w, h := title.Size()
		ui.Image(c, title).Size(20*float32(w)/float32(h), 20).
			Attach(ui.AnchorCenter, ui.AnchorCenter).PassThrough()

		ui.Spacer(c)
		windowButton(c, t, "minimise", assets.Minimise, false, p.OnMinimise)
		maxIcon := assets.Maximise
		if p.Maximised {
			maxIcon = assets.Restore
		}
		windowButton(c, t, "maximise", maxIcon, false, p.OnMaximise).MaximizeButton()
		windowButton(c, t, "close", assets.Close, true, p.OnClose)
	})
}

// windowButton：h-[28px] w-[44px]，hover:bg-brand-200 dark:hover:bg-brand-700，关闭按钮 hover:bg-red-500，active:scale-[.98]。
func windowButton(c *ui.Context, t *theme.Theme, key string, svg *ui.SVG, danger bool, onClick func()) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(44, env.TopbarHeight).Center().FocusRing(false)
	color := t.ControlIcon
	if btn.Hovered() {
		if danger {
			btn.Background(theme.Red500)
			color = theme.White
		} else {
			btn.Background(t.ControlHover)
		}
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	s := btn.Animate("press", 1-0.02*theme.Bool(btn.Pressed()), 150*time.Millisecond)
	color = btn.AnimateColor("icon", color, theme.Tween(150*time.Millisecond))
	return btn.Scale(s, s).OnClick(onClick).Children(func() {
		ui.Icon(c, svg).Size(10, 10).TextColor(color)
	})
}
