package chart

import (
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// HeatmapProps 是 PlayHeatmap 的属性：vo.PeriodStats.heatmap（每天一格，YYYY-MM-DD 与秒数）。
type HeatmapProps struct {
	Cells []vo.HeatmapCell
}

// PlayHeatmap.tsx 的固定尺寸。
const (
	heatWeekdayW  = 22 // WEEKDAY_LABEL_WIDTH
	heatGutter    = 4  // LABEL_GUTTER
	heatMonthRowH = 12 // MONTH_ROW_HEIGHT
	heatRowGap    = 4  // ROW_GAP
	heatMinCell   = 6  // MIN_CELL_SIZE
	heatMaxCell   = 14 // MAX_CELL_SIZE
)

// heatCell 是网格里的一天；placeholder 是首尾补齐整周的空格。
type heatCell struct {
	date        time.Time
	iso         string
	duration    int
	placeholder bool
}

// monthLabel 是月份标签：所在列与“10月”这样的文字（Intl.DateTimeFormat(i18n.language, {month: "short"})，en-US 是 Oct）。
type monthLabel struct {
	col   int
	label string
}

// heatGrid 是按周排好的网格：每列一周（周日在上），以及月份标签与活跃天数。
type heatGrid struct {
	columns [][]heatCell
	months  []monthLabel
	active  int
}

// buildHeatmap 对应组件里的第一个 useMemo：解析日期、排序，从第一天所在周的周日排到最后一天所在周的周六，
// 每 7 天一列；每列第一个真实日期换了月份就记一个月份标签。
func buildHeatmap(cells []vo.HeatmapCell) heatGrid {
	type day struct {
		date     time.Time
		iso      string
		duration int
	}
	var valid []day
	for _, c := range cells {
		d, err := time.Parse("2006-01-02", c.Date)
		if err != nil {
			continue
		}
		valid = append(valid, day{d, c.Date, c.Duration})
	}
	if len(valid) == 0 {
		return heatGrid{}
	}
	slices.SortStableFunc(valid, func(a, b day) int { return a.date.Compare(b.date) })
	first, last := valid[0].date, valid[len(valid)-1].date
	start := first.AddDate(0, 0, -int(first.Weekday()))
	end := last.AddDate(0, 0, 6-int(last.Weekday()))
	data := map[string]int{}
	for _, v := range valid {
		data[v.iso] = v.duration
	}
	var g heatGrid
	var col []heatCell
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		iso := d.Format("2006-01-02")
		c := heatCell{date: d, iso: iso, duration: data[iso], placeholder: d.Before(first) || d.After(last)}
		if !c.placeholder && c.duration > 0 {
			g.active++
		}
		col = append(col, c)
		if len(col) == 7 {
			g.columns = append(g.columns, col)
			col = nil
		}
	}
	lastMonth := -1
	monthFormat := i18n.DateOptions{Skeleton: "MMM"} // {month: "short"}
	for i, col := range g.columns {
		for _, c := range col {
			if c.placeholder {
				continue
			}
			if m := int(c.date.Month()); m != lastMonth {
				g.months = append(g.months, monthLabel{col: i, label: i18n.FormatDate(timefmt.FormatLocale(), c.date, monthFormat)})
				lastMonth = m
			}
			break
		}
	}
	return g
}

// heatLayout 对应第二个 useMemo：按容器宽度反推格子与间距（间距约为格子的 1/4），放不下先缩间距再缩格子。
// 还不知道宽度时用 12 与 3。
func heatLayout(cols int, containerW float32) (size, gap, gridW float32) {
	if cols == 0 || containerW <= 0 {
		return 12, 3, 0
	}
	inner := max(0, containerW-heatWeekdayW-heatGutter)
	s := int(math.Floor(float64(inner / float32(cols) / 1.25)))
	s = max(heatMinCell, min(heatMaxCell, s))
	g := max(2, int(math.Round(float64(s)/4)))
	fits := func(s, g int) bool { return float32(cols*s+(cols-1)*g) <= inner }
	for !fits(s, g) && g > 1 {
		g--
	}
	for !fits(s, g) && s > heatMinCell {
		s--
	}
	return float32(s), float32(g), float32(cols*s + max(0, cols-1)*g)
}

// heatBuckets 是第三个 useMemo：有游玩的日子时长的 p50 与 p90。
func heatBuckets(cells []vo.HeatmapCell) (p50, p90 int) {
	var active []int
	for _, c := range cells {
		if c.Duration > 0 {
			active = append(active, c.Duration)
		}
	}
	if len(active) == 0 {
		return 0, 0
	}
	slices.Sort(active)
	n := len(active)
	return active[n/2], active[min(n-1, int(math.Floor(float64(n)*0.9)))]
}

// bucketize 把时长分成 0–4 五级：没玩、不到 max(1 分钟, p50 的 40%)、不到 p50、不到 p90、其余。
func bucketize(d, p50, p90 int) int {
	switch {
	case d <= 0:
		return 0
	case float64(d) < math.Max(60, float64(p50)*0.4):
		return 1
	case d < p50:
		return 2
	case d < p90:
		return 3
	}
	return 4
}

