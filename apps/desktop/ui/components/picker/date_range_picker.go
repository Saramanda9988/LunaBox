package picker

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/theme"
)

// SelectionMode 对应 selectionMode。
type SelectionMode int

const (
	SelectRange  SelectionMode = iota // "range"（默认）：先点开始日期，再点结束日期
	SelectSingle                      // "single"：点一下同时设为开始与结束日期
)

// CalendarMode 对应 calendarMode。
type CalendarMode int

const (
	CalendarDouble CalendarMode = iota // "double"（默认）：左右（窄窗口时上下）两个月
	CalendarSingle                     // "single"：一个月
)

// PanelAnchor 对应 panelAnchor。
type PanelAnchor int

const (
	AnchorBottomStart PanelAnchor = iota // "bottom start"（默认）：面板在按钮下方左对齐
	AnchorBottom                         // "bottom"：面板在按钮下方居中
)

// DateRangeProps 是 BetterDateRangePicker 的属性。日期都是 "YYYY-MM-DD"，空串表示未选。
type DateRangeProps struct {
	StartDate, EndDate string
	TriggerLabel       string // 没有选中日期时按钮上的文字
	ApplyLabel         string
	ResetLabel         string
	OnStartDateChange  func(value string)
	OnEndDateChange    func(value string)
	OnApply            func() // 点“应用”：单选且有日期，或区间完整且开始早于结束时随后关闭面板
	OnReset            func() // 点“重置”：随后关闭面板
	SelectionMode      SelectionMode
	CalendarMode       CalendarMode
	PanelAnchor        PanelAnchor
	Active             bool // 按钮用 primary 样式（aria-pressed）
	Disabled           bool
}

// dateRangeState 是组件自己的状态：面板开关与两侧显示的月份（挂载时按属性初始化，之后只由翻页改变，同原版的 useState）。
type dateRangeState struct {
	open        bool
	left, right month
	scroll      components.ScrollBar
}

