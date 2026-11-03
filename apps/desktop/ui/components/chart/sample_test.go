package chart

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
)

// 截图与参照页（testdata/reference.html）共用的样例数据：固定种子生成，Go 一侧写成 chart-data.js 交给参照页。

type rng uint64

func (r *rng) next() float64 {
	*r = *r*6364136223846793005 + 1442695040888963407
	return float64(uint64(*r)>>11) / float64(1<<53)
}

// lineCase 是一张折线图的场景：尺寸、属性与悬停的下标。
type lineCase struct {
	Name   string
	W, H   float32
	Props  DurationLineProps `json:"-"`
	Hover  int
	Scroll float32 // 可滚动时滚到的位置（DIP）

	// 给参照页的数据（与 Props 一致）
	Labels      []string    `json:"labels"`
	Times       []int64     `json:"times"`
	Series      []jsSeries  `json:"series"`
	ScaleType   string      `json:"scaleType"`
	Min         int64       `json:"min,omitempty"`
	Max         int64       `json:"max,omitempty"`
	ShowLegend  bool        `json:"showLegend"`
	YAxisTitle  string      `json:"yAxisTitle,omitempty"`
	Tension     float32     `json:"tension"`
	PointRadius [2]float32  `json:"pointRadius"`
	HasPlayData bool        `json:"hasPlayData"`
	MinPointW   float32     `json:"minPointWidth"`
	Extra       interface{} `json:"extra,omitempty"`
}

type jsSeries struct {
	Label string `json:"label"`
	Data  []int  `json:"data"`
	Color string `json:"color"`
}

func cssColor(c ui.Color) string { return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B) }

var gameTrendColors = []ui.Color{
	ui.RGBA(255, 99, 132, 1), ui.RGBA(54, 162, 235, 1), ui.RGBA(255, 206, 86, 1), ui.RGBA(75, 192, 192, 1),
	ui.RGBA(153, 102, 255, 1), ui.RGBA(255, 159, 64, 1), ui.RGBA(99, 255, 132, 1), ui.RGBA(199, 99, 255, 1),
	ui.RGBA(64, 224, 208, 1), ui.RGBA(255, 105, 180, 1),
}

var sampleToday = time.Date(2026, 10, 15, 0, 0, 0, 0, time.Local)

func days(from time.Time, n int) []time.Time {
	out := make([]time.Time, n)
	for i := range out {
		out[i] = from.AddDate(0, 0, i)
	}
	return out
}

// playSeconds 是一天的游玩时长：约 40% 的日子没玩，其余 10 分钟到 4 小时。
func playSeconds(r *rng, rate float64) int {
	if r.next() > rate {
		return 0
	}
	return 600 + int(r.next()*r.next()*14400)/60*60
}

