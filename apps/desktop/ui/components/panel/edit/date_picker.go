package edit

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/theme"
)

var weekdayLabels = []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}

var dateRe = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)

// parseDateInputValue 对应 parseDateInputValue：YYYY-MM-DD 且日期真实存在时返回本地零点。
func parseDateInputValue(s string) (time.Time, bool) {
	m := dateRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	date := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
	if date.Year() != y || int(date.Month()) != mo || date.Day() != d {
		return time.Time{}, false
	}
	return date, true
}

// formatDateInputValue 对应 utils/time.ts 的 formatDateInputValue：合法的 YYYY-MM-DD 原样返回，否则为空。
func formatDateInputValue(s string) string {
	s = strings.TrimSpace(s)
	if _, ok := parseDateInputValue(s); !ok {
		return ""
	}
	return s
}

func startOfMonth(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
}

// calendarWeek 是 ReleaseDateRow：一周七天。
type calendarWeek struct {
	key   string
	dates [7]time.Time
}

// calendarRows 对应 getCalendarRows：从本月一号所在周的周日开始的六周。
func calendarRows(month time.Time) []calendarWeek {
	first := startOfMonth(month)
	start := first.AddDate(0, 0, -int(first.Weekday()))
	rows := make([]calendarWeek, 6)
	for w := range rows {
		rows[w].key = fmt.Sprintf("week-%d", w)
		for d := range 7 {
			rows[w].dates[d] = start.AddDate(0, 0, w*7+d)
		}
	}
	return rows
}

// datePickerProps 是 ReleaseDatePicker 的属性。
type datePickerProps struct {
	Value      string // YYYY-MM-DD 或空
	Label      string
	ClearLabel string
	CanClear   bool
	Open       *bool
	Month      *time.Time // 显示的月份（组件挂载时取选中日期或今天所在的月）
	OnChange   func(value string)
}

// datePicker 对应 ReleaseDatePicker：只读的 BetterActionInput（点击开关日历，右侧日历与清除按钮），
// 下方 absolute left-0 top-full mt-2 w-[22rem] 的日历面板。
//
// 原版是 readOnly 的 <input className="cursor-pointer">：这里用按钮画出同样的输入框外观。
func datePicker(c *ui.Context, t *theme.Theme, key string, p datePickerProps) {
	if p.Month.IsZero() {
		if d, ok := parseDateInputValue(p.Value); ok {
			*p.Month = startOfMonth(d)
		} else {
			*p.Month = startOfMonth(time.Now())
		}
	}
	toggle := func() { *p.Open = !*p.Open }
	row := ui.Row(c.Key(key)).FillWidth().MinWidth(0).Gap(4).Padding(0, 4, 0, 0).AlignItems(ui.Center).Radius(6).
		Background(t.Pick(theme.White, theme.Brand700)).Border(t.HL, t.Pick(theme.Brand300, theme.Brand600))
	var field ui.Element
	row.Children(func() {
		field = ui.ButtonBase(c.Key("input")).Grow(1).Basis(0).MinWidth(0).Padding(8, 12).Justify(ui.Start).
			FocusRing(false).Cursor(ui.CursorPointer).Label(p.Label)
		field.OnClick(toggle).Children(func() {
			if p.Value == "" {
				// placeholder:text-brand-400 dark:placeholder:text-brand-500
				components.Text(c, p.Label, theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand400, theme.Brand500)).SingleLine()
			} else {
				components.Text(c, strings.ReplaceAll(p.Value, "-", "/"), theme.TextBase, theme.WeightNormal, t.TextStrong).SingleLine()
			}
		})
		actionButton(c, t, inputAction{Icon: "calendar-month-outline", Label: p.Label, OnClick: toggle})
		if p.CanClear {
			actionButton(c, t, inputAction{Icon: "close-circle-outline", Label: p.ClearLabel, OnClick: func() {
				p.OnChange("")
				*p.Open = false
			}})
		}
	})
	if field.Focused() || *p.Open {
		row.Shadow(0, 0, 0, 2, theme.Neutral500) // focus-within（打开时焦点在输入框上）
	}

	winW, _ := c.Size()
	ui.PopoverBase(c, row, p.Open, func(panel ui.Element) {
		// rounded-xl border border-brand-200 bg-white p-3 shadow-xl dark:border-brand-700 dark:bg-brand-800，max-w-[calc(100vw-2rem)]；
		// 弹出层就在页面里（不挂到 body），玻璃模式下 data-glass:bg-white/90 data-glass:backdrop-blur-20 data-glass:dark:bg-brand-900/90。
		panel.Width(min(352, winW-32)).Margin(8, 0, 0, 0).Radius(12).Border(t.HL, t.PanelBorder).
			Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
		body := func() {
			ui.Column(c).Gap(12).Children(func() {
				calendarHeader(c, t, p.Month)
				calendarTable(c, t, p)
			})
		}
		if t.Glass {
			// Material 取代背景，半透明的底色画在里层
			panel.Material(glass.Blur{Radius: 20}).Children(func() {
				ui.Column(c).Padding(12).Radius(12).Background(t.Pick(theme.White.Alpha(0.9), theme.Brand900.Alpha(0.9))).Children(body)
			})
			return
		}
		panel.Padding(12).Background(t.PanelBg).Children(body)
	})
}

