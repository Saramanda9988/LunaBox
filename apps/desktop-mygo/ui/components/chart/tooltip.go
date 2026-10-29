package chart

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// tipItem 是提示框正文的一行：文字与它前面的色块（tooltip 的 labelColor）。
type tipItem struct {
	text         string
	border, fill ui.Color // 色块的描边与内部颜色
	borderW      float32  // 描边宽度（labelColor.borderWidth），0 为 1
}

// tipSpec 是一个提示框：标题（可以为空）、正文各行，以及它指向的点（canvas 坐标）。
type tipSpec struct {
	title string
	items []tipItem
	x, y  float32
}

// Chart.js tooltip 的默认值。
const (
	tipPadding      = 6
	tipCaret        = 5 // caretSize
	tipCaretPadding = 2
	tipRadius       = 6 // cornerRadius
	tipBox          = 12
	tipTitleMargin  = 6 // titleMarginBottom
	tipBodySpacing  = 2
)

type tipAlign int

const (
	alignStart tipAlign = iota // left / top
	alignCenter
	alignEnd // right / bottom
)

// tipGeom 是提示框的位置与对齐方式。
type tipGeom struct {
	x, y, w, h     float32
	xAlign, yAlign tipAlign
}

// tipSize 是 getTooltipSize：内边距 6，标题（粗体）一行 14.4 加 6 的下边距，正文每行 14.4、行间 2，
// 正文前留 12px 色块加 2px 间隔。
func tipSize(s tipSpec, m measureFunc) (w, h float32) {
	h = 2 * tipPadding
	if s.title != "" {
		h += jsLineHeight + tipTitleMargin
		w = m(s.title, true)
	}
	if n := len(s.items); n > 0 {
		h += float32(n)*jsLineHeight + float32(n-1)*tipBodySpacing
		for _, it := range s.items {
			w = max(w, m(it.text, false)+tipBox+2)
		}
	}
	return w + 2*tipPadding, h
}

// layoutTip 是 Tooltip 的 determineAlignment 与 getBackgroundPoint：竖直方向放不下一半时贴着点的上方
// 或下方，否则与点居中、左右按点在图表区的哪一半决定；最后限制在 canvas 之内。
func layoutTip(s tipSpec, w, h, W, H float32, area ui.Rect) tipGeom {
	g := tipGeom{w: w, h: h, yAlign: alignCenter, xAlign: alignCenter}
	switch {
	case s.y < h/2:
		g.yAlign = alignStart
	case s.y > H-h/2:
		g.yAlign = alignEnd
	}
	caret := float32(tipCaret + tipCaretPadding)
	switch {
	case g.yAlign == alignCenter:
		if s.x <= area.X+area.W/2 {
			g.xAlign = alignStart
		} else {
			g.xAlign = alignEnd
		}
	case s.x <= w/2:
		g.xAlign = alignStart
	case s.x >= W-w/2:
		g.xAlign = alignEnd
	}
	if (g.xAlign == alignStart && s.x+w+caret > W) || (g.xAlign == alignEnd && s.x-w-caret < 0) {
		g.xAlign = alignCenter
	}
	x := s.x
	switch g.xAlign {
	case alignEnd:
		x -= w
	case alignCenter:
		x -= w / 2
	}
	y := s.y
	switch g.yAlign {
	case alignStart:
		y += caret
	case alignEnd:
		y -= h + caret
	default:
		y -= h / 2
	}
	if g.yAlign == alignCenter {
		switch g.xAlign {
		case alignStart:
			x += caret
		case alignEnd:
			x -= caret
		}
	} else {
		switch g.xAlign {
		case alignStart:
			x -= tipRadius + tipCaret
		case alignEnd:
			x += tipRadius + tipCaret
		}
	}
	g.x = max(0, min(x, W-w))
	g.y = max(0, min(y, H-h))
	return g
}

