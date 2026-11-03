package picker

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// rangeHarness 是 stats 页（StatsToolbar）那样的区间用法，single 时是 AddPlaySessionModal 的单日用法。
type rangeHarness struct {
	th              *theme.Theme
	start, end      string
	single          bool
	inModal         bool
	disabled        bool
	applied, resets int
	starts, ends    []string
}

func (h *rangeHarness) props() DateRangeProps {
	p := DateRangeProps{
		StartDate: h.start, EndDate: h.end,
		TriggerLabel: "自定义", ApplyLabel: "应用", ResetLabel: "重置",
		Active: h.start != "" && h.end != "", Disabled: h.disabled,
		OnStartDateChange: func(v string) { h.start = v; h.starts = append(h.starts, v) },
		OnEndDateChange:   func(v string) { h.end = v; h.ends = append(h.ends, v) },
		OnApply:           func() { h.applied++ },
		OnReset:           func() { h.resets++; h.start, h.end = "", "" },
	}
	if h.single {
		p.TriggerLabel, p.ApplyLabel = "选择日期", "确定"
		p.SelectionMode, p.CalendarMode, p.PanelAnchor = SelectSingle, CalendarSingle, AnchorBottom
		p.Active = h.start != ""
	}
	return p
}

func (h *rangeHarness) view(c *ui.Context) {
	t := h.th
	body := func() {
		ui.Column(c).Width(455).Gap(4).Children(func() {
			components.SectionLabel(c, t, "日期")
			btn := DateRangePicker(c, t, "date", h.props())
			if h.single {
				btn.FillWidth()
			}
		})
	}
	ui.Column(c).Fill().Padding(24).Background(t.AppBg).Children(func() {
		if !h.inModal {
			body()
			return
		}
		components.Modal(c, t, "modal", components.ModalProps{Open: true, MaxWidth: 512, OnClose: func() {}}, func(panel ui.Element) {
			components.ModalPanel(t, panel).Children(body)
		})
	})
}

func newRangeTester(h *rangeHarness, width, height int) *ui.Tester {
	if h.th == nil {
		h.th = theme.New(true, 1.5)
	}
	tt := ui.NewTester(h.view, width, height)
	tt.SetScale(1.5)
	tt.SetDark(h.th.Dark)
	tt.Frame()
	return tt
}

func click(t *testing.T, tt *ui.Tester, s string) {
	t.Helper()
	if err := tt.Click(s); err != nil {
		t.Fatalf("%v; texts: %q", err, tt.Texts())
	}
	tt.Frame()
}

func day(m month, d int) string { return FormatDate(m.first().AddDate(0, 0, d-1)) }

func TestDateRangeSelectApplyReset(t *testing.T) {
	h := &rangeHarness{}
	tt := newRangeTester(h, 1280, 800)
	this := monthOf(time.Now())
	if tt.HasText("Su") {
		t.Fatal("panel shows before opening")
	}
	click(t, tt, "自定义")
	for _, s := range []string{"Su", "Sa", (this - 1).label(), this.label(), "应用", "重置"} {
		if !tt.HasText(s) {
			t.Fatalf("open panel lacks %q: %q", s, tt.Texts())
		}
	}
	// 第一次点设开始、清空结束；第二次点更早的日期改开始；再点更晚的日期设结束。
	click(t, tt, day(this-1, 10))
	if h.start != day(this-1, 10) || h.end != "" {
		t.Fatalf("after first pick start=%q end=%q", h.start, h.end)
	}
	click(t, tt, day(this-1, 5))
	if h.start != day(this-1, 5) || h.end != "" {
		t.Fatalf("earlier pick should move start: start=%q end=%q", h.start, h.end)
	}
	click(t, tt, day(this-1, 20))
	if h.end != day(this-1, 20) {
		t.Fatalf("end=%q", h.end)
	}
	want := h.start + " - " + h.end
	if !tt.HasText(want) {
		t.Fatalf("trigger/footer should show %q: %q", want, tt.Texts())
	}
	// 区间完整后再点：重新开始。
	click(t, tt, day(this-1, 8))
	if h.start != day(this-1, 8) || h.end != "" {
		t.Fatalf("restart: start=%q end=%q", h.start, h.end)
	}
	// 区间不完整时“应用”不关闭面板。
	click(t, tt, "应用")
	if h.applied != 1 || !tt.HasText("Su") {
		t.Fatalf("incomplete apply: applied=%d open=%v", h.applied, tt.HasText("Su"))
	}
	click(t, tt, day(this-1, 9))
	click(t, tt, "应用")
	if h.applied != 2 || tt.HasText("Su") {
		t.Fatalf("complete apply should close: applied=%d open=%v", h.applied, tt.HasText("Su"))
	}
	click(t, tt, h.start+" - "+h.end)
	click(t, tt, "重置")
	if h.resets != 1 || tt.HasText("Su") || !tt.HasText("自定义") {
		t.Fatalf("reset: resets=%d open=%v texts=%q", h.resets, tt.HasText("Su"), tt.Texts())
	}
}

