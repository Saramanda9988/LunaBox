package picker

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/theme"
)

// WheelOption 是滚轮的一项（WheelPickerOption<number>）。
type WheelOption struct {
	Value int
	Label string // 显示文字，也用于输入跳转
}

// WheelOptions 对应 createWheelOptions：每个值一项，文字补零到两位。
func WheelOptions(values []int) []WheelOption {
	out := make([]WheelOption, len(values))
	for i, v := range values {
		out[i] = WheelOption{Value: v, Label: fmt.Sprintf("%02d", v)}
	}
	return out
}

// valueRange 对应 createRange：[start, end] 的整数，end < start 时为空。
func valueRange(start, end int) []int {
	if end < start {
		return nil
	}
	out := make([]int, end-start+1)
	for i := range out {
		out[i] = start + i
	}
	return out
}

// NumberWheelProps 是 BetterNumberWheelPicker 的属性。ThirdOptions 为 nil 或 OnThirdChange 为 nil 时只有两列。
type NumberWheelProps struct {
	FirstValue, SecondValue, ThirdValue       int
	FirstOptions, SecondOptions, ThirdOptions []WheelOption
	OnFirstChange, OnSecondChange             func(value int)
	OnThirdChange                             func(value int)
	FirstLabel, SecondLabel, ThirdLabel       string // 有任一个时在滚轮下方显示一行标签
	Separator                                 string // 默认 ":"
	Disabled                                  bool
}

// NumberWheelPicker 对应 BetterNumberWheelPicker：glass-input overflow-hidden rounded-xl border border-brand-200
// bg-brand-50/75 p-3 dark:border-brand-700 dark:bg-brand-900/35，disabled 时 pointer-events-none opacity-60；
// 里面 relative overflow-hidden 的一行滚轮（gap-1，列之间 h-8 w-7 的分隔符 text-xl font-semibold leading-none
// text-brand-800 dark:text-brand-100），中间横贯一条 h-8 rounded-md bg-brand-200/70 dark:bg-white/10 的高亮底。
// 宽度填满父元素（块级 div）。
func NumberWheelPicker(c *ui.Context, t *theme.Theme, key string, p NumberWheelProps) ui.Element {
	third := p.ThirdOptions != nil && p.OnThirdChange != nil
	sep := p.Separator
	if sep == "" {
		sep = ":"
	}
	bg, border := t.GlassInput(t.Pick(theme.Brand50.Alpha(0.75), theme.Brand900.Alpha(0.35)), t.Pick(theme.Brand200, theme.Brand700)) // glass-input
	box := ui.Column(c.Key(key)).FillWidth().Padding(12).Radius(12).Clip().Background(bg).Border(t.HL, border)
	if p.Disabled {
		box.Opacity(0.6)
	}
	band := t.Pick(theme.Brand200.Alpha(0.7), theme.White.Alpha(0.1))
	box.Children(func() {
		// 高亮底画在这一行自己的背景上，在各列（及其中的文字）之下，同原版的 absolute 底 + relative z-10 的滚轮。
		row := ui.Row(c.Key("wheels")).Height(wheelHeight).Gap(4).AlignItems(ui.Stretch).Clip()
		if !p.Disabled {
			row.FocusGroup(ui.Horizontal)
		}
		row.Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2 - itemH/2, W: r.W, H: itemH}, band, 6)
		}).Children(func() {
			wheelColumn(c, t, wheelProps{key: "first", value: p.FirstValue, options: p.FirstOptions, onChange: p.OnFirstChange, disabled: p.Disabled})
			separator(c, t, sep)
			wheelColumn(c, t, wheelProps{key: "second", value: p.SecondValue, options: p.SecondOptions, onChange: p.OnSecondChange, disabled: p.Disabled})
			if third {
				separator(c, t, sep)
				wheelColumn(c, t, wheelProps{key: "third", value: p.ThirdValue, options: p.ThirdOptions, onChange: p.OnThirdChange, disabled: p.Disabled})
			}
		})
		if p.FirstLabel != "" || p.SecondLabel != "" || p.ThirdLabel != "" {
			labels := []string{p.FirstLabel, p.SecondLabel}
			if third {
				labels = append(labels, p.ThirdLabel)
			}
			wheelLabels(c, t, labels)
		}
	})
	return box
}

// separator：flex h-8 w-7 shrink-0 self-center items-center justify-center text-xl font-semibold leading-none。
func separator(c *ui.Context, t *theme.Theme, s string) {
	ui.Box(c).Size(28, itemH).Shrink(0).AlignSelf(ui.Center).Center().Children(func() {
		components.Text(c, s, theme.FontStep{Size: 20, Line: 20}, theme.WeightSemibold, t.Pick(theme.Brand800, theme.Brand100)).NoWrap()
	})
}

// wheelLabels：grid pt-1 text-center text-xs font-medium leading-4 text-brand-500 dark:text-brand-400，
// grid-cols-[minmax(0,1fr)_1.75rem_minmax(0,1fr)(_1.75rem_minmax(0,1fr))]。
func wheelLabels(c *ui.Context, t *theme.Theme, labels []string) {
	ui.Row(c.Key("labels")).Padding(4, 0, 0, 0).Children(func() {
		for i, s := range labels {
			if i > 0 {
				ui.Box(c).Width(28).Shrink(0)
			}
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Justify(ui.Center).Children(func() {
				components.Text(c, s, theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand500, theme.Brand400)).NoWrap()
			})
		}
	})
}

// TimeWheelProps 是 BetterTimeWheelPicker 的属性：一天中的分钟数（0–1439）。
type TimeWheelProps struct {
	ValueMinutes int
	OnChange     func(valueMinutes int)
	Disabled     bool
}