// DateRangePicker 对应 BetterDateRangePicker：BetterButton（size="md" h-10，active 时 primary，否则 secondary，
// 图标 i-mdi-calendar-range，文字 max-w-[14rem] truncate）加 Headless UI 的 Popover 面板
// （mt-2 + --anchor-gap:8px，rounded-xl border border-brand-200 bg-white p-3 dark:border-brand-700 dark:bg-brand-800，
// max-w-[calc(100vw-2rem)] overflow-auto，space-y-3）。
//
// 返回按钮元素；原版的 className/triggerClassName="w-full" 由调用方对返回值调用 FillWidth()。
func DateRangePicker(c *ui.Context, t *theme.Theme, key string, p DateRangeProps) ui.Element {
	single := p.SelectionMode == SelectSingle
	singleCalendar := p.CalendarMode == CalendarSingle
	now := c.Now()
	today := FormatDate(now)
	latest := monthOf(now) // getLatestCalendarMonth

	label := p.TriggerLabel
	switch {
	case single && p.StartDate != "":
		label = p.StartDate
	case p.StartDate != "" && p.EndDate != "":
		label = p.StartDate + " - " + p.EndDate
	}
	variant := better.ButtonSecondary
	if p.Active {
		variant = better.ButtonPrimary
	}
	btn := better.Button(c, t, key, better.ButtonProps{
		Variant: variant, Size: better.ButtonMD, Icon: "calendar-range", Disabled: p.Disabled,
		Children: func(color ui.Color) {
			fitText(c, label, theme.FontStep{Size: 14, Line: 14}, theme.WeightMedium, color, 224)
		},
	})
	st := ui.Local(btn, "date-range", func() dateRangeState {
		left := initialLeftMonth(p.StartDate, now)
		s := dateRangeState{left: left, right: initialRightMonth(left, p.EndDate, now)}
		if singleCalendar {
			s.left = initialSingleMonth(p.StartDate, now)
		}
		return s
	})
	btn.Label(label).OnClick(func() { st.open = !st.open })
	closePanel := func() { st.open = false }
	if st.open {
		// Headless UI 在焦点位于按钮或面板时用 Esc 关闭面板。弹层自己的 Esc 只拿到焦点周围没人处理的键，
		// 放在模态框（它的 Esc 关闭模态框）里时会先被模态框拿走，所以按钮与面板上也挂一份。
		btn.OnShortcut(0, ui.KeyEscape, closePanel)
	}

	selectedText := p.StartDate // selectedRangeText
	if !single && p.StartDate != "" && p.EndDate != "" {
		selectedText = p.StartDate + " - " + p.EndDate
	}
	selectDate := func(value string) { // handleSelectDate
		switch {
		case single:
			call(p.OnStartDateChange, value)
			call(p.OnEndDateChange, value)
		case p.StartDate == "" || p.EndDate != "":
			call(p.OnStartDateChange, value)
			call(p.OnEndDateChange, "")
		case value < p.StartDate:
			call(p.OnStartDateChange, value)
		default:
			call(p.OnEndDateChange, value)
		}
	}
	base := monthProps{start: p.StartDate, end: p.EndDate, today: today, onSelect: selectDate}

	winW, winH := c.Size()
	stacked := !singleCalendar && winW < 1280 // flex-col gap-4 xl:flex-row
	trig := btn.Bounds()
	ui.PopoverBase(c, btn, &st.open, func(panel ui.Element) {
		at, self := ui.AnchorBottomLeft, ui.AnchorTopLeft
		if p.PanelAnchor == AnchorBottom {
			at, self = ui.AnchorBottom, ui.AnchorTop
		}
		panel.AttachTo(btn, at, self).Margin(16, 0, 0, 0).MaxWidth(winW-32).
			Radius(12).Background(t.PanelBg).Border(t.HL, t.PanelBorder).OnShortcut(0, ui.KeyEscape, closePanel)
		content := func(box ui.Element) {
			box.Column().Gap(12).Children(func() {
				if singleCalendar {
					ui.Row(c).Justify(ui.Center).Children(func() {
						m := base
						m.month, m.key = st.left, "single"
						m.canPrevMonth, m.canPrevYear = true, true
						m.canNextMonth, m.canNextYear = st.left+1 <= latest, st.left+12 <= latest
						m.shift = func(n int) { st.left += month(n) }
						calendarMonth(c, t, m)
					})
				} else {
					months := ui.Row(c).Gap(16)
					if stacked {
						months.Column()
					}
					months.Children(func() {
						l := base
						l.month, l.key = st.left, "left"
						l.canPrevMonth, l.canPrevYear = true, true
						l.canNextMonth, l.canNextYear = st.left+1 < st.right, st.left+12 < st.right
						l.shift = func(n int) { st.left += month(n) }
						calendarMonth(c, t, l)
						r := base
						r.month, r.key = st.right, "right"
						r.canPrevMonth, r.canPrevYear = st.right-1 > st.left, st.right-12 > st.left
						r.canNextMonth, r.canNextYear = st.right+1 <= latest, st.right+12 <= latest
						r.shift = func(n int) { st.right += month(n) }
						calendarMonth(c, t, r)
					})
				}
				footer(c, t, p, selectedText, single, &st.open)
			})
		}
		// 面板放不下时（窄窗口的上下两个月）像原版的 overflow-auto 一样滚动：Floating UI 的 size 中间件把
		// max-height 设为按钮下方（翻到上方时是上方）的可用高度。
		contentH := float32(320 + 12 + 12 + 32)
		if stacked {
			contentH += 320 + 16
		}
		contentH += t.HL
		avail := max(winH-(trig.Y+trig.H)-16, trig.Y-16)
		if contentH+24+2*t.HL <= avail || avail <= 0 {
			panel.Padding(12)
			content(ui.Column(c))
			return
		}
		components.ScrollArea(c, t, "scroll", &st.scroll, func(w ui.Element) {
			w.Height(avail - 2*t.HL)
		}, func(scroll ui.Element) {
			scroll.Padding(12, 12+t.Gutter(), 12, 12).Children(func() { content(ui.Column(c)) })
		})
	})
	return btn
}

