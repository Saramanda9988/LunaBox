package chart

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/theme"
)

func TestNiceNum(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{210, 500}, {100, 100}, {101, 200}, {0.1, 0.1}, {13666, 20000}, {1, 1}, {3.2, 5}, {6000, 10000},
	} {
		if got := niceNum(tc.in); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("niceNum(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// 期望值来自 Chart.js（testdata/chartjs-layout.json 与参照页的截图）。
func TestLinearTicks(t *testing.T) {
	for _, tc := range []struct {
		hi       float64
		maxTicks int
		step     float64
		want     []float64
	}{
		{5400, 11, 0, []float64{0, 1000, 2000, 3000, 4000, 5000, 6000}},
		{600, 0, 60, []float64{0, 60, 120, 180, 240, 300, 360, 420, 480, 540, 600}}, // 没有游玩数据
		{41000, 4, 0, []float64{0, 20000, 40000, 60000}},                            // 时段分布，maxTicksLimit 4
		{2100, 11, 0, []float64{0, 500, 1000, 1500, 2000, 2500}},
		{0, 11, 0, []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1}}, // min === max === 0 时 max 加 1
	} {
		if got := linearTicks(tc.hi, tc.maxTicks, tc.step); !slices.Equal(got, tc.want) {
			t.Errorf("linearTicks(%v, %d, %v) = %v, want %v", tc.hi, tc.maxTicks, tc.step, got, tc.want)
		}
	}
	if n := linearTickLimit(297.4, false, 0); n != 11 {
		t.Errorf("tick limit of a tall axis = %d, want 11", n)
	}
	if n := linearTickLimit(60, false, 0); n != 5 {
		t.Errorf("tick limit of a 60px axis = %d, want ceil(60/14.4) = 5", n)
	}
	if n := linearTickLimit(400, true, 4); n != 4 {
		t.Errorf("horizontal tick limit = %d, want maxTicksLimit 4", n)
	}
}

func TestLabelRotation(t *testing.T) {
	const dateW = 61.3828125 // Arial 12px 的 "2026-10-09"（Chrome measureText）
	// 一周 7 个标签放得下，不旋转
	if r := labelRotation(rotationInput{numTicks: 7, widest: dateW, height: jsLineHeight, chartWidth: 900, maxWidth: 805, maxHeight: 168, maxRotation: 45}); r != 0 {
		t.Errorf("week rotation = %v, want 0", r)
	}
	// 一个月 31 个标签：asin(20.4 / 27.5) > 45°，取 maxRotation
	if r := labelRotation(rotationInput{numTicks: 31, widest: dateW, height: jsLineHeight, chartWidth: 900, maxWidth: 825, maxHeight: 168, maxRotation: 45}); r != 45 {
		t.Errorf("month rotation = %v, want 45", r)
	}
	// 介于两者之间时按格宽取角度
	r := labelRotation(rotationInput{numTicks: 12, widest: 45, height: jsLineHeight, chartWidth: 560, maxWidth: 465, maxHeight: 144, maxRotation: 45})
	want := float32(math.Asin((jsLineHeight+6)/(465.0/11)) * 180 / math.Pi)
	if math.Abs(float64(r-want)) > 0.01 || r <= 0 || r >= 45 {
		t.Errorf("rotation = %v, want %v", r, want)
	}
	// 柱状图（offset）的矮坐标轴：受最大高度限制（Chart.js 截图里约 42.7°）
	r = labelRotation(rotationInput{numTicks: 24, widest: 13.34, height: jsLineHeight, chartWidth: 360, maxWidth: 315, maxHeight: 56, offset: true, maxRotation: 50})
	if r < 40 || r > 45 {
		t.Errorf("hourly rotation = %v, want about 42.7", r)
	}
	if r := labelRotation(rotationInput{numTicks: 1, widest: 500, height: jsLineHeight, chartWidth: 100, maxWidth: 80, maxHeight: 50, maxRotation: 45}); r != 0 {
		t.Errorf("single tick rotates %v", r)
	}
}

func TestAutoSkip(t *testing.T) {
	size := tickSize(45, 61.38, jsLineHeight, 12)
	if math.Abs(float64(size)-26.4/math.Sin(math.Pi/4)) > 0.01 {
		t.Errorf("tickSize at 45° = %v", size)
	}
	// 31 天、45° 时每隔一个
	got := autoSkip(31, size, 825.46, 900, false)
	if len(got) != 16 || got[1] != 2 || got[15] != 30 {
		t.Errorf("autoSkip(31) = %v", got)
	}
	// 放得下就全留
	if got := autoSkip(7, tickSize(0, 61.38, jsLineHeight, 12), 805, 900, false); len(got) != 7 {
		t.Errorf("autoSkip(7) = %v", got)
	}
	// 一个都放不下时只留第一个
	if got := autoSkip(5, 1000, 100, 100, false); !slices.Equal(got, []int{0}) {
		t.Errorf("autoSkip of a tiny axis = %v", got)
	}
}

func TestTimeUnitAndTicks(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	if got := capacityLabel(time.Date(2026, 1, 1, 0, 0, 0, 0, loc)); got != "12:00:00.000 a.m." {
		t.Errorf("capacity label = %q", got)
	}
	if got := capacityLabel(time.Date(2026, 1, 1, 14, 5, 9, 7e6, loc)); got != "2:05:09.007 p.m." {
		t.Errorf("capacity label = %q", got)
	}
	capacity := labelCapacity(831.6, 96.74) // Chrome 量的 "12:00:00.000 a.m."
	if capacity != 9 {
		t.Errorf("capacity = %d, want 9", capacity)
	}
	from := time.Date(2026, 9, 18, 12, 0, 0, 0, loc)
	to := time.Date(2026, 10, 12, 12, 0, 0, 0, loc)
	if u := determineUnit(from, to, capacity); u != unitDay {
		t.Errorf("25 days -> unit %v", u)
	}
	ticks := timeTicks(from, to, unitDay)
	if len(ticks) != 24 || ticks[0].Day() != 19 || ticks[23].Day() != 12 {
		t.Errorf("day ticks = %v", ticks)
	}
	far := from.AddDate(1, 8, 0)
	if u := determineUnit(from, far, capacity); u != unitMonth {
		t.Errorf("20 months -> unit %v", u)
	}
	months := timeTicks(from, far, unitMonth)
	if months[0].Month() != time.October || months[0].Day() != 1 || len(months) != 20 {
		t.Errorf("month ticks = %v", months)
	}
	if u := determineUnit(from, from.AddDate(12, 0, 0), capacity); u != unitYear {
		t.Errorf("12 years -> unit %v", u)
	}
	if unitMonth.layout() != "2006-01" || unitYear.layout() != "2006" || unitDay.layout() != "2006-01-02" {
		t.Error("display formats")
	}
}

func TestPaddedRange(t *testing.T) {
	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	from, to := PaddedRange(day, day)
	if !from.Equal(day.AddDate(0, 0, -7)) || !to.Equal(day.AddDate(0, 0, 7)) {
		t.Errorf("one day: %v..%v", from, to)
	}
	from, to = PaddedRange(day, day.AddDate(0, 0, 10))
	if !from.Equal(day.AddDate(0, 0, -1)) || !to.Equal(day.AddDate(0, 0, 11)) {
		t.Errorf("ten days pad one day: %v..%v", from, to)
	}
	from, _ = PaddedRange(day, day.AddDate(3, 0, 0))
	if !from.Equal(day.AddDate(0, 0, -14)) {
		t.Errorf("three years pad 14 days: %v", from)
	}
}

func TestHoverColor(t *testing.T) {
	// 期望值是 Chart.js 的 getHoverColor（node 里用 chart.js/helpers 算的）
	for _, tc := range []struct{ in, want ui.Color }{
		{ui.RGBA(59, 130, 246, 0.5), ui.Hex("#136EFF")},
		{ui.Hex("#10b981"), ui.Hex("#00B579")},
		{ui.Hex("#ffffff"), ui.Hex("#E6E6E6")},
		{ui.Hex("#1f2937"), ui.Hex("#172437")},
		{ui.Hex("#ef4444"), ui.Hex("#FF1515")},
		{ui.RGBA(75, 192, 192, 0.5), ui.Hex("#22CFCF")},
	} {
		got := hoverColor(tc.in)
		if got.R != tc.want.R || got.G != tc.want.G || got.B != tc.want.B || got.A != tc.in.A {
			t.Errorf("hoverColor(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// fixedMeasure 是每个字符 6px 的假字宽，单元测试不依赖字体。
func fixedMeasure(s string, _ bool) float32 { return 6 * float32(len([]rune(s))) }

func TestLayoutLineInvariants(t *testing.T) {
	labels := make([]string, 31)
	for i := range labels {
		labels[i] = time.Date(2026, 10, i+1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	g := layoutLine(lineInput{W: 600, H: 300, measure: fixedMeasure, labels: labels, maxValue: 7200, minPad: 4,
		legend: []string{"甲", "乙"}, yTitle: "时长"})
	a := g.area
	if a.X < 4 || a.Y < g.legendH || a.X+a.W > 600-4+0.01 || a.Y+a.H > 300 {
		t.Errorf("area %+v outside the canvas", a)
	}
	if g.rotation <= 0 || len(g.xTicks) >= 31 || g.xTicks[0].label != labels[0] {
		t.Errorf("rotation %v, ticks %d", g.rotation, len(g.xTicks))
	}
	if g.legendH != 32 || len(g.legend) != 2 || g.legend[0].y != 10 {
		t.Errorf("legend %+v height %v", g.legend, g.legendH)
	}
	// 图例项一行居中：每项 40 + 6 + 字宽 6，加 padding 10
	if left := g.legend[0].x; math.Abs(float64(left-(10+600-2*(52+10))/2)) > 0.01 {
		t.Errorf("legend left %v", left)
	}
	if g.yLabels[0] != "0分" || g.yMax < 7200 {
		t.Errorf("y labels %v", g.yLabels)
	}
	// y 轴标题占 14.4 + 8，标签两侧 3+3，tick 8
	var labelW float32
	for _, s := range g.yLabels {
		labelW = max(labelW, fixedMeasure(s, false))
	}
	if want := tickLength + 22.4 + labelW + 6; math.Abs(float64(a.X-want)) > 0.01 && a.X < want {
		t.Errorf("area left %v, y axis width %v", a.X, want)
	}
	// 窄的图例换行
	items, h := layoutLegend([]string{"一个很长很长的游戏名称", "另一个很长很长的游戏名称"}, 200, fixedMeasure)
	if h != 54 || items[1].y != 32 {
		t.Errorf("wrapped legend %+v height %v", items, h)
	}
}

// TestLayoutMatchesChartJS 用 MyGo 实际的字宽排每个样例，与原版 Chart.js 在 Chromium 里排出的结果
// （testdata/chartjs-layout.json，由 testdata/reference.html 的 data-layout 导出）比较。
func TestLayoutMatchesChartJS(t *testing.T) {
	if chartFont != "Arial" {
		t.Skip("Chart.js 的默认字体在本机不是 Arial")
	}
	raw, err := os.ReadFile("testdata/chartjs-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]struct {
		Area     [4]float32
		Rotation float32
		Ticks    []string
		YTicks   []float64
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	for _, lc := range lineCases() {
		w, ok := want[lc.Name]
		if !ok {
			t.Errorf("no Chart.js layout for %s", lc.Name)
			continue
		}
		var g lineGeom
		render(t, true, lc.W, lc.H, func(c *ui.Context, th *theme.Theme) {
			W, H := lc.W, lc.H
			if n := float32(len(lc.Props.Points)); n*20 > W {
				W, H = n*20, H-scrollbarSize
			}
			g = layoutLine(lc.Props.input(&State{}, jsMeasure(c), W, H))
		})
		got := [4]float32{g.area.X, g.area.Y, g.area.W, g.area.H}
		for i := range got {
			if math.Abs(float64(got[i]-w.Area[i])) > 0.05 {
				t.Errorf("%s: area %v, Chart.js %v", lc.Name, got, w.Area)
				break
			}
		}
		if math.Abs(float64(g.rotation-w.Rotation)) > 0.01 {
			t.Errorf("%s: rotation %v, Chart.js %v", lc.Name, g.rotation, w.Rotation)
		}
		var labels []string
		for _, tk := range g.xTicks {
			labels = append(labels, tk.label)
		}
		if !slices.Equal(labels, w.Ticks) {
			t.Errorf("%s: ticks %v, Chart.js %v", lc.Name, labels, w.Ticks)
		}
		if !slices.Equal(g.yTicks, w.YTicks) {
			t.Errorf("%s: y ticks %v, Chart.js %v", lc.Name, g.yTicks, w.YTicks)
		}
	}
}

func TestHeatmapGrid(t *testing.T) {
	// 2026-10-01 是周四：网格从 9 月 27 日（周日）开始，到 11 月 14 日（周六）结束
	var cells []vo.HeatmapCell
	for d := 1; d <= 40; d++ {
		day := time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC)
		dur := 0
		if d%3 == 0 {
			dur = d * 60
		}
		cells = append(cells, vo.HeatmapCell{Date: day.Format("2006-01-02"), Duration: dur})
	}
	cells = append(cells, vo.HeatmapCell{Date: "bad", Duration: 100})
	g := buildHeatmap(cells)
	if len(g.columns) != 7 {
		t.Fatalf("columns = %d, want 7", len(g.columns))
	}
	first := g.columns[0]
	if first[0].iso != "2026-09-27" || !first[0].placeholder || !first[3].placeholder || first[4].iso != "2026-10-01" || first[4].placeholder {
		t.Errorf("first week %+v", first)
	}
	if last := g.columns[6]; last[6].iso != "2026-11-14" || !last[6].placeholder || last[1].iso != "2026-11-09" || last[1].placeholder || !last[2].placeholder {
		t.Errorf("last week %+v", last)
	}
	if g.active != 13 {
		t.Errorf("active days = %d, want 13", g.active)
	}
	if len(g.months) != 2 || g.months[0] != (monthLabel{0, "10月"}) || g.months[1] != (monthLabel{5, "11月"}) {
		t.Errorf("months %+v", g.months)
	}
	if g := buildHeatmap(nil); len(g.columns) != 0 {
		t.Error("empty heatmap has columns")
	}

	// 宽 720：内宽 694，53 列 → 694/53/1.25 = 10.4 → 10，间距 round(2.5) = 3，网格宽 53×10+52×3 = 686
	if s, gap, w := heatLayout(53, 720); s != 10 || gap != 3 || w != 686 {
		t.Errorf("layout(53, 720) = %v %v %v", s, gap, w)
	}
	// 太窄：先把间距缩到 1，再把格子缩到最小 6
	if s, gap, _ := heatLayout(53, 300); s != 6 || gap != 1 {
		t.Errorf("layout(53, 300) = %v %v", s, gap)
	}
	if s, gap, w := heatLayout(10, 0); s != 12 || gap != 3 || w != 0 {
		t.Errorf("layout before measuring = %v %v %v", s, gap, w)
	}
	if s, _, _ := heatLayout(5, 2000); s != heatMaxCell {
		t.Errorf("wide layout cell %v", s)
	}

	p50, p90 := heatBuckets(cells)
	if p50 != 1260 || p90 != 2160 {
		t.Errorf("p50 %d p90 %d", p50, p90)
	}
	for _, tc := range []struct{ d, want int }{{0, 0}, {59, 1}, {503, 1}, {504, 2}, {1259, 2}, {1260, 3}, {2160, 4}} {
		if got := bucketize(tc.d, 1260, 2160); got != tc.want {
			t.Errorf("bucketize(%d) = %d, want %d", tc.d, got, tc.want)
		}
	}
}

func TestBarLayout(t *testing.T) {
	labels := make([]string, 24)
	for _, h := range []int{0, 6, 12, 18, 23} {
		labels[h] = fmt.Sprint(h)
	}
	g := layoutBars(barInput{W: 400, H: 112, measure: fixedMeasure, labels: labels, maxValue: 41000, maxTicks: 4})
	if !slices.Equal(g.vTicks, []float64{0, 20000, 40000, 60000}) {
		t.Errorf("value ticks %v", g.vTicks)
	}
	if g.rotation <= 0 {
		t.Errorf("hour labels do not rotate on a narrow axis")
	}
	// 柱宽：格宽 × 0.8 × 0.9，不超过 maxBarThickness 18
	r := g.barRect(5, 24, 20000, false, 0, 18)
	if band := g.area.W / 24; math.Abs(float64(r.W-min(18, band*0.72))) > 0.01 {
		t.Errorf("bar width %v for band %v", r.W, band)
	}
	if math.Abs(float64(r.Y-(g.area.Y+g.area.H*2/3))) > 0.01 || math.Abs(float64(r.Y+r.H-(g.area.Y+g.area.H))) > 0.01 {
		t.Errorf("bar %+v in area %+v", r, g.area)
	}
	// 横向：类目在左，barThickness 14
	days := weekdayLabels()
	h := layoutBars(barInput{W: 400, H: 176, measure: fixedMeasure, horizontal: true, labels: days[:], maxValue: 61000, maxTicks: 4})
	r = h.barRect(6, 7, 61000, true, 14, 0)
	if r.H != 14 || math.Abs(float64(r.X-h.area.X)) > 0.01 || r.W <= 0 {
		t.Errorf("horizontal bar %+v in %+v", r, h.area)
	}
}
