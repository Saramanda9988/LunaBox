package chart

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	plot "github.com/egoist/mygo/ui/chart"

	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// Point 是一天（年维度为一个月）的游戏时长。
type Point struct {
	Label   string    // 分类轴的标签，后端返回的 YYYY-MM-DD 或 YYYY-MM
	Time    time.Time // 时间轴上的位置
	Seconds int
}

// Series 是一个数据集（Chart.js 的 dataset），stats 页“常玩游戏趋势”每个游戏一条。
type Series struct {
	Label   string
	Seconds []int    // 与 DurationLineProps.Points 一一对应
	Color   ui.Color // borderColor；backgroundColor（点的填充、图例与提示框色块）是它的 50% 透明度
}

// State 是图表跨帧保留的状态，由调用方持有（不传时挂在图表元素上）。
type State struct {
	selected int          // 悬停（或键盘）选中的下标，-1 为没有
	over     bool         // 上一帧指针在 canvas 上
	ready    bool         // selected 已初始化
	width    float32      // 上一帧容器的宽度
	hidden   map[int]bool // 图例里点掉的数据集（Chart.js 图例的默认 onClick）
	scroll   ui.ScrollState
	thumb    ui.Point // 拖动滚动条滑块时的位置
}

// DurationLineProps 是 DurationLineChart 的属性。前六项是最初的接口，其余可选，零值即原版默认值。
type DurationLineProps struct {
	Points    []Point
	TimeScale bool      // scaleType="time"：按日期稀疏分布的点
	From, To  time.Time // 时间轴的范围（TimeScale 时），见 PaddedRange、DataRange
	Height    float32
	Label     string // 数据集名称，显示在提示框和图例里
	State     *State

	// Series 是多个数据集，Points 只提供标签与时间；为空时 Points 本身是唯一的数据集（名称 Label）。
	Series []Series
	// Color 是单数据集的 borderColor，默认 rgb(59, 130, 246)（GameStatsPanel）。
	Color ui.Color
	// Tension 是 Chart.js 数据集的 tension（GameStatsPanel 与 stats 页用 0.3，时间轴 0.2）。
	// 0 用单调曲线（cubicInterpolationMode "monotone"）。
	Tension float32
	// PointRadius、PointHoverRadius 是点的半径，默认 3 与 4（TimeScale 时 4 与 6，同 GameStatsPanel）。
	PointRadius, PointHoverRadius float32
	// ShowLegend 显示顶部图例（原版默认显示，GameStatsPanel 关掉了），点击图例项隐藏/显示数据集。
	ShowLegend bool
	// YAxisTitle 是 y 轴标题（stats 页的“时长”）。
	YAxisTitle string
	// MinPointWidth 是 HorizontalScrollChart 的 minPointWidth：点数 × 它超过容器宽度时图表变宽、可横向滚动。
	// 0 为原版默认的 20，负数不滚动。
	MinPointWidth float32
}

// lineColor 是原版 GameStatsPanel 数据集的 borderColor（backgroundColor 是它的 50% 透明度）。
var lineColor = ui.RGBA(59, 130, 246, 1)

const (
	dayMS         = 24 * time.Hour
	labelPadding  = 12 // ticks.autoSkipPadding
	lineWidth     = 3  // elements.line.borderWidth
	scrollbarSize = 6  // ::-webkit-scrollbar 的 height
)

// series 返回要画的数据集：Series，或由 Points 组成的一个。
func (p DurationLineProps) series() []Series {
	if len(p.Series) > 0 {
		return p.Series
	}
	color := p.Color
	if color == (ui.Color{}) {
		color = lineColor
	}
	values := make([]int, len(p.Points))
	for i, pt := range p.Points {
		values[i] = pt.Seconds
	}
	return []Series{{Label: p.Label, Seconds: values, Color: color}}
}

