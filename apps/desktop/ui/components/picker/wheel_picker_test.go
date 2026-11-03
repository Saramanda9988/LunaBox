package picker

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// wheelHarness 是 AddPlaySessionModal 里的开始时间与游玩时长。
type wheelHarness struct {
	th       *theme.Theme
	minutes  int
	seconds  int
	duration bool // 显示时长滚轮（否则显示时间滚轮）
	disabled bool
	changes  int
}

const harnessWidth = 455

func (h *wheelHarness) view(c *ui.Context) {
	t := h.th
	ui.Column(c).Fill().Padding(24).Background(t.AppBg).Children(func() {
		ui.Column(c).Width(harnessWidth).Children(func() {
			if h.duration {
				DurationWheelPicker(c, t, "duration", DurationWheelProps{
					ValueSeconds: h.seconds, OnChange: func(v int) { h.seconds = v; h.changes++ },
					HourLabel: "时", MinuteLabel: "分", SecondLabel: "秒", Disabled: h.disabled,
				})
			} else {
				TimeWheelPicker(c, t, "time", TimeWheelProps{
					ValueMinutes: h.minutes, OnChange: func(v int) { h.minutes = v; h.changes++ }, Disabled: h.disabled,
				})
			}
		})
	})
}

func newWheelTester(h *wheelHarness) *ui.Tester {
	h.th = theme.New(true, 1.5)
	tt := ui.NewTester(h.view, 800, 600)
	tt.SetScale(1.5)
	tt.SetDark(true)
	tt.Frame()
	return tt
}

// column 返回第 i 列滚轮的位置（列数 n）：外框 24 的内边距、边框、p-3，列间 gap-1 与 w-7 的分隔符。
func (h *wheelHarness) column(i, n int) ui.Rect {
	hl := h.th.HL
	x0, y0 := 24+hl+12, 24+hl+12
	rowW := harnessWidth - 2*hl - 24
	colW := (rowW - float32(n-1)*(28+8)) / float32(n)
	return ui.Rect{X: x0 + float32(i)*(colW+36), Y: y0, W: colW, H: wheelHeight}
}

// settle 把测试时钟推进 d，等缓动跑完（滚一项 sqrt(1/5)≈0.45s），再构建两帧让回调生效。
func settle(tt *ui.Tester, d time.Duration) {
	tt.Frame()
	tt.Advance(d)
	tt.Frame()
	tt.Frame()
}

func TestTimeWheelScroll(t *testing.T) {
	h := &wheelHarness{minutes: 19*60 + 52}
	tt := newWheelTester(h)
	hour := h.column(0, 2)
	cx, cy := hour.X+hour.W/2, hour.Y+hour.H/2
	tt.Scroll(cx, cy, 0, 40)
	settle(tt, 600*time.Millisecond)
	if h.minutes != 20*60+52 {
		t.Fatalf("scroll down on hours: got %d want %d", h.minutes, 20*60+52)
	}
	// 100ms 内的第二次滚动被忽略。
	tt.Scroll(cx, cy, 0, -40)
	tt.Scroll(cx, cy, 0, -40)
	settle(tt, 600*time.Millisecond)
	if h.minutes != 19*60+52 {
		t.Fatalf("throttled scroll up: got %d want %d", h.minutes, 19*60+52)
	}
	// 分钟列循环：59 往下是 00，小时不变。
	h.minutes = 23*60 + 59
	tt.Frame()
	minute := h.column(1, 2)
	tt.Advance(wheelThrottle)
	tt.Scroll(minute.X+minute.W/2, cy, 0, 40)
	settle(tt, 600*time.Millisecond)
	if h.minutes != 23*60 {
		t.Fatalf("minute wrap: got %d want %d", h.minutes, 23*60)
	}
	// 小时列循环：00 往上是 23。
	h.minutes = 5
	tt.Frame()
	tt.Advance(wheelThrottle)
	tt.Scroll(cx, cy, 0, -40)
	settle(tt, 600*time.Millisecond)
	if h.minutes != 23*60+5 {
		t.Fatalf("hour wrap: got %d want %d", h.minutes, 23*60+5)
	}
}

// itemCenterY 是点击表里中心以下第 off 项的中点（相对列顶部）。
func itemCenterY(off int) float32 {
	acc := 0.0
	for s, r := 0, halfVisible-1; r >= -halfVisible+1; s, r = s+1, r-1 {
		h := itemH * math.Cos(float64(r)*stepRad)
		if s-(halfVisible-1) == off {
			return float32(acc + h/2)
		}
		acc += h
	}
	panic("offset out of range")
}

