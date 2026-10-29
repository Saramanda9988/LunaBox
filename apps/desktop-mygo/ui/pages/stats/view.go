package stats

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/better"
	"lunabox/desktop-mygo/ui/components/chart"
	"lunabox/desktop-mygo/ui/components/picker"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/tagname"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

const (
	pagePadding  = 32   // p-8
	sectionGap   = 24   // space-y-6、gap-6
	containerLG  = 1024 // @[1024px]：主内容区（@container 的 main）不小于它时两列
	cardPadding  = 20   // p-5
	cardTitleH   = 24 + 12
	trendMinH    = 288 // min-h-[18rem]
	gameTrendH   = 320 // h-80
	tagChartMinH = 128 // min-h-32
	monoFont     = "Consolas"
)

// 视口断点（Tailwind 的 sm、md、lg、2xl），按窗口宽度判断。
const (
	breakSM  = 640
	breakMD  = 768
	breakLG  = 1024
	break2XL = 1536
)

// 原版用到、theme 里没有的 Tailwind 颜色。
var (
	yellow50   = ui.Hex("#fefce8")
	yellow100  = ui.Hex("#fef9c3")
	yellow200  = ui.Hex("#fef08a")
	yellow300  = ui.Hex("#fde047")
	yellow400  = ui.Hex("#facc15")
	yellow600  = ui.Hex("#ca8a04")
	yellow800  = ui.Hex("#854d0e")
	yellow900  = ui.Hex("#713f12")
	orange900  = ui.Hex("#7c2d12")
	accent500  = ui.Hex("#3b82f6") // uno.config.ts 的 accent-500
	blue50     = ui.Hex("#eff6ff")
	blue100    = ui.Hex("#dbeafe")
	blue400    = ui.Hex("#60a5fa")
	blue600    = ui.Hex("#2563eb")
	blue800    = ui.Hex("#1e40af")
	blue900    = ui.Hex("#1e3a8a")
	totalColor = ui.RGBA(75, 192, 192, 1) // 总时长趋势的 borderColor
)

// gameTrendColors 是 GAME_TREND_COLORS：常玩游戏趋势里每个游戏一条线，依次取色。
var gameTrendColors = []ui.Color{
	ui.RGBA(255, 99, 132, 1), ui.RGBA(54, 162, 235, 1), ui.RGBA(255, 206, 86, 1), ui.RGBA(75, 192, 192, 1),
	ui.RGBA(153, 102, 255, 1), ui.RGBA(255, 159, 64, 1), ui.RGBA(99, 255, 132, 1), ui.RGBA(199, 99, 255, 1),
	ui.RGBA(64, 224, 208, 1), ui.RGBA(255, 105, 180, 1),
}

// View 构建页面：space-y-6 p-8 的标题、StatsToolbar、AI 总结卡片与 StatsContent（外层 max-w-8xl 在 UnoCSS 里
// 没有定义，不限宽）。第一次加载超过 300ms 时显示 StatsSkeleton。
func (p *Page) View(c *ui.Context) {
	t := p.env.Theme
	for _, n := range p.toasts {
		p.env.Toast(c, n())
	}
	p.toasts = nil

	if p.loading && p.stats == nil {
		if wait := skeletonDelay - c.Now().Sub(p.openedAt); wait > 0 {
			c.After(wait)
			return
		}
		p.skeleton(c, t)
		return
	}

	// main 是 overflow-auto（没有 scrollbar-stable），滚动条出现时占用内容盒的宽度，@container 的宽度随之变窄。
	gutter := float32(0)
	if p.scroll.Overflowing() {
		gutter = t.Gutter()
	}
	viewW := p.env.MainWidth - gutter
	winW, _ := c.Size()
	components.ScrollArea(c, t, "stats-scroll", &p.scroll, func(w ui.Element) { w.Fill() }, func(sc ui.Element) {
		sc.Padding(0, gutter, 0, 0)
		sc.Children(func() {
			ui.Column(c.Key("page")).Width(viewW).Padding(pagePadding).Gap(sectionGap).Children(func() {
				// flex items-center justify-between：h1 text-4xl font-bold
				ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					components.Text(c, locales.StatsTitle(), theme.Text4XL, theme.WeightBold, t.TextStrong)
				})
				p.toolbar(c, t, winW)
				if summary := p.aiSummary[p.period]; p.aiLoading || summary != "" {
					p.aiCard(c, t, summary, winW)
				}
				if p.stats != nil {
					p.content(c, t, viewW >= containerLG, winW)
				}
			})
		})
	})
	p.leaderboardModal(c, t)
}

