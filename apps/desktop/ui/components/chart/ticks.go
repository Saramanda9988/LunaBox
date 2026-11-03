package chart

import (
	"fmt"
	"math"
	"time"
)

// 这里是 Chart.js 4.5 core.scale / scale.linear / scale.time 里决定刻度的算法，逐条移植，
// 不依赖 UI，单元测试直接校验。

// niceNum 是 helpers.math 的 niceNum：把间距向上取到 1、2、5、10 乘 10 的幂。
func niceNum(r float64) float64 {
	if rr := math.Round(r); almostEquals(r, rr, r/1000) {
		r = rr
	}
	p := math.Pow(10, math.Floor(math.Log10(r)))
	f := r / p
	switch {
	case f <= 1:
		f = 1
	case f <= 2:
		f = 2
	case f <= 5:
		f = 5
	default:
		f = 10
	}
	return f * p
}

func almostEquals(x, y, eps float64) bool { return math.Abs(x-y) < eps }

// linearTickLimit 是 LinearScale.getTickLimit：没有 stepSize 时取 computeTickLimit
// （竖轴 ceil(长度 / min(40, 行高))，横轴 ceil(长度 / 40)）与 maxTicksLimit（默认 11）的较小者。
func linearTickLimit(length float32, horizontal bool, maxTicksLimit int) int {
	per := float32(jsLineHeight)
	if horizontal {
		per = 40 // lineHeight / sin(0) 取 0.001 后远大于 40
	}
	n := int(math.Ceil(float64(length / min(40, per))))
	if maxTicksLimit <= 0 {
		maxTicksLimit = 11
	}
	return max(2, min(n, maxTicksLimit))
}

// linearTicks 是 generateTicks$1 在 beginAtZero、bounds: "ticks"、只可能设置 max 与 stepSize 时的取值：
// 数据范围 [0, hi]，返回刻度值（最后一个即坐标轴的最大值）。step > 0 时 max 也必须给出（原版没有数据时
// 固定 max 600、stepSize 60）。
func linearTicks(hi float64, maxTicks int, step float64) []float64 {
	if step > 0 {
		maxTicks = int(math.Ceil(hi/step)) + 1
	}
	maxTicks = max(2, maxTicks)
	if hi <= 0 {
		hi = 1 // handleTickRangeOptions：min === max === 0 时把 max 加 1
	}
	unit := step
	if unit <= 0 {
		unit = 1
	}
	maxSpaces := float64(maxTicks - 1)
	spacing := niceNum(hi/maxSpaces/unit) * unit
	if spacing < 1e-14 {
		return []float64{0, hi}
	}
	numSpaces := math.Ceil(hi / spacing)
	if numSpaces > maxSpaces {
		spacing = niceNum(numSpaces*spacing/maxSpaces/unit) * unit
	}
	niceMax := math.Ceil(hi/spacing) * spacing
	n := niceMax / spacing
	if almostEquals(n, math.Round(n), spacing/1000) {
		n = math.Round(n)
	} else {
		n = math.Ceil(n)
	}
	factor := math.Pow(10, float64(decimalPlaces(spacing)))
	ticks := make([]float64, 0, int(n)+1)
	for j := 0; j < int(n); j++ {
		ticks = append(ticks, math.Round(float64(j)*spacing*factor)/factor)
	}
	return append(ticks, math.Round(niceMax*factor)/factor)
}

// decimalPlaces 是 helpers.math 的 _decimalPlaces。
func decimalPlaces(x float64) int {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return 0
	}
	e, p := 1.0, 0
	for math.Round(x*e)/e != x && p < 15 {
		e *= 10
		p++
	}
	return p
}

// rotationInput 是 calculateLabelRotation 需要的量。
type rotationInput struct {
	numTicks       int     // 自动跳过之前的刻度数
	widest, height float32 // 最宽的标签宽度、标签高度（行高）
	chartWidth     float32 // 整个 canvas 的宽度
	maxWidth       float32 // 坐标轴可用的宽度（图表区宽度）
	maxHeight      float32 // 坐标轴可用的高度（canvas 高度的一半）
	offset         bool    // scale.offset：柱状图的类目轴为 true
	maxRotation    float32 // ticks.maxRotation（默认 50）
}

// labelRotation 是 Scale.calculateLabelRotation（minRotation 0）：标签加左右各 3px 放不进一格时才旋转，
// 角度取 asin((行高 + 6) / 格宽)，并受坐标轴最大高度与 maxRotation 限制。返回角度（度）。
func labelRotation(in rotationInput) float32 {
	if in.numTicks <= 1 || in.maxRotation <= 0 {
		return 0
	}
	maxWidth := max(0, min(in.chartWidth-in.widest, in.maxWidth))
	var tickWidth float32
	if in.offset {
		tickWidth = in.maxWidth / float32(in.numTicks)
	} else {
		tickWidth = maxWidth / float32(in.numTicks-1)
	}
	if in.widest+6 <= tickWidth {
		return 0
	}
	if in.offset {
		tickWidth = maxWidth / (float32(in.numTicks) - 0.5)
	} else {
		tickWidth = maxWidth / float32(in.numTicks-1)
	}
	maxHeight := float64(in.maxHeight - tickLength - tickPadding)
	diag := math.Hypot(float64(in.widest), float64(in.height))
	a := math.Asin(limit(float64(in.height+6)/float64(tickWidth), -1, 1))
	if diag > 0 {
		a = min(a, math.Asin(limit(maxHeight/diag, -1, 1))-math.Asin(limit(float64(in.height)/diag, -1, 1)))
	}
	deg := float32(a * 180 / math.Pi)
	return max(0, min(in.maxRotation, deg))
}

