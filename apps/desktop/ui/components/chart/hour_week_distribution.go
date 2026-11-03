package chart

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// HourWeekState 是 HourWeekDistribution 跨帧的状态：两张柱状图各自悬停的柱子。
type HourWeekState struct {
	hour, week barState
}

// HourWeekProps 是 HourWeekDistribution 的属性，即 vo.PeriodStats 的 hourly_distribution 与
// weekday_distribution（weekday 0 为周日，与 JS Date.getDay 一致）。
type HourWeekProps struct {
	Hourly  []vo.HourPlayPoint
	Weekday []vo.WeekdayPlayPoint
	State   *HourWeekState
}

// weekdayLabels 是 stats.heatmap.weekdays，周日在前（与 time.Weekday 一致）。每帧取，跟随语言。
func weekdayLabels() [7]string {
	return [7]string{
		locales.StatsHeatmapWeekdaysSun(), locales.StatsHeatmapWeekdaysMon(), locales.StatsHeatmapWeekdaysTue(),
		locales.StatsHeatmapWeekdaysWed(), locales.StatsHeatmapWeekdaysThu(), locales.StatsHeatmapWeekdaysFri(),
		locales.StatsHeatmapWeekdaysSat(),
	}
}

// 柱子的颜色：emerald-500/400，高峰为 amber-500/400。
func barFills(t *theme.Theme) (fill, peak ui.Color) {
	return t.Pick(ui.Hex("#10b981"), ui.Hex("#34d399")), t.Pick(ui.Hex("#f59e0b"), ui.Hex("#fbbf24"))
}

// peakOf 返回时长最大的一项（同为最大取先出现的；全为 0 时为 -1）。
func peakOf(n int, at func(i int) (key, duration int)) (peak, maxDuration, total int) {
	peak = -1
	for i := range n {
		k, d := at(i)
		total += d
		if d > maxDuration {
			maxDuration, peak = d, k
		}
	}
	return peak, maxDuration, total
}

// HourWeekDistribution 对应 HourWeekDistribution.tsx：上面是 24 小时的柱状图（h-28，只标 0、6、12、18、23 时），
// 下面是星期的横向柱状图（h-44），高峰的柱子换成琥珀色，标题行右侧写“高峰 21:00 · 3小时”。
// 全部为 0 时只显示“本期间内无游玩记录”。
func HourWeekDistribution(c *ui.Context, t *theme.Theme, p HourWeekProps) ui.Element {
	peakHour, hourMax, totalHour := peakOf(len(p.Hourly), func(i int) (int, int) { return p.Hourly[i].Hour, p.Hourly[i].Duration })
	peakDay, dayMax, _ := peakOf(len(p.Weekday), func(i int) (int, int) { return p.Weekday[i].Weekday, p.Weekday[i].Duration })
	muted := t.Pick(theme.Brand500, theme.Brand400)
	if totalHour == 0 {
		// flex items-center justify-center text-sm text-brand-500 dark:text-brand-400
		return ui.Row(c).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			components.Text(c, locales.StatsHeatmapEmpty(), theme.TextSM, theme.WeightNormal, muted)
		})
	}
	root := ui.Column(c).Gap(16) // flex flex-col gap-4
	st := p.State
	if st == nil {
		st = ui.Local(root, "chart.HourWeekDistribution", func() HourWeekState { return HourWeekState{} })
	}
	fill, peakFill := barFills(t)
	strong := t.Pick(theme.Brand600, theme.Brand300)
	header := func(left, right string) {
		// flex items-center justify-between mb-2，两侧 text-xs
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Margin(0, 0, 8, 0).Children(func() {
			components.Text(c, left, theme.TextXS, theme.WeightNormal, muted)
			if right != "" {
				components.Text(c, right, theme.TextXS, theme.WeightNormal, strong)
			}
		})
	}
	return root.Children(func() {
		ui.Column(c.Key("hourly")).Children(func() {
			right := ""
			if peakHour >= 0 {
				right = locales.StatsTimeOfDayPeakHour(fmt.Sprintf("%02d:00", peakHour), timefmt.Duration(hourMax))
			}
			header(locales.StatsTimeOfDayHourLabel(), right)
			labels := make([]string, 24)
			for _, h := range []int{0, 6, 12, 18, 23} {
				labels[h] = fmt.Sprint(h)
			}
			values := make([]float64, len(p.Hourly))
			colors := make([]ui.Color, len(p.Hourly))
			for i, h := range p.Hourly {
				values[i] = float64(h.Duration)
				colors[i] = fill
				if h.Hour == peakHour && h.Duration > 0 {
					colors[i] = peakFill
				}
			}
			barChart(c, t, &st.hour, barSpec{
				H: 112, labels: labels, values: values, colors: colors, maxThickness: 18,
				title: func(i int) string { return fmt.Sprintf("%02d:00", i) },
				body:  playedOrNot,
			})
		})
		ui.Column(c.Key("weekday")).Children(func() {
			days := weekdayLabels()
			right := ""
			if peakDay >= 0 {
				right = locales.StatsTimeOfDayPeakWeekday(days[peakDay], timefmt.Duration(dayMax))
			}
			header(locales.StatsTimeOfDayWeekdayLabel(), right)
			labels := make([]string, len(p.Weekday))
			values := make([]float64, len(p.Weekday))
			colors := make([]ui.Color, len(p.Weekday))
			for i, w := range p.Weekday {
				labels[i] = days[w.Weekday%7]
				values[i] = float64(w.Duration)
				colors[i] = fill
				if w.Weekday == peakDay && w.Duration > 0 {
					colors[i] = peakFill
				}
			}
			barChart(c, t, &st.week, barSpec{
				H: 176, horizontal: true, labels: labels, values: values, colors: colors, thickness: 14,
				title: func(i int) string { return labels[i] },
				body:  playedOrNot,
			})
		})
	})
}

