package game

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/chart"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// statsPanel 是“游戏统计”标签页（components/panel/GameStatsPanel.tsx）的状态。
type statsPanel struct {
	gen      int
	gameID   string
	period   enums.Period
	stats    *vo.GameDetailStats
	sessions []models.PlaySession
	loading  bool
	failed   func() string // 待显示的错误提示（显示时按当前语言取）
	sessView bool          // viewMode === "sessions"
	chart    chart.State
	list     components.ScrollBar

	deleteID   string // 待确认删除的游玩记录
	confirmDel bool
}

// periodOptions 是时间维度的选项，文字按当前语言取，每帧重建。
func periodOptions() []components.SlideButtonOption {
	return []components.SlideButtonOption{
		{Label: locales.GameStatsPeriodWeek(), Value: string(enums.Week)},
		{Label: locales.GameStatsPeriodMonth(), Value: string(enums.Month)},
		{Label: locales.GameStatsPeriodYear(), Value: string(enums.Year)},
		{Label: locales.GameStatsPeriodAll(), Value: string(enums.All)},
	}
}

// periodLabel 对应 getStatsPeriodLabelKey。
func periodLabel(p enums.Period) string {
	switch p {
	case enums.Month:
		return locales.GameStatsPeriodStatsLabelMonth()
	case enums.Year:
		return locales.GameStatsPeriodStatsLabelYear()
	case enums.All:
		return locales.GameStatsPeriodStatsLabelAll()
	}
	return locales.GameStatsPeriodStatsLabelWeek()
}

// load 同时请求统计与游玩记录（原版切换时间维度时两者都会重新加载）。
func (s *statsPanel) load(e *env.Env, id string) {
	s.gameID = id
	if s.period == "" {
		s.period = enums.Week
	}
	s.gen++
	s.loading = true
	gen, period := s.gen, s.period
	e.Async(func() func() {
		stats, statsErr := e.Svc.Stats.GetGameStats(vo.GameStatsRequest{GameID: id, Dimension: period})
		sessions, sessErr := e.Svc.Session.GetPlaySessions(id)
		return func() {
			if gen != s.gen {
				return
			}
			s.loading = false
			if statsErr == nil {
				s.stats = &stats
			} else {
				s.failed = locales.GameStatsToastLoadStatsFailed
			}
			if sessErr == nil {
				s.sessions = sessions
			} else {
				s.failed = locales.GameStatsToastLoadSessionsFailed
			}
		}
	})
}

// statsView 对应 GameStatsPanel：三张统计卡片，下面是图表/游玩记录与时间维度、视图切换。
func (p *Page) statsView(c *ui.Context, t *theme.Theme) {
	s := &p.stats
	if s.failed != nil {
		p.env.Toast(c, env.Error(s.failed()))
		s.failed = nil
	}
	_, winH := c.Size()
	bodyH := min(544, max(320, winH*0.42)) // h-[clamp(20rem,42vh,34rem)]
	period := periodLabel(s.period)

	ui.Column(c.Key("stats-panel")).Gap(32).Children(func() {
		ui.Grid(c).Columns(3).GapX(24).GapY(24).Children(func() {
			count, today, total := "-", "-", "-"
			switch {
			case s.stats != nil:
				count = fmt.Sprint(s.stats.TotalPlayCount)
				today, total = timefmt.Duration(s.stats.TodayPlayTime), timefmt.Duration(s.stats.TotalPlayTime)
			case !s.loading:
				count, today, total = "0", locales.GameStatsZeroMinutes(), locales.GameStatsZeroMinutes()
			}
			statCard(c, t, locales.GameStatsPeriodPlayCount(period), count)
			statCard(c, t, locales.GameStatsTodayPlayTime(), today)
			statCard(c, t, locales.GameStatsPeriodTotalPlayTime(period), total)
		})

		// glass-card overflow-hidden rounded-lg bg-white dark:bg-brand-800
		card := glassCard(t, ui.Column(c.Key("chart-card")).Radius(8).Clip())
		card.Children(func() {
			ui.Column(c).Padding(24).Children(func() {
				switch {
				case s.loading && s.stats == nil:
					ui.Box(c).Height(bodyH).Center().Children(func() {
						components.Spinner(c, 30, theme.Brand500)
					})
				case s.sessView:
					p.sessionList(c, t, bodyH)
				default:
					p.durationChart(c, t, bodyH)
				}
			})
			// border-t-1 border-brand-200 dark:border-brand-700
			ui.Box(c).Height(t.HL).Background(t.Divider)
			p.statsFooter(c, t)
		})
	})
}