func (p DurationLineProps) radii() (r, hover float32) {
	r, hover = 3, 4
	if p.TimeScale {
		r, hover = 4, 6
	}
	if p.PointRadius > 0 {
		r = p.PointRadius
	}
	if p.PointHoverRadius > 0 {
		hover = p.PointHoverRadius
	}
	return r, hover
}

// DurationLineChart 对应 DurationLineChart.tsx 与它外面的 HorizontalScrollChart.tsx：Chart.js 折线图，
// y 轴从 0 开始、以“5时 2分”标注（没有游玩数据时固定 0–10 分钟、每分钟一格）；x 轴是类目轴，或
// minUnit 为 day 的时间轴（刻度按 yyyy-MM-dd、yyyy-MM、yyyy 标注）。标签放不下时先旋转（最多 45°），
// 再按 autoSkipPadding 12 跳过。悬停（interaction mode index、intersect false）时放大该下标的点并显示
// 提示框；点数 × MinPointWidth 超过容器宽度时可以横向滚动。
func DurationLineChart(c *ui.Context, t *theme.Theme, p DurationLineProps) ui.Element {
	wrap := ui.Column(c).Height(p.Height)
	s := p.State
	if s == nil {
		s = ui.Local(wrap, "chart.DurationLineChart", func() State { return State{} })
	}
	if !s.ready {
		s.selected, s.ready = -1, true
	}
	if b := wrap.Bounds(); b.W > 0 {
		s.width = b.W
	}
	contW := s.width
	if contW <= 0 {
		contW = 800 // 第一帧还不知道宽度（原版此时也不滚动）
	}
	n := len(p.Points)
	minPW := p.MinPointWidth
	if minPW == 0 {
		minPW = 20
	}
	var minW float32
	if n > 1 && minPW > 0 {
		minW = float32(n) * minPW
	}
	scrollable := s.width > 0 && minW > s.width
	return wrap.Children(func() {
		if !scrollable {
			s.scroll = ui.ScrollState{}
			lineCanvas(c, t, p, s, contW, p.Height).Grow(1)
			return
		}
		h := p.Height - scrollbarSize
		ui.ScrollHorizontal(c.Key("chart-scroll")).Height(h).TrackScroll(&s.scroll).Children(func() {
			lineCanvas(c, t, p, s, minW, h).Width(minW).Shrink(0)
		})
		hScrollbar(c, t, s, contW)
	})
}

// input 汇总一次布局的输入：可见数据集决定 y 轴的范围；hasPlayData 看全部数据（原版在组件外算好）。
func (p DurationLineProps) input(s *State, m measureFunc, W, H float32) lineInput {
	all := p.series()
	n := len(p.Points)
	hasData, maxV := false, 0.0
	for i, sr := range all {
		for _, v := range sr.Seconds {
			hasData = hasData || v > 0
			if !s.hidden[i] {
				maxV = max(maxV, float64(v))
			}
		}
	}
	radius, hoverRadius := p.radii()
	// layout.autoPadding：getMaxOverflow，首尾点的（较大的）半径加 1px 边框，没有点时是线宽的一半
	overflow := float32(lineWidth) / 2
	if n > 0 {
		overflow = max(overflow, max(radius, hoverRadius)+1)
	}
	in := lineInput{W: W, H: H, measure: m, timeScale: p.TimeScale, from: p.From, to: p.To,
		maxValue: maxV, noData: !hasData, yTitle: p.YAxisTitle, minPad: overflow}
	if !p.TimeScale {
		in.labels = make([]string, n)
		for i, pt := range p.Points {
			in.labels[i] = pt.Label
		}
	}
	if p.ShowLegend {
		for _, sr := range all {
			in.legend = append(in.legend, sr.Label)
		}
	}
	return in
}