// playedOrNot 是两张图提示框的正文：时长，0 时为“未游玩”。
func playedOrNot(v float64) string {
	if v > 0 {
		return timefmt.Duration(int(v))
	}
	return locales.StatsHeatmapNoPlay()
}

// barState 是一张柱状图的悬停状态。
type barState struct {
	hover int  // 悬停的柱子，-1 为没有
	over  bool // 上一帧指针在 canvas 上
	ready bool
	width float32
}

// barSpec 是一张单数据集柱状图。
type barSpec struct {
	H            float32
	horizontal   bool
	labels       []string
	values       []float64
	colors       []ui.Color
	thickness    float32 // barThickness
	maxThickness float32 // maxBarThickness
	title        func(i int) string
	body         func(v float64) string
}

// barChart 画一张 Chart.js 柱状图：数值轴 maxTicksLimit 4、以“5时 2分”标注、显示网格；类目轴不显示网格。
// 柱子圆角 2（贴着基线的一端不圆），悬停（mode nearest、intersect true）时颜色加深并显示提示框。
func barChart(c *ui.Context, t *theme.Theme, s *barState, b barSpec) ui.Element {
	if !s.ready {
		s.hover, s.ready = -1, true
	}
	col := chartColors(t)
	m := jsMeasure(c)
	canvas := ui.Box(c.Key("bar-canvas")).Height(b.H)
	if r := canvas.Bounds(); r.W > 0 {
		s.width = r.W
	}
	W := s.width
	if W <= 0 {
		W = 400
	}
	maxV := 0.0
	for _, v := range b.values {
		maxV = max(maxV, v)
	}
	n := len(b.labels) // 类目数取标签数（原版 labels 固定 24 个小时），数据少于标签时缺的为 0
	if len(b.values) < n {
		b.values = append(b.values, make([]float64, n-len(b.values))...)
		b.colors = append(b.colors, make([]ui.Color, n-len(b.colors))...)
	}
	g := layoutBars(barInput{W: W, H: b.H, measure: memoize(m), horizontal: b.horizontal, labels: b.labels, maxValue: maxV, maxTicks: 4})
	a := g.area
	rects := make([]ui.Rect, n)
	for i := range rects {
		rects[i] = g.barRect(i, n, b.values[i], b.horizontal, b.thickness, b.maxThickness)
	}

	// Chart.js 只在指针位于图表区内时重新取悬停项（落在柱子上才算），在 canvas 的其余部分保持，离开 canvas 清除。
	x, y, over := canvas.PointerPosition()
	switch {
	case !over:
		if s.over { // mouseout
			s.hover = -1
		}
	case a.Contains(x, y):
		s.hover = -1
		for i, r := range rects {
			if barContains(r, x, y, b.horizontal) {
				s.hover = i
			}
		}
	}
	s.over = over

	canvas.Draw(func(p *ui.Painter, r ui.Rect) {
		ox, oy := r.X, r.Y
		for _, v := range g.vTicks {
			at := g.vAt(v, b.horizontal)
			if b.horizontal {
				p.Line(ox+at, oy+a.Y, ox+at, oy+a.Y+a.H+tickLength, 1, col.grid)
			} else {
				p.Line(ox+a.X-tickLength, oy+at, ox+a.X+a.W, oy+at, 1, col.grid)
			}
		}
		p.Line(ox+a.X, oy+a.Y, ox+a.X, oy+a.Y+a.H, 1, jsBorder)
		p.Line(ox+a.X, oy+a.Y+a.H, ox+a.X+a.W, oy+a.Y+a.H, 1, jsBorder)
		px, py := pixelOrigin(p, ox, oy)
		for i, rr := range rects {
			color := b.colors[i]
			if i == s.hover {
				color = hoverColor(color)
			}
			rr.X += px
			rr.Y += py
			p.FillPath(barPath(rr, b.horizontal), color)
		}
	})
	return canvas.Children(func() {
		// 数值轴标签
		for i, v := range g.vTicks {
			at := g.vAt(v, b.horizontal)
			if b.horizontal {
				place(c, at, a.Y+a.H+tickLength+tickPadding+jsLineHeight/2, ui.Center, ui.Center, func() {
					jsText(c, g.vLabels[i], col.text, false)
				})
			} else {
				place(c, a.X-tickLength-tickPadding, at, ui.End, ui.Center, func() {
					jsText(c, g.vLabels[i], col.text, false)
				})
			}
		}
		// 类目轴标签
		for i, s := range b.labels {
			if s == "" {
				continue
			}
			at := g.cAt(i, n, b.horizontal)
			switch {
			case b.horizontal:
				place(c, a.X-tickLength-tickPadding, at, ui.End, ui.Center, func() { jsText(c, s, col.text, false) })
			case g.rotation == 0:
				place(c, at, a.Y+a.H+tickLength+tickPadding+jsLineHeight/2, ui.Center, ui.Center, func() {
					jsText(c, s, col.text, false)
				})
			default:
				rot := g.rotation
				place(c, at, a.Y+a.H+tickLength+tickPadding+jsLineHeight/2*cosDeg(rot), ui.End, ui.Center, func() {
					jsText(c, s, col.text, false).Rotate(-rot).TransformOrigin(1, 0.5)
				})
			}
		}
		if i := s.hover; i >= 0 && i < n {
			// tooltipPosition：竖直柱是顶端中点，水平柱是末端中点
			x, y := rects[i].X+rects[i].W/2, rects[i].Y
			if b.horizontal {
				x, y = rects[i].X+rects[i].W, rects[i].Y+rects[i].H/2
			}
			tooltip(c, m, W, b.H, a, tipSpec{title: b.title(i), x: x, y: y,
				items: []tipItem{{text: b.body(b.values[i]), border: jsBorder, fill: b.colors[i]}}})
		}
	})
}

