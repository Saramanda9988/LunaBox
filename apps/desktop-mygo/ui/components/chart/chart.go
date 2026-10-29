// Package chart 对应原版 frontend/src/components/chart：DurationLineChart（含 HorizontalScrollChart）、
// HourWeekDistribution、PlayHeatmap、TagDistributionChart。
//
// 原版前三种用 Chart.js 4 画在 canvas 上，这里按 Chart.js 的源码复刻它的布局算法（刻度取法、标签旋转与
// 自动跳过、图表区边距、提示框位置），用 MyGo 的元素与 Painter 画出来；热力图与标签列表本来就是 HTML，
// 按 Tailwind 类名还原。颜色取自 hooks/useChartTheme.ts 与各组件里写死的色值。
//
// 组件不读页面状态：数据通过 Props 传入（与后端 vo 类型一致），跨帧状态放在调用方持有的 State 里。
package chart

import (
	"math"
	"runtime"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// chartFont 是 Chart.defaults.font.family（"'Helvetica Neue', 'Helvetica', 'Arial', sans-serif"）在本机
// 实际生效的字体：Windows 上没有前两个，是 Arial。
var chartFont = func() string {
	switch runtime.GOOS {
	case "windows":
		return "Arial"
	case "darwin":
		return "Helvetica Neue"
	}
	return ""
}()

// Chart.js 的默认值（Chart.defaults 与 scale 的默认选项）。
const (
	jsFontSize    = 12   // font.size
	jsLineHeight  = 14.4 // font.lineHeight 1.2
	tickLength    = 8    // grid.tickLength
	tickPadding   = 3    // ticks.padding
	titlePadding  = 4    // scale.title.padding（上下各 4）
	legendPadding = 10   // legend.labels.padding
	legendBoxW    = 40   // legend.labels.boxWidth
)

var (
	jsBorder  = ui.RGBA(0, 0, 0, 0.1)     // Chart.defaults.borderColor：坐标轴线、柱子的 borderColor
	tooltipBg = ui.RGBA(0, 0, 0, 0.8)     // tooltip.backgroundColor
	white     = ui.RGBA(255, 255, 255, 1) // tooltip.titleColor / bodyColor / multiKeyBackground
)

// pixelOrigin 把路径的原点 (x, y) 移到设备像素上。MyGo 按路径在像素网格上的位置（精确到 1/4 像素）缓存它的蒙版，
// 页面滚动到非整数像素（150% 缩放下滚动一个 DIP 是 1.5 像素）时，同一个扇区、柱子每帧都要重新光栅化并上传到
// 蒙版图集：统计页滚动时每秒上传几百 MB，图集反复撑满重建，显卡驱动的暂存内存随之暴涨。原点对齐像素后，
// 平移不改变蒙版，滚动时直接复用。
func pixelOrigin(p *ui.Painter, x, y float32) (float32, float32) {
	s := p.Scale()
	if s <= 0 {
		return x, y
	}
	return float32(math.Round(float64(x*s))) / s, float32(math.Round(float64(y*s))) / s
}

// colors 是 useChartTheme 的 textColor 与 gridColor。
type colors struct {
	text, grid ui.Color
	dark       bool
}

func chartColors(t *theme.Theme) colors {
	return colors{
		text: t.Pick(ui.Hex("#374151"), ui.Hex("#e5e7eb")), // brand-700 : brand-200（原版注释）
		grid: t.Pick(ui.Hex("#e5e7eb"), ui.Hex("#374151")),
		dark: t.Dark,
	}
}

// jsText 是 Chart.js 在 canvas 上画的一行字：12px、行高 1.2。
func jsText(c *ui.Context, s string, color ui.Color, bold bool) ui.Element {
	w := theme.WeightNormal
	if bold {
		w = theme.WeightBold
	}
	e := ui.Text(c, s).FontSize(jsFontSize).FixedLineHeight(jsLineHeight).FontWeight(w).TextColor(color).NoWrap().
		Margin(-2*textNudge, 0, 0, 0)
	if chartFont != "" {
		e.Font(chartFont)
	}
	return e
}

// textNudge 是 canvas 的 textBaseline "middle"（em 框的中线）与 MyGo 把一行字在行盒里居中之间的差：
// Arial 的升部 0.905、降部 0.212 em，canvas 的基线在中线下 (0.905-0.212)/(0.905+0.212)/2 em，
// MyGo 在 (0.905-0.212)/2 em，相差约 0.44 DIP。文字在居中的行里，上外边距 -2×它把字形上移这么多。
var textNudge = func() float32 {
	if chartFont != "Arial" {
		return 0
	}
	const asc, desc = 0.905, 0.212
	return jsFontSize * ((asc-desc)/2 - (asc-desc)/(asc+desc)/2)
}()

// measureFunc 返回一行 Chart.js 文字的宽度（ctx.measureText）。测试里换成固定字宽。
type measureFunc func(s string, bold bool) float32

func jsMeasure(c *ui.Context) measureFunc {
	return func(s string, bold bool) float32 {
		if s == "" {
			return 0
		}
		w := theme.WeightNormal
		if bold {
			w = theme.WeightBold
		}
		width, _ := c.MeasureText(0, ui.Span{Text: s, Font: chartFont, Size: jsFontSize, Weight: w})
		return width
	}
}

// place 在 (x, y)（父元素坐标，DIP）放一个零尺寸的锚点，h、v 决定内容哪条边对齐它：
// ui.Center 居中，ui.Start 左（上）边，ui.End 右（下）边。与 chart.Anchor 相同，只是不用百分比。
func place(c *ui.Context, x, y float32, h, v ui.Align, build func()) ui.Element {
	return ui.Row(c).Absolute().Left(x).Top(y).Size(0, 0).AlignItems(v).PassThrough().Children(func() {
		ui.Column(c).Width(0).AlignItems(h).Children(build)
	})
}

// hoverColor 是 Chart.js 的 getHoverColor：new Color(v).saturate(0.5).darken(0.1)，用于悬停中的点、柱子与扇区。
// 两步各自换回整数 RGB，与 @kurkle/color 的 modHSL 相同。
func hoverColor(c ui.Color) ui.Color {
	h, s, l := rgbToHSL(c)
	r, g, b := hslToRGB(h, clamp01(s*1.5), l)
	h, s, l = rgbToHSL(ui.Color{R: r, G: g, B: b})
	r, g, b = hslToRGB(h, s, clamp01(l*0.9))
	return ui.Color{R: r, G: g, B: b, A: c.A}
}

// rgbToHSL 与 @kurkle/color 的 rgb2hsl 一致（色相取整到度）。
func rgbToHSL(c ui.Color) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := max(r, g, b), min(r, g, b)
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return math.Trunc(h*60 + 0.5), s, l
}

// hslToRGB 是 @kurkle/color 的 hsl2rgbn。
func hslToRGB(h, s, l float64) (r, g, b uint8) {
	a := s * min(l, 1-l)
	f := func(n float64) uint8 {
		k := math.Mod(n+h/30, 12)
		v := l - a*max(min(k-3, 9-k, 1), -1)
		return uint8(max(0, min(255, math.Round(v*255))))
	}
	return f(0), f(8), f(4)
}

func clamp01(v float64) float64 { return max(0, min(1, v)) }

func deg2rad(d float32) float64 { return float64(d) * math.Pi / 180 }

func cosDeg(d float32) float32 { return float32(math.Cos(deg2rad(d))) }