func TestDateRangeMonthNavigation(t *testing.T) {
	h := &rangeHarness{}
	tt := newRangeTester(h, 1280, 800)
	this := monthOf(time.Now())
	click(t, tt, "自定义")
	// 左侧下一月会追上右侧，右侧下一月超过本月：都禁用。
	click(t, tt, "Next month")
	click(t, tt, "Next year")
	if !tt.HasText((this - 1).label()) || !tt.HasText(this.label()) {
		t.Fatalf("disabled buttons moved the months: %q", tt.Texts())
	}
	click(t, tt, "Previous month") // 左侧
	if !tt.HasText((this - 2).label()) {
		t.Fatalf("left should show %s: %q", (this - 2).label(), tt.Texts())
	}
	click(t, tt, "Previous year")
	if !tt.HasText((this - 14).label()) {
		t.Fatalf("left should show %s: %q", (this - 14).label(), tt.Texts())
	}
	click(t, tt, "Next year")
	if !tt.HasText((this - 2).label()) {
		t.Fatalf("left should be back at %s: %q", (this - 2).label(), tt.Texts())
	}
	// 关闭再打开，月份保留（状态在组件上，不在面板上）。
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText("Su") {
		t.Fatal("Escape should close the panel")
	}
	click(t, tt, "自定义")
	if !tt.HasText((this - 2).label()) {
		t.Fatalf("months should persist across opening: %q", tt.Texts())
	}
	// 窄窗口下两个月上下排列，仍然都在。
	tt.SetSize(1000, 1200)
	tt.Frame()
	l, ok1 := tt.Find((this - 2).label())
	r, ok2 := tt.Find(this.label())
	if !ok1 || !ok2 || r.Y <= l.Y {
		t.Fatalf("stacked calendars: left %v %v right %v %v", l, ok1, r, ok2)
	}
}

func TestDateRangeInitialMonths(t *testing.T) {
	now := time.Now()
	this := monthOf(now)
	cases := []struct {
		start, end  string
		left, right month
	}{
		{"", "", this - 1, this},
		{day(this-5, 3), day(this-2, 1), this - 5, this - 2},
		{day(this-5, 3), day(this-5, 9), this - 5, this - 4}, // 同一个月：右侧取下一个月
		{day(this, 3), "", this - 1, this},                   // 左侧最晚是上个月
	}
	for _, tc := range cases {
		left := initialLeftMonth(tc.start, now)
		right := initialRightMonth(left, tc.end, now)
		if left != tc.left || right != tc.right {
			t.Errorf("%q..%q: got %s/%s want %s/%s", tc.start, tc.end, left.label(), right.label(), tc.left.label(), tc.right.label())
		}
	}
	if m := initialSingleMonth(day(this+3, 1), now); m != this {
		t.Errorf("single calendar month should clamp to this month, got %s", m.label())
	}
	if _, ok := parseDateValue("2026-02-30"); ok {
		t.Error("2026-02-30 should be invalid")
	}
}