// toolbar 对应 StatsToolbar：左侧时间维度与自定义日期范围，右侧“美化导出”与“AI总结”图标按钮。
// flex flex-col gap-3 md:flex-row md:items-center md:justify-between（my-4 被外层的 space-y-6 覆盖）。
func (p *Page) toolbar(c *ui.Context, t *theme.Theme, winW float32) {
	var bar ui.Element
	if winW >= breakMD {
		bar = ui.Row(c.Key("toolbar")).AlignItems(ui.Center).Justify(ui.SpaceBetween)
	} else {
		bar = ui.Column(c.Key("toolbar"))
	}
	bar.Gap(12).Children(func() {
		var left ui.Element
		if winW >= breakSM {
			left = ui.Row(c.Key("range")).AlignItems(ui.Center)
		} else {
			left = ui.Column(c.Key("range"))
		}
		left.MinWidth(0).Gap(8).Children(func() {
			value := string(p.period)
			if p.custom {
				value = ""
			}
			components.SlideButton(c, t, "period", components.SlideButtonProps{
				Options: []components.SlideButtonOption{
					{Label: locales.StatsPeriodWeek(), Value: string(enums.Week)},
					{Label: locales.StatsPeriodMonth(), Value: string(enums.Month)},
					{Label: locales.StatsPeriodYear(), Value: string(enums.Year)},
				},
				Value:    value,
				Disabled: p.loading,
				OnChange: func(v string) { p.changePeriod(enums.Period(v)) },
			})
			picker.DateRangePicker(c, t, "custom-range", picker.DateRangeProps{
				StartDate:         p.start,
				EndDate:           p.end,
				TriggerLabel:      locales.StatsCustomRange(),
				ApplyLabel:        locales.StatsApplyBtn(),
				ResetLabel:        locales.StatsResetBtn(),
				Active:            p.custom,
				Disabled:          p.loading,
				OnStartDateChange: func(v string) { p.start = v },
				OnEndDateChange:   func(v string) { p.end = v },
				OnApply:           p.applyRange,
				OnReset:           p.resetRange,
			})
		})
		ui.Row(c.Key("actions")).Shrink(0).AlignItems(ui.Center).Gap(8).Children(func() {
			export := locales.StatsExportTitle()
			better.Button(c, t, "export", better.ButtonProps{Icon: "image-filter-hdr", Disabled: p.loading}).Label(export).
				OnClick(func() { p.env.Toast(c, env.Info(locales.NativeFeatureNotMigrated(export))) })
			better.Button(c, t, "ai-summarize", better.ButtonProps{Icon: "robot-happy", Disabled: p.loading, Loading: p.aiLoading}).
				Label(locales.StatsAiSummarizeTitle()).OnClick(p.summarize)
		})
	})
}

// card 是统计卡片：glass-card bg-white dark:bg-brand-800 rounded-xl border border-brand-200 dark:border-brand-700。
func card(c *ui.Context, t *theme.Theme) ui.Element {
	bg, border := t.GlassCard(t.Pick(theme.White, theme.Brand800), t.Pick(theme.Brand200, theme.Brand700))
	return ui.Column(c).Radius(12).Background(bg).Border(t.HL, border)
}

// cardTitle：h3 text-base font-semibold text-brand-900 dark:text-white mb-3。
func cardTitle(c *ui.Context, t *theme.Theme, s string) {
	components.TextMargin(c, s, theme.TextBase, theme.WeightSemibold, t.TextStrong, 0, 0, 12, 0)
}

