package chart

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/timefmt"
)

// lineInput 是 DurationLineChart 一次布局需要的量（canvas 尺寸与数据），与 UI 无关，便于单元测试。
type lineInput struct {
	W, H    float32 // canvas 的尺寸（可横向滚动时是滚动内容的宽度）
	measure measureFunc

	timeScale bool
	labels    []string  // 类目轴的标签
	from, to  time.Time // 时间轴的 min、max

	maxValue float64 // 可见数据集的最大值
	noData   bool    // hasPlayData 为 false：y 轴固定 0–600、stepSize 60
	yTitle   string
	legend   []string // 图例项（为空则不显示图例）
	minPad   float32  // layout.autoPadding：数据集的 getMaxOverflow（点的最大半径加边框）
}

// xTick 是 x 轴上的一个刻度：位置（图表区宽度的比例）、标签与标签宽度。
type xTick struct {
	at    float32
	label string
	w     float32
}

// legendItem 是图例里的一项在 canvas 中的位置（hitbox）。
type legendItem struct {
	x, y, w float32
}

// lineGeom 是 Chart.js 布局的结果，坐标都在 canvas 内（DIP）。
type lineGeom struct {
	W, H     float32
	area     ui.Rect
	legendH  float32
	legend   []legendItem
	yTicks   []float64
	yLabels  []string
	yMax     float64
	rotation float32  // x 轴标签的旋转角（度）
	xTicks   []xTick  // 自动跳过后显示的刻度
	allTicks int      // 自动跳过前的刻度数
	unit     timeUnit // 时间轴的单位
	xAxisH   float32
}

// layoutLegend 是 Legend._fitRows 与 _draw 的位置计算：居中的行，每项是 40×12 的色块、6px 间隔与文字，
// 项之间 padding 10；高度为行数 × (12 + 10) + 10。
func layoutLegend(texts []string, W float32, measure measureFunc) ([]legendItem, float32) {
	if len(texts) == 0 {
		return nil, 0
	}
	const pad = legendPadding
	lineH := float32(jsFontSize + pad)
	widths := make([]float32, len(texts))
	var rows [][]int
	var lineW []float32
	for i, s := range texts {
		widths[i] = legendBoxW + jsFontSize/2 + measure(s, false)
		if i == 0 || lineW[len(lineW)-1]+widths[i]+2*pad > W {
			rows = append(rows, nil)
			lineW = append(lineW, 0)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], i)
		lineW[len(lineW)-1] += widths[i] + pad
	}
	items := make([]legendItem, len(texts))
	for r, row := range rows {
		x := (pad + W - lineW[r]) / 2
		y := pad + float32(r)*lineH
		for _, i := range row {
			items[i] = legendItem{x: x, y: y, w: widths[i]}
			x += widths[i] + pad
		}
	}
	return items, float32(len(rows))*lineH + 10
}

