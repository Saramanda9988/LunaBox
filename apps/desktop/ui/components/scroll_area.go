package components

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/theme"
)

// ScrollBar 是 ScrollArea 的状态：滚动位置。放在页面状态里，页面可以读写滚动位置。
type ScrollBar struct {
	State  ui.ScrollState
	hidden bool // 上一帧是区域刚出现、没有绘制内容的一帧
}

// overflowEpsilon 是算作超出的最小距离：布局的浮点误差会留下 1e-5 量级的 MaxY，不应显示滚动条。
const overflowEpsilon = 0.01

// Overflowing 报告内容是否超出视口（上一帧）。
func (sb *ScrollBar) Overflowing() bool { return sb.State.MaxY > overflowEpsilon }

// ScrollArea 是还原 style.css 里 ::-webkit-scrollbar 的纵向滚动区域：常显的圆角滑块，9px 槽的右侧 6px。
// 滑块用 MyGo 原生滚动条画（样式见 theme.Theme.ScrollbarStyle，主题里对所有滚动容器生效），这里让它常显，
// 与原版一致；原生滚动条常显时一直是悬停的样子，所以把悬停色设回静止色，拖动时才变深。
//
// 原生滚动条浮在内容上面、不占宽度。wrap 设置外框尺寸，content 设置滚动容器的内边距并构建内容；
// 滚动条要占宽度（超出时，或 scrollbar-gutter: stable）时，由调用方在右内边距里加上 Theme.Gutter()。
//
// 是否超出（Overflowing）要等布局之后才知道，MyGo 一帧只布局一次。所以区域刚出现的那一帧（进入页面、
// 骨架屏换成内容、打开弹窗）只布局、不绘制内容，紧接着再构建一帧：内容、滚动条与它占的宽度同时出现，
// 不会先按没有滚动条排一次版、下一帧再变窄。
func ScrollArea(c *ui.Context, t *theme.Theme, key string, sb *ScrollBar, wrap func(ui.Element), content func(scroll ui.Element)) {
	w := ui.Box(c.Key(key)).Clip()
	wrap(w)
	b := w.Bounds()
	// 上一帧没有这个区域（Bounds 为空）：sb 里的超出还不是它的。连续两帧为空（真的是零大小）时照常构建，
	// 免得每帧都隐藏、重新构建。
	fresh := b == ui.Rect{} && !sb.hidden
	sb.hidden = fresh
	w.Children(func() {
		sc := ui.Scroll(c).Fill().TrackScroll(&sb.State).KeepScrollbars(true).
			ScrollbarStyle(ui.ScrollbarStyle{Hover: t.ScrollThumb})
		content(sc)
		if fresh {
			sc.Invisible()
			c.Invalidate()
		}
	})
}

// ScrollToTopButton 对应 ScrollToTopButton.tsx：fixed bottom-8 right-8 h-11 w-11 rounded-full，
// 隐藏时 translate-y-2 opacity-0，悬停 -translate-y-0.5。玻璃模式下是 glass-btn-neutral，
// 底色 data-glass:bg-white/35 data-glass:dark:bg-black/35。
func ScrollToTopButton(c *ui.Context, t *theme.Theme, label string, visible bool, onClick func()) {
	g := t.GlassButton(theme.GlassNeutral, theme.BtnColors{
		Bg: t.PanelBg.Alpha(0.92), Hover: t.Pick(theme.Brand100, theme.Brand700), Border: t.PanelBorder, Text: t.Pick(theme.Brand700, theme.Brand200),
	})
	if t.Glass {
		g.Bg = t.Pick(theme.White.Alpha(0.35), theme.Black.Alpha(0.35))
	}
	btn := ui.ButtonBase(c.Key("scroll-top")).Absolute().Right(32).Bottom(32).Size(44, 44).Radius(999).Center().
		Border(t.HL, g.Border).Background(g.Bg).FocusRing(false).Label(label).
		Shadow(0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
	show := btn.AnimateWith("show", theme.Bool(visible), 200*time.Millisecond, theme.EaseTailwind)
	if show == 0 {
		btn.Invisible()
		return
	}
	hover := btn.AnimateWith("hover", theme.Bool(btn.Hovered()), 200*time.Millisecond, theme.EaseTailwind)
	if btn.Hovered() {
		btn.Background(g.Hover)
	}
	btn.Opacity(show).Translate(0, 8*(1-show)-2*hover)
	if !visible {
		btn.PassThrough()
	}
	btn.OnClick(onClick).Children(func() {
		Icon(c, "arrow-up", 20, g.Text)
	})
}