// content 对应 StatsContent：汇总卡片；排行榜与标签分布；游玩时段分布与总时长趋势；常玩游戏趋势。
// wide 是 @[1024px]：两张卡片一行（grid-cols-12，7 + 5 列）。
func (p *Page) content(c *ui.Context, t *theme.Theme, wide bool, winW float32) {
	s := p.stats
	p.summaryCards(c, t, s, winW)

	cols, major, minor := 1, 1, 1
	if wide {
		cols, major, minor = 12, 7, 5
	}
	ui.Grid(c.Key("leaderboard-row")).Columns(cols).GapX(sectionGap).GapY(sectionGap).Children(func() {
		p.leaderboardCard(c, t, s, winW).ColumnSpan(major)
		// flex flex-col：标签分布填满与排行榜同高的卡片（flex-1 min-h-0，图表 min-h-32 h-full）
		card(c.Key("tags"), t).ColumnSpan(minor).Padding(cardPadding).Children(func() {
			cardTitle(c, t, locales.StatsTagDistributionTitle())
			tagCols := 1
			if winW >= breakMD {
				tagCols = 2
			}
			chart.TagDistributionChart(c, t, chart.TagDistributionProps{
				Tags: p.translatedTags(), Columns: tagCols, State: &p.tags,
			}).Grow(1).MinHeight(tagChartMinH)
		})
	})

	// 总时长趋势的卡片与游玩时段分布同高，图表（flex-1 min-h-[18rem]）填满标题以下的部分。
	// 游玩时段分布的高度取上一帧的布局结果。
	trendH := float32(trendMinH)
	ui.Grid(c.Key("time-row")).Columns(cols).GapX(sectionGap).GapY(sectionGap).Children(func() {
		card(c.Key("time-of-day"), t).ColumnSpan(minor).Padding(cardPadding).Children(func() {
			cardTitle(c, t, locales.StatsTimeOfDayTitle())
			hw := chart.HourWeekDistribution(c, t, chart.HourWeekProps{
				Hourly: s.HourlyDistribution, Weekday: s.WeekdayDistribution, State: &p.hourWeek,
			})
			if b := hw.Bounds(); wide && b.H > trendH {
				trendH = b.H
			}
		})
		card(c.Key("total-trend"), t).ColumnSpan(major).Padding(cardPadding).Children(func() {
			cardTitle(c, t, locales.StatsChartsTotalTrend())
			chart.DurationLineChart(c, t, chart.DurationLineProps{
				Points:     timelinePoints(s),
				Height:     trendH,
				Label:      locales.StatsTotalDurationDataset(),
				Color:      totalColor,
				Tension:    0.3,
				ShowLegend: true,
				YAxisTitle: locales.StatsChartYAxis(),
				State:      &p.totalChart,
			})
		})
	})

	card(c.Key("game-trend"), t).Padding(cardPadding).Children(func() {
		cardTitle(c, t, locales.StatsChartsGameTrend())
		series := make([]chart.Series, len(s.LeaderboardSeries))
		for i, sr := range s.LeaderboardSeries {
			values := make([]int, len(sr.Points))
			for j, pt := range sr.Points {
				values[j] = pt.Duration
			}
			series[i] = chart.Series{Label: sr.GameName, Seconds: values, Color: gameTrendColors[i%len(gameTrendColors)]}
		}
		props := chart.DurationLineProps{
			Points:     timelinePoints(s),
			Series:     series,
			Height:     gameTrendH,
			Tension:    0.3,
			ShowLegend: true,
			YAxisTitle: locales.StatsChartYAxis(),
			State:      &p.gameChart,
		}
		if len(series) == 0 {
			// 原版没有数据集时 Chart.js 只画坐标轴；这里给一个空的数据集，图例里不显示。
			props.Series = []chart.Series{{Seconds: make([]int, len(props.Points)), Color: ui.Transparent}}
		}
		chart.DurationLineChart(c, t, props)
	})
}

// translatedTags 对应 translatedTagDistribution：标签名按界面语言显示译名（enable_tag_translation 关闭时原样）。
func (p *Page) translatedTags() []vo.TagPlayStats {
	enabled := tagname.Enabled(p.env.Config())
	tags := make([]vo.TagPlayStats, len(p.stats.TagDistribution))
	for i, tag := range p.stats.TagDistribution {
		tag.Name = tagname.Display(tag.Name, enabled)
		tags[i] = tag
	}
	return tags
}

// timelinePoints 是两张趋势图共用的 x 轴：timeline 的标签与总时长。
func timelinePoints(s *vo.PeriodStats) []chart.Point {
	pts := make([]chart.Point, len(s.Timeline))
	for i, tp := range s.Timeline {
		pts[i] = chart.Point{Label: tp.Label, Seconds: tp.Duration}
	}
	return pts
}

// summaryItem 是一张汇总卡片。
type summaryItem struct {
	key, label, value, suffix string
	noWrap                    bool // whitespace-nowrap（时长）
}

// summaryCards：grid grid-cols-2 sm:grid-cols-4 2xl:grid-cols-8 gap-3，每张 px-4 py-3 rounded-xl，
// 标签 text-xs font-medium mb-1 truncate，数值 text-xl font-bold，单位 text-xs font-normal ml-1。
func (p *Page) summaryCards(c *ui.Context, t *theme.Theme, s *vo.PeriodStats, winW float32) {
	items := []summaryItem{
		{key: "play-count", label: locales.StatsSummaryTotalPlayCount(), value: strconv.Itoa(s.TotalPlayCount)},
		{key: "play-duration", label: locales.StatsSummaryTotalPlayDuration(), value: durationCompact(s.TotalPlayDuration), noWrap: true},
		{key: "games", label: locales.StatsSummaryGamesPlayed(), value: strconv.Itoa(s.TotalGamesCount)},
		{key: "completed", label: locales.StatsSummaryCompletedGames(), value: strconv.Itoa(s.CompletedGamesCount)},
		{key: "avg-daily", label: locales.StatsSummaryAvgDailyDuration(), value: durationCompact(s.AvgDailyDuration), noWrap: true},
		{key: "avg-session", label: locales.StatsSummaryAvgSessionDuration(), value: durationCompact(s.AvgSessionDuration), noWrap: true},
		{key: "max-streak", label: locales.StatsSummaryMaxStreak(), value: strconv.Itoa(s.MaxStreak), suffix: locales.StatsSummaryDayUnit()},
		{key: "new-games", label: locales.StatsSummaryNewGames(), value: strconv.Itoa(s.NewGamesCount)},
	}
	cols := 2
	switch {
	case winW >= break2XL:
		cols = 8
	case winW >= breakSM:
		cols = 4
	}
	muted := t.Pick(theme.Brand500, theme.Brand400)
	ui.Grid(c.Key("summary")).Columns(cols).GapX(12).GapY(12).Children(func() {
		for _, it := range items {
			card(c.Key(it.key), t).Padding(12, 16).Label(it.label).Children(func() {
				components.TextMargin(c, it.label, theme.TextXS, theme.WeightMedium, muted, 0, 0, 4, 0).SingleLine()
				spans := []ui.Span{{Text: it.value}}
				if it.suffix != "" {
					// ml-1：宽 4 的行内占位，单位与数值按基线对齐
					spans = append(spans, ui.Span{Text: "\ufffc", ObjectWidth: 4}, ui.Span{Text: it.suffix, Size: 12, Weight: theme.WeightNormal, Color: muted})
				}
				d := components.LineOverflow(ui.Span{Text: it.value, Font: theme.UIFont, Size: theme.TextXL.Size, Weight: theme.WeightBold}, theme.TextXL.Line)
				v := ui.RichText(c, spans...).FontSize(theme.TextXL.Size).FixedLineHeight(theme.TextXL.Line).
					FontWeight(theme.WeightBold).TextColor(t.TextStrong).Margin(-d, 0, -d, 0)
				if theme.UIFont != "" {
					v.Font(theme.UIFont)
				}
				if it.noWrap {
					v.NoWrap()
				}
			})
		}
	})
}