// layoutLine 复刻 Chart.js 的 layouts.update：图例（fullSize）先占顶部；y 轴按当时的图表区高度取刻度、
// 定宽度；x 轴按当时的宽度取刻度、旋转、自动跳过、定高度；高度变了再按新高度重排一次 y 轴（x 轴不再重排）。
// 每个坐标轴伸出图表区的标签（getPadding）与数据集的 minPadding（点的半径）累计成 maxPadding，
// 最后图表区四边取各自的较大者（handleMaxPadding）。
func layoutLine(in lineInput) lineGeom {
	g := lineGeom{W: in.W, H: in.H}
	in.measure = memoize(in.measure)
	var titleW float32
	if in.yTitle != "" {
		titleW = jsLineHeight + 2*titlePadding
	}
	// chartArea 里累计的四边盒子尺寸与 maxPadding
	var caL, caT, caB float32
	padT, padL, padB, padR := in.minPad, in.minPad, in.minPad, in.minPad
	var w, h float32
	dims := func() (wChanged, hChanged bool) {
		nw := max(0, in.W-(max(padL, caL)+max(padR, 0)))
		nh := max(0, in.H-(max(padT, caT)+max(padB, caB)))
		wChanged, hChanged = nw != w, nh != h
		w, h = nw, nh
		return
	}
	w, h = in.W, in.H
	g.legend, g.legendH = layoutLegend(in.legend, in.W, in.measure)
	caT = g.legendH
	dims()

	var yW float32
	fitY := func() {
		if in.noData {
			g.yTicks = linearTicks(600, 0, 60)
		} else {
			g.yTicks = linearTicks(in.maxValue, linearTickLimit(h, false, 0), 0)
		}
		g.yMax = g.yTicks[len(g.yTicks)-1]
		g.yLabels = make([]string, len(g.yTicks))
		var labelW float32
		for i, v := range g.yTicks {
			g.yLabels[i] = timefmt.DurationChart(int(v))
			labelW = max(labelW, in.measure(g.yLabels[i], false))
		}
		caL -= yW
		yW = min(in.W/2, tickLength+titleW+labelW+2*tickPadding) // Scale.fit：标签两侧各一份 ticks.padding
		caL += yW
		half := float32(jsLineHeight/2 + tickPadding) // 首尾标签的半个行高加 ticks.padding
		padT, padB = max(padT, half), max(padB, half)
		dims()
	}
	fitX := func() bool {
		axisW := w
		all, unit := in.xTicksAll(axisW)
		g.unit, g.allTicks = unit, len(all)
		var widest float32
		for _, t := range all {
			widest = max(widest, t.w)
		}
		g.rotation = labelRotation(rotationInput{
			numTicks: len(all), widest: widest, height: jsLineHeight,
			chartWidth: in.W, maxWidth: axisW, maxHeight: in.H / 2, maxRotation: 45,
		})
		maxLength := axisW + max(caL, padL) + max(0, padR)
		keep := autoSkip(len(all), tickSize(g.rotation, widest, jsLineHeight, labelPadding), axisW, maxLength, false)
		g.xTicks = g.xTicks[:0]
		widest = 0
		for _, i := range keep {
			g.xTicks = append(g.xTicks, all[i])
			widest = max(widest, all[i].w)
		}
		rad := deg2rad(g.rotation)
		sin, cos := float32(math.Sin(rad)), float32(math.Cos(rad))
		xH := float32(tickLength)
		if len(g.xTicks) > 0 {
			xH = min(in.H/2, tickLength+sin*widest+cos*jsLineHeight+2*tickPadding)
		}
		// _calculatePadding：首尾标签伸出坐标轴两端的部分
		if n := len(g.xTicks); n > 0 && axisW > 0 {
			first, last := g.xTicks[0], g.xTicks[n-1]
			offL, offR := first.at*axisW, axisW-last.at*axisW
			var pl, pr float32
			if g.rotation != 0 {
				pl, pr = cos*first.w, sin*jsLineHeight
			} else {
				pl, pr = first.w/2, last.w/2
			}
			padL = max(padL, (pl-offL+tickPadding)*axisW/(axisW-offL))
			padR = max(padR, (pr-offR+tickPadding)*axisW/(axisW-offR))
		}
		caB += xH - g.xAxisH
		g.xAxisH = xH
		_, hChanged := dims()
		return hChanged
	}
	fitY()
	if fitX() {
		fitY()
	}
	left, top := max(caL, padL), max(caT, padT)
	g.area = ui.Rect{X: left, Y: top, W: w, H: h}
	return g
}

// memoize 记住每个标签的宽度：反复布局时同一批标签只量一次。
func memoize(m measureFunc) measureFunc {
	cache := map[string]float32{}
	return func(s string, bold bool) float32 {
		if bold {
			return m(s, bold)
		}
		w, ok := cache[s]
		if !ok {
			w = m(s, false)
			cache[s] = w
		}
		return w
	}
}

// xTicksAll 是自动跳过之前 x 轴的全部刻度：类目轴每个标签一个（offset false，首尾贴着图表区边缘；
// 只有一个标签时在最左边），时间轴按 capacity 选单位后每个单位一个。
func (in lineInput) xTicksAll(areaW float32) ([]xTick, timeUnit) {
	if in.timeScale {
		if !in.to.After(in.from) {
			return nil, unitDay
		}
		capacity := labelCapacity(areaW, in.measure(capacityLabel(in.from), false))
		u := determineUnit(in.from, in.to, capacity)
		times := timeTicks(in.from, in.to, u)
		out := make([]xTick, len(times))
		span := float32(in.to.Sub(in.from))
		for i, t := range times {
			label := t.Format(u.layout())
			out[i] = xTick{at: float32(t.Sub(in.from)) / span, label: label, w: in.measure(label, false)}
		}
		return out, u
	}
	n := len(in.labels)
	out := make([]xTick, n)
	for i, s := range in.labels {
		at := float32(0)
		if n > 1 {
			at = float32(i) / float32(n-1)
		}
		out[i] = xTick{at: at, label: s, w: in.measure(s, false)}
	}
	return out, unitDay
}

// yAt 是数值在 canvas 中的 y 坐标。
func (g lineGeom) yAt(v float64) float32 {
	if g.yMax <= 0 {
		return g.area.Y + g.area.H
	}
	return g.area.Y + g.area.H*(1-float32(v/g.yMax))
}

func (g lineGeom) xAt(f float32) float32 { return g.area.X + f*g.area.W }