// fitText 是 truncate 的文字：宽度取实测宽度（最多 maxW）。components.Text 加 SingleLine 时，
// 浮点误差会让刚好放得下的文字也被截成省略号（better.Button 的 Label 有同样的问题），所以显式给宽度。
func fitText(c *ui.Context, s string, f theme.FontStep, weight int, color ui.Color, maxW float32) ui.Element {
	w, _ := c.MeasureText(0, ui.Span{Text: s, Font: theme.UIFont, Size: f.Size, Weight: weight})
	e := components.Text(c, s, f, weight, color).SingleLine().Shrink(1).MinWidth(0)
	if maxW > 0 {
		w = min(w+0.5, maxW)
	} else {
		w += 0.5
	}
	return e.Width(w)
}

// buttonLabel 是 BetterButton size="sm" 的文字（text-xs font-medium leading-none）。
func buttonLabel(c *ui.Context, s string) func(ui.Color) {
	return func(color ui.Color) { fitText(c, s, theme.FontStep{Size: 12, Line: 12}, theme.WeightMedium, color, 0) }
}

func call(fn func(string), v string) {
	if fn != nil {
		fn(v)
	}
}

// footer：flex items-center justify-between gap-3 border-t border-brand-200 pt-3 dark:border-brand-700；
// 左侧 min-w-0 truncate text-xs text-brand-500 dark:text-brand-400 的已选日期，右侧 gap-2 的
// 重置（secondary sm i-mdi-restore）与应用（primary sm i-mdi-check）按钮。
func footer(c *ui.Context, t *theme.Theme, p DateRangeProps, selected string, single bool, open *bool) {
	ui.Row(c.Key("footer")).AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(12).Padding(12, 0, 0, 0).
		BorderWidth(t.HL, 0, 0, 0).BorderColor(t.PanelBorder).Children(func() {
		components.Text(c, selected, theme.TextXS, theme.WeightNormal, t.Pick(theme.Brand500, theme.Brand400)).
			SingleLine().MinWidth(0)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			better.Button(c, t, "reset", better.ButtonProps{
				Variant: better.ButtonSecondary, Size: better.ButtonSM, Icon: "restore",
				Disabled: p.Disabled, Children: buttonLabel(c, p.ResetLabel),
			}).OnClick(func() {
				if p.OnReset != nil {
					p.OnReset()
				}
				*open = false
			})
			better.Button(c, t, "apply", better.ButtonProps{
				Variant: better.ButtonPrimary, Size: better.ButtonSM, Icon: "check",
				Disabled: p.Disabled, Children: buttonLabel(c, p.ApplyLabel),
			}).OnClick(func() {
				if p.OnApply != nil {
					p.OnApply()
				}
				// 关闭条件用的是点击前这一帧的属性，同原版。
				if single && p.StartDate != "" || p.StartDate != "" && p.EndDate != "" && p.StartDate < p.EndDate {
					*open = false
				}
			})
		})
	})
}

// monthProps 是 CalendarMonth 的属性。
type monthProps struct {
	key                       string
	month                     month
	start, end, today         string
	onSelect                  func(value string)
	canPrevMonth, canPrevYear bool
	canNextMonth, canNextYear bool
	shift                     func(n int) // 翻页：±1 个月或 ±12 个月
}

var weekdayLabels = [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}

