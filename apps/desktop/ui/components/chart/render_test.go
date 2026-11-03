package chart

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/theme"
)

func caseNamed(t *testing.T, name string) lineCase {
	t.Helper()
	for _, lc := range lineCases() {
		if lc.Name == name {
			return lc
		}
	}
	t.Fatalf("no case %s", name)
	return lineCase{}
}

func frames(tt *ui.Tester, n int) {
	for range n {
		tt.Frame()
	}
}

func expectText(t *testing.T, tt *ui.Tester, s string, want bool) {
	t.Helper()
	if got := tt.HasText(s); got != want {
		t.Errorf("HasText(%q) = %v, want %v; texts %q", s, got, want, tt.Texts())
	}
}

// 指针移到图表区里时显示最近下标的提示框；移到坐标轴上保持（Chart.js 只在图表区内更新），离开 canvas 才消失。
func TestLineChartTooltip(t *testing.T) {
	lc := caseNamed(t, "game-week")
	p := lc.Props
	var g lineGeom
	tt := render(t, true, lc.W, lc.H, func(c *ui.Context, th *theme.Theme) {
		DurationLineChart(c, th, p)
		g = layoutLine(p.input(p.State, jsMeasure(c), lc.W, lc.H))
	})
	expectText(t, tt, "游戏时长: 1小时", false)
	x := cardPad + g.xAt(2.0/6) + 10 // 第三个点（1 小时）右边一点
	tt.Move(x, cardPad+g.area.Y+g.area.H/2)
	frames(tt, 3)
	expectText(t, tt, "游戏时长: 1小时", true)
	if p.State.selected != 2 {
		t.Errorf("selected = %d, want 2", p.State.selected)
	}
	tt.Move(x, cardPad+g.area.Y+g.area.H+20) // x 轴标签上：仍在 canvas 内
	frames(tt, 3)
	expectText(t, tt, "游戏时长: 1小时", true)
	tt.Move(5, 5) // 卡片的内边距：离开 canvas
	frames(tt, 3)
	expectText(t, tt, "游戏时长: 1小时", false)
	if p.State.selected != -1 {
		t.Errorf("selected after leaving = %d", p.State.selected)
	}
	for _, s := range []string{"2026-10-09", "2026-10-15", "0分", "1时 40分"} {
		expectText(t, tt, s, true)
	}
}

// 多数据集：index 模式列出各数据集在该下标的值；点击图例隐藏数据集，提示框与 y 轴随之变化。
func TestLineChartLegend(t *testing.T) {
	lc := caseNamed(t, "stats-games")
	p := lc.Props
	tt := render(t, false, lc.W, lc.H, func(c *ui.Context, th *theme.Theme) { DurationLineChart(c, th, p) })
	for _, name := range []string{"サクラノ詩", "白色相簿2", "Summer Pockets", "时长"} {
		expectText(t, tt, name, true)
	}
	p.State.selected = 12
	frames(tt, 2)
	expectText(t, tt, "白色相簿2: 1小时9分钟", true)
	expectText(t, tt, "2026-10-13", true)
	if err := tt.Click("白色相簿2"); err != nil {
		t.Fatal(err)
	}
	frames(tt, 2)
	if !p.State.hidden[1] {
		t.Fatalf("legend click did not hide the dataset: %v", p.State.hidden)
	}
	expectText(t, tt, "白色相簿2: 1小时9分钟", false)
	expectText(t, tt, "サクラノ詩: 0秒", true)
	if err := tt.Click("白色相簿2"); err != nil {
		t.Fatal(err)
	}
	frames(tt, 2)
	expectText(t, tt, "白色相簿2: 1小时9分钟", true)
}

// 点数 × 20 超过容器宽度时变成可横向滚动，滚动条可以拖动。
func TestLineChartScroll(t *testing.T) {
	lc := caseNamed(t, "stats-long-scroll")
	p := lc.Props
	tt := render(t, true, lc.W, lc.H, func(c *ui.Context, th *theme.Theme) { DurationLineChart(c, th, p) })
	st := &p.State.scroll
	if want := float32(120*20) - lc.W; st.MaxX < want-1 || st.MaxX > want+1 {
		t.Fatalf("MaxX = %v, want %v", st.MaxX, want)
	}
	// 滑块在左端，宽 viewW²/contentW
	thumbW := lc.W * lc.W / (lc.W + st.MaxX)
	y := cardPad + lc.H - scrollbarSize/2
	tt.Press(cardPad+thumbW/2, y)
	tt.Move(cardPad+thumbW/2+100, y)
	tt.Release(cardPad+thumbW/2+100, y)
	frames(tt, 3)
	if st.X <= 0 {
		t.Errorf("dragging the thumb did not scroll: X = %v", st.X)
	}
	// 不需要滚动的图表没有滚动条
	week := caseNamed(t, "game-week")
	wp := week.Props
	render(t, true, week.W, week.H, func(c *ui.Context, th *theme.Theme) { DurationLineChart(c, th, wp) })
	if wp.State.scroll.MaxX != 0 {
		t.Errorf("week chart scrolls: %v", wp.State.scroll.MaxX)
	}
}

// 没有游玩数据时 y 轴固定 0–10 分钟；不传 State 也能用（状态挂在元素上）。
func TestLineChartNoDataWithoutState(t *testing.T) {
	lc := caseNamed(t, "game-nodata")
	p := lc.Props
	p.State = nil
	tt := render(t, false, lc.W, lc.H, func(c *ui.Context, th *theme.Theme) { DurationLineChart(c, th, p) })
	for _, s := range []string{"0分", "5分", "10分"} {
		expectText(t, tt, s, true)
	}
	tt.Move(cardPad+lc.W/2, cardPad+lc.H/2)
	frames(tt, 3)
	expectText(t, tt, "游戏时长: 0秒", true)
}