// lineCanvas 是 Chart.js 的 canvas：W×H 的元素，网格、折线与点画在它的 Draw 里，文字是绝对定位的子元素。
func lineCanvas(c *ui.Context, t *theme.Theme, p DurationLineProps, s *State, W, H float32) ui.Element {
	col := chartColors(t)
	m := jsMeasure(c)
	all := p.series()
	if s.hidden == nil {
		s.hidden = map[int]bool{}
	}
	n := len(p.Points)
	radius, hoverRadius := p.radii()
	in := p.input(s, m, W, H)
	overflow := in.minPad
	g := layoutLine(in)
	a := g.area

	// 点在图表区里的位置（比例）与 canvas 坐标
	fx := make([]float32, n)
	for i, pt := range p.Points {
		switch {
		case p.TimeScale && p.To.After(p.From):
			fx[i] = float32(pt.Time.Sub(p.From)) / float32(p.To.Sub(p.From))
		case n > 1:
			fx[i] = float32(i) / float32(n-1)
		}
	}
	px := make([]float32, n)
	for i := range fx {
		px[i] = g.xAt(fx[i])
	}
	py := make([][]float32, len(all))
	for k, sr := range all {
		py[k] = make([]float32, n)
		for i := range n {
			v := 0
			if i < len(sr.Seconds) {
				v = sr.Seconds[i]
			}
			py[k][i] = g.yAt(float64(v))
		}
	}
	sel := s.selected
	if sel >= n {
		sel = -1
	}

	canvas := ui.Box(c.Key("chart-canvas")).Height(H).Label(p.Label)
	canvas.Draw(func(pt *ui.Painter, r ui.Rect) {
		ox, oy := r.X, r.Y
		ax, ay := ox+a.X, oy+a.Y
		// 网格：y 刻度横线伸进 y 轴 8px（tick mark），x 刻度竖线伸进 x 轴 8px
		for _, v := range g.yTicks {
			y := oy + g.yAt(v)
			pt.Line(ax-tickLength, y, ax+a.W, y, 1, col.grid)
		}
		for _, tk := range g.xTicks {
			x := ox + g.xAt(tk.at)
			pt.Line(x, ay, x, ay+a.H+tickLength, 1, col.grid)
		}
		// 坐标轴线（border，Chart.defaults.borderColor）
		pt.Line(ax, ay, ax, ay+a.H, 1, jsBorder)
		pt.Line(ax, ay+a.H, ax+a.W, ay+a.H, 1, jsBorder)

		// 数据集从后往前画，第一个在最上面（Chart.js 的 _drawDatasets）
		clip := overflow
		pt.Clip(ui.Rect{X: ax - clip, Y: ay - clip, W: a.W + 2*clip, H: a.H + 2*clip}, 0, func() {
			for k := len(all) - 1; k >= 0; k-- {
				if s.hidden[k] {
					continue
				}
				color := all[k].Color
				xs, ys := make([]float32, n), make([]float32, n)
				for i := range n {
					xs[i], ys[i] = ox+px[i], oy+py[k][i]
				}
				area := ui.Rect{X: ax, Y: ay, W: a.W, H: a.H}
				switch {
				case n > 1 && p.Tension > 0:
					v := pt.Visible()
					pt.StrokePolyline(linePolyline(xs, ys, p.Tension, area, v.X-lineWidth, v.X+v.W+lineWidth), lineWidth, color)
				case n > 1:
					pt.StrokePath(linePath(xs, ys, p.Tension, area), lineWidth, color)
				}
				for i := range n {
					rr, fill, stroke := radius, color.Alpha(0.5), color
					if i == sel {
						rr, fill, stroke = hoverRadius, hoverColor(fill), hoverColor(stroke)
					}
					drawPoint(pt, xs[i], ys[i], rr, fill, stroke)
				}
			}
		})

		// 图例色块：40×12，填 backgroundColor、描 3px borderColor；隐藏的数据集文字加删除线
		for k, it := range g.legend {
			color := all[k].Color
			box := ui.Rect{X: ox + it.x, Y: oy + it.y, W: legendBoxW, H: jsFontSize}
			pt.Fill(box, color.Alpha(0.5), 0)
			pt.Stroke(ui.Rect{X: box.X - lineWidth/2, Y: box.Y - lineWidth/2, W: box.W + lineWidth, H: box.H + lineWidth}, color, 0, lineWidth)
			if s.hidden[k] {
				x0 := box.X + legendBoxW + jsFontSize/2
				pt.Line(x0, box.Y+jsFontSize/2, x0+it.w-legendBoxW-jsFontSize/2, box.Y+jsFontSize/2, 2, col.text)
			}
		}
	})

	return canvas.Children(func() {
		// y 轴标签：右对齐在图表区左边 11px 处，竖直居中在刻度上
		for i, v := range g.yTicks {
			place(c, a.X-tickLength-tickPadding, g.yAt(v), ui.End, ui.Center, func() {
				jsText(c, g.yLabels[i], col.text, false)
			})
		}
		// x 轴标签：不旋转时居中在刻度下方；旋转时右端对着刻度，绕右端中点转 -rotation
		rad := deg2rad(g.rotation)
		for _, tk := range g.xTicks {
			x := g.xAt(tk.at)
			top := a.Y + a.H + tickLength + tickPadding
			if g.rotation == 0 {
				place(c, x, top+jsLineHeight/2, ui.Center, ui.Center, func() {
					jsText(c, tk.label, col.text, false)
				})
				continue
			}
			place(c, x, top+jsLineHeight/2*float32(math.Cos(rad)), ui.End, ui.Center, func() {
				jsText(c, tk.label, col.text, false).Rotate(-g.rotation).TransformOrigin(1, 0.5)
			})
		}
		// y 轴标题：竖排在 y 轴最左侧（padding.top 4 + 半行高），居中于图表区
		if p.YAxisTitle != "" {
			place(c, titlePadding+jsLineHeight/2, a.Y+a.H/2, ui.Center, ui.Center, func() {
				jsText(c, p.YAxisTitle, col.text, false).Rotate(-90)
			})
		}
		// 图例文字与点击区域
		for k, it := range g.legend {
			k := k
			ui.Box(c.Key(k)).Absolute().Left(it.x).Top(it.y).Size(it.w, jsFontSize).OnClick(func() {
				s.hidden[k] = !s.hidden[k]
			}).Children(func() {
				place(c, legendBoxW+jsFontSize/2, jsFontSize/2, ui.Start, ui.Center, func() {
					jsText(c, all[k].Label, col.text, false)
				})
			})
		}

		// 图表区：指针选中最近的下标（mode index），键盘左右键移动
		area := ui.Box(c.Key("chart-area")).Absolute().Left(a.X).Top(a.Y).Size(a.W, a.H)
		if n > 0 {
			describe := func(i int) string { return tipTitle(p, i) + " " + tipItems(p, all, s, i)[0].text }
			prev := s.selected
			_, _, overArea := area.PointerPosition()
			_, _, overCanvas := canvas.PointerPosition()
			plot.Select(c, area, &s.selected, fx, describe)
			// Chart.js 只在指针位于图表区内时更新选中项，在 canvas 的其余部分保持不变，离开 canvas（mouseout）才清除。
			switch {
			case !overCanvas && s.over:
				s.selected = -1
			case s.selected < 0 && prev >= 0 && !overArea && overCanvas:
				s.selected = prev
			}
			s.over = overCanvas
		} else {
			s.selected = -1
		}

		if i := s.selected; i >= 0 && i < n {
			items := tipItems(p, all, s, i)
			if len(items) > 0 {
				var sx, sy float32
				cnt := 0
				for k := range all {
					if !s.hidden[k] {
						sx += px[i]
						sy += py[k][i]
						cnt++
					}
				}
				tooltip(c, m, W, H, a, tipSpec{title: tipTitle(p, i), items: items, x: sx / float32(cnt), y: sy / float32(cnt)})
			}
		}
	})
}

