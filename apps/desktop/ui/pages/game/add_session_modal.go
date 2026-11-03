package game

import (
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/picker"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// addSessionModal 是 AddPlaySessionModal 的状态。
type addSessionModal struct {
	open         bool
	date         string // YYYY-MM-DD
	startMinutes int
	seconds      int
	submitting   bool
	scroll       components.ScrollBar
}

// reset 对应 createDefaultSessionDraft：一小时前开始，时长一小时。
func (m *addSessionModal) reset(now time.Time) {
	ago := now.Add(-time.Hour)
	m.date, m.startMinutes, m.seconds = picker.FormatDate(ago), ago.Hour()*60+ago.Minute(), 3600
}

// setDate 对应 handleDateChange：选了今天且开始时间晚于现在时，把开始时间改成现在。
func (m *addSessionModal) setDate(v string, now time.Time) {
	m.date = v
	if v == picker.FormatDate(now) && m.startMinutes > now.Hour()*60+now.Minute() {
		m.startMinutes = now.Hour()*60 + now.Minute()
	}
}

// start 是选中的开始时间（本地时区），日期无效时 ok 为 false。
func (m *addSessionModal) start() (time.Time, bool) {
	d, err := time.ParseInLocation("2006-01-02", m.date, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return d.Add(time.Duration(m.startMinutes) * time.Minute), true
}

// submitSession 对应 handleSubmit：校验开始时间、时长与结束时间后 AddPlaySession，成功后重新加载统计。
func (p *Page) submitSession(c *ui.Context) {
	m := &p.addSession
	start, ok := m.start()
	switch {
	case !ok:
		p.env.Toast(c, env.Error(locales.AddPlaySessionToastInvalidStartTime()))
		return
	case m.seconds <= 0:
		p.env.Toast(c, env.Error(locales.AddPlaySessionToastZeroDuration()))
		return
	case start.Add(time.Duration(m.seconds) * time.Second).After(time.Now()):
		p.env.Toast(c, env.Error(locales.AddPlaySessionToastFutureEnd()))
		return
	}
	m.submitting = true
	id, seconds := p.id, m.seconds
	p.env.Async(func() func() {
		_, err := p.env.Svc.Session.AddPlaySession(id, start, seconds)
		return func() {
			m.submitting = false
			if err != nil {
				p.env.Toast(c, env.Error(locales.AddPlaySessionToastFailed()))
				return
			}
			p.env.Toast(c, env.Success(locales.AddPlaySessionToastSuccess()))
			p.stats.load(p.env, p.stats.gameID)
			m.open = false
			m.reset(time.Now())
		}
	})
}

// addSessionView 对应 AddPlaySessionModal：mx-4 max-h-[86vh] w-full max-w-lg rounded-lg shadow-xl 的面板，
// 标题与说明，下面可滚动的表单：日期（单日历）、开始时间与游玩时长（滚轮选择器）、预计结束，以及取消 / 添加。
func (p *Page) addSessionView(c *ui.Context, t *theme.Theme) {
	m := &p.addSession
	_, winH := c.Size()
	now := time.Now()
	closeModal := func() { m.open = false }
	components.Modal(c, t, "add-session", components.ModalProps{Open: m.open, TopOffset: env.TopbarHeight, MaxWidth: 512},
		func(panel ui.Element) {
			panel.MaxHeight(winH*0.86).Radius(8).Clip().Background(t.PanelBg).
				Shadow(0, 20, 25, -5, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 8, 10, -6, ui.RGBA(0, 0, 0, 0.1))
			panel.Children(func() {
				// flex items-center justify-between px-6 pb-3 pt-6
				ui.Row(c).Padding(24, 24, 12, 24).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					components.Text(c, locales.AddPlaySessionTitle(), theme.TextXL, theme.WeightSemibold, t.TextStrong)
					close := ui.ButtonBase(c.Key("close")).FocusRing(false).Label(locales.CommonClose())
					color := t.Pick(theme.Brand500, theme.Brand400)
					if close.Hovered() {
						color = t.Pick(theme.Brand700, theme.White)
					}
					close.OnClick(closeModal).Children(func() { components.Icon(c, "close", 20, color) })
				})
				components.Text(c, locales.AddPlaySessionDesc(), theme.TextSM, theme.WeightNormal,
					t.Pick(theme.Brand600, theme.Brand400)).Padding(0, 24, 16, 24)
				// flex-1 space-y-4 overflow-y-auto px-6 pb-6：标题与说明之外的高度
				components.ScrollArea(c, t, "form", &m.scroll, func(w ui.Element) { w.MaxHeight(max(0, winH*0.86-120)) }, func(sc ui.Element) {
					right := float32(24)
					if m.scroll.Overflowing() {
						right += t.Gutter()
					}
					sc.Padding(0, right, 24, 24).Children(func() { p.addSessionForm(c, t, now, closeModal) })
				})
			})
		})
}