// heatColors 是 colorScale。
func heatColors(t *theme.Theme) [5]ui.Color {
	if t.Dark {
		return [5]ui.Color{ui.RGBA(255, 255, 255, 0.06), ui.Hex("#0f3a2e"), ui.Hex("#14563f"), ui.Hex("#1f8a5c"), ui.Hex("#34d399")}
	}
	return [5]ui.Color{ui.RGBA(15, 23, 42, 0.06), ui.Hex("#bbf7d0"), ui.Hex("#86efac"), ui.Hex("#34d399"), ui.Hex("#059669")}
}

// PlayHeatmap 对应 PlayHeatmap.tsx：GitHub 式的日历热力图。格子大小随容器宽度在 6–14px 之间取，
// 整体水平居中；上方是“过去一年活跃 N 天”与月份，左侧隔行标出一、三、五，下方是“少 … 多”的图例。
// 悬停的格子放大 1.25 倍（hover:scale-125，150ms）。没有数据时显示“本期间内无游玩记录”。
func PlayHeatmap(c *ui.Context, t *theme.Theme, p HeatmapProps) ui.Element {
	muted := t.Pick(theme.Brand500, theme.Brand400)
	g := buildHeatmap(p.Cells)
	if len(g.columns) == 0 {
		// flex min-h-16 items-center justify-center text-xs
		return ui.Row(c).MinHeight(64).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			components.Text(c, locales.StatsHeatmapEmpty(), theme.TextXS, theme.WeightNormal, muted)
		})
	}
	wrap := ui.Column(c) // w-full
	size, gap, gridW := heatLayout(len(g.columns), wrap.Bounds().W)
	step := size + gap
	p50, p90 := heatBuckets(p.Cells)
	scale := heatColors(t)
	label10 := func(s string, line float32) ui.Element {
		return components.Text(c, s, theme.FontStep{Size: 10, Line: line}, theme.WeightNormal, muted).NoWrap()
	}
	return wrap.Children(func() {
		// mx-auto，宽度为星期标签列 + 网格
		ui.Column(c).Width(heatWeekdayW + heatGutter + gridW).AlignSelf(ui.Center).Children(func() {
			// mb-1 flex items-baseline justify-between text-[11px]；font-medium text-brand-700 dark:text-white/85
			ui.Row(c).Margin(0, 0, 4, 0).Children(func() {
				components.Text(c, locales.StatsHeatmapSummary(strconv.Itoa(g.active)), theme.FontStep{Size: 11, Line: 16.5},
					theme.WeightMedium, t.Pick(theme.Brand700, theme.White.Alpha(0.85))).NoWrap()
			})
			ui.Column(c).Gap(heatRowGap).Children(func() {
				// 月份：最后一个标签离右端不到 3 列时右对齐，免得超出
				ui.Row(c).Height(heatMonthRowH).Children(func() {
					ui.Box(c).Width(heatWeekdayW + heatGutter).Shrink(0)
					ui.Box(c).Width(gridW).Height(heatMonthRowH).Children(func() {
						for i, m := range g.months {
							e := ui.Box(c).Absolute().Top(0).Children(func() { label10(m.label, 10) })
							if i == len(g.months)-1 && len(g.columns)-m.col <= 3 {
								e.Right(0)
							} else {
								e.Left(float32(m.col) * step)
							}
						}
					})
				})
				ui.Row(c).Children(func() {
					ui.Column(c).Width(heatWeekdayW).Margin(0, heatGutter, 0, 0).Gap(gap).Shrink(0).Children(func() {
						for i, w := range weekdayLabels() {
							// pr-1 text-right text-[10px]，高与行高都是格子大小，只显示奇数行（一、三、五）
							row := ui.Row(c).Height(size).Padding(0, 4, 0, 0).Justify(ui.End).AlignItems(ui.Center)
							row.Children(func() {
								l := label10(w, size)
								if i%2 == 0 {
									l.Invisible()
								}
							})
						}
					})
					ui.Row(c).Gap(gap).Width(gridW).Children(func() {
						for ci, col := range g.columns {
							ui.Column(c.Key(ci)).Gap(gap).Width(size).Shrink(0).Children(func() {
								for _, cell := range col {
									if cell.placeholder {
										ui.Box(c).Size(size, size)
										continue
									}
									heatSquare(c, cell, size, scale[bucketize(cell.duration, p50, p90)])
								}
							})
						}
					})
				})
				// 图例：flex items-center justify-end gap-1 text-[10px]
				ui.Row(c).Justify(ui.End).AlignItems(ui.Center).Gap(4).Children(func() {
					label10(locales.StatsHeatmapLess(), 15)
					for _, color := range scale {
						ui.Box(c).Size(size, size).Radius(2).Background(color)
					}
					label10(locales.StatsHeatmapMore(), 15)
				})
			})
		})
	})
}

// heatSquare 是一天的格子：rounded-[2px]，transition-transform hover:scale-125。
func heatSquare(c *ui.Context, cell heatCell, size float32, color ui.Color) {
	sq := ui.Box(c).Size(size, size).Radius(2).Background(color).Label(cell.iso + " " + timefmt.Duration(cell.duration))
	s := float32(1)
	if sq.Hovered() {
		s = 1.25
	}
	sq.AnimatePose(ui.Pose{Scale: s}, theme.Tween(150*time.Millisecond))
}