// glassCard 设置 glass-card bg-white dark:bg-brand-800 的底色，玻璃模式下加上 glass-card 的边框。
func glassCard(t *theme.Theme, e ui.Element) ui.Element {
	bg, border := t.GlassCard(t.Pick(theme.White, theme.Brand800), ui.Transparent)
	if t.Glass {
		e.Border(t.HL, border)
	}
	return e.Background(bg)
}

// statCard：glass-card bg-white dark:bg-brand-800 p-6 rounded-lg。
func statCard(c *ui.Context, t *theme.Theme, label, value string) {
	glassCard(t, ui.Column(c).Padding(24).Radius(8)).Children(func() {
		components.TextMargin(c, label, theme.TextSM, theme.WeightNormal, t.TextMuted, 0, 0, 8, 0)
		components.Text(c, value, theme.FontStep{Size: 24, Line: 32}, theme.WeightBold, t.TextStrong)
	})
}

func (p *Page) durationChart(c *ui.Context, t *theme.Theme, height float32) {
	s := &p.stats
	var points []chart.Point
	// GameStatsPanel 的数据集：tension 0.3，“全部”时间轴 0.2 并画大一点的点（pointRadius 4 / 6）。
	props := chart.DurationLineProps{Height: height, Label: locales.GameStatsChartLabel(), State: &s.chart, Tension: 0.3}
	if s.stats != nil {
		loc := timefmt.Location(p.env.Config())
		props.TimeScale = s.period == enums.All
		if props.TimeScale {
			props.Tension = 0.2
		}
		for _, h := range s.stats.RecentPlayHistory {
			pt := chart.Point{Label: h.Date, Seconds: h.Duration}
			if props.TimeScale {
				day, ok := timefmt.ParseDate(h.Date, loc)
				if !ok {
					continue
				}
				pt.Time = day
			}
			points = append(points, pt)
		}
		if props.TimeScale {
			start, okStart := timefmt.ParseDate(s.stats.StartDate, loc)
			end, okEnd := timefmt.ParseDate(s.stats.EndDate, loc)
			if okStart && okEnd {
				props.From, props.To = chart.PaddedRange(start, end)
			} else {
				// 后端的 start_date/end_date 实际是 "2026-07-16T00:00:00Z"，原版的 parseDateOnlyToLocalTimestamp
				// 解析不了，于是不设 min/max，由 Chart.js 按数据定范围。这里同样取数据的首尾。
				props.From, props.To = chart.DataRange(points, time.Now().In(loc))
			}
		}
	}
	props.Points = points
	chart.DurationLineChart(c, t, props)
}

// sessionList：max-h-[clamp(...)] space-y-2 overflow-y-auto pr-1，每条记录 p-3 rounded-lg。
func (p *Page) sessionList(c *ui.Context, t *theme.Theme, maxH float32) {
	s := &p.stats
	if len(s.sessions) == 0 {
		ui.Column(c).Padding(48, 0).AlignItems(ui.Center).Children(func() {
			components.Icon(c, "clock-outline", 36, theme.Brand500).Margin(0, 0, 8, 0)
			components.Text(c, locales.GameStatsNoPlaySessions(), theme.TextBase, theme.WeightNormal, theme.Brand500)
		})
		return
	}
	loc := timefmt.Location(p.env.Config())
	gutter := float32(0)
	if s.list.Overflowing() {
		gutter = t.Gutter()
	}
	components.ScrollArea(c, t, "sessions", &s.list, func(w ui.Element) { w.MaxHeight(maxH) }, func(sc ui.Element) {
		sc.Padding(0, 4+gutter, 0, 0).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				for _, sess := range s.sessions {
					sessionRow(c.Key(sess.ID), t, sess, loc, func() { s.deleteID, s.confirmDel = sess.ID, true })
				}
			})
		})
	})
}