// tipTitle 是提示框标题：类目轴的标签，时间轴按 tooltipFormat yyyy-MM-dd。
func tipTitle(p DurationLineProps, i int) string {
	if p.TimeScale {
		return p.Points[i].Time.Format("2006-01-02")
	}
	return p.Points[i].Label
}

// tipItems 是 index 模式下各可见数据集在下标 i 的值：“名称: 1小时5分钟”。
func tipItems(p DurationLineProps, all []Series, s *State, i int) []tipItem {
	var items []tipItem
	for k, sr := range all {
		if s.hidden[k] {
			continue
		}
		v := 0
		if i < len(sr.Seconds) {
			v = sr.Seconds[i]
		}
		text := timefmt.Duration(v)
		if sr.Label != "" {
			text = sr.Label + ": " + text
		}
		items = append(items, tipItem{text: text, border: sr.Color, fill: sr.Color.Alpha(0.5)})
	}
	if len(items) == 0 {
		items = append(items, tipItem{})
	}
	return items
}

// drawPoint 画 Chart.js 的圆点：填充半径 r，再以 1px 线宽沿半径 r 描边（线宽一半在外）。
func drawPoint(p *ui.Painter, x, y, r float32, fill, stroke ui.Color) {
	p.Fill(ui.Rect{X: x - r, Y: y - r, W: 2 * r, H: 2 * r}, fill, r)
	o := r + 0.5
	p.Stroke(ui.Rect{X: x - o, Y: y - o, W: 2 * o, H: 2 * o}, stroke, o, 1)
}

