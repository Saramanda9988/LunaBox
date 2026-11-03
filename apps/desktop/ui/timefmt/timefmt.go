// Package timefmt 对应原版 frontend/src/utils/time.ts：时长与本地日期的显示格式，跟随应用的语言。
package timefmt

import (
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/ui/locales"
)

// Duration 对应 formatDuration(seconds, t)：1小时5分钟、5分钟、30秒（en-US 是 1H5M）。
func Duration(seconds int) string {
	h, m, s := seconds/3600, seconds%3600/60, seconds%60
	switch {
	case h > 0 && m > 0:
		return strconv.Itoa(h) + locales.CommonDurationHours() + strconv.Itoa(m) + locales.CommonDurationMinutes()
	case h > 0:
		return strconv.Itoa(h) + locales.CommonDurationHours()
	case m > 0:
		return strconv.Itoa(m) + locales.CommonDurationMinutes()
	}
	return strconv.Itoa(s) + locales.CommonDurationSeconds()
}

// DurationCompact 对应 formatDurationCompact(seconds, t)：灵动岛上的 1时5分、2时、0分（不足一分钟按 0 分）。
func DurationCompact(seconds int) string {
	total := max(0, seconds) / 60
	h, m := total/60, total%60
	hs, ms := locales.CommonDurationHoursShort(), locales.CommonDurationMinutesShort()
	switch {
	case h > 0 && m > 0:
		return strconv.Itoa(h) + hs + strconv.Itoa(m) + ms
	case h > 0:
		return strconv.Itoa(h) + hs
	}
	return strconv.Itoa(m) + ms
}

// DurationChart 对应 formatDurationChart(seconds, t)：图表坐标轴上的 5时 2分、0分（en-US 是 5h 2m）。
func DurationChart(seconds int) string {
	total := max(0, seconds) / 60
	m := strconv.Itoa(total%60) + locales.CommonDurationMinutesShort()
	if h := total / 60; h > 0 {
		return strconv.Itoa(h) + locales.CommonDurationHoursShort() + " " + m
	}
	return m
}

// Location 是配置里的时区（time_zone），没有配置或无法识别时用本地时区。
func Location(cfg *appconf.AppConfig) *time.Location {
	if cfg != nil && cfg.TimeZone != "" {
		if loc, err := time.LoadLocation(cfg.TimeZone); err == nil {
			return loc
		}
	}
	return time.Local
}

// Date 对应 formatLocalDate：年、两位的月与日，按语言排列（zh-CN 2026/09/11，en-US 09/11/2026）。
func Date(t time.Time, loc *time.Location) string {
	return i18n.FormatDate(FormatLocale(), t.In(loc), i18n.DateOptions{Skeleton: "yMMdd"})
}

// DateTime 对应 formatLocalDateTime：日期与 24 小时制的时分秒（zh-CN 2026/09/11 13:05:09）。
func DateTime(t time.Time, loc *time.Location) string {
	return i18n.FormatDate(FormatLocale(), t.In(loc), i18n.DateOptions{Skeleton: "yMMddHHmmss", HourCycle: 24})
}

// DateTimeMinutes 是不带秒的 DateTime（formatLocalDateTime(…, {second: undefined})）：2026/09/11 13:05。
func DateTimeMinutes(t time.Time, loc *time.Location) string {
	return i18n.FormatDate(FormatLocale(), t.In(loc), i18n.DateOptions{Skeleton: "yMMddHHmm", HourCycle: 24})
}

// FormatLocale 是格式化日期、数字用的 locale（i18n.FormatDate 的第一个参数）。原版用 toLocaleString(undefined)，即 WebView 的默认语言；
// 这里用应用的语言与用户在系统里设置的区域格式（Region.FormatLocale）。应用没有运行（测试）时是源语言 zh-CN。
func FormatLocale() string {
	lang := i18n.Locale()
	if lang == "" {
		return "zh-CN"
	}
	if mygo.App.IsReady() {
		return mygo.App.Region().FormatLocale(lang)
	}
	return lang
}

// ParseDate 解析后端返回的本地日期 YYYY-MM-DD（parseDateOnlyToLocalTimestamp）。
func ParseDate(s string, loc *time.Location) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	return t, err == nil
}