func sessionRow(c *ui.Context, t *theme.Theme, sess models.PlaySession, loc *time.Location, onDelete func()) {
	ui.Row(c).Padding(12).Radius(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Background(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.5))).Children(func() {
		ui.Column(c).Grow(1).Children(func() {
			components.Text(c, timefmt.DateTime(sess.StartTime, loc), theme.TextSM, theme.WeightNormal, t.TextStrong)
			components.Text(c, locales.GameStatsDuration()+" "+timefmt.Duration(sess.Duration), theme.TextXS, theme.WeightNormal, t.TextMuted)
		})
		// p-1.5 text-brand-400 hover:text-error-500 hover:bg-error-50 dark:hover:bg-error-900/20 rounded
		del := ui.ButtonBase(c.Key("delete")).Padding(6).Radius(4).FocusRing(false).Label(locales.GameStatsModalDeleteSessionTitle())
		color := theme.Brand400
		if del.Hovered() {
			color = theme.Error500
			del.Background(t.Pick(ui.Hex("#fef2f2"), ui.Hex("#7f1d1d").Alpha(0.2)))
		}
		del.Transition(theme.ColorsTransition(150 * time.Millisecond))
		del.OnClick(onDelete).Children(func() {
			components.Icon(c, "delete-outline", 18, color)
		})
	})
}

// statsFooter：flex items-center justify-between p-4，左侧时间维度与视图切换，右侧“手动添加”。
func (p *Page) statsFooter(c *ui.Context, t *theme.Theme) {
	s := &p.stats
	ui.Row(c.Key("stats-footer")).Padding(16).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			components.SlideButton(c, t, "period", components.SlideButtonProps{
				Options:  periodOptions(),
				Value:    string(s.period),
				Disabled: s.loading,
				OnChange: func(v string) {
					s.period = enums.Period(v)
					s.load(p.env, s.gameID)
				},
			})
			ui.Row(c).Gap(8).Children(func() {
				viewButton(c, t, "view-chart", "chart-line", locales.GameStatsViewChart(), !s.sessView, func() { s.sessView = false })
				viewButton(c, t, "view-sessions", "format-list-bulleted", locales.GameStatsViewSessions(), s.sessView, func() { s.sessView = true })
			})
		})
		better.Button(c, t, "manual-add", better.ButtonProps{
			Variant: better.ButtonPrimary, Icon: "plus", Label: locales.GameStatsManualAdd(),
		}).OnClick(func() {
			if p.addSession.date == "" { // 原版的草稿只在挂载时与添加成功后重置，关闭再打开时保留
				p.addSession.reset(time.Now())
			}
			p.addSession.open = true
		})
	})
}

func viewButton(c *ui.Context, t *theme.Theme, key, icon, label string, active bool, onClick func()) {
	v := better.ButtonSecondary
	if active {
		v = better.ButtonPrimary
	}
	better.Button(c, t, key, better.ButtonProps{Variant: v, Icon: icon}).Label(label).OnClick(onClick)
}

// deleteSession 对应 handleDeleteSession：删除后重新加载统计与游玩记录。
func (p *Page) deleteSession(c *ui.Context) {
	s := &p.stats
	id := s.deleteID
	if id == "" {
		return
	}
	s.deleteID = ""
	p.env.Async(func() func() {
		err := p.env.Svc.Session.DeletePlaySession(id)
		return func() {
			if err != nil {
				p.env.Toast(c, env.Error(locales.GameStatsToastDeleteFailed()))
				return
			}
			p.env.Toast(c, env.Success(locales.GameStatsToastDeleteSuccess()))
			s.load(p.env, s.gameID)
		}
	})
}

// statsModals 是 GameStatsPanel 的弹窗：删除游玩记录的确认。
func (p *Page) statsModals(c *ui.Context, t *theme.Theme) {
	s := &p.stats
	components.ConfirmModal(c, t, "delete-session", components.ConfirmProps{
		Open:      &s.confirmDel,
		Title:     locales.GameStatsModalDeleteSessionTitle(),
		Message:   locales.GameStatsModalDeleteSessionMsg(),
		Danger:    true,
		TopOffset: env.TopbarHeight,
		OnConfirm: func() { p.deleteSession(c) },
	})
}