func TestDateSingleInModal(t *testing.T) {
	now := time.Now()
	this := monthOf(now)
	today := FormatDate(now)
	h := &rangeHarness{single: true, inModal: true, start: today, end: today}
	tt := newRangeTester(h, 1280, 800)
	click(t, tt, today)
	if !tt.HasText("Su") || !tt.HasText(this.label()) || tt.HasText((this - 1).label()) {
		t.Fatalf("single calendar should show only %s: %q", this.label(), tt.Texts())
	}
	// 面板居中在按钮下方（panelAnchor="bottom"），间隔 8px 的 anchor-gap 加 mt-2。800 高的窗口里按钮上下都放不下
	// 整个面板，面板滚动（overflow-auto）：滚动条占在右侧，内容（月份标题）比按钮中线偏左半个滚动条宽。
	btn, _ := tt.Find(today)
	title, _ := tt.Find(this.label())
	if d := (title.X + title.W/2) - (btn.X + btn.W/2) + h.th.Gutter()/2; d < -1 || d > 1 {
		t.Errorf("panel not centered under the trigger: title %v trigger %v", title, btn)
	}
	// 未来的日期不能选。
	click(t, tt, FormatDate(now.AddDate(0, 0, 1)))
	if h.start != today {
		t.Fatalf("future day was selectable: %q", h.start)
	}
	// 单选：开始、结束都设成同一天，按钮显示这一天。
	pick := day(this, 1)
	if pick == today {
		pick = FormatDate(now.AddDate(0, 0, -1))
	}
	click(t, tt, pick)
	if h.start != pick || h.end != pick || len(h.starts) != 1 || len(h.ends) != 1 {
		t.Fatalf("single pick: start=%q end=%q calls %v %v", h.start, h.end, h.starts, h.ends)
	}
	click(t, tt, "确定")
	if tt.HasText("Su") || h.applied != 1 {
		t.Fatalf("confirm should close: open=%v applied=%d", tt.HasText("Su"), h.applied)
	}
	// 在模态框里 Esc 也关闭日历。
	click(t, tt, pick)
	if !tt.HasText("Su") {
		t.Fatal("reopen failed")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText("Su") {
		t.Fatal("Escape inside the modal should close the calendar")
	}
}

func TestDateRangeDisabled(t *testing.T) {
	h := &rangeHarness{disabled: true}
	tt := newRangeTester(h, 1280, 800)
	click(t, tt, "自定义")
	if tt.HasText("Su") {
		t.Fatal("disabled trigger opened the panel")
	}
}

func TestDayColors(t *testing.T) {
	light, dark := theme.New(false, 1), theme.New(true, 1)
	// 和运行中的原版核对过的计算样式。
	cases := []struct {
		th      *theme.Theme
		d       dayState
		hovered bool
		bg, fg  ui.Color
	}{
		{light, dayState{future: true}, false, ui.Color{}, theme.Brand700},
		{light, dayState{future: true, outside: true}, false, ui.Color{}, theme.Brand300},
		{light, dayState{future: true}, true, ui.Transparent, theme.Brand200},
		{dark, dayState{future: true}, false, ui.Color{}, theme.Brand700},
		{dark, dayState{future: true, outside: true}, false, ui.Color{}, theme.Brand700},
		{dark, dayState{outside: true}, false, ui.Color{}, theme.Brand600},
		{dark, dayState{}, false, ui.Color{}, theme.Brand200},
		{dark, dayState{selected: true}, false, theme.White, ui.Hex("#0a0a0a")},
		{dark, dayState{ranged: true}, false, theme.Brand700.Alpha(0.7), theme.Brand200},
		{dark, dayState{ranged: true, outside: true}, false, theme.Brand700.Alpha(0.7), theme.Brand600},
		{dark, dayState{ranged: true}, true, theme.Brand700, theme.Brand200},
		{dark, dayState{selected: true, outside: true}, true, theme.White, theme.Brand300},
		{light, dayState{selected: true}, false, ui.Hex("#1e293b"), theme.Brand700},
		{light, dayState{selected: true, outside: true}, false, ui.Hex("#1e293b"), theme.Brand300},
		{light, dayState{ranged: true, outside: true}, false, theme.Brand100, theme.Brand800},
		{light, dayState{ranged: true, outside: true}, true, theme.Brand100, theme.Brand600},
		{light, dayState{future: true, outside: true}, true, ui.Transparent, theme.Brand600},
	}
	for i, tc := range cases {
		bg, fg := dayColors(tc.th, tc.d, tc.hovered)
		if bg != tc.bg || fg != tc.fg {
			t.Errorf("case %d: got bg %v fg %v want %v %v", i, bg, fg, tc.bg, tc.fg)
		}
	}
}