// durationCompact 对应 formatDurationCompact：5时2分、5时、0分（en-US 是 5h2m）。
func durationCompact(seconds int) string {
	total := max(0, seconds) / 60
	h, m := total/60, total%60
	switch {
	case h > 0 && m > 0:
		return strconv.Itoa(h) + locales.CommonDurationHoursShort() + strconv.Itoa(m) + locales.CommonDurationMinutesShort()
	case h > 0:
		return strconv.Itoa(h) + locales.CommonDurationHoursShort()
	}
	return strconv.Itoa(m) + locales.CommonDurationMinutesShort()
}

// blurNSFW 是 blur_nsfw_game_covers（默认开）。
func (p *Page) blurNSFW() bool {
	if cfg := p.env.Config(); cfg != nil {
		return cfg.BlurNSFWGameCovers
	}
	return true
}

// cover 对应 GameCoverImage：rounded bg-brand-200 dark:bg-brand-700 的 w×h 封面（object-cover），NSFW 时模糊。
func (p *Page) cover(c *ui.Context, t *theme.Theme, g vo.GamePlayStats, w, h float32) ui.Element {
	box := ui.Box(c.Key("cover")).Size(w, h).Radius(4).Clip().Shrink(0).Background(t.Pick(theme.Brand200, theme.Brand700))
	box.Children(func() {
		if p.env.Covers == nil {
			return
		}
		if bmp := p.env.Covers.Get(&models.Game{ID: g.GameID, CoverURL: g.CoverUrl, IsNSFW: g.IsNSFW}); bmp != nil {
			img := ui.Image(c, bmp).Fill().Fit(ui.Cover)
			if g.IsNSFW && p.blurNSFW() {
				components.CoverBlur(c, box, img, false)
			}
		}
	})
	return box
}

// shadowMD、shadowSM 是 Tailwind 的 shadow-md、shadow-sm。
func shadowMD(e ui.Element) ui.Element {
	return e.Shadow(0, 4, 6, -1, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 2, 4, -2, ui.RGBA(0, 0, 0, 0.1))
}

func shadowSM(e ui.Element) ui.Element { return e.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05)) }

// leaderboardCard：p-5 flex flex-col 的“游玩时长排行榜 Top 10”。第 1 名是横向的大卡片，2–10 名两列（只有 2 名时一列），
// 满 10 名时最后一格是“查看更多”。
func (p *Page) leaderboardCard(c *ui.Context, t *theme.Theme, s *vo.PeriodStats, winW float32) ui.Element {
	games := leaderboard(s, previewCount)
	muted := t.Pick(theme.Brand500, theme.Brand400)
	return card(c.Key("leaderboard"), t).Padding(cardPadding).Children(func() {
		cardTitle(c, t, locales.StatsLeaderboardFullTitle())
		if len(games) == 0 {
			// px-6 py-12 text-center
			ui.Column(c).Padding(48, 24).AlignItems(ui.Center).Children(func() {
				components.Text(c, locales.StatsLeaderboardNoData(), theme.TextBase, theme.WeightNormal, muted)
			})
			return
		}
		ui.Column(c).Gap(12).Children(func() { // space-y-3
			p.leaderboardHero(c, t, games[0])
			if len(games) == 1 {
				return
			}
			cols := 1
			if len(games) != 2 && winW >= breakMD {
				cols = 2
			}
			ui.Grid(c.Key("rest")).Columns(cols).GapX(8).GapY(8).Children(func() {
				for i, g := range games[1:] {
					p.leaderboardRow(c.Key(g.GameID), t, g, i+2)
				}
				if len(games) >= previewCount {
					p.viewMoreButton(c, t, len(leaderboard(s, leaderboardSize)))
				}
			})
		})
	})
}