// calendarMonth 对应 CalendarMonth：w-[18.5rem]；表头 mb-3 grid h-9 grid-cols-[4.5rem_1fr_4.5rem] items-center，
// 两侧各两个翻页按钮（gap-1），中间 text-center text-sm font-semibold text-brand-900 dark:text-white 的 YYYY/MM；
// 日期 grid grid-cols-7 gap-1，周标题 h-8 text-[11px] font-medium leading-8 text-brand-400 dark:text-brand-500。
func calendarMonth(c *ui.Context, t *theme.Theme, p monthProps) {
	ui.Column(c.Key(p.key)).Width(296).Shrink(0).Children(func() {
		ui.Row(c.Key("header")).Height(36).AlignItems(ui.Center).Margin(0, 0, 12, 0).Children(func() {
			ui.Row(c).Width(72).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
				navButton(c, t, "chevron-double-left", "Previous year", p.canPrevYear, func() { p.shift(-12) })
				navButton(c, t, "chevron-left", "Previous month", p.canPrevMonth, func() { p.shift(-1) })
			})
			ui.Row(c).Grow(1).Basis(0).MinWidth(0).Justify(ui.Center).Children(func() {
				components.Text(c, p.month.label(), theme.TextSM, theme.WeightSemibold, t.TextStrong).NoWrap()
			})
			ui.Row(c).Width(72).Gap(4).AlignItems(ui.Center).Justify(ui.End).Shrink(0).Children(func() {
				navButton(c, t, "chevron-right", "Next month", p.canNextMonth, func() { p.shift(1) })
				navButton(c, t, "chevron-double-right", "Next year", p.canNextYear, func() { p.shift(12) })
			})
		})
		days := calendarDays(p.month)
		ui.Column(c.Key("grid")).Gap(4).Children(func() {
			ui.Row(c.Key("weekdays")).Gap(4).Children(func() {
				for _, wd := range weekdayLabels {
					ui.Row(c).Grow(1).Basis(0).Height(32).Center().Children(func() {
						components.Text(c, wd, theme.FontStep{Size: 11, Line: 32}, theme.WeightMedium, t.TextFaint).NoWrap()
					})
				}
			})
			for w := range 6 {
				ui.Row(c.Key(w)).Gap(4).Children(func() {
					for _, date := range days[w*7 : w*7+7] {
						value := FormatDate(date)
						dayButton(c, t, dayState{
							value:    value,
							day:      date.Day(),
							outside:  monthOf(date) != p.month,
							future:   value > p.today,
							selected: value == p.start || value == p.end,
							ranged:   p.start != "" && p.end != "" && value > p.start && value < p.end,
						}, p.onSelect)
					}
				})
			}
		})
	})
}

// navButton 对应 CalendarNavButton：flex h-8 w-8 rounded-lg border border-transparent text-brand-500
// hover:border-brand-200 hover:bg-brand-100 hover:text-brand-900 disabled:opacity-35，dark:text-brand-400
// dark:hover:border-brand-700 dark:hover:bg-brand-700 dark:hover:text-white，transition-colors，图标 text-lg。
// CSS 的 :hover 对禁用的按钮同样生效，所以先取悬停状态再禁用。
func navButton(c *ui.Context, t *theme.Theme, icon, label string, enabled bool, onClick func()) {
	btn := ui.ButtonBase(c.Key(label)).Size(32, 32).Radius(8).Center().Shrink(0).FocusRing(false).Label(label)
	hovered := btn.Hovered()
	bg, border, color := ui.Transparent, ui.Transparent, t.Pick(theme.Brand500, theme.Brand400)
	if hovered {
		bg, border, color = t.Pick(theme.Brand100, theme.Brand700), t.Pick(theme.Brand200, theme.Brand700), t.Pick(theme.Brand900, theme.White)
	}
	btn.Background(bg).Border(t.HL, border).Transition(theme.ColorsTransition(150 * time.Millisecond))
	color = btn.AnimateColor("text", color, theme.Tween(150*time.Millisecond))
	if enabled {
		btn.OnClick(onClick)
	} else {
		btn.Disabled(true).Opacity(0.35 / 0.5).Cursor(ui.CursorNotAllowed) // MyGo 已把禁用的元素画成半透明
	}
	btn.Children(func() { components.Icon(c, icon, 18, color) })
}

// dayState 是一格日期的状态。
type dayState struct {
	value                             string
	day                               int
	outside, future, selected, ranged bool
}