// barContains 是 BarElement.inRange：指针在柱子的矩形里（0 长度的柱子也有一条线可以命中）。
func barContains(r ui.Rect, x, y float32, horizontal bool) bool {
	if horizontal {
		return y >= r.Y && y <= r.Y+r.H && x >= r.X && x <= r.X+r.W
	}
	return x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H
}

// barPath 是 borderRadius 2 的柱子：borderSkipped "start" 的一端（基线）是直角，另一端两角半径 2，
// 半径不超过柱子短边的一半。
func barPath(r ui.Rect, horizontal bool) *ui.Path {
	rad := min(float32(2), r.W/2, r.H/2)
	p := &ui.Path{}
	if rad <= 0 {
		return p.MoveTo(r.X, r.Y).LineTo(r.X+r.W, r.Y).LineTo(r.X+r.W, r.Y+r.H).LineTo(r.X, r.Y+r.H).Close()
	}
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	if horizontal { // 右端圆
		return p.MoveTo(x0, y0).LineTo(x1-rad, y0).QuadTo(x1, y0, x1, y0+rad).
			LineTo(x1, y1-rad).QuadTo(x1, y1, x1-rad, y1).LineTo(x0, y1).Close()
	}
	return p.MoveTo(x0, y1).LineTo(x0, y0+rad).QuadTo(x0, y0, x0+rad, y0).
		LineTo(x1-rad, y0).QuadTo(x1, y0, x1, y0+rad).LineTo(x1, y1).Close()
}