// 时间轴：刻度按 yyyy-MM 标注，提示框标题是点的日期。
func TestLineChartTimeScale(t *testing.T) {
	lc := caseNamed(t, "game-all")
	p := lc.Props
	tt := render(t, true, lc.W, lc.H, func(c *ui.Context, th *theme.Theme) { DurationLineChart(c, th, p) })
	expectText(t, tt, "2025-04", true)
	expectText(t, tt, "2026-08", true)
	p.State.selected = 5
	frames(tt, 2)
	expectText(t, tt, p.Points[5].Time.Format("2006-01-02"), true)
}

func TestHourWeekDistribution(t *testing.T) {
	hours, days := sampleHourly()
	st := &HourWeekState{}
	tt := render(t, true, 400, 340, func(c *ui.Context, th *theme.Theme) {
		HourWeekDistribution(c, th, HourWeekProps{Hourly: hours, Weekday: days, State: st})
	})
	for _, s := range []string{"时段（24h）", "高峰 20:00 · 11小时23分钟", "星期分布", "高峰 六 · 16小时56分钟", "日", "六", "0", "23", "5时 33分"} {
		expectText(t, tt, s, true)
	}
	if slices.Contains(tt.Texts(), "7") { // 只标 0、6、12、18、23 时
		t.Errorf("hour 7 is labeled: %q", tt.Texts())
	}
	// 指针移到 20 时的柱子上
	var g barGeom
	render(t, true, 400, 340, func(c *ui.Context, th *theme.Theme) {
		labels := make([]string, 24)
		for _, h := range []int{0, 6, 12, 18, 23} {
			labels[h] = string(rune('0'+h/10)) + string(rune('0'+h%10))
		}
		g = layoutBars(barInput{W: 400, H: 112, measure: memoize(jsMeasure(c)), labels: labels, maxValue: 41000, maxTicks: 4})
	})
	r, ok := tt.Find("时段（24h）")
	if !ok {
		t.Fatal("no header")
	}
	canvasTop := r.Y + r.H + 8
	bar := g.barRect(20, 24, float64(hours[20].Duration), false, 0, 18)
	tt.Move(cardPad+bar.X+bar.W/2, canvasTop+bar.Y+bar.H/2)
	frames(tt, 3)
	if st.hour.hover != 20 {
		t.Fatalf("hover = %d, want 20", st.hour.hover)
	}
	expectText(t, tt, "20:00", true)
	tt.Move(cardPad+bar.X+bar.W/2, canvasTop+g.area.Y+2) // 图表区里柱子上方的空白
	frames(tt, 3)
	if st.hour.hover != -1 {
		t.Errorf("hover over empty area = %d", st.hour.hover)
	}

	empty := render(t, false, 400, 100, func(c *ui.Context, th *theme.Theme) {
		HourWeekDistribution(c, th, HourWeekProps{Hourly: []vo.HourPlayPoint{{Hour: 1}}})
	})
	expectText(t, empty, "本期间内无游玩记录", true)
}

func TestTagDistributionChart(t *testing.T) {
	st := &TagState{}
	tt := render(t, false, 460, 520, func(c *ui.Context, th *theme.Theme) {
		TagDistributionChart(c, th, TagDistributionProps{Tags: sampleTags(), State: st})
	})
	for _, s := range []string{"#1", "纯爱", "27.1%", "#10", "Visual Novel", "0.8%"} {
		expectText(t, tt, s, true)
	}
	// 饼图在 200×200 的 canvas 里居中；第一块从 12 点钟顺时针，指向 1 点钟方向
	cx := float32(cardPad + 460/2)
	cy := float32(cardPad + pieH/2)
	tt.Move(cx+30, cy-60)
	frames(tt, 3)
	if st.hover != 0 {
		t.Fatalf("hover = %d, want 0", st.hover)
	}
	expectText(t, tt, "纯爱: 50小时33分钟 (27.1%)", true)
	tt.Move(cx+120, cy) // canvas 外
	frames(tt, 3)
	expectText(t, tt, "纯爱: 50小时33分钟 (27.1%)", false)

	empty := render(t, false, 400, 100, func(c *ui.Context, th *theme.Theme) {
		TagDistributionChart(c, th, TagDistributionProps{})
	})
	expectText(t, empty, "本期间内暂无标签数据", true)
}

func TestPlayHeatmap(t *testing.T) {
	cells := sampleHeatmap()
	tt := render(t, true, 720, 160, func(c *ui.Context, th *theme.Theme) {
		PlayHeatmap(c, th, HeatmapProps{Cells: cells})
	})
	g := buildHeatmap(cells)
	expectText(t, tt, "过去一年活跃 163 天", g.active == 163)
	for _, s := range []string{"10月", "1月", "一", "三", "五", "少", "多"} {
		expectText(t, tt, s, true)
	}
	// 月份标签与所在列对齐：11 月的标签在第 col 列的左边
	size, gap, gridW := heatLayout(len(g.columns), 720)
	left := cardPad + (720-(heatWeekdayW+heatGutter+gridW))/2 + heatWeekdayW + heatGutter
	r, _ := tt.Find("11月")
	if want := left + float32(g.months[1].col)*(size+gap); r.X < want-0.5 || r.X > want+0.5 {
		t.Errorf("11月 at %v, want %v", r.X, want)
	}
	empty := render(t, false, 400, 100, func(c *ui.Context, th *theme.Theme) {
		PlayHeatmap(c, th, HeatmapProps{})
	})
	expectText(t, empty, "本期间内无游玩记录", true)
}