// linePath 是折线的路径（窗口坐标）。tension > 0 时是 Chart.js 默认的样条（splineCurve，控制点限制在
// 图表区内，capBezierPoints）；否则用单调曲线。
func linePath(xs, ys []float32, tension float32, area ui.Rect) *ui.Path {
	n := len(xs)
	if tension <= 0 {
		fx, fy := make([]float32, n), make([]float32, n)
		for i := range n {
			fx[i] = (xs[i] - area.X) / area.W
			fy[i] = 1 - (ys[i]-area.Y)/area.H
		}
		return plot.Line(area, fx, fy, plot.CurveMonotone)
	}
	cps := splineControls(xs, ys, tension, area)
	path := &ui.Path{}
	path.MoveTo(xs[0], ys[0])
	for i := 1; i < n; i++ {
		path.CubeTo(cps[i-1].x2, cps[i-1].y2, cps[i].x1, cps[i].y1, xs[i], ys[i])
	}
	return path
}

// splineControl 是一个点两侧的贝塞尔控制点：(x1, y1) 在它之前，(x2, y2) 在它之后。
type splineControl struct{ x1, y1, x2, y2 float32 }

// splineControls 是 Chart.js 的 splineCurve 与 capBezierPoints：tension 决定控制点离开数据点的远近，
// 图表区内的点的控制点限制在图表区里。
func splineControls(xs, ys []float32, tension float32, area ui.Rect) []splineControl {
	n := len(xs)
	cps := make([]splineControl, n)
	for i := range n {
		pi, ni := max(i-1, 0), min(i+1, n-1)
		d01 := float32(math.Hypot(float64(xs[i]-xs[pi]), float64(ys[i]-ys[pi])))
		d12 := float32(math.Hypot(float64(xs[ni]-xs[i]), float64(ys[ni]-ys[i])))
		var s01, s12 float32
		if d01+d12 > 0 {
			s01, s12 = d01/(d01+d12), d12/(d01+d12)
		}
		fa, fb := tension*s01, tension*s12
		dx, dy := xs[ni]-xs[pi], ys[ni]-ys[pi]
		cps[i] = splineControl{xs[i] - fa*dx, ys[i] - fa*dy, xs[i] + fb*dx, ys[i] + fb*dy}
	}
	in := func(i int) bool {
		const eps = 0.5
		return xs[i] >= area.X-eps && xs[i] <= area.X+area.W+eps && ys[i] >= area.Y-eps && ys[i] <= area.Y+area.H+eps
	}
	capX := func(v float32) float32 { return max(area.X, min(area.X+area.W, v)) }
	capY := func(v float32) float32 { return max(area.Y, min(area.Y+area.H, v)) }
	for i := range n {
		if !in(i) {
			continue
		}
		if i > 0 && in(i-1) {
			cps[i].x1, cps[i].y1 = capX(cps[i].x1), capY(cps[i].y1)
		}
		if i < n-1 && in(i+1) {
			cps[i].x2, cps[i].y2 = capX(cps[i].x2), capY(cps[i].y2)
		}
	}
	return cps
}

