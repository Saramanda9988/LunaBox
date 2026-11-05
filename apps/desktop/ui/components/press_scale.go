package components

import "github.com/egoist/mygo/ui"

// pressScaleKey 是 PressScale 记下的当前缩放。
type pressScaleKey struct{}

// PressScale 把 e 围绕中心缩放到 s（active:scale-95 一类的按下效果），并记下 s，
// 让挂在 e 上的弹层（AttachPopover）按未缩放的边框定位。
func PressScale(e ui.Element, s float32) ui.Element {
	*ui.Local(e, pressScaleKey{}, func() float32 { return 1 }) = s
	return e.Scale(s, s)
}

// AttachPopover 是 panel.AttachTo(anchor, at, self)，并抵消 anchor 的按下缩放（PressScale）：
// MyGo 按 anchor 缩放后的可视边框定位弹层，按钮按下、回弹的动画会让弹层跟着漂移。
// 缩放围绕中心，可视边框每边比布局边框少 (1-s)/2；把它加进 panel 的外边距，
// MyGo 翻到 anchor 另一侧时外边距随之反向，翻转后仍然贴着 anchor 未缩放的边框。
// margin 是 panel 原本的外边距（上、右、下、左），省略时为 0。按左到右排版：Start 视为左，End 视为右。
func AttachPopover(panel, anchor ui.Element, at, self ui.Anchor, margin ...float32) ui.Element {
	var m [4]float32
	copy(m[:], margin)
	panel.AttachTo(anchor, at, self)
	if s := *ui.Local(anchor, pressScaleKey{}, func() float32 { return 1 }); s != 1 {
		b := anchor.LayoutBounds()
		fx, fy := anchorFractions(at)
		// alongTarget 的偏移：横向是左外边距减右外边距，纵向是上外边距减下外边距。
		addOffset(&m[3], &m[1], (fx-0.5)*b.W*(1-s))
		addOffset(&m[0], &m[2], (fy-0.5)*b.H*(1-s))
	}
	return panel.Margin(m[0], m[1], m[2], m[3])
}

// addOffset 把 off 加到 lead - trail 上，外边距保持非负。
func addOffset(lead, trail *float32, off float32) {
	if off >= 0 {
		*lead += off
	} else {
		*trail -= off
	}
}

// anchorFractions 是 a 在框内横向、纵向的位置（0 到 1）。
func anchorFractions(a ui.Anchor) (fx, fy float32) {
	if a >= ui.AnchorTopStart {
		row := (a - ui.AnchorTopStart) % 3
		col := ui.Anchor(0)
		if a >= ui.AnchorTopEnd {
			col = 2
		}
		a = row*3 + col
	}
	return float32(a%3) / 2, float32(a/3) / 2
}