// leaderboardHero 是第 1 名：relative flex items-center gap-3 p-3 rounded-lg overflow-hidden，
// bg-gradient-to-r from-yellow-100/80 via-amber-50/70 to-yellow-50/20（dark:from-yellow-900/10 dark:via-orange-900/20
// dark:to-transparent），border-yellow-200/60（dark:border-yellow-800/40）。
func (p *Page) leaderboardHero(c *ui.Context, t *theme.Theme, g vo.GamePlayStats) {
	stops := []ui.GradientStop{{Color: yellow100.Alpha(0.8)}, {Color: theme.Warning50.Alpha(0.7), At: 0.5}, {Color: yellow50.Alpha(0.2), At: 1}}
	if t.Dark {
		stops = []ui.GradientStop{{Color: yellow900.Alpha(0.1)}, {Color: orange900.Alpha(0.2), At: 0.5}, {Color: orange900.Alpha(0), At: 1}}
	}
	ui.Row(c.Key("hero")).Padding(12).Gap(12).AlignItems(ui.Center).Radius(8).Clip().
		LinearGradient(ui.LinearGradient{Angle: 90, Stops: stops}).
		Border(t.HL, t.Pick(yellow200.Alpha(0.6), yellow800.Alpha(0.4))).Children(func() {
		// w-7 h-7 rounded-full bg-yellow-100 dark:bg-yellow-900/40 text-yellow-600 dark:text-yellow-400 text-xs font-bold
		ui.Box(c).Size(28, 28).Radius(999).Center().Shrink(0).Background(t.Pick(yellow100, yellow900.Alpha(0.4))).Children(func() {
			components.Text(c, "#1", theme.TextXS, theme.WeightBold, t.Pick(yellow600, yellow400)) // i18n:keep：名次
		})
		shadowMD(p.cover(c, t, g, 40, 56))
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, g.GameName, theme.TextSM, theme.WeightBold, t.TextStrong).SingleLine()
			// text-base font-mono font-semibold text-neutral-700 dark:text-neutral-300 mt-0.5
			components.TextMargin(c, timefmt.Duration(g.TotalDuration), theme.TextBase, theme.WeightSemibold,
				t.Pick(theme.Neutral700, theme.Neutral300), 2, 0, 0, 0).Font(monoFont)
		})
	})
}

// leaderboardRow 是第 2–10 名：flex items-center gap-2.5 p-2 rounded-lg bg-brand-50 dark:bg-brand-700/40，
// hover:bg-brand-100 dark:hover:bg-brand-700/60 transition-colors；玻璃模式下 data-glass:bg-white/5 dark:bg-black/5
// （比 hover:bg 更具体，悬停不变色）。
func (p *Page) leaderboardRow(c *ui.Context, t *theme.Theme, g vo.GamePlayStats, rank int) {
	row := ui.Row(c).Padding(8).Gap(10).AlignItems(ui.Center).Radius(8).Label(g.GameName)
	bg := t.Pick(theme.Brand50, theme.Brand700.Alpha(0.4))
	if row.Hovered() {
		bg = t.Pick(theme.Brand100, theme.Brand700.Alpha(0.6))
	}
	bg = t.IfGlass(bg, theme.White.Alpha(0.05), theme.Black.Alpha(0.05))
	row.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).Children(func() {
		// w-6 text-xs text-brand-500 dark:text-brand-400 font-medium tabular-nums text-center
		ui.Row(c).Width(24).Shrink(0).Justify(ui.Center).Children(func() {
			components.Text(c, "#"+strconv.Itoa(rank), theme.TextXS, theme.WeightMedium, t.Pick(theme.Brand500, theme.Brand400)).NoWrap() // i18n:keep：名次
		})
		shadowSM(p.cover(c, t, g, 28, 40))
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, g.GameName, theme.TextXS, theme.WeightMedium, t.TextStrong).SingleLine()
			// text-[11px] font-mono text-brand-600 dark:text-brand-300 mt-0.5（行高继承 1.5）
			components.TextMargin(c, timefmt.Duration(g.TotalDuration), theme.FontStep{Size: 11, Line: 16.5}, theme.WeightNormal,
				t.Pick(theme.Brand600, theme.Brand300), 2, 0, 0, 0).Font(monoFont)
		})
	})
}