// tipPath 是 drawBackground：圆角矩形，箭头（caret）画在对齐的那一边，一次填充。ox、oy 是提示框左上角
// 在窗口中的位置，caretX 是指向的点（窗口坐标）。
func tipPath(g tipGeom, ox, oy, caretX float32) *ui.Path {
	x, y, w, h, r := ox, oy, g.w, g.h, float32(tipRadius)
	const cs = tipCaret
	p := &ui.Path{}
	p.MoveTo(x+r, y)
	caretX2 := caretX
	switch g.xAlign {
	case alignStart:
		caretX2 = x + r + cs
	case alignEnd:
		caretX2 = x + w - r - cs
	}
	if g.yAlign == alignStart {
		p.LineTo(caretX2-cs, y).LineTo(caretX2, y-cs).LineTo(caretX2+cs, y)
	}
	p.LineTo(x+w-r, y).QuadTo(x+w, y, x+w, y+r)
	if g.yAlign == alignCenter && g.xAlign == alignEnd {
		my := y + h/2
		p.LineTo(x+w, my-cs).LineTo(x+w+cs, my).LineTo(x+w, my+cs)
	}
	p.LineTo(x+w, y+h-r).QuadTo(x+w, y+h, x+w-r, y+h)
	if g.yAlign == alignEnd {
		p.LineTo(caretX2+cs, y+h).LineTo(caretX2, y+h+cs).LineTo(caretX2-cs, y+h)
	}
	p.LineTo(x+r, y+h).QuadTo(x, y+h, x, y+h-r)
	if g.yAlign == alignCenter && g.xAlign == alignStart {
		my := y + h/2
		p.LineTo(x, my+cs).LineTo(x-cs, my).LineTo(x, my-cs)
	}
	p.LineTo(x, y+r).QuadTo(x, y, x+r, y)
	return p.Close()
}

// easeOutQuart 是 Chart.js 动画的默认缓动。
func easeOutQuart(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u*u
}

// tooltip 画 Chart.js 默认的提示框（rgba(0,0,0,.8) 底、圆角 6、5px 箭头、白字），位置随指向的点
// 以 400ms easeOutQuart 移动，与 Chart.js 的 tooltip 动画相同。W、H 是 canvas 的尺寸，area 是图表区。
func tooltip(c *ui.Context, m measureFunc, W, H float32, area ui.Rect, s tipSpec) {
	w, h := tipSize(s, m)
	g := layoutTip(s, w, h, W, H, area)
	caretX := s.x - g.x // 箭头相对提示框的位置（上下对齐且居中时指向点）
	box := ui.Column(c.Key("chart-tooltip")).Absolute().Left(g.x).Top(g.y).Size(w, h).Padding(tipPadding).PassThrough()
	box.Transition(ui.ElementTransition{Position: true, Duration: 400 * time.Millisecond, Ease: easeOutQuart})
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		x, y := pixelOrigin(p, r.X, r.Y)
		p.FillPath(tipPath(g, x, y, x+caretX), tooltipBg)
	})
	box.Children(func() {
		if s.title != "" {
			jsLine(c, s.title, white, true).Margin(0, 0, tipTitleMargin, 0)
		}
		for i, it := range s.items {
			row := ui.Row(c).Height(jsLineHeight).AlignItems(ui.Center).Gap(2)
			if i > 0 {
				row.Margin(tipBodySpacing, 0, 0, 0)
			}
			row.Children(func() {
				colorBox(c, it)
				jsText(c, it.text, white, false)
			})
		}
	})
}

// jsLine 把一行 Chart.js 文字放进 14.4 高的行盒里竖直居中：回退字体比行高高时也不撑开行。
func jsLine(c *ui.Context, s string, color ui.Color, bold bool) ui.Element {
	return ui.Row(c).Height(jsLineHeight).AlignItems(ui.Center).Children(func() {
		jsText(c, s, color, bold)
	})
}

// colorBox 是 _drawColorBox：12×12 先铺白底（multiKeyBackground），沿边描 borderWidth 宽的线（一半在外），
// 再在里面填 10×10 的数据集颜色。
func colorBox(c *ui.Context, it tipItem) {
	bw := it.borderW
	if bw <= 0 {
		bw = 1
	}
	ui.Box(c).Size(tipBox, tipBox).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, white, 0)
		p.Stroke(ui.Rect{X: r.X - bw/2, Y: r.Y - bw/2, W: r.W + bw, H: r.H + bw}, it.border, 0, bw)
		p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, it.fill, 0)
	})
}