func TestTimeWheelClickAndDrag(t *testing.T) {
	h := &wheelHarness{minutes: 10 * 60}
	tt := newWheelTester(h)
	hour := h.column(0, 2)
	cx := hour.X + hour.W/2
	// 点中心下方第二项：滚到 12 点（用时 sqrt(2/5)≈0.63s）。
	tt.ClickAt(cx, hour.Y+itemCenterY(2))
	settle(tt, 800*time.Millisecond)
	if h.minutes != 12*60 {
		t.Fatalf("click +2: got %d", h.minutes)
	}
	tt.ClickAt(cx, hour.Y+itemCenterY(-3))
	settle(tt, 900*time.Millisecond)
	if h.minutes != 9*60 {
		t.Fatalf("click -3: got %d", h.minutes)
	}
	// 向上拖两项后停住再松手（速度为 0）：停在 11 点。
	cy := hour.Y + hour.H/2
	tt.Press(cx, cy)
	tt.Move(cx, cy-30)
	tt.Move(cx, cy-64)
	tt.Move(cx, cy-64)
	tt.Release(cx, cy-64)
	settle(tt, 50*time.Millisecond)
	if h.minutes != 11*60 {
		t.Fatalf("drag up two items: got %d", h.minutes)
	}
	// 同库里一样，停住松手时截断而不是四舍五入：往上拖 1.7 项停在 +1。
	tt.Press(cx, cy)
	tt.Move(cx, cy-20)
	tt.Move(cx, cy-1.7*itemH)
	tt.Move(cx, cy-1.7*itemH)
	tt.Release(cx, cy-1.7*itemH)
	settle(tt, 50*time.Millisecond)
	if h.minutes != 12*60 {
		t.Fatalf("drag up 1.7 items: got %d", h.minutes)
	}
}

func TestTimeWheelKeyboard(t *testing.T) {
	h := &wheelHarness{minutes: 8*60 + 30}
	tt := newWheelTester(h)
	tt.Key(0, ui.KeyTab) // 焦点组：Tab 进入第一列
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	settle(tt, 600*time.Millisecond)
	if h.minutes != 9*60+30 {
		t.Fatalf("ArrowDown on hours: got %d", h.minutes)
	}
	tt.Key(0, ui.KeyRight) // ←→ 在列之间移动焦点
	tt.Frame()
	tt.Key(0, ui.KeyUp)
	settle(tt, 600*time.Millisecond)
	if h.minutes != 9*60+29 {
		t.Fatalf("ArrowUp on minutes: got %d", h.minutes)
	}
	// 输入跳转：快速输入“45”跳到 45 分。
	tt.Type("4")
	tt.Type("5")
	settle(tt, 2200*time.Millisecond) // 跳 16 项用时 sqrt(16/5)≈1.8s
	if h.minutes != 9*60+45 {
		t.Fatalf("typeahead 45: got %d", h.minutes)
	}
}

func TestDurationWheel(t *testing.T) {
	h := &wheelHarness{duration: true, seconds: 60}
	tt := newWheelTester(h)
	for _, s := range []string{"时", "分", "秒"} {
		if !tt.HasText(s) {
			t.Fatalf("missing label %q: %q", s, tt.Texts())
		}
	}
	// 0 时 1 分往上滚到 0 分：秒最少 1，0 秒被夹到 1 秒。
	minute := h.column(1, 3)
	tt.Scroll(minute.X+minute.W/2, minute.Y+minute.H/2, 0, -40)
	settle(tt, 600*time.Millisecond)
	if h.seconds != 1 {
		t.Fatalf("minute to 0 should coerce seconds to 1: got %d", h.seconds)
	}
	// 小时列循环：00 往上是 99。
	hour := h.column(0, 3)
	tt.Advance(wheelThrottle)
	tt.Scroll(hour.X+hour.W/2, hour.Y+hour.H/2, 0, -40)
	settle(tt, 600*time.Millisecond)
	if h.seconds != 99*3600+1 {
		t.Fatalf("hour wrap to 99: got %d", h.seconds)
	}
	// 外部改值：滚轮跟着跳过去，不回调。
	changes := h.changes
	h.seconds = 3*3600 + 25*60 + 7
	tt.Frame()
	tt.Frame()
	if h.changes != changes || h.seconds != 3*3600+25*60+7 {
		t.Fatalf("external value change called back: %d %d", h.changes, h.seconds)
	}
}

func TestDurationOptions(t *testing.T) {
	if got := allowedSeconds(0, 0, 1, DefaultMaxSeconds); got[0] != 1 || len(got) != 59 {
		t.Errorf("seconds at 0:00 = %v", got)
	}
	if got := allowedMinutes(99, 1, DefaultMaxSeconds); len(got) != 60 {
		t.Errorf("minutes at 99h = %d values", len(got))
	}
	if got := allowedMinutes(2, 3600+30*60, 2*3600+15*60); got[0] != 0 || got[len(got)-1] != 15 {
		t.Errorf("minutes at max hour = %v", got)
	}
	if got := coerceValue(0, allowedSeconds(0, 0, 1, 100)); got != 1 {
		t.Errorf("coerce 0 → %d", got)
	}
	if got := len(repeatOptions(WheelOptions(valueRange(1, 3)))); got != 12 {
		t.Errorf("short option lists repeat to at least 10 items, got %d", got)
	}
}

func TestWheelDisabled(t *testing.T) {
	h := &wheelHarness{minutes: 60, disabled: true}
	tt := newWheelTester(h)
	hour := h.column(0, 2)
	tt.Scroll(hour.X+hour.W/2, hour.Y+hour.H/2, 0, 40)
	tt.ClickAt(hour.X+hour.W/2, hour.Y+itemCenterY(2))
	settle(tt, 700*time.Millisecond)
	if h.minutes != 60 || h.changes != 0 {
		t.Fatalf("disabled wheel changed: %d", h.minutes)
	}
}
