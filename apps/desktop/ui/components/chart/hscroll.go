package chart

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// hScrollbar 是 HorizontalScrollChart 外层 overflow-x-auto 的滚动条：style.css 里 ::-webkit-scrollbar 的
// 6px 高的槽，滑块左侧 3px 透明边框（background-clip: content-box）、圆角 6、悬停变深，可以拖动。
// MyGo 自带的滚动条在应用里关掉了（Theme.ScrollbarWidth = 0、透明），所以与 components.ScrollArea 一样自己画。
func hScrollbar(c *ui.Context, t *theme.Theme, s *State, viewW float32) {
	track := ui.Box(c.Key("chart-scrollbar")).Height(scrollbarSize).Shrink(0)
	st := &s.scroll
	if st.MaxX <= 0 || viewW <= 0 {
		return
	}
	contentW := viewW + st.MaxX
	thumbW := max(viewW*viewW/contentW, 20)
	travel := viewW - thumbW
	border := theme.DevicePx(3, t.Scale)
	track.Children(func() {
		thumb := ui.Box(c.Key("thumb")).Absolute().Left(0).Top(0).Width(thumbW).Height(scrollbarSize).
			Padding(0, 0, 0, border)
		if !thumb.Dragging() {
			s.thumb = ui.Point{X: st.X / st.MaxX * travel}
		}
		thumb.Draggable(&s.thumb, ui.DraggableOptions{
			Axis:      ui.Horizontal,
			Bounds:    &ui.Rect{W: travel},
			Rigid:     true,
			Animation: ui.Animation{Duration: time.Nanosecond, Ease: ui.Linear},
		})
		if thumb.Dragging() && travel > 0 {
			st.X = s.thumb.X / travel * st.MaxX
		}
		color := t.ScrollThumb
		if thumb.Hovered() || thumb.Dragging() {
			color = t.ScrollThumbHover
		}
		thumb.Children(func() {
			// 6px 高的 border-radius: 6 收成 3；左边是透明边框，内容区左侧没有圆角
			ui.Box(c).Fill().Background(color).Radius(0, 3, 3, 0)
		})
	})
}