// dayColors 解出日期按钮的颜色。原版把各状态的类名拼在一起：
//
//	本月      text-brand-700 hover:bg-brand-100 dark:text-brand-200 dark:hover:bg-brand-700
//	非本月    text-brand-300 hover:text-brand-600 dark:text-brand-600 dark:hover:text-brand-300
//	区间内    bg-brand-100 text-brand-800 dark:bg-brand-700/70 dark:text-white
//	选中      bg-neutral-800 text-white hover:bg-neutral-800 dark:bg-white dark:text-neutral-950 dark:hover:bg-white
//	未来      text-brand-200 hover:bg-transparent hover:text-brand-200 dark:text-brand-700 dark:hover:bg-transparent dark:hover:text-brand-700
//
// 同一属性、同样优先级的冲突由 UnoCSS 生成样式表的顺序决定（white 在前，其余按色阶从浅到深，后者生效），
// 不是类名在字符串里的顺序。结果和直觉不同，以下都和运行中的原版计算样式逐一核对过：
// 浅色主题下选中日期的文字是 text-brand-700（本月）/ text-brand-300（非本月），压在 neutral-800 的底上；
// 区间内非本月的日期是 brand-800；未来的日期不变淡（brand-700 / brand-300），只在悬停时变成 brand-200。
// 深色主题下区间内的文字是 brand-200 / brand-600（不是 white），未来的日期是 brand-700。
func dayColors(t *theme.Theme, d dayState, hovered bool) (bg, fg ui.Color) {
	inside := !d.outside
	if !t.Dark {
		switch { // text-white < text-brand-200 < -300 / -700 < -800
		case d.ranged:
			fg = theme.Brand800
		case inside:
			fg = theme.Brand700
		default:
			fg = theme.Brand300
		}
		switch {
		case d.selected:
			bg = ui.Hex("#1e293b") // neutral-800
		case d.ranged:
			bg = theme.Brand100
		}
		if hovered {
			switch { // hover:text-brand-600 > hover:text-brand-200
			case d.outside:
				fg = theme.Brand600
			case d.future:
				fg = theme.Brand200
			}
			switch { // hover:bg-brand-100 < hover:bg-neutral-800 < hover:bg-transparent
			case d.future:
				bg = ui.Transparent
			case d.selected:
				bg = ui.Hex("#1e293b")
			case inside:
				bg = theme.Brand100
			}
		}
		return bg, fg
	}
	switch { // dark:text-white < dark:text-brand-200 < -600 < -700 < dark:text-neutral-950
	case d.selected:
		fg = ui.Hex("#0a0a0a") // neutral-950（uno 预设色板）
	case d.future:
		fg = theme.Brand700
	case d.outside:
		fg = theme.Brand600
	default:
		fg = theme.Brand200
	}
	switch {
	case d.selected:
		bg = theme.White
	case d.ranged:
		bg = theme.Brand700.Alpha(0.7)
	}
	if hovered {
		switch { // dark:hover:text-brand-300 < dark:hover:text-brand-700
		case d.future:
			fg = theme.Brand700
		case d.outside:
			fg = theme.Brand300
		}
		switch { // dark:hover:bg-brand-700 < dark:hover:bg-transparent < dark:hover:bg-white
		case d.selected:
			bg = theme.White
		case d.future:
			bg = ui.Transparent
		case inside:
			bg = theme.Brand700
		}
	}
	return bg, fg
}

// dayButton：flex h-9 items-center justify-center rounded-lg text-sm transition-colors
// focus:outline-none focus:ring-2 focus:ring-neutral-500/30；未来的日期 disabled cursor-not-allowed。
//
// 原版禁用的日期不变淡（颜色见 dayColors），而 MyGo 把 Disabled 的元素画成半透明，
// 所以未来的日期不用 ButtonBase，而是一个不能点、不能聚焦的格子。
func dayButton(c *ui.Context, t *theme.Theme, d dayState, onSelect func(string)) {
	var btn ui.Element
	if d.future {
		btn = ui.Box(c.Key(d.value)).Cursor(ui.CursorNotAllowed).Description("disabled")
	} else {
		btn = ui.ButtonBase(c.Key(d.value)).FocusRing(false).OnClick(func() { onSelect(d.value) })
	}
	btn.Grow(1).Basis(0).Height(36).Radius(8).Center().Label(d.value)
	bg, fg := dayColors(t, d, btn.Hovered())
	if !d.future && btn.Focused() {
		btn.Shadow(0, 0, 0, 2, theme.Neutral500.Alpha(0.3))
	}
	btn.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
	fg = btn.AnimateColor("text", fg, theme.Tween(150*time.Millisecond))
	btn.Children(func() {
		components.Text(c, strconv.Itoa(d.day), theme.TextSM, theme.WeightNormal, fg).NoWrap()
	})
}
