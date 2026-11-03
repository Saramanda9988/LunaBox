package better

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// DrawerProps 是 BetterDrawer（placement="right"）的属性。
type DrawerProps struct {
	Open         *bool
	Title        string
	HeaderAction func() // 标题右侧的按钮
	CloseLabel   string
	TopOffset    float32 // 抽屉与遮罩从窗口顶部让出的高度（顶栏）
	Width        float32 // 默认 w-[min(92vw,26rem)] 的 26rem
	Body         func()
}

// Drawer 对应 BetterDrawer：遮罩 bg-black/35 backdrop-blur-[2px]，面板 bg-white/96 dark:bg-brand-800/96 backdrop-blur-20，
// 打开 300ms cubic-bezier(.22,1,.36,1)、关闭 200ms ease-in，面板从右侧滑入并由 opacity .95 变为 1。
// Esc 与点击遮罩关闭。要放在页面的最后构建，浮层才会盖住页面内容。
func Drawer(c *ui.Context, t *theme.Theme, key string, p DrawerProps) {
	width := p.Width
	if width == 0 {
		width = 416
	}
	if *p.Open && c.Shortcut(0, ui.KeyEscape) {
		*p.Open = false
	}
	components.Overlay(c, t, func() {
		host := ui.Box(c.Key(key)).Absolute().Top(p.TopOffset).Left(0).Right(0).Bottom(0)
		dur, ease := 300*time.Millisecond, theme.EaseDrawer
		if !*p.Open {
			dur, ease = 200*time.Millisecond, theme.EaseIn
		}
		k := host.AnimateWith("open", theme.Bool(*p.Open), dur, ease)
		if k == 0 {
			host.Invisible()
			return
		}
		if !*p.Open {
			host.PassThrough()
		}
		sb := ui.Local(host, "scroll", func() components.ScrollBar { return components.ScrollBar{} })
		host.Children(func() {
			backdrop := ui.ButtonBase(c.Key("backdrop")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				Material(glass.Blur{Radius: 2}).Opacity(k).FocusRing(false).Label(p.CloseLabel)
			backdrop.OnClick(func() { *p.Open = false }).Children(func() {
				ui.Box(c).Fill().Background(ui.RGBA(0, 0, 0, 0.35))
			})

			// translate-x-full 用 right 偏移代替 Translate：变换下的裁剪（滚动区、圆角控件）每帧都要在 CPU 上重新光栅化
			// 裁剪蒙版，滑入时会卡顿；按布局移动的裁剪由着色器直接裁，与静止时一样快。
			panel := ui.Column(c.Key("panel")).Absolute().Top(0).Right(-(1-k)*width).Bottom(0).Width(width).
				Material(glass.Blur{Radius: 20}).Opacity(0.95 + 0.05*k)
			panel.Children(func() {
				ui.Column(c).Fill().Background(t.PanelBg.Alpha(0.96)).BorderWidth(0, 0, 0, t.HL).BorderColor(t.PanelBorder).Children(func() {
					drawerHeader(c, t, p)
					components.ScrollArea(c, t, "body", sb, func(w ui.Element) { w.Grow(1).Basis(0) }, func(sc ui.Element) {
						// p-4，没有 scrollbar-gutter: stable，滚动条只在溢出时占位
						right := float32(16)
						if sb.Overflowing() {
							right += t.Gutter()
						}
						sc.Padding(16, right, 16, 16).Children(p.Body)
					})
				})
			})
		})
	})
}

// drawerHeader：flex justify-between gap-4 border-b px-5 py-4，标题行 min-h-8 gap-1，关闭按钮 -mr-1 h-8 w-8 rounded-lg。
func drawerHeader(c *ui.Context, t *theme.Theme, p DrawerProps) {
	ui.Row(c).Padding(16, 20).Gap(16).Justify(ui.SpaceBetween).AlignItems(ui.Center).
		BorderWidth(0, 0, t.HL, 0).BorderColor(t.Divider).Shrink(0).Children(func() {
		ui.Row(c).MinHeight(32).Gap(4).AlignItems(ui.Center).Children(func() {
			components.Text(c, p.Title, theme.TextBase, theme.WeightSemibold, t.TextStrong)
			if p.HeaderAction != nil {
				p.HeaderAction()
			}
		})
		closeBtn := ui.ButtonBase(c.Key("close")).Size(32, 32).Radius(8).Center().Margin(0, -4, 0, 0).FocusRing(false).Label(p.CloseLabel)
		color := t.TextMuted
		if closeBtn.Hovered() {
			closeBtn.Background(t.MenuHover)
			color = t.Pick(theme.Brand800, theme.White)
		}
		closeBtn.Transition(theme.ColorsTransition(150 * time.Millisecond))
		closeBtn.OnClick(func() { *p.Open = false }).Children(func() {
			components.Icon(c, "close", 20, color)
		})
	})
}
