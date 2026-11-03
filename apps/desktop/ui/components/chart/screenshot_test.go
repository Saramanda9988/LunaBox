package chart

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// cardPad 是截图里图表外的卡片内边距（stats 页卡片的 p-5）。
const cardPad = 20

// render 在 w×h 的卡片（bg-white dark:bg-brand-800）里画 build，按应用的设置关掉 MyGo 自带的滚动条。
func render(t *testing.T, dark bool, w, h float32, build func(c *ui.Context, th *theme.Theme)) *ui.Tester {
	t.Helper()
	th := theme.New(dark, 1.5)
	tt := ui.NewTester(func(c *ui.Context) {
		mt := *c.Theme()
		mt.Scrollbar = ui.Transparent
		c.SetTheme(&mt)
		ui.Column(c).Fill().Padding(cardPad).Background(th.Pick(theme.White, theme.Brand800)).Children(func() {
			build(c, th)
		})
	}, int(w+2*cardPad), int(h+2*cardPad))
	tt.SetScale(1.5)
	tt.SetDark(dark)
	for range 4 {
		tt.Frame()
	}
	return tt
}

func savePNG(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// TestScreenshotDump 在 DESKTOP_MYGO_PNG=<目录> 时把每种图表的样例（深色、浅色，150% 缩放）存成
// chart-<场景>-<dark|light>.png，并写出 chart-data.js 供 testdata/reference.html 用原版的 Chart.js 画同一份数据。
func TestScreenshotDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	hours, weekdays := sampleHourly()
	data := map[string]any{
		"line":    lineCases(),
		"hourly":  hours,
		"weekday": weekdays,
		"tags":    sampleTags(),
		"heatmap": sampleHeatmap(),
	}
	js, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chart-data.js"), append([]byte("window.DATA = "), js...), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{true, false} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		for _, lc := range lineCases() {
			p := lc.Props
			var g lineGeom
			tt := render(t, dark, lc.W, lc.H, func(c *ui.Context, th *theme.Theme) {
				DurationLineChart(c, th, p)
				w := lc.W
				if n := float32(len(p.Points)); n*20 > w {
					w = n * 20
				}
				h := lc.H
				if w > lc.W {
					h -= scrollbarSize
				}
				g = layoutLine(p.input(p.State, jsMeasure(c), w, h))
			})
			if dark {
				var labels []string
				for _, tk := range g.xTicks {
					labels = append(labels, tk.label)
				}
				t.Logf("%s area=%+v rotation=%.4f ticks=%v yTicks=%v", lc.Name, g.area, g.rotation, labels, g.yTicks)
			}
			if lc.Hover >= 0 || lc.Scroll > 0 {
				p.State.selected = lc.Hover
				p.State.scroll.X = lc.Scroll
				for range 3 {
					tt.Frame()
				}
			}
			savePNG(t, tt, filepath.Join(dir, "chart-"+lc.Name+"-"+mode+".png"))
		}

		hw := &HourWeekState{hour: barState{hover: 21, ready: true}, week: barState{hover: -1, ready: true}}
		tt := render(t, dark, 400, 340, func(c *ui.Context, th *theme.Theme) {
			HourWeekDistribution(c, th, HourWeekProps{Hourly: hours, Weekday: weekdays, State: hw})
		})
		savePNG(t, tt, filepath.Join(dir, "chart-hourweek-"+mode+".png"))

		ts := &TagState{hover: 1, ready: true}
		tt = render(t, dark, 460, 520, func(c *ui.Context, th *theme.Theme) {
			TagDistributionChart(c, th, TagDistributionProps{Tags: sampleTags(), State: ts})
		})
		savePNG(t, tt, filepath.Join(dir, "chart-tags-"+mode+".png"))

		tt = render(t, dark, 720, 160, func(c *ui.Context, th *theme.Theme) {
			PlayHeatmap(c, th, HeatmapProps{Cells: sampleHeatmap()})
		})
		savePNG(t, tt, filepath.Join(dir, "chart-heatmap-"+mode+".png"))
	}
}
