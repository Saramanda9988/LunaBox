// Package stats 是统计页面，对应原版 routes/stats.tsx：时间维度与自定义日期范围、AI 总结、汇总卡片、
// 游玩时长排行榜（含 StatsLeaderboardModal）、标签分布、游玩时段分布与两张时长趋势图，
// 以及“美化导出”（TemplateExportModal，见 export.go：模板在 WebView 里预览与截图）。
package stats

import (
	"time"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/chart"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
)

const (
	skeletonDelay   = 300 * time.Millisecond // 原版显示骨架屏前的等待，数据很快返回时不闪一下骨架
	previewCount    = 10                     // 页面上的排行榜条数
	leaderboardSize = 50                     // StatsLeaderboardModal 的条数
)

// Page 是统计页面的状态。原版每次进入页面重新挂载组件，Open 重置除 AI 总结以外的状态；
// AI 总结按时间维度存在 store 的 aiSummaryCache 里，离开页面后仍保留。
type Page struct {
	env *env.Env

	period   enums.Period // dimension
	stats    *vo.PeriodStats
	loading  bool
	openedAt time.Time
	gen      int // 每次进入页面加一，丢弃上一次进入时还没返回的结果
	seq      int // 每次加载加一，只采用最新一次加载的结果

	custom     bool   // customDateRange
	start, end string // 自定义范围的 YYYY-MM-DD

	aiSummary map[enums.Period]string // aiSummaryCache
	aiLoading bool
	webSearch bool // webSearchUsed

	leaderboardOpen bool
	export          exporter

	scroll      components.ScrollBar
	modalScroll components.ScrollBar
	totalChart  chart.State
	gameChart   chart.State
	hourWeek    chart.HourWeekState
	tags        chart.TagState

	toasts []func() env.Notice // 待显示的提示（显示时按当前语言取）
}

// New 创建页面；数据在 Open 时加载。
func New(e *env.Env) *Page {
	return &Page{env: e, period: enums.Week, aiSummary: map[enums.Period]string{}}
}

// Open 进入页面：回到“周”维度并加载统计（原版挂载时的 useEffect）。
func (p *Page) Open(now time.Time) {
	p.gen++
	p.period, p.stats, p.openedAt = enums.Week, nil, now
	p.custom, p.start, p.end = false, "", ""
	p.aiLoading, p.webSearch = false, false
	p.leaderboardOpen = false
	p.closeExport()
	p.export.templates, p.export.selected = nil, mosaicID
	p.scroll, p.modalScroll = components.ScrollBar{}, components.ScrollBar{}
	p.totalChart, p.gameChart, p.hourWeek, p.tags = chart.State{}, chart.State{}, chart.HourWeekState{}, chart.TagState{}
	p.toasts = nil
	p.load(p.period, "", "")
}

func (p *Page) toast(n func() env.Notice) { p.toasts = append(p.toasts, n) }

// load 对应 loadStats：失败时提示并保留上一次的统计。
func (p *Page) load(period enums.Period, start, end string) {
	p.seq++
	p.loading = true
	gen, seq := p.gen, p.seq
	req := vo.PeriodStatsRequest{Dimension: period, StartDate: start, EndDate: end}
	p.env.Async(func() func() {
		data, err := p.env.Svc.Stats.GetGlobalPeriodStats(req)
		return func() {
			if gen != p.gen || seq != p.seq {
				return
			}
			p.loading = false
			if err != nil {
				p.toast(func() env.Notice { return env.Error(locales.StatsToastLoadStatsFailed()) })
				return
			}
			p.stats = &data
		}
	})
}

// changePeriod 对应 handlePeriodChange 与它触发的 useEffect：切换维度时退出自定义范围并重新加载。
func (p *Page) changePeriod(v enums.Period) {
	if v == p.period && !p.custom {
		return
	}
	p.period = v
	p.custom, p.start, p.end = false, "", ""
	p.load(p.period, "", "")
}

// applyRange 对应 handleApplyDateRange：自定义范围按“周”维度请求（后端用请求里的日期）。
func (p *Page) applyRange() {
	switch {
	case p.start == "" || p.end == "":
		p.toast(func() env.Notice { return env.Error(locales.StatsToastSelectDateRange()) })
		return
	case p.start >= p.end: // YYYY-MM-DD 按字符串比较即按日期比较
		p.toast(func() env.Notice { return env.Error(locales.StatsToastStartBeforeEnd()) })
		return
	}
	p.custom = true
	p.load(enums.Week, p.start, p.end)
}

// resetRange 对应 handleResetDateRange。
func (p *Page) resetRange() {
	p.custom, p.start, p.end = false, "", ""
	p.load(p.period, "", "")
}

// summarize 对应 handleAISummarize：按当前时间维度（自定义范围时同样如此）请求总结，结果写进跨页面保留的缓存。
func (p *Page) summarize() {
	period, gen := p.period, p.gen
	p.aiLoading, p.webSearch = true, false
	p.aiSummary[period] = ""
	p.env.Async(func() func() {
		res, err := p.env.Svc.AI.AISummarize(vo.AISummaryRequest{Dimension: string(period)})
		return func() {
			if err != nil {
				p.aiSummary[period] = ""
				p.toast(func() env.Notice { return env.Error(locales.StatsAiSummarizeFailed()) })
			} else {
				p.aiSummary[period] = res.Summary
			}
			if gen == p.gen { // 离开页面后组件已卸载，加载状态与“联网增强”不再更新
				p.aiLoading = false
				p.webSearch = err == nil && res.WebSearchUsed
			}
		}
	})
}

// leaderboard 返回排行榜的前 n 项。
func leaderboard(s *vo.PeriodStats, n int) []vo.GamePlayStats {
	return s.PlayTimeLeaderboard[:min(n, len(s.PlayTimeLeaderboard))]
}