// viewMoreButton：flex min-h-[3.5rem] items-center gap-3 rounded-lg border border-dashed border-brand-300
// dark:border-brand-600 px-4 py-2，hover:border-brand-500 hover:bg-brand-50/40（dark:hover:border-brand-400
// dark:hover:bg-brand-700/20）；左侧 i-mdi-format-list-numbered text-3xl，右侧“查看更多”与条数。
func (p *Page) viewMoreButton(c *ui.Context, t *theme.Theme, count int) {
	btn := ui.ButtonBase(c.Key("view-more")).MinHeight(56).Padding(8, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).
		Radius(8).FocusRing(false).Label(locales.StatsLeaderboardViewMore())
	border, bg := t.Pick(theme.Brand300, theme.Brand600), ui.Transparent
	if btn.Hovered() {
		border, bg = t.Pick(theme.Brand500, theme.Brand400), t.Pick(theme.Brand50.Alpha(0.4), theme.Brand700.Alpha(0.2))
		if t.Glass {
			bg = theme.White.Alpha(0.05) // data-glass:hover:bg-white/5
		}
	}
	btn.Border(t.HL, border).BorderStyle(ui.BorderDashed).Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).
		OnClick(func() { p.leaderboardOpen, p.modalScroll = true, components.ScrollBar{} }).Children(func() {
		components.Icon(c, "format-list-numbered", 30, t.Pick(theme.Brand500, theme.Brand300))
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, locales.StatsLeaderboardViewMore(), theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine()
			components.TextMargin(c, locales.StatsLeaderboardCountHint(strconv.Itoa(count)), theme.TextXS, theme.WeightNormal,
				t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 0)
		})
	})
}

// aiCard 对应 AiSummaryCard：relative overflow-hidden p-6 rounded-xl 的卡片，右上、左下两团 blur-3xl 的光晕，
// 右下角 -rotate-6 的 i-mdi-robot-happy 水印；标题行 mb-4 gap-3 带“思考中”或“联网增强”徽标，
// 加载时是三行 animate-pulse 的占位条，否则是 whitespace-pre-wrap leading-relaxed 的总结。
func (p *Page) aiCard(c *ui.Context, t *theme.Theme, summary string, winW float32) {
	primary := t.Pick(theme.Primary500, theme.Primary400)
	// data-glass:bg-white/18 dark:bg-black/22（盖过 glass-card 的底色）
	box := card(c.Key("ai-summary"), t).Padding(24).Clip()
	if t.Glass {
		box.Background(t.Pick(theme.White.Alpha(0.18), theme.Black.Alpha(0.22)))
	}
	box.Children(func() {
		glow := func(key string, color ui.Color, top, right, bottom, left float32, fromTop, fromRight bool) {
			e := ui.Box(c.Key(key)).Absolute().Size(160, 160).Radius(999).Background(color).Blur(64).PassThrough()
			if fromTop {
				e.Top(top)
			} else {
				e.Bottom(bottom)
			}
			if fromRight {
				e.Right(right)
			} else {
				e.Left(left)
			}
		}
		// -top-12 -right-12 w-40 h-40 bg-primary-500/10 dark:bg-primary-500/20（data-glass:bg-primary-500/20 dark:/30）；
		// -bottom-12 -left-12 bg-accent-500/10 dark:/20
		primaryGlow := pickF(t, 0.1, 0.2)
		if t.Glass {
			primaryGlow = pickF(t, 0.2, 0.3)
		}
		glow("glow-primary", theme.Primary500.Alpha(primaryGlow), -48, -48, 0, 0, true, true)
		glow("glow-accent", accent500.Alpha(pickF(t, 0.1, 0.2)), 0, 0, -48, -48, false, false)
		// -right-4 -bottom-6 size-36 -rotate-6 text-primary-500 opacity-[0.06]（dark:text-primary-400 opacity-[0.08]），
		// data-glass:opacity-[0.12] dark:opacity-[0.16]
		mark := pickF(t, 0.06, 0.08)
		if t.Glass {
			mark = pickF(t, 0.12, 0.16)
		}
		ui.Box(c.Key("watermark")).Absolute().Right(-16).Bottom(-24).PassThrough().Children(func() {
			components.Icon(c, "robot-happy", 144, primary).Rotate(-6).Opacity(mark)
		})

		ui.Column(c.Key("ai-body")).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(0, 0, 16, 0).Children(func() {
				components.Text(c, locales.StatsAiTitle(), theme.TextLG, theme.WeightSemibold, t.TextStrong)
				switch {
				case p.aiLoading:
					b := badge(c.Key("thinking"), t.Pick(theme.Primary50, theme.Primary900.Alpha(0.3)), t.Pick(theme.Primary100, theme.Primary800))
					b.Opacity(components.Pulse(b)).Children(func() {
						components.Text(c, locales.StatsAiThinking(), theme.TextXS, theme.WeightMedium, t.Pick(theme.Primary600, theme.Primary400))
					})
				case p.webSearch:
					fg := t.Pick(blue600, blue400)
					badge(c.Key("web"), t.Pick(blue50, blue900.Alpha(0.3)), t.Pick(blue100, blue800)).Gap(4).Children(func() {
						components.Icon(c, "web", 12, fg)
						components.Text(c, locales.StatsAiWebEnhanced(), theme.TextXS, theme.WeightMedium, fg)
					})
				}
			})
			if p.aiLoading {
				// space-y-3 py-1：h-4 rounded 的 w-3/4、w-full、w-5/6，bg-brand-50 dark:bg-brand-700/50
				// （data-glass:bg-white/15 dark:bg-white/10）
				barBg := t.IfGlass(t.Pick(theme.Brand50, theme.Brand700.Alpha(0.5)), theme.White.Alpha(0.15), theme.White.Alpha(0.1))
				ui.Column(c.Key("ai-loading")).Padding(4, 0).Gap(12).Children(func() {
					for i, frac := range []float32{0.75, 1, 5.0 / 6} {
						bar := ui.Box(c.Key(strconv.Itoa(i))).Height(16).Radius(4).Background(barBg)
						bar.WidthPercent(frac * 100).Opacity(components.Pulse(bar))
					}
				})
				return
			}
			// text-neutral-600 dark:text-neutral-300 leading-relaxed text-sm md:text-base
			f := theme.FontStep{Size: 14, Line: 22.75}
			if winW >= breakMD {
				f = theme.FontStep{Size: 16, Line: 26}
			}
			components.Text(c, summary, f, theme.WeightNormal, t.Pick(theme.Neutral600, theme.Neutral300))
		})
	})
}

