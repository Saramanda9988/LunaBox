package chart

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	plot "github.com/egoist/mygo/ui/chart"

	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// TagState 是 TagDistributionChart 跨帧的状态：悬停的扇区。
type TagState struct {
	hover int
	over  bool // 上一帧指针在饼图上
	ready bool
}

// TagDistributionProps 是 TagDistributionChart 的属性：vo.PeriodStats.tag_distribution（已按时长排序、
// 由调用方翻译好名称）。
type TagDistributionProps struct {
	Tags []vo.TagPlayStats
	// Columns 是下方标签列表的列数：原版 grid-cols-1 md:grid-cols-2，窗口宽度 ≥ 768 时两列。0 为 2。
	Columns int
	State   *TagState
}

// tagColors 是 TAG_COLORS：orange、red、pink、purple、indigo、blue、cyan、emerald、lime、yellow 的 500。
var tagColors = []ui.Color{
	ui.Hex("#f97316"), ui.Hex("#ef4444"), ui.Hex("#ec4899"), ui.Hex("#a855f7"), ui.Hex("#6366f1"),
	ui.Hex("#3b82f6"), ui.Hex("#06b6d4"), ui.Hex("#10b981"), ui.Hex("#84cc16"), ui.Hex("#eab308"),
}

// 饼图的 canvas：原版容器是 w-[200px] aspect-square，统计页里实际是 200×200（对照运行中的原版截图）。
// 单独的参照页（testdata/reference.html）里 Chart.js 初始化时按默认的 300 宽把 canvas 撑到 200×300，
// 那是参照页的布局所致，统计页里不会出现。
const (
	pieW      = 200
	pieH      = 200
	pieBorder = 2 // borderWidth
)

// TagDistributionChart 对应 TagDistributionChart.tsx：居中的 200px 饼图（每块 2px 描边，颜色与卡片底色相近），
// 下面是两列的标签排行：序号、名称、占比与 h-1.5 的进度条。悬停扇区时颜色加深，提示框标题是标签名，
// 正文“名称: 时长 (占比%)”。
// 没有标签时显示“本期间内暂无标签数据”。
func TagDistributionChart(c *ui.Context, t *theme.Theme, p TagDistributionProps) ui.Element {
	muted := t.Pick(theme.Brand500, theme.Brand400)
	if len(p.Tags) == 0 {
		return ui.Row(c).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			components.Text(c, locales.StatsTagDistributionEmpty(), theme.TextSM, theme.WeightNormal, muted)
		})
	}
	total := 0
	values := make([]float64, len(p.Tags))
	for i, tag := range p.Tags {
		total += tag.TotalDuration
		values[i] = float64(tag.TotalDuration)
	}
	pct := func(i int) float64 {
		if total == 0 {
			return 0
		}
		return float64(p.Tags[i].TotalDuration) / float64(total) * 100
	}
	cols := p.Columns
	if cols <= 0 {
		cols = 2
	}
	root := ui.Column(c).Gap(20) // flex flex-col gap-5
	st := p.State
	if st == nil {
		st = ui.Local(root, "chart.TagDistributionChart", func() TagState { return TagState{} })
	}
	if !st.ready {
		st.hover, st.ready = -1, true
	}
	return root.Children(func() {
		ui.Row(c).Justify(ui.Center).Children(func() {
			pie(c, t, st, p.Tags, values, total)
		})
		ui.Grid(c).Columns(cols).GapX(24).GapY(8).Children(func() {
			for i, tag := range p.Tags {
				tagRow(c.Key(i), t, i, tag.Name, pct(i), muted)
			}
		})
	})
}

