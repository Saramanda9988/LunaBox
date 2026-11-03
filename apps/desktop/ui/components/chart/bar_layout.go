package chart

import (
	"math"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/timefmt"
)

// barInput 是 Chart.js 柱状图（一个数据集、一条类目轴加一条数值轴）一次布局的输入。
type barInput struct {
	W, H       float32
	measure    measureFunc
	horizontal bool     // indexAxis "y"：类目在左边、数值在下边
	labels     []string // 类目轴每个刻度的标签（"" 不显示，仍占位置）
	maxValue   float64
	maxTicks   int // 数值轴的 ticks.maxTicksLimit
}

// barGeom 是布局结果（canvas 坐标）。
type barGeom struct {
	area     ui.Rect
	vTicks   []float64 // 数值轴刻度（自动跳过后）
	vLabels  []string
	vMax     float64
	rotation float32 // 下边坐标轴标签的旋转角
}

// vAt 是数值在数值轴上的位置：竖直柱是 y 坐标，水平柱是 x 坐标。
func (g barGeom) vAt(v float64, horizontal bool) float32 {
	f := float32(0)
	if g.vMax > 0 {
		f = float32(v / g.vMax)
	}
	if horizontal {
		return g.area.X + f*g.area.W
	}
	return g.area.Y + g.area.H*(1-f)
}

// cAt 是第 i 个类目的中心（offset: true，每个类目占一格，刻度在格子中间）。
func (g barGeom) cAt(i, n int, horizontal bool) float32 {
	f := (float32(i) + 0.5) / float32(n)
	if horizontal {
		return g.area.Y + f*g.area.H
	}
	return g.area.X + f*g.area.W
}

// layoutBars 与 layoutLine 相同的 Chart.js 布局顺序（左轴、下轴、必要时再排左轴），坐标轴换成
// 类目轴（offset，网格不显示）与 beginAtZero 的数值轴；柱状图没有 minPadding。
func layoutBars(in barInput) barGeom {
	var g barGeom
	n := len(in.labels)
	widths := make([]float32, n)
	var widest float32
	for i, s := range in.labels {
		widths[i] = in.measure(s, false)
		widest = max(widest, widths[i])
	}
	var caL, caB, padT, padL, padB, padR float32
	w, h := in.W, in.H
	dims := func() bool {
		nw := max(0, in.W-(max(padL, caL)+padR))
		nh := max(0, in.H-(padT+max(padB, caB)))
		changed := nh != h
		w, h = nw, nh
		return changed
	}
	values := func(length float32, horizontal bool) {
		g.vTicks = linearTicks(in.maxValue, linearTickLimit(length, horizontal, in.maxTicks), 0)
		g.vMax = g.vTicks[len(g.vTicks)-1]
		g.vLabels = make([]string, len(g.vTicks))
		for i, v := range g.vTicks {
			g.vLabels[i] = timefmt.DurationChart(int(v))
		}
	}
	half := float32(jsLineHeight/2 + tickPadding)
	var leftW, bottomH float32
	fitLeft := func() {
		var labelW float32
		if in.horizontal {
			labelW = widest
		} else {
			values(h, false)
			for _, s := range g.vLabels {
				labelW = max(labelW, in.measure(s, false))
			}
		}
		caL -= leftW
		leftW = min(in.W/2, tickLength+labelW+2*tickPadding)
		caL += leftW
		padT, padB = max(padT, half), max(padB, half)
		dims()
	}
	fitBottom := func() bool {
		axisW := w
		var ticksW []float32
		var at []float32 // 刻度在坐标轴上的比例
		offset := !in.horizontal
		if in.horizontal {
			values(axisW, true)
			for i, s := range g.vLabels {
				ticksW = append(ticksW, in.measure(s, false))
				at = append(at, float32(i)/float32(max(1, len(g.vLabels)-1)))
			}
		} else {
			ticksW = widths
			for i := range n {
				at = append(at, (float32(i)+0.5)/float32(n))
			}
		}
		var wd float32
		for _, x := range ticksW {
			wd = max(wd, x)
		}
		g.rotation = labelRotation(rotationInput{
			numTicks: len(ticksW), widest: wd, height: jsLineHeight, chartWidth: in.W, maxWidth: axisW,
			maxHeight: in.H / 2, offset: offset, maxRotation: 50,
		})
		keep := make([]int, len(ticksW))
		for i := range keep {
			keep[i] = i
		}
		if in.horizontal { // 数值轴默认 autoSkip（autoSkipPadding 3）；类目轴原版设了 autoSkip: false
			keep = autoSkip(len(ticksW), tickSize(g.rotation, wd, jsLineHeight, 3), axisW, axisW+max(caL, padL)+padR, false)
			var kt []float64
			var kl []string
			for _, i := range keep {
				kt, kl = append(kt, g.vTicks[i]), append(kl, g.vLabels[i])
			}
			g.vTicks, g.vLabels = kt, kl
		}
		wd = 0
		for _, i := range keep {
			wd = max(wd, ticksW[i])
		}
		rad := deg2rad(g.rotation)
		sin, cos := float32(math.Sin(rad)), float32(math.Cos(rad))
		xH := min(in.H/2, tickLength+sin*wd+cos*jsLineHeight+2*tickPadding)
		if len(keep) > 0 && axisW > 0 {
			first, last := keep[0], keep[len(keep)-1]
			offL, offR := at[first]*axisW, axisW-at[last]*axisW
			var pl, pr float32
			if g.rotation != 0 {
				pl, pr = cos*ticksW[first], sin*jsLineHeight
			} else {
				pl, pr = ticksW[first]/2, ticksW[last]/2
			}
			padL = max(padL, (pl-offL+tickPadding)*axisW/(axisW-offL))
			padR = max(padR, (pr-offR+tickPadding)*axisW/(axisW-offR))
		}
		caB += xH - bottomH
		bottomH = xH
		return dims()
	}
	fitLeft()
	if fitBottom() {
		fitLeft()
	}
	g.area = ui.Rect{X: max(caL, padL), Y: padT, W: w, H: h}
	return g
}

// barRect 是第 i 根柱子（canvas 坐标）：类目格宽 × categoryPercentage 0.8 × barPercentage 0.9，
// 不超过 maxBarThickness；给了 barThickness 就用它。
func (g barGeom) barRect(i, n int, v float64, horizontal bool, thickness, maxThickness float32) ui.Rect {
	band := g.area.W / float32(n)
	if horizontal {
		band = g.area.H / float32(n)
	}
	size := band * 0.8 * 0.9
	if thickness > 0 {
		size = thickness
	}
	if maxThickness > 0 {
		size = min(size, maxThickness)
	}
	c := g.cAt(i, n, horizontal)
	base, head := g.vAt(0, horizontal), g.vAt(v, horizontal)
	if horizontal {
		return ui.Rect{X: base, Y: c - size/2, W: head - base, H: size}
	}
	return ui.Rect{X: c - size/2, Y: head, W: size, H: base - head}
}