// pickF 是 `light dark:dark` 的数值（不透明度）。
func pickF(t *theme.Theme, light, dark float32) float32 {
	if t.Dark {
		return dark
	}
	return light
}

// badge：text-xs px-2.5 py-0.5 rounded-full border。
func badge(c *ui.Context, bg, border ui.Color) ui.Element {
	return ui.Row(c).Padding(2, 10).Radius(999).AlignItems(ui.Center).Background(bg).Border(1, border)
}

// leaderboardModal 对应 StatsLeaderboardModal：max-h-[84vh] max-w-4xl 的面板，border-b p-5 的标题栏（标题、条数与关闭按钮），
// 可滚动的 p-5 两列列表（前 3 名黄色），border-t p-4 的“确定”。
func (p *Page) leaderboardModal(c *ui.Context, t *theme.Theme) {
	if p.stats == nil {
		return
	}
	games := leaderboard(p.stats, leaderboardSize)
	closeModal := func() { p.leaderboardOpen = false }
	_, winH := c.Size()
	maxH := winH * 0.84
	divider := t.Pick(theme.Brand200, theme.Brand700)
	components.Modal(c, t, "leaderboard-modal", components.ModalProps{
		Open: p.leaderboardOpen, TopOffset: env.TopbarHeight, MaxWidth: 896, OnClose: closeModal, Label: locales.CommonCancel(),
	}, func(panel ui.Element) {
		// rounded-xl bg-white shadow-2xl dark:bg-brand-800 overflow-hidden
		panel.MaxHeight(maxH).Radius(12).Clip().Background(t.Pick(theme.White, theme.Brand800)).
			Shadow(0, 25, 50, -12, ui.RGBA(0, 0, 0, 0.25)).Children(func() {
			ui.Row(c.Key("header")).Padding(20).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					components.Text(c, locales.StatsLeaderboardModalTitle(), theme.TextLG, theme.WeightBold, t.TextStrong).SingleLine()
					components.TextMargin(c, locales.StatsLeaderboardCountHint(strconv.Itoa(len(games))), theme.TextSM, theme.WeightNormal,
						t.Pick(theme.Brand500, theme.Brand400), 4, 0, 0, 0)
				})
				// i-mdi-close rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700
				// dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200
				x := ui.ButtonBase(c.Key("close")).Padding(4).Radius(8).FocusRing(false).Label(locales.CommonCancel())
				fg := t.Pick(theme.Brand500, theme.Brand400)
				if x.Hovered() {
					x.Background(t.Pick(theme.Brand100, theme.Brand700))
					fg = t.Pick(theme.Brand700, theme.Brand200)
				}
				x.OnClick(closeModal).Children(func() { components.Icon(c, "close", 24, fg) })
			})
			ui.Box(c).Height(t.HL).Background(divider)
			// 标题栏 p-5 + text-lg + mt-1 text-sm，底栏 p-4 + py-2 的按钮，各带一条细线。
			bodyMax := max(0, maxH-(40+28+4+20)-(32+16+20)-2*t.HL)
			components.ScrollArea(c, t, "body", &p.modalScroll, func(w ui.Element) { w.FillWidth().MaxHeight(bodyMax) }, func(sc ui.Element) {
				gutter := float32(0)
				if p.modalScroll.Overflowing() {
					gutter = t.Gutter()
				}
				winW, _ := c.Size()
				cols := 1
				if winW >= breakMD {
					cols = 2
				}
				sc.Padding(20, 20+gutter, 20, 20).Children(func() {
					ui.Grid(c.Key("list")).Columns(cols).GapX(8).GapY(8).Children(func() {
						for i, g := range games {
							p.modalRow(c.Key(g.GameID), t, g, i+1)
						}
					})
				})
			})
			ui.Box(c).Height(t.HL).Background(divider)
			ui.Row(c.Key("footer")).Padding(16).Justify(ui.End).Children(func() {
				// rounded-lg bg-brand-700 px-5 py-2 text-sm font-medium text-white hover:bg-brand-800
				// dark:bg-brand-600 dark:hover:bg-brand-500
				ok := ui.ButtonBase(c.Key("confirm")).Padding(8, 20).Radius(8).FocusRing(false)
				bg := t.Pick(theme.Brand700, theme.Brand600)
				if ok.Hovered() {
					bg = t.Pick(theme.Brand800, theme.Brand500)
				}
				ok.Background(bg).Transition(theme.ColorsTransition(150 * time.Millisecond)).OnClick(closeModal).Children(func() {
					components.Text(c, locales.CommonConfirm(), theme.TextSM, theme.WeightMedium, theme.White).NoWrap()
				})
			})
		})
	})
}

