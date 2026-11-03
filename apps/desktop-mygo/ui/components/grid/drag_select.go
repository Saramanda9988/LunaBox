package grid

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// 与 hooks/useDragSelection.ts 相同的自动滚动参数（DIP）。
const (
	autoScrollEdge    = 64 // AUTO_SCROLL_EDGE_PX
	autoScrollMaxStep = 18 // AUTO_SCROLL_MAX_STEP_PX
)

// DragSelection 对应 hooks/useDragSelection.ts：批量模式下在卡片上按住鼠标右键并拖过其他卡片，经过的每张卡片
// （各一次）都设成起点卡片切换后的状态：起点原本没选中就全部选中，否则全部取消。按下时起点卡片立即切换，
// 也不弹右键菜单；指针离滚动区域上下边缘不到 64 DIP 时每帧最多滚动 18 DIP，越靠近边缘越快。
//
// 原版的触控与触控笔要移动 10px 才开始（DRAG_START_DISTANCE_PX），左键不参与；MyGo 的指针事件不区分触控、
// 触控笔与鼠标，这里只有鼠标右键。
//
// 页面在构建网格外层（原版挂事件的 measureRef）时调用 Surface，构建每个可选的格子时用 Target。
// 状态放在页面里，跨帧保留。
type DragSelection struct {
	surface ui.Handle
	origin  ui.Rect      // surface 上一帧在窗口中的位置
	targets []dragTarget // 本次构建登记的格子，相对 surface
	prev    []dragTarget // 上一次构建登记的格子：自动滚动在构建格子之前执行
	props   DragProps    // 本次构建的属性，输入回调用它
	gesture *dragGesture // 进行中的拖动（gestureRef）
}

// DragProps 是 Surface 的输入。
type DragProps struct {
	Enabled        bool                           // 批量模式
	Selected       func(id string) bool           // selectedIds.has
	OnSelectChange func(id string, selected bool) // 设定一张卡片的选中状态
	// Scroll 是网格所在的滚动容器（findScrollParent），nil 时不自动滚动。
	Scroll *ui.ScrollState
}

type dragTarget struct {
	id string
	r  ui.Rect
}

type dragGesture struct {
	desired bool            // desiredSelection
	visited map[string]bool // visitedIds
	// x、y 是指针在窗口中的位置（lastClientX/Y），view 是滚动容器的可见区域，都在指针事件时取。
	x, y float32
	view ui.Rect
	// stepAt 是上一步自动滚动所在帧的时间：一帧里重建多次时只滚一步。
	stepAt time.Time
}

// Active 报告是否正在拖动选择。
func (d *DragSelection) Active() bool { return d.gesture != nil }

// Surface 让 surface 接收拖动选择的指针事件，要在 surface 构建子元素之前调用。不在批量模式时结束进行中的拖动
// （原版 enabled 变为 false 时的 finishGesture）。
func (d *DragSelection) Surface(c *ui.Context, surface ui.Element, p DragProps) {
	d.prev, d.targets = d.targets, d.prev[:0]
	d.origin = surface.Bounds()
	d.props = p
	if !p.Enabled || p.Selected == nil || p.OnSelectChange == nil {
		d.gesture = nil
		return
	}
	surface.Bind(&d.surface).HandleInput(func(ev ui.InputEvent) bool { return d.input(c, ev) })
	d.autoScroll(c)
}

// Target 构建一个可选的格子（原版带 data-drag-selection-id 的卡片）：在一个不加样式的 Box 里构建，记下它上一帧的位置。
// c 带着格子的 key（如游戏 ID）。
func (d *DragSelection) Target(c *ui.Context, id string, build func(c *ui.Context)) {
	box := ui.Box(c)
	if b := box.Bounds(); d.props.Enabled && b.W > 0 && b.H > 0 {
		d.targets = append(d.targets, dragTarget{id: id, r: ui.Rect{X: b.X - d.origin.X, Y: b.Y - d.origin.Y, W: b.W, H: b.H}})
	}
	box.Children(func() { build(c) })
}

// input 对应 onPointerDown、onPointerMove、onPointerUp 与 onContextMenu（接下右键按下，MyGo 就不弹右键菜单）。
func (d *DragSelection) input(c *ui.Context, ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 1 {
			return false
		}
		id, ok := hit(d.targets, ev.X, ev.Y)
		if !ok {
			return false
		}
		g := &dragGesture{desired: !d.props.Selected(id), visited: map[string]bool{id: true}}
		d.gesture = g
		d.track(c, ev)
		d.props.OnSelectChange(id, g.desired)
		return true
	case ui.InputPointerMove:
		if d.gesture == nil {
			return false
		}
		d.track(c, ev)
		d.selectAt(d.targets, ev.X, ev.Y)
		return true
	case ui.InputPointerUp:
		if d.gesture == nil || ev.Button != 1 {
			return false
		}
		d.gesture = nil
		return true
	}
	return false
}

// track 记下指针在窗口中的位置与滚动容器的可见区域（输入回调在两次构建之间，Handle 取已提交的布局）。
func (d *DragSelection) track(c *ui.Context, ev ui.InputEvent) {
	b := d.surface.Bounds(c)
	d.gesture.x, d.gesture.y = b.X+ev.X, b.Y+ev.Y
	if d.props.Scroll != nil {
		d.gesture.view = d.props.Scroll.Bounds(c)
	}
}

// selectAt 对应 selectAtPoint：x、y 相对 surface。
func (d *DragSelection) selectAt(targets []dragTarget, x, y float32) {
	g := d.gesture
	id, ok := hit(targets, x, y)
	if !ok || g.visited[id] {
		return
	}
	g.visited[id] = true
	d.props.OnSelectChange(id, g.desired)
}

// hit 对应 elementFromPoint 加 findSelectionId：返回 x、y（相对 surface）处的格子。
func hit(targets []dragTarget, x, y float32) (string, bool) {
	for _, t := range targets {
		if x >= t.r.X && x < t.r.X+t.r.W && y >= t.r.Y && y < t.r.Y+t.r.H {
			return t.id, true
		}
	}
	return "", false
}

// autoScroll 对应 runAutoScroll，每帧构建时执行一步：先按上一步滚动后的布局选中指针下的卡片，再滚下一步。
// 指针不在边缘区域时停止；在边缘但已滚到头时与原版一样继续逐帧检查。
func (d *DragSelection) autoScroll(c *ui.Context) {
	g, s := d.gesture, d.props.Scroll
	if g == nil || s == nil || g.view.H <= 0 {
		return
	}
	top, bottom := g.y-g.view.Y, g.view.Y+g.view.H-g.y
	var step float32
	switch {
	case top >= 0 && top < autoScrollEdge:
		step = -autoScrollMaxStep * (1 - top/autoScrollEdge)
	case bottom >= 0 && bottom < autoScrollEdge:
		step = autoScrollMaxStep * (1 - bottom/autoScrollEdge)
	}
	if step == 0 {
		return
	}
	c.AnimationFrame()
	now := c.Now()
	if now.Equal(g.stepAt) {
		return
	}
	g.stepAt = now
	prev := s.Y
	s.Y = min(max(s.Y+step, 0), s.MaxY)
	if moved := s.Y - prev; moved != 0 {
		// 这一帧的内容上移 moved，指针相对 surface 下移同样的距离。
		d.selectAt(d.prev, g.x-d.origin.X, g.y-d.origin.Y+moved)
	}
}
