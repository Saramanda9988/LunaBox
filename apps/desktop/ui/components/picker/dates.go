// Package picker 对应原版 components/ui/better 里的两类输入控件：
// BetterDateRangePicker（按钮 + 日历弹层，单选或区间）与 BetterWheelPicker（iOS 式滚轮，时间与时长）。
// 日期沿用原版的 "YYYY-MM-DD" 字符串（空串表示未选），时间与时长是分钟数、秒数；
// 组件不持有选中值，变化都通过回调交给调用方，组件自己的临时状态（弹层开关、显示的月份、滚轮位置）用 ui.Local 挂在自己的元素上。
package picker

import (
	"regexp"
	"strconv"
	"time"
)

var dateRe = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)

// parseDateValue 对应 parseDateValue：YYYY-MM-DD 且日期真实存在时返回本地零点。
func parseDateValue(s string) (time.Time, bool) {
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

// FormatDate 对应 utils/time.ts 的 formatDateToYYYYMMDD：本地日期的 YYYY-MM-DD。
func FormatDate(t time.Time) string { return t.Format("2006-01-02") }

// month 是从公元 0 年 1 月起的月序号（year*12 + month-1），代替原版里“某月一号”的 Date，方便比较与加减。
type month int

func monthOf(t time.Time) month { return month(t.Year()*12 + int(t.Month()) - 1) }

// first 是这个月一号的本地零点。
func (m month) first() time.Time {
	return time.Date(int(m)/12, time.Month(int(m)%12+1), 1, 0, 0, 0, 0, time.Local)
}

// label 对应 formatCalendarMonth：YYYY/MM。
func (m month) label() string {
	return m.first().Format("2006/01")
}

// initialLeftMonth 对应 getInitialLeftMonth：起始日期（没有时取今天）所在的月，最晚是上个月（右侧日历留给本月）。
func initialLeftMonth(startDate string, today time.Time) month {
	selected := monthOf(today)
	if d, ok := parseDateValue(startDate); ok {
		selected = monthOf(d)
	}
	return min(selected, monthOf(today)-1)
}

// initialSingleMonth 对应 getInitialSingleMonth：起始日期所在的月，最晚是本月。
func initialSingleMonth(startDate string, today time.Time) month {
	selected := monthOf(today)
	if d, ok := parseDateValue(startDate); ok {
		selected = monthOf(d)
	}
	return min(selected, monthOf(today))
}

// initialRightMonth 对应 getInitialRightMonth：结束日期所在的月（没有时是左侧的下一个月），
// 必须在左侧之后，且最晚是本月。
func initialRightMonth(left month, endDate string, today time.Time) month {
	fallback := left + 1
	right := fallback
	if d, ok := parseDateValue(endDate); ok {
		right = monthOf(d)
	}
	if right <= left {
		return fallback
	}
	return min(right, monthOf(today))
}

// calendarDays 对应 getCalendarDays：从本月一号所在周的周日开始的 42 天。
func calendarDays(m month) [42]time.Time {
	first := m.first()
	start := first.AddDate(0, 0, -int(first.Weekday()))
	var days [42]time.Time
	for i := range days {
		days[i] = start.AddDate(0, 0, i)
	}
	return days
}