func lineCases() []lineCase {
	r := rng(7)
	var cases []lineCase
	category := func(name string, w, h float32, labels []string, series []Series, hover int) lineCase {
		lc := lineCase{Name: name, W: w, H: h, Hover: hover, Labels: labels, ScaleType: "category", Tension: 0.3,
			PointRadius: [2]float32{3, 4}, MinPointW: 20}
		var pts []Point
		for _, l := range labels {
			pts = append(pts, Point{Label: l})
		}
		for _, s := range series {
			lc.Series = append(lc.Series, jsSeries{Label: s.Label, Data: s.Seconds, Color: cssColor(s.Color)})
			for _, v := range s.Seconds {
				lc.HasPlayData = lc.HasPlayData || v > 0
			}
		}
		lc.Props = DurationLineProps{Points: pts, Series: series, Height: h, Tension: 0.3, State: &State{}}
		return lc
	}
	dateLabels := func(ts []time.Time, layout string) []string {
		out := make([]string, len(ts))
		for i, t := range ts {
			out[i] = t.Format(layout)
		}
		return out
	}
	values := func(n int, rate float64) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = playSeconds(&r, rate)
		}
		return out
	}
	blue := []Series{{Label: "游戏时长", Color: lineColor}}

	// 游戏详情页：周、月
	week := dateLabels(days(sampleToday.AddDate(0, 0, -6), 7), "2006-01-02")
	blue[0].Seconds = []int{0, 1200, 3600, 0, 2100, 5400, 600}
	cases = append(cases, category("game-week", 900, 336, week, blue, 2))
	month := dateLabels(days(time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local), 31), "2006-01-02")
	cases = append(cases, category("game-month", 900, 336, month, []Series{{Label: "游戏时长", Color: lineColor, Seconds: values(31, 0.6)}}, 24))
	nodata := category("game-nodata", 900, 336, week, []Series{{Label: "游戏时长", Color: lineColor, Seconds: make([]int, 7)}}, -1)
	cases = append(cases, nodata)

	// 游戏详情页：全部（时间轴，稀疏的点）
	timeCase := func(name string, n int, start time.Time, spanDays int, hover int) lineCase {
		var ts []time.Time
		step := float64(spanDays) / float64(n)
		for i := range n {
			ts = append(ts, start.AddDate(0, 0, int(float64(i)*step+r.next()*step*0.8)))
		}
		vals := make([]int, n)
		for i := range vals {
			vals[i] = 600 + int(r.next()*r.next()*14400)/60*60
		}
		from, to := PaddedRange(ts[0], ts[n-1])
		lc := lineCase{Name: name, W: 900, H: 336, Hover: hover, ScaleType: "time", Tension: 0.2,
			PointRadius: [2]float32{4, 6}, HasPlayData: true, MinPointW: 20, Min: from.UnixMilli(), Max: to.UnixMilli(),
			Series: []jsSeries{{Label: "游戏时长", Data: vals, Color: cssColor(lineColor)}}}
		var pts []Point
		for i, t := range ts {
			lc.Times = append(lc.Times, t.UnixMilli())
			pts = append(pts, Point{Label: t.Format("2006-01-02"), Time: t, Seconds: vals[i]})
		}
		lc.Props = DurationLineProps{Points: pts, TimeScale: true, From: from, To: to, Height: 336, Label: "游戏时长",
			Tension: 0.2, State: &State{}}
		return lc
	}
	cases = append(cases, timeCase("game-all", 26, time.Date(2025, 2, 10, 0, 0, 0, 0, time.Local), 600, 5))
	sc := timeCase("game-all-scroll", 70, time.Date(2024, 6, 3, 0, 0, 0, 0, time.Local), 860, 40)
	sc.Scroll = 400
	cases = append(cases, sc)
	short := timeCase("game-all-short", 6, time.Date(2026, 9, 20, 0, 0, 0, 0, time.Local), 25, -1)
	cases = append(cases, short)

	// stats 页：总时长趋势（年维度 12 个月，图例与 y 轴标题）
	months := make([]string, 12)
	for i := range months {
		months[i] = fmt.Sprintf("2026-%02d", i+1)
	}
	mv := make([]int, 12)
	for i := range mv {
		mv[i] = 3600 * (8 + int(r.next()*60))
	}
	total := category("stats-total", 560, 288, months, []Series{{Label: "总游玩时长", Color: ui.RGBA(75, 192, 192, 1), Seconds: mv}}, 7)
	total.ShowLegend, total.YAxisTitle = true, "时长"
	total.Props.ShowLegend, total.Props.YAxisTitle = true, "时长"
	cases = append(cases, total)

	// stats 页：常玩游戏趋势（月维度，5 个游戏）
	names := []string{"サクラノ詩", "白色相簿2", "千恋＊万花", "Summer Pockets", "星空列车与白的旅行"}
	var games []Series
	for i, name := range names {
		games = append(games, Series{Label: name, Color: gameTrendColors[i], Seconds: values(31, 0.35)})
	}
	gt := category("stats-games", 1000, 320, month, games, 12)
	gt.ShowLegend, gt.YAxisTitle = true, "时长"
	gt.Props.ShowLegend, gt.Props.YAxisTitle = true, "时长"
	cases = append(cases, gt)

	// 自定义范围的 120 天：点太多，横向滚动
	long := dateLabels(days(time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local), 120), "2006-01-02")
	lc := category("stats-long-scroll", 1000, 320, long, []Series{{Label: "总游玩时长", Color: ui.RGBA(75, 192, 192, 1), Seconds: values(120, 0.55)}}, -1)
	lc.ShowLegend, lc.YAxisTitle = true, "时长"
	lc.Props.ShowLegend, lc.Props.YAxisTitle = true, "时长"
	cases = append(cases, lc)
	return cases
}

func sampleHourly() ([]vo.HourPlayPoint, []vo.WeekdayPlayPoint) {
	shape := []float64{0.5, 0.3, 0.1, 0, 0, 0, 0, 0, 0, 0.05, 0.1, 0.15, 0.3, 0.25, 0.2, 0.3, 0.4, 0.5, 0.6, 0.8, 1, 0.95, 0.9, 0.7}
	var hours []vo.HourPlayPoint
	for h, f := range shape {
		hours = append(hours, vo.HourPlayPoint{Hour: h, Duration: int(f*41000) / 60 * 60})
	}
	week := []int{52000, 18000, 21000, 15500, 24000, 39000, 61000}
	var wd []vo.WeekdayPlayPoint
	for d, v := range week {
		wd = append(wd, vo.WeekdayPlayPoint{Weekday: d, Duration: v})
	}
	return hours, wd
}

func sampleTags() []vo.TagPlayStats {
	names := []string{"纯爱", "校园", "奇幻", "悬疑", "治愈", "百合", "科幻", "日常", "剧情向", "Visual Novel"}
	secs := []int{182000, 151000, 98000, 76000, 54000, 41000, 30500, 21000, 12600, 5400}
	out := make([]vo.TagPlayStats, len(names))
	for i := range names {
		out[i] = vo.TagPlayStats{Name: names[i], TotalDuration: secs[i], GameCount: 10 - i}
	}
	return out
}

func sampleHeatmap() []vo.HeatmapCell {
	r := rng(11)
	var cells []vo.HeatmapCell
	start := sampleToday.AddDate(-1, 0, 1)
	for d := start; !d.After(sampleToday); d = d.AddDate(0, 0, 1) {
		rate := 0.35
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			rate = 0.7
		}
		cells = append(cells, vo.HeatmapCell{Date: d.Format("2006-01-02"), Duration: playSeconds(&r, rate)})
	}
	return cells
}