// calendarHeader：grid h-9 grid-cols-[4rem_1fr_4rem] items-center，两侧各两个 h-8 w-8 的翻页按钮，中间 text-sm font-semibold 的年月。
func calendarHeader(c *ui.Context, t *theme.Theme, month *time.Time) {
	shift := func(n int) func() {
		return func() { *month = time.Date(month.Year(), month.Month()+time.Month(n), 1, 0, 0, 0, 0, time.Local) }
	}
	ui.Row(c.Key("calendar-header")).Height(36).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Width(64).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
			calendarNavButton(c, t, "chevron-double-left", "Previous year", shift(-12))
			calendarNavButton(c, t, "chevron-left", "Previous month", shift(-1))
		})
		ui.Row(c).Grow(1).Basis(0).MinWidth(0).Justify(ui.Center).Children(func() {
			components.Text(c, fmt.Sprintf("%d/%02d", month.Year(), int(month.Month())), theme.TextSM, theme.WeightSemibold, t.TextStrong).NoWrap()
		})
		ui.Row(c).Width(64).Gap(4).AlignItems(ui.Center).Justify(ui.End).Shrink(0).Children(func() {
			calendarNavButton(c, t, "chevron-right", "Next month", shift(1))
			calendarNavButton(c, t, "chevron-double-right", "Next year", shift(12))
		})
	})
}

// calendarNavButton：flex h-8 w-8 rounded-lg text-brand-500 hover:bg-brand-100 hover:text-brand-900
// dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-white，图标 text-lg。
func calendarNavButton(c *ui.Context, t *theme.Theme, icon, label string, onClick func()) {
	btn := ui.ButtonBase(c.Key(label)).Size(32, 32).Radius(8).Center().Shrink(0).FocusRing(false).Label(label)
	color := t.Pick(theme.Brand500, theme.Brand400)
	if btn.Hovered() {
		btn.Background(t.Pick(theme.Brand100, theme.Brand700))
		color = t.Pick(theme.Brand900, theme.White)
	}
	btn.Transition(theme.ColorsTransition(150 * time.Millisecond))
	btn.OnClick(onClick).Children(func() { components.Icon(c, icon, 18, color) })
}

// calendarTable 是日历的 BetterDataTable（maxHeightClassName="max-h-none"），每格 p-1 text-center 的日期按钮。
func calendarTable(c *ui.Context, t *theme.Theme, p datePickerProps) {
	month := *p.Month
	cols := make([]better.DataTableColumn[calendarWeek], len(weekdayLabels))
	for i, wd := range weekdayLabels {
		cols[i] = better.DataTableColumn[calendarWeek]{
			Key: wd, Header: wd, Center: true, ContentWidth: 32,
			Cell: func(row calendarWeek, _ int) {
				date := row.dates[i]
				value := date.Format("2006-01-02")
				dayButton(c, t, date, date.Month() != month.Month(), value == p.Value, func() {
					p.OnChange(value)
					*p.Open = false
				})
			},
		}
	}
	better.DataTable(c, t, "calendar", better.DataTableProps[calendarWeek]{
		Rows: calendarRows(month), Columns: cols,
		RowKey: func(row calendarWeek, _ int) string { return row.key },
	})
}

// dayButton：inline-flex h-8 w-8 rounded-md text-sm transition-colors。本月以外的日期
// text-brand-300 hover:text-brand-600 dark:text-brand-600 dark:hover:text-brand-300；本月 text-brand-700 hover:bg-brand-100
// dark:text-brand-200 dark:hover:bg-brand-700；选中 bg-neutral-800 text-white dark:bg-white dark:text-neutral-950。
func dayButton(c *ui.Context, t *theme.Theme, date time.Time, outside, selected bool, onClick func()) {
	btn := ui.ButtonBase(c.Key("day")).Size(32, 32).Radius(6).Center().Shrink(0).FocusRing(false).
		Label(date.Format("2006-01-02"))
	hovered := btn.Hovered()
	var bg, fg ui.Color
	switch {
	case selected:
		bg, fg = t.Pick(ui.Hex("#1e293b"), theme.White), t.Pick(theme.White, ui.Hex("#0a0a0a"))
	case outside:
		fg = t.Pick(theme.Brand300, theme.Brand600)
		if hovered {
			fg = t.Pick(theme.Brand600, theme.Brand300)
		}
	default:
		fg = t.Pick(theme.Brand700, theme.Brand200)
		if hovered {
			bg = t.Pick(theme.Brand100, theme.Brand700)
		}
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	fg = btn.AnimateColor("text", fg, theme.Tween(150*time.Millisecond))
	btn.OnClick(onClick).Children(func() {
		components.Text(c, strconv.Itoa(date.Day()), theme.TextSM, theme.WeightNormal, fg).NoWrap()
	})
}
