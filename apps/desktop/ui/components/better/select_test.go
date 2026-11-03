package better

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

func openSelect(t *testing.T, n int) (*ui.Tester, ui.Rect) {
	t.Helper()
	th := theme.New(true, 1)
	opts := make([]SelectOption, n)
	for i := range opts {
		opts[i] = SelectOption{Label: fmt.Sprintf("option-%d", i), Value: fmt.Sprint(i)}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(20).Width(300).Children(func() {
			Select(c, th, "s", SelectProps{Value: "0", Options: opts, Placeholder: "pick"})
		})
	}, 400, 800)
	tt.Frame()
	btn, ok := tt.Find("option-0")
	if !ok {
		t.Fatal("button label not found")
	}
	if err := tt.Click("option-0"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	return tt, btn
}

// 选项少时列表按内容撑开（py-1 + 每项 40）。
func TestSelectListFitsContent(t *testing.T) {
	tt, btn := openSelect(t, 3)
	last, ok := tt.Find("option-2")
	if !ok {
		t.Fatal("last option not shown")
	}
	// 按钮文字下边 + py-2 + 按钮边框 + mt-1 + 列表边框 + py-1 + 两项 40，再加本项的 py-2。
	want := btn.Y + btn.H + 8 + 1 + 4 + 1 + 4 + 2*40 + 8
	if d := last.Y - want; d < -2 || d > 2 {
		t.Errorf("last option at y=%v, want about %v", last.Y, want)
	}
}

// 选项多时列表限高 max-h-60（240），超出的选项被裁掉，滚动后能看到最后一项。
func TestSelectListScrolls(t *testing.T) {
	tt, btn := openSelect(t, 10)
	// 列表视口：按钮文字下边 + py-2 + 按钮边框 + mt-1 + 列表边框，高 240 减上下边框。
	top := btn.Y + btn.H + 8 + 1 + 4 + 1
	bottom := top + 240 - 2
	if last, _ := tt.Find("option-9"); last.H > 0 {
		t.Fatalf("last option visible at y=%v before scrolling; list should stop at %v", last.Y, bottom)
	}
	first, _ := tt.Find("option-1")
	tt.Wheel(first.X, first.Y, 0, 1000)
	for range 20 { // 平滑滚动
		tt.Advance(50 * time.Millisecond)
		tt.Frame()
	}
	last, _ := tt.Find("option-9")
	if last.H == 0 || last.Y < top || last.Y+last.H > bottom {
		t.Errorf("after scrolling, last option at y=%v..%v, want inside %v..%v", last.Y, last.Y+last.H, top, bottom)
	}
}
