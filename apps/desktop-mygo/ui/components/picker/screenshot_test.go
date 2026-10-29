package picker

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// screenScene 把控件放在与原版截图相同的位置（1280×800 视口、150% 缩放）：
// modal 是 AddPlaySessionModal 的表单（x=408 y=170 宽 455.33），stats 是统计页工具栏的“自定义”按钮（x=442 y=124）。
type screenScene struct {
	th            *theme.Theme
	modal         bool
	date          string
	start, end    string
	minutes, secs int
}

func (s *screenScene) view(c *ui.Context) {
	t := s.th
	ui.Box(c).Fill().Background(t.AppBg).Children(func() {
		if !s.modal {
			ui.Box(c).Absolute().Left(442).Top(124).Children(func() {
				DateRangePicker(c, t, "range", DateRangeProps{
					StartDate: s.start, EndDate: s.end, TriggerLabel: "自定义", ApplyLabel: "应用", ResetLabel: "重置",
					OnStartDateChange: func(v string) { s.start = v }, OnEndDateChange: func(v string) { s.end = v },
				})
			})
			return
		}
		// 模态框面板：rounded-lg bg-white dark:bg-brand-800。
		ui.Box(c).Absolute().Left(384).Top(70).Size(512, 690).Radius(8).Background(t.PanelBg)
		ui.Column(c).Absolute().Left(408).Top(170).Width(455.33).Gap(16).Children(func() {
			ui.Column(c).Children(func() {
				d := components.LineOverflow(ui.Span{Text: "日期", Size: theme.TextSM.Size, Weight: theme.WeightMedium}, theme.TextSM.Line)
				components.SectionLabel(c, t, "日期").Margin(-d, 0, 4-d, 0)
				DateRangePicker(c, t, "date", DateRangeProps{
					StartDate: s.date, EndDate: s.date, TriggerLabel: "选择日期", ApplyLabel: "确定", ResetLabel: "重置",
					SelectionMode: SelectSingle, CalendarMode: CalendarSingle, PanelAnchor: AnchorBottom, Active: s.date != "",
					OnStartDateChange: func(v string) { s.date = v }, OnEndDateChange: func(v string) { s.date = v },
				}).FillWidth()
			})
			ui.Column(c).Children(func() {
				d := components.LineOverflow(ui.Span{Text: "开始时间", Size: theme.TextSM.Size, Weight: theme.WeightMedium}, theme.TextSM.Line)
				components.SectionLabel(c, t, "开始时间").Margin(-d, 0, 4-d, 0)
				TimeWheelPicker(c, t, "start", TimeWheelProps{ValueMinutes: s.minutes, OnChange: func(v int) { s.minutes = v }})
			})
			ui.Column(c).Children(func() {
				d := components.LineOverflow(ui.Span{Text: "游玩时长", Size: theme.TextSM.Size, Weight: theme.WeightMedium}, theme.TextSM.Line)
				components.SectionLabel(c, t, "游玩时长").Margin(-d, 0, 8-d, 0)
				DurationWheelPicker(c, t, "duration", DurationWheelProps{
					ValueSeconds: s.secs, OnChange: func(v int) { s.secs = v },
					HourLabel: "时", MinuteLabel: "分", SecondLabel: "秒",
				})
			})
		})
	})
}

// TestScreenshotDump 在 DESKTOP_MYGO_PNG 目录里保存各控件关闭与打开、深浅两种主题的截图，用来和原版截图对照。
func TestScreenshotDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	save := func(tt *ui.Tester, name string) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	today := FormatDate(time.Now())
	for _, dark := range []bool{true, false} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		run := func(s *screenScene, width int, name, open string) {
			s.th = theme.New(dark, 1.5)
			tt := ui.NewTester(s.view, width, 800)
			tt.SetScale(1.5)
			tt.SetDark(dark)
			tt.Frame()
			save(tt, name+"-"+mode+".png")
			if err := tt.Click(open); err != nil {
				t.Fatal(err)
			}
			tt.Move(5, 5)
			tt.Frame()
			tt.Advance(300 * time.Millisecond) // 等颜色过渡
			tt.Frame()
			save(tt, name+"-open-"+mode+".png")
		}
		run(&screenScene{modal: true, date: today, minutes: 19*60 + 52, secs: 3600}, 1280, "modal", today)
		run(&screenScene{}, 1280, "stats", "自定义")
		run(&screenScene{}, 1000, "stats-narrow", "自定义")
	}
}