// polylineStep 是把样条切成折线时每一小段的长度（DIP），2px 宽的线看不出折角。
const polylineStep = 3

// linePolyline 把 linePath 的样条（tension > 0）按 polylineStep 切成折线，只保留横向与 [lo, hi] 相交的那几段。
// 交给 StrokePolyline：GPU 上不透明的折线逐段画成圆角矩形，不像 StrokePath 那样把整条线光栅化成蒙版、
// 放进 MyGo 的蒙版图集——年视图的趋势图宽 365×20 DIP，十几条线滚动时的蒙版会把图集撑到 4096²
// （内存与显存各 16MB），之后不再缩小。
func linePolyline(xs, ys []float32, tension float32, area ui.Rect, lo, hi float32) [][2]float32 {
	n := len(xs)
	cps := splineControls(xs, ys, tension, area)
	var pts [][2]float32
	for i := 1; i < n; i++ {
		a, b := cps[i-1], cps[i]
		x0, y0, x3, y3 := xs[i-1], ys[i-1], xs[i], ys[i]
		if max(x0, x3, a.x2, b.x1) < lo || min(x0, x3, a.x2, b.x1) > hi {
			if len(pts) > 0 {
				break // x 单调递增：看得见的段是连续的一段，已经过去了
			}
			continue
		}
		if len(pts) == 0 {
			pts = append(pts, [2]float32{x0, y0})
		}
		// 段数按控制多边形的长度估计（不短于曲线本身）
		poly := hypot(a.x2-x0, a.y2-y0) + hypot(b.x1-a.x2, b.y1-a.y2) + hypot(x3-b.x1, y3-b.y1)
		k := min(max(int(math.Ceil(float64(poly/polylineStep))), 1), 64)
		for j := 1; j <= k; j++ {
			t := float32(j) / float32(k)
			u := 1 - t
			c0, c1, c2, c3 := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
			pts = append(pts, [2]float32{c0*x0 + c1*a.x2 + c2*b.x1 + c3*x3, c0*y0 + c1*a.y2 + c2*b.y1 + c3*y3})
		}
	}
	return pts
}

func hypot(dx, dy float32) float32 { return float32(math.Hypot(float64(dx), float64(dy))) }

// PaddedRange 对应 createPaddedTimeRange：首尾留出跨度的 4%（1–14 天），只有一天时前后各留 7 天。
func PaddedRange(start, end time.Time) (time.Time, time.Time) {
	if start.Equal(end) {
		return start.Add(-7 * dayMS), end.Add(7 * dayMS)
	}
	span := max(dayMS, end.Sub(start))
	pad := min(14*dayMS, max(dayMS, time.Duration(float64(span)*0.04)))
	return start.Add(-pad), end.Add(pad)
}

// DataRange 是不设 min/max 时 Chart.js 时间轴的范围：首尾两个点；只有一个点（或没有点）时前后各留一天。
func DataRange(points []Point, now time.Time) (time.Time, time.Time) {
	if len(points) == 0 {
		return now.Add(-dayMS), now.Add(dayMS)
	}
	from, to := points[0].Time, points[0].Time
	for _, p := range points[1:] {
		if p.Time.Before(from) {
			from = p.Time
		}
		if p.Time.After(to) {
			to = p.Time
		}
	}
	if from.Equal(to) {
		return from.Add(-dayMS), to.Add(dayMS)
	}
	return from, to
}