var (
	hourOptions   = WheelOptions(valueRange(0, 23))
	minuteOptions = WheelOptions(valueRange(0, 59))
)

// TimeWheelPicker 对应 BetterTimeWheelPicker：时、分两列（00–23、00–59，循环滚动），没有标签。
func TimeWheelPicker(c *ui.Context, t *theme.Theme, key string, p TimeWheelProps) ui.Element {
	v := clampInt(p.ValueMinutes, 0, 23*60+59)
	hour, minute := v/60, v%60
	change := func(minutes int) {
		if p.OnChange != nil {
			p.OnChange(minutes)
		}
	}
	return NumberWheelPicker(c, t, key, NumberWheelProps{
		FirstValue: hour, SecondValue: minute,
		FirstOptions: hourOptions, SecondOptions: minuteOptions,
		Disabled:       p.Disabled,
		OnFirstChange:  func(h int) { change(h*60 + minute) },
		OnSecondChange: func(m int) { change(hour*60 + m) },
	})
}

// DefaultMaxSeconds 是 BetterDurationWheelPicker 默认的上限 99:59:59。
const DefaultMaxSeconds = 99*60*60 + 59*60 + 59

// DurationWheelProps 是 BetterDurationWheelPicker 的属性：时长（秒）。
type DurationWheelProps struct {
	ValueSeconds int
	OnChange     func(valueSeconds int)
	// MinSeconds 为 0 时取原版默认的 1 秒；要允许 0 秒，传负数（原版同样把负数夹到 0）。
	MinSeconds int
	// MaxSeconds 为 0 时取 DefaultMaxSeconds。
	MaxSeconds int
	// 三列下方的标签，为空时取原版默认的 "h"、"m"、"s"。
	HourLabel, MinuteLabel, SecondLabel string
	Disabled                            bool
}

// DurationWheelPicker 对应 BetterDurationWheelPicker：时、分、秒三列，下方带标签。可选的分、秒随所选的时、分
// 受 [MinSeconds, MaxSeconds] 限制（例如默认最少 1 秒时，0 时 0 分的秒只能选 01–59）；
// 换时、分时把分、秒夹到新的可选范围里。
func DurationWheelPicker(c *ui.Context, t *theme.Theme, key string, p DurationWheelProps) ui.Element {
	minS := p.MinSeconds
	if minS == 0 {
		minS = 1
	}
	maxS := p.MaxSeconds
	if maxS == 0 {
		maxS = DefaultMaxSeconds
	}
	minS = max(0, minS)
	maxS = max(minS, maxS)
	hl, ml, sl := p.HourLabel, p.MinuteLabel, p.SecondLabel
	if hl == "" {
		hl = "h"
	}
	if ml == "" {
		ml = "m"
	}
	if sl == "" {
		sl = "s"
	}
	v := clampInt(p.ValueSeconds, minS, maxS)
	hour, minute, second := v/3600, v%3600/60, v%60

	update := func(h, m, s int) {
		if p.OnChange != nil {
			p.OnChange(clampInt(h*3600+m*60+s, minS, maxS))
		}
	}
	return NumberWheelPicker(c, t, key, NumberWheelProps{
		FirstValue: hour, SecondValue: minute, ThirdValue: second,
		FirstOptions:  WheelOptions(allowedHours(minS, maxS)),
		SecondOptions: WheelOptions(allowedMinutes(hour, minS, maxS)),
		ThirdOptions:  WheelOptions(allowedSeconds(hour, minute, minS, maxS)),
		FirstLabel:    hl, SecondLabel: ml, ThirdLabel: sl,
		Disabled: p.Disabled,
		OnFirstChange: func(h int) { // handleHourChange
			m := coerceValue(minute, allowedMinutes(h, minS, maxS))
			update(h, m, coerceValue(second, allowedSeconds(h, m, minS, maxS)))
		},
		OnSecondChange: func(m int) { // handleMinuteChange
			update(hour, m, coerceValue(second, allowedSeconds(hour, m, minS, maxS)))
		},
		OnThirdChange: func(s int) { update(hour, minute, s) },
	})
}

func clampInt(v, lo, hi int) int { return min(max(v, lo), hi) }

// allowedHours 对应 getAllowedHours。
func allowedHours(minS, maxS int) []int { return valueRange(minS/3600, maxS/3600) }

// allowedMinutes 对应 getAllowedMinutesForHour：在最小（最大）的那个小时里，分钟不能低于（超过）边界。
func allowedMinutes(hour, minS, maxS int) []int {
	lo, hi := 0, 59
	if hour == minS/3600 {
		lo = minS % 3600 / 60
	}
	if hour == maxS/3600 {
		hi = maxS % 3600 / 60
	}
	return valueRange(lo, hi)
}

// allowedSeconds 对应 getAllowedSecondsForHourAndMinute。
func allowedSeconds(hour, minute, minS, maxS int) []int {
	lo, hi := 0, 59
	if hour == minS/3600 && minute == minS%3600/60 {
		lo = minS % 60
	}
	if hour == maxS/3600 && minute == maxS%3600/60 {
		hi = maxS % 60
	}
	return valueRange(lo, hi)
}

// coerceValue 对应 coerceValue：不在可选范围里的值夹到范围两端。
func coerceValue(v int, allowed []int) int {
	for _, a := range allowed {
		if a == v {
			return v
		}
	}
	if len(allowed) == 0 {
		return clampInt(v, 0, 0)
	}
	return clampInt(v, allowed[0], allowed[len(allowed)-1])
}