func limit(v, lo, hi float64) float64 { return max(lo, min(hi, v)) }

// tickSize 是 Scale._tickSize：旋转后相邻两个标签（含 autoSkipPadding）沿坐标轴至少要占的长度。
func tickSize(rotation, widest, height, padding float32) float32 {
	r := deg2rad(rotation)
	cos, sin := float32(math.Abs(math.Cos(r))), float32(math.Abs(math.Sin(r)))
	w, h := widest+padding, height+padding
	if h*cos > w*sin {
		return w / cos
	}
	return h / sin
}

// autoSkip 是 core.scale.autoskip 的 autoSkip（major 刻度未启用）：按 _tickSize 算出最多放得下几个标签，
// 再每隔 ceil(总数 / 上限) 个取一个。length 是坐标轴长度，maxLength 是加上两侧边距后的长度
// （Scale._maxLength）。返回保留的刻度下标。
func autoSkip(n int, size, length, maxLength float32, offset bool) []int {
	idx := make([]int, 0, n)
	if n == 0 {
		return idx
	}
	extra := float32(1)
	if offset {
		extra = 0
	}
	limitN := int(math.Floor(float64(min(length/size+extra, maxLength/size))))
	spacing := max(float64(n)/float64(max(limitN, 0)), 1)
	if limitN <= 0 {
		spacing = float64(n)
	}
	spacing = math.Ceil(spacing)
	next, count := 0, 0
	for i := 0; i < n; i++ {
		if i == next {
			idx = append(idx, i)
			count++
			next = int(math.Round(float64(count) * spacing))
		}
	}
	return idx
}

// timeUnit 是时间轴能用的刻度单位（minUnit 为 day，week 与 quarter 不是 common，不会自动选到）。
type timeUnit int

const (
	unitDay timeUnit = iota
	unitMonth
	unitYear
)

// INTERVALS 里 day、month 的 size 与 steps。
const (
	dayMillis   = 86400000
	monthMillis = 2.628e9
)

// timeLabelLayout 是 displayFormats：day yyyy-MM-dd、month yyyy-MM、year yyyy。
func (u timeUnit) layout() string {
	switch u {
	case unitMonth:
		return "2006-01"
	case unitYear:
		return "2006"
	}
	return "2006-01-02"
}

// capacityLabel 是 TimeScale._getLabelCapacity 量的示例标签：没有设置 time.unit，于是用
// displayFormats.millisecond（date-fns 适配器的 "h:mm:ss.SSS aaaa"）格式化坐标轴的最小值。
func capacityLabel(t time.Time) string {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	ampm := "a.m."
	if t.Hour() >= 12 {
		ampm = "p.m."
	}
	return fmt.Sprintf("%d:%02d:%02d.%03d %s", h, t.Minute(), t.Second(), t.Nanosecond()/1e6, ampm)
}

// labelCapacity 是 _getLabelCapacity：示例标签按 maxRotation 45° 投影到坐标轴上的宽度能放下几个，再减一。
func labelCapacity(axisWidth, exampleWidth float32) int {
	a := deg2rad(45)
	w := exampleWidth*float32(math.Cos(a)) + jsFontSize*float32(math.Sin(a))
	n := int(math.Floor(float64(axisWidth/w))) - 1
	return max(n, 1)
}

// determineUnit 是 determineUnitForAutoTicks(minUnit "day")：从 day 起找第一个能用不超过 capacity 个
// 刻度（按单位的最大步长 steps 计）覆盖范围的单位，都不行就用 year。
func determineUnit(from, to time.Time, capacity int) timeUnit {
	span := float64(to.Sub(from).Milliseconds())
	if math.Ceil(span/(30*dayMillis)) <= float64(capacity) {
		return unitDay
	}
	if math.Ceil(span/(12*monthMillis)) <= float64(capacity) {
		return unitMonth
	}
	return unitYear
}

// timeTicks 是 TimeScale._generate 加上 _filterBetween：从 startOf(min, unit) 起每次加一个单位，
// 只留 [min, max] 之内的。
func timeTicks(from, to time.Time, u timeUnit) []time.Time {
	loc := from.Location()
	var t time.Time
	switch u {
	case unitMonth:
		t = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, loc)
	case unitYear:
		t = time.Date(from.Year(), 1, 1, 0, 0, 0, 0, loc)
	default:
		t = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	}
	var out []time.Time
	for i := 0; !t.After(to) && i < 100000; i++ {
		if !t.Before(from) {
			out = append(out, t)
		}
		switch u {
		case unitMonth:
			t = t.AddDate(0, 1, 0)
		case unitYear:
			t = t.AddDate(1, 0, 0)
		default:
			t = t.AddDate(0, 0, 1)
		}
	}
	return out
}