func (p *Page) addSessionForm(c *ui.Context, t *theme.Theme, now time.Time, closeModal func()) {
	m := &p.addSession
	label := func(s string, bottom float32) {
		components.TextMargin(c, s, theme.TextSM, theme.WeightMedium, t.SectionLabel, 0, 0, bottom, 0)
	}
	ui.Column(c).Gap(16).Children(func() {
		ui.Column(c).Children(func() {
			label(locales.AddPlaySessionDate(), 4)
			today := picker.FormatDate(now)
			picker.DateRangePicker(c, t, "date", picker.DateRangeProps{
				StartDate: m.date, EndDate: m.date,
				TriggerLabel: locales.AddPlaySessionSelectDate(), ApplyLabel: locales.CommonConfirm(), ResetLabel: locales.StatsResetBtn(),
				SelectionMode: picker.SelectSingle, CalendarMode: picker.CalendarSingle, PanelAnchor: picker.AnchorBottom,
				Active: m.date != "", Disabled: m.submitting,
				OnStartDateChange: func(v string) { m.setDate(v, time.Now()) },
				OnEndDateChange:   func(v string) { m.setDate(v, time.Now()) },
				OnApply:           func() {},
				OnReset:           func() { m.setDate(today, time.Now()) },
			}).FillWidth()
		})
		ui.Column(c).Children(func() {
			label(locales.AddPlaySessionStartTime(), 4)
			picker.TimeWheelPicker(c, t, "start", picker.TimeWheelProps{
				ValueMinutes: m.startMinutes, Disabled: m.submitting,
				OnChange: func(v int) { m.startMinutes = v },
			})
		})
		ui.Column(c).Children(func() {
			label(locales.AddPlaySessionDuration(), 8)
			picker.DurationWheelPicker(c, t, "duration", picker.DurationWheelProps{
				ValueSeconds: m.seconds, Disabled: m.submitting,
				HourLabel:   locales.CommonDurationHoursShort(),
				MinuteLabel: locales.CommonDurationMinutesShort(),
				SecondLabel: locales.CommonDurationSecondsShort(),
				OnChange:    func(v int) { m.seconds = v },
			})
		})
		// rounded-lg bg-brand-50 p-3 dark:bg-brand-700/50：预计结束
		ui.Column(c).Padding(12).Radius(8).Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.5))).Children(func() {
			components.Text(c, locales.AddPlaySessionEndPreview(), theme.TextXS, theme.WeightNormal, t.TextMuted)
			ui.Row(c).Margin(4, 0, 0, 0).Wrap().GapX(8).GapY(8).Justify(ui.SpaceBetween).AlignItems(ui.End).Children(func() {
				end := "--"
				if start, ok := m.start(); ok {
					end = timefmt.DateTime(start.Add(time.Duration(m.seconds)*time.Second), time.Local)
				}
				components.Text(c, end, theme.TextSM, theme.WeightMedium, t.Pick(theme.Brand700, theme.Brand200))
				components.Text(c, timefmt.Duration(m.seconds), theme.FontStep{Size: 18, Line: 28}, theme.WeightSemibold, t.TextStrong)
			})
		})
		// flex justify-end gap-3 pt-4
		ui.Row(c).Padding(16, 0, 0, 0).Gap(12).Justify(ui.End).Children(func() {
			cancel := ui.ButtonBase(c.Key("cancel")).Padding(8, 16).Radius(6).FocusRing(false)
			if cancel.Hovered() {
				cancel.Background(t.Pick(theme.Brand100, theme.Brand700))
			}
			cancel.Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(closeModal).Children(func() {
				components.Text(c, locales.CommonCancel(), theme.TextBase, theme.WeightNormal, t.Pick(theme.Brand600, theme.Brand400)).NoWrap()
			})
			submit := ui.ButtonBase(c.Key("submit")).Padding(8, 16).Radius(6).FocusRing(false)
			bg := theme.Neutral600
			if submit.Hovered() && !m.submitting {
				bg = theme.Neutral700
			}
			submit.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond))
			text := locales.CommonAdd()
			if m.submitting {
				text = locales.AddPlaySessionSubmitting()
				components.Disable(submit, 0.5)
			}
			submit.OnClick(func() { p.submitSession(c) }).Children(func() {
				components.Text(c, text, theme.TextBase, theme.WeightNormal, theme.White).NoWrap()
			})
		})
	})
}