// modalRow：flex items-center gap-3 rounded-lg border p-3。前 3 名 border-yellow-200/70 bg-yellow-50/80
// （dark:border-yellow-800/40 dark:bg-yellow-900/10），其余 border-brand-200 bg-brand-50/80（dark:border-brand-700
// dark:bg-brand-900/30）；名次 w-8 text-sm font-semibold，封面 h-14 w-10，名称 text-sm font-medium，时长 text-xs font-mono。
func (p *Page) modalRow(c *ui.Context, t *theme.Theme, g vo.GamePlayStats, rank int) {
	top := rank <= 3
	border, bg := t.Pick(theme.Brand200, theme.Brand700), t.Pick(theme.Brand50.Alpha(0.8), theme.Brand900.Alpha(0.3))
	rankColor := t.Pick(theme.Brand500, theme.Brand400)
	if top {
		border, bg = t.Pick(yellow200.Alpha(0.7), yellow800.Alpha(0.4)), t.Pick(yellow50.Alpha(0.8), yellow900.Alpha(0.1))
		rankColor = t.Pick(yellow600, yellow300)
	}
	ui.Row(c).Padding(12).Gap(12).AlignItems(ui.Center).Radius(8).Border(t.HL, border).Background(bg).Label(g.GameName).Children(func() {
		ui.Row(c).Width(32).Shrink(0).Justify(ui.Center).Children(func() {
			components.Text(c, "#"+strconv.Itoa(rank), theme.TextSM, theme.WeightSemibold, rankColor).NoWrap() // i18n:keep：名次
		})
		shadowSM(p.cover(c, t, g, 40, 56))
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
			components.Text(c, g.GameName, theme.TextSM, theme.WeightMedium, t.TextStrong).SingleLine()
			components.TextMargin(c, timefmt.Duration(g.TotalDuration), theme.TextXS, theme.WeightNormal,
				t.Pick(theme.Brand600, theme.Brand300), 2, 0, 0, 0).Font(monoFont)
		})
	})
}

// skeleton 对应 StatsSkeleton：animate-pulse 的 space-y-6 p-8，bg-brand-200 dark:bg-brand-800 的色块
// （标题 h-12 w-32 mb-8；工具栏 h-10 w-48 与两个 h-8 w-8 圆；两块 h-32；1:2 两块 h-[450px]；两块 h-96），
// 玻璃模式下是 data-glass:bg-white/2 dark:bg-black/2。
func (p *Page) skeleton(c *ui.Context, t *theme.Theme) {
	block := t.IfGlass(t.Pick(theme.Brand200, theme.Brand800), theme.White.Alpha(0.02), theme.Black.Alpha(0.02))
	winW, _ := c.Size()
	root := ui.Column(c.Key("skeleton")).Width(p.env.MainWidth).Padding(pagePadding).Gap(sectionGap)
	root.Opacity(components.Pulse(root)).Children(func() {
		ui.Box(c).Size(128, 48).Radius(8).Background(block).Margin(0, 0, 8, 0) // mb-8 与下一项的 mt-6 合并为 32
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Box(c).Size(192, 40).Radius(8).Background(block)
			ui.Row(c).Gap(8).Children(func() {
				ui.Box(c).Size(32, 32).Radius(999).Background(block)
				ui.Box(c).Size(32, 32).Radius(999).Background(block)
			})
		})
		cols2, cols3, wide, narrow := 1, 1, 1, 1
		if winW >= breakMD {
			cols2 = 2
		}
		if winW >= breakLG {
			cols3, wide, narrow = 3, 2, 1
		}
		ui.Grid(c).Columns(cols2).GapX(sectionGap).GapY(sectionGap).Children(func() {
			ui.Box(c).Height(128).Radius(12).Background(block)
			ui.Box(c).Height(128).Radius(12).Background(block)
		})
		ui.Grid(c).Columns(cols3).GapX(sectionGap).GapY(sectionGap).Children(func() {
			ui.Box(c).Height(450).Radius(12).Background(block).ColumnSpan(narrow)
			ui.Box(c).Height(450).Radius(12).Background(block).ColumnSpan(wide)
		})
		ui.Box(c).Height(384).Radius(12).Background(block)
		ui.Box(c).Height(384).Radius(12).Background(block)
	})
}
