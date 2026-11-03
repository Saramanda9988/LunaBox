package picker

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

func TestClockMinutes(t *testing.T) {
	for in, want := range map[string]int{"03:30": 210, "3:5": 185, " 23:59 ": 1439, "24:00": 0, "12:60": 0, "abc": 0} {
		if got := ParseClockMinutes(in); got != want {
			t.Errorf("ParseClockMinutes(%q) = %d, want %d", in, got, want)
		}
	}
	for in, want := range map[int]string{0: "00:00", 210: "03:30", 1440: "00:00", -1: "23:59"} {
		if got := FormatClockMinutes(in); got != want {
			t.Errorf("FormatClockMinutes(%d) = %q, want %q", in, got, want)
		}
	}
}

// panelOpen 报告滚轮面板是否打开：滚轮的数字是画出来的，面板里能找到的文字只有两列之间的“:”。
func panelOpen(tt *ui.Tester) bool {
	return slices.Contains(tt.Texts(), ":")
}

// 点击输入框打开滚轮面板，Esc 关闭；禁用时不打开。
func TestTimeWheelInput(t *testing.T) {
	th := theme.New(true, 1.5)
	value, disabled := "03:30", false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(24).Background(th.AppBg).Children(func() {
			ui.Column(c).Width(360).Children(func() {
				TimeWheelInput(c, th, "time", TimeWheelInputProps{
					Value: value, ActionLabel: "选择时间", Disabled: disabled, // i18n:keep：测试数据
					OnChange: func(v string) { value = v },
				})
			})
		})
	}, 800, 600)
	tt.SetScale(1.5)
	tt.SetDark(true)
	tt.Frame()
	if !tt.HasText("03:30") {
		t.Fatalf("value not shown: %q", tt.Texts())
	}
	if err := tt.Click("03:30"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// 滚轮的数字是画出来的，面板打开时能找到的文字是两列之间的“:”。
	if !panelOpen(tt) {
		t.Fatalf("wheel panel not open: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if panelOpen(tt) {
		t.Errorf("Escape keeps the panel open: %q", tt.Texts())
	}

	disabled = true
	tt.Frame()
	if err := tt.Click("03:30"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if panelOpen(tt) {
		t.Errorf("disabled input opened the panel")
	}
}