// pie 是 Chart.js 的饼图：图表区即 canvas（layout.padding 0、无图例），半径 (200 - 描边 2) / 2 = 99，
// 从 12 点钟方向顺时针。
func pie(c *ui.Context, t *theme.Theme, st *TagState, tags []vo.TagPlayStats, values []float64, total int) {
	border := t.Pick(theme.White, ui.Hex("#1f2937"))
	slices := plot.Pie(values, 0)
	const cx, cy, radius = pieW / 2, pieH / 2, (min(pieW, pieH) - pieBorder) / 2
	canvas := ui.Box(c.Key("pie")).Size(pieW, pieH).Shrink(0).Label(locales.StatsTagDistributionTitle()) // 原版的 canvas 没有标签，用所在卡片的标题
	// mode nearest、intersect true：指针落在扇区（含半个描边）里才算；离开图表区保持，离开 canvas 清除
	x, y, over := canvas.PointerPosition()
	if over {
		st.hover = -1
		if i, ok := plot.SliceAt(slices, x-cx, y-cy, 0, radius+pieBorder/2); ok {
			st.hover = i
		}
	} else if st.over { // mouseout
		st.hover = -1
	}
	st.over = over
	canvas.Draw(func(p *ui.Painter, r ui.Rect) {
		ox, oy := pixelOrigin(p, r.X, r.Y)
		for i, s := range slices {
			fill, stroke := tagColors[i%len(tagColors)], border
			if i == st.hover {
				fill, stroke = hoverColor(fill), hoverColor(stroke)
			}
			path := plot.Sector(ox+cx, oy+cy, 0, radius, s)
			p.FillPath(path, fill)
			p.StrokePath(path, pieBorder, stroke)
		}
	})
	canvas.Children(func() {
		if i := st.hover; i >= 0 && i < len(tags) {
			// tooltipPosition：扇区角平分线上半径一半处
			x, y := plot.Polar(cx, cy, radius/2, slices[i].Mid())
			p := 0.0
			if total > 0 {
				p = values[i] / float64(total) * 100
			}
			text := fmt.Sprintf("%s: %s (%.1f%%)", tags[i].Name, timefmt.Duration(tags[i].TotalDuration), p)
			tooltip(c, jsMeasure(c), pieW, pieH, ui.Rect{W: pieW, H: pieH}, tipSpec{title: tags[i].Name, x: x, y: y,
				items: []tipItem{{text: text, border: border, borderW: pieBorder, fill: tagColors[i%len(tagColors)]}}})
		}
	})
}

// tagRow 是排行的一项：flex items-center gap-3；序号 w-6 text-xs font-medium tabular-nums text-right，
// 名称 text-sm font-medium truncate，占比 text-xs font-mono，进度条 h-1.5 rounded-full。
func tagRow(c *ui.Context, t *theme.Theme, i int, name string, pct float64, muted ui.Color) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Width(24).Shrink(0).Justify(ui.End).Children(func() {
			components.Text(c, fmt.Sprintf("#%d", i+1), theme.TextXS, theme.WeightMedium, muted).FontFeatures("tnum").NoWrap()
		})
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Margin(0, 0, 4, 0).Children(func() {
				components.Text(c, name, theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine().Shrink(1).MinWidth(0)
				components.Text(c, fmt.Sprintf("%.1f%%", pct), theme.TextXS, theme.WeightNormal, muted).
					Font(monoFont).NoWrap().Shrink(0)
			})
			// h-1.5 rounded-full bg-brand-100 dark:bg-brand-700 overflow-hidden，里面 transition-all 的色条
			ui.Box(c).Height(6).Radius(3).Clip().Background(t.Pick(theme.Brand100, theme.Brand700)).Children(func() {
				ui.Box(c).Height(6).WidthPercent(float32(math.Max(2, pct))).Radius(3).
					Background(tagColors[i%len(tagColors)]).
					Transition(ui.ElementTransition{Size: true, Duration: 150 * time.Millisecond, Ease: theme.EaseTailwind})
			})
		})
	})
}

// monoFont 是 font-mono 在 Windows 的 Chromium 里实际生效的字体（ui-monospace、SFMono-Regular、Menlo、Monaco 都没有）。
const monoFont = "Consolas"
