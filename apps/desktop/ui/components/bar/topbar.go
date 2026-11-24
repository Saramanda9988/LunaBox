// Package bar 对应原版 frontend/src/components/bar：窗口顶栏与侧边栏。
package bar

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/assets"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/theme"
)

const (
	windowButtonWidth   = 44
	WindowControlsWidth = 3 * windowButtonWidth // 三个窗口控制按钮的总宽度
	topFadeHeight       = 32                    // 与页面顶部 pagePadding（32px）对齐：未滚动时不触及正文标题，滚动时覆盖 28px 控制区并柔和渐隐
)

// TopBarProps 是窗口控制按钮的状态与动作。
type TopBarProps struct {
	Maximised                       bool
	HideWindowButtons               bool // macOS 使用原生红绿灯时隐藏右侧自绘窗口按钮
	OnMinimise, OnMaximise, OnClose func()
	PluginItems                     func()
}

// TopBar 对应 TopBar.tsx：h-[28px] 可拖动窗口，标题图片绝对居中，右侧三个窗口按钮。
// 最大化按钮标记为窗口的最大化按钮（MaximizeButton）：Windows 11 上悬停时显示贴靠布局，同系统标题栏。
func TopBar(c *ui.Context, t *theme.Theme, p TopBarProps) {
	bar := ui.Row(c).Height(env.TopbarHeight).Shrink(0).Background(t.TopbarBg).
		BorderWidth(0, 0, t.HL, 0).BorderColor(t.TopbarBorder).DragWindow()
	bar.Children(func() {
		// 居中的字标可由主题插件替换；主题隐藏字标时改放图标，两者都隐藏时留空。
		if mark := brandWordmark(t); mark != nil {
			brandImage(c, mark, 20).Attach(ui.AnchorCenter, ui.AnchorCenter).PassThrough()
		} else if icon := brandIcon(t); icon != nil {
			brandImage(c, icon, 20).Attach(ui.AnchorCenter, ui.AnchorCenter).PassThrough()
		}

		ui.Spacer(c)
		if p.PluginItems != nil {
			ui.Row(c.Key("plugin-topbar-items")).AlignItems(ui.Center).Children(p.PluginItems)
		}
		if !p.HideWindowButtons {
			windowButtons(c, t, p)
		}
	})
}

// TopBarFade 在隐藏顶栏时覆盖主内容区顶部 32px，提供渐进式毛玻璃与同底色渐隐：
// 未滚动时与页面底色融为一体；内容滚动到顶部时在拖动区与右上角窗口按钮后柔和虚化，
// 消除卡片与滚动条直接撞到窗口边缘的无界感并保证按钮可读性。
// visible 为 false（页面内容贴顶的全出血头图尚未滚动）时淡出，滚动后再淡入。
func TopBarFade(c *ui.Context, t *theme.Theme, left float32, visible bool) {
	fade := ui.Box(c.Key("topbar-fade"))
	opacity := fade.Animate("show", theme.Bool(visible), 200*time.Millisecond)
	if opacity <= 0 {
		return
	}
	blurMask := &ui.LinearGradient{
		From:  ui.RGB(0, 0, 0),
		To:    ui.Transparent,
		Angle: 180,
		Start: 0.5,
		End:   1,
	}
	topAlpha := float32(0.88)
	if t.Glass {
		// 自定义背景玻璃模式下随主区不透明度缩放，避免在低不透明度壁纸上压出一条实色带。
		topAlpha = 0.32 + 0.48*t.BgOpacity
	}
	bg := t.AppBg
	tint := ui.LinearGradient{
		Angle: 180,
		Stops: []ui.GradientStop{
			{Color: bg.Alpha(topAlpha), At: 0},
			{Color: bg.Alpha(topAlpha * 0.90), At: 0.5},
			{Color: bg.Alpha(topAlpha * 0.50), At: 0.78},
			{Color: bg.Alpha(0), At: 1},
		},
	}
	fade.Absolute().Top(0).Left(left).Right(0).Height(topFadeHeight).PassThrough().Opacity(opacity).
		Material(glass.Blur{Radius: 12, Mask: blurMask}).Children(func() {
		ui.Box(c.Key("tint")).Fill().PassThrough().LinearGradient(tint)
	})
}

// WindowControls 是隐藏顶栏时浮在窗口右上角的最小化、最大化/还原、关闭按钮。
func WindowControls(c *ui.Context, t *theme.Theme, p TopBarProps) {
	if p.HideWindowButtons {
		return
	}
	ui.Row(c.Key("window-controls")).Height(env.TopbarHeight).Shrink(0).Children(func() {
		windowButtons(c, t, p)
	})
}

func windowButtons(c *ui.Context, t *theme.Theme, p TopBarProps) {
	windowButton(c, t, "minimise", assets.Minimise, false, p.OnMinimise)
	maxIcon := assets.Maximise
	if p.Maximised {
		maxIcon = assets.Restore
	}
	windowButton(c, t, "maximise", maxIcon, false, p.OnMaximise).MaximizeButton()
	windowButton(c, t, "close", assets.Close, true, p.OnClose)
}

// windowButton：h-[28px] w-[44px]，hover:bg-brand-200 dark:hover:bg-brand-700，关闭按钮 hover:bg-red-500，active:scale-[.98]。
func windowButton(c *ui.Context, t *theme.Theme, key string, svg *ui.SVG, danger bool, onClick func()) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(windowButtonWidth, env.TopbarHeight).Center().FocusRing(false)
	color := t.ControlIcon
	if btn.Hovered() {
		if danger {
			btn.Background(theme.Red500)
			color = theme.White
		} else {
			hover := t.ControlHover
			if t.TopbarHidden {
				hover = t.Pick(theme.Black.Alpha(0.08), theme.White.Alpha(0.12))
			}
			btn.Background(hover)
		}
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	s := btn.Animate("press", 1-0.02*theme.Bool(btn.Pressed()), 150*time.Millisecond)
	color = btn.AnimateColor("icon", color, theme.Tween(150*time.Millisecond))
	return btn.Scale(s, s).OnClick(onClick).Children(func() {
		ui.Icon(c, svg).Size(10, 10).TextColor(color)
	})
}
