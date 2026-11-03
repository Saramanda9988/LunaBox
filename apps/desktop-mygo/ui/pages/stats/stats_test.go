package stats

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/picker"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

type harness struct {
	*Page
	svc       *host.Backend
	tt        *ui.Tester
	announced []string
}

// newHarness 写入游戏与游玩记录（setup）后打开统计页。mainW 是主内容区的宽度（决定 @[1024px] 的两列）。
func newHarness(t *testing.T, mainW float32, setup func(svc *host.Backend)) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	svc.Config.TimeZone = ""
	if setup != nil {
		setup(svc)
	}
	h := &harness{svc: svc}
	e := &env.Env{Backend: envtest.NewBackend(0), Svc: svc.API(), Router: ui.NewRouter(env.PathStats), Covers: covers.New(),
		Theme: theme.New(true, 1.5), MainWidth: mainW}
	h.Page = New(e)
	h.Open(time.Now())
	h.tt = ui.NewTester(h.View, int(mainW)+180, 900)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(2)
	return h
}

// games 写入 n 个游戏，第 i 个昨天玩了 (n-i) 小时，排行榜按 g1、g2… 排列。
func games(t *testing.T, n int) func(svc *host.Backend) {
	return func(svc *host.Backend) {
		yesterday := time.Now().Add(-24 * time.Hour)
		for i := 1; i <= n; i++ {
			id := fmt.Sprintf("g%d", i)
			hosttest.AddGame(t, svc, models.Game{ID: id, Name: name(i)})
			hosttest.AddSession(t, svc, id, yesterday, (n-i+1)*3600)
		}
	}
}

func name(i int) string { return fmt.Sprintf("测试游戏%02d", i) }

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, label string) {
	t.Helper()
	if err := h.tt.Click(label); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) expectNot(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if h.tt.HasText(s) {
			t.Errorf("unexpected %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) expectToast(t *testing.T, msg string) {
	t.Helper()
	h.announced = append(h.announced, h.tt.Announcements()...)
	if !slices.ContainsFunc(h.announced, func(s string) bool { return strings.Contains(s, msg) }) {
		t.Errorf("no toast %q in %q", msg, h.announced)
	}
}

func TestEmpty(t *testing.T) {
	h := newHarness(t, 1100, nil)
	h.expect(t, locales.StatsTitle(), locales.StatsSummaryTotalPlayCount(), locales.StatsSummaryMaxStreak(),
		locales.StatsLeaderboardFullTitle(), locales.StatsLeaderboardNoData(), locales.StatsTagDistributionEmpty(),
		locales.StatsHeatmapEmpty(), locales.StatsChartsTotalTrend(), locales.StatsChartsGameTrend())
	h.expect(t, "0"+locales.CommonDurationMinutesShort()) // formatDurationCompact(0)
	h.expectNot(t, locales.StatsAiTitle(), locales.StatsLeaderboardViewMore())
}

func TestShowsLeaderboardAndTags(t *testing.T) {
	h := newHarness(t, 1100, func(svc *host.Backend) {
		games(t, 3)(svc)
		hosttest.AddTag(t, svc, "g1", "t1", "纯爱", "user", false)
		hosttest.AddTag(t, svc, "g2", "t2", "剧透标签", "user", true)
		hosttest.AddTag(t, svc, "g3", "t3", "ADV", "vndb", false)
	})
	h.expect(t, name(1), name(2), name(3), "#1", "#2", "#3", timefmt.Duration(3*3600), "纯爱", "冒险游戏（ADV）")
	h.expectNot(t, locales.StatsLeaderboardNoData(), locales.StatsLeaderboardViewMore(), "剧透标签")
	if h.stats.TotalGamesCount != 3 {
		t.Fatalf("games played = %d", h.stats.TotalGamesCount)
	}
	h.expect(t, "6"+locales.CommonDurationHoursShort()) // 总时长 3+2+1 小时

	h.svc.Config.EnableTagTranslation = false
	h.frames(2)
	h.expect(t, "ADV")
	h.expectNot(t, "冒险游戏（ADV）")
}

func TestLeaderboardModal(t *testing.T) {
	h := newHarness(t, 1100, games(t, 12))
	h.expect(t, name(10), locales.StatsLeaderboardCountHint("12"))
	h.expectNot(t, name(11), locales.StatsLeaderboardModalTitle())

	h.click(t, locales.StatsLeaderboardViewMore())
	h.expect(t, locales.StatsLeaderboardModalTitle(), name(11), name(12), "#12")
	h.click(t, locales.CommonConfirm())
	h.expectNot(t, locales.StatsLeaderboardModalTitle(), name(11))

	h.click(t, locales.StatsLeaderboardViewMore())
	h.tt.Key(0, ui.KeyEscape)
	h.frames(2)
	h.expectNot(t, locales.StatsLeaderboardModalTitle())
}

func TestPeriodAndCustomRange(t *testing.T) {
	h := newHarness(t, 1100, func(svc *host.Backend) {
		hosttest.AddGame(t, svc, models.Game{ID: "old", Name: "二十天前"})
		hosttest.AddSession(t, svc, "old", time.Now().AddDate(0, 0, -20), 1800)
	})
	h.expect(t, locales.StatsLeaderboardNoData())

	h.click(t, locales.StatsPeriodMonth())
	if h.period != enums.Month {
		t.Fatalf("period = %q", h.period)
	}
	h.expect(t, "二十天前")

	h.click(t, locales.StatsPeriodWeek())
	h.expect(t, locales.StatsLeaderboardNoData())

	// 自定义范围：校验与原版相同，开始日期须早于结束日期
	h.applyRange()
	h.frames(1)
	h.expectToast(t, locales.StatsToastSelectDateRange())
	today := picker.FormatDate(time.Now())
	h.start, h.end = today, today
	h.applyRange()
	h.frames(1)
	h.expectToast(t, locales.StatsToastStartBeforeEnd())
	if h.custom {
		t.Fatal("custom range applied with start == end")
	}

	h.start = picker.FormatDate(time.Now().AddDate(0, 0, -25))
	h.applyRange()
	h.frames(2)
	if !h.custom {
		t.Fatal("custom range not applied")
	}
	h.expect(t, "二十天前", h.start+" - "+h.end)

	// 切换维度退出自定义范围
	h.click(t, locales.StatsPeriodWeek())
	if h.custom || h.start != "" || h.end != "" {
		t.Fatalf("custom range kept: %v %q %q", h.custom, h.start, h.end)
	}
	h.expect(t, locales.StatsLeaderboardNoData(), locales.StatsCustomRange())
}

func TestResetRange(t *testing.T) {
	h := newHarness(t, 1100, nil)
	h.period = enums.Year
	h.start, h.end = "2026-01-01", "2026-02-01"
	h.applyRange()
	h.frames(2)
	if !h.custom || h.stats.StartDate != "2026-01-01" {
		t.Fatalf("custom = %v, start %q", h.custom, h.stats.StartDate)
	}
	h.resetRange()
	h.frames(2)
	if h.custom || h.start != "" || h.stats.Dimension != enums.Year {
		t.Fatalf("after reset: custom %v start %q dimension %q", h.custom, h.start, h.stats.Dimension)
	}
}

// 测试没有窗口，也就没有 WebView：模板列表照常加载，选中第一个并用真实服务渲染，预览区提示缺少 WebView，
// 导出按钮不可用。
func TestExportModal(t *testing.T) {
	h := newHarness(t, 1100, games(t, 2))
	h.click(t, locales.StatsExportTitle())
	x := &h.export
	if len(x.templates) < 3 || x.selected != x.templates[0].ID {
		t.Fatalf("templates %v, selected %q", x.templates, x.selected)
	}
	h.expect(t, locales.StatsTemplateExportTitle(), locales.StatsTemplateExportSelectTemplate(), x.templates[0].Name,
		x.templates[1].Name, locales.NativeTemplateExportWebViewUnavailable(), locales.StatsTemplateExportTipText())
	if x.loading || !strings.Contains(x.html, name(1)) {
		t.Fatalf("not rendered: loading %v, html has game %v", x.loading, strings.Contains(x.html, name(1)))
	}
	h.exportImage() // 没有 WebView 时什么都不做
	if x.exporting {
		t.Fatal("exporting without a web view")
	}

	first := x.html
	h.click(t, x.templates[1].Name)
	if x.selected != x.templates[1].ID || x.html == first {
		t.Fatalf("switch template: selected %q, html changed %v", x.selected, x.html != first)
	}

	h.click(t, locales.StatsTemplateExportCancelBtn())
	h.expectNot(t, locales.StatsTemplateExportTipText())
	h.click(t, locales.StatsExportTitle()) // 再次打开时保留选中的模板
	if x.selected != x.templates[1].ID || !h.tt.HasText(locales.StatsTemplateExportTipText()) {
		t.Fatalf("reopen: selected %q", x.selected)
	}
	h.click(t, locales.CommonClose())
	h.expectNot(t, locales.StatsTemplateExportTipText())
}

func TestAISummarizeFails(t *testing.T) {
	h := newHarness(t, 1100, nil) // 没有配置 AI API Key
	h.click(t, locales.StatsAiSummarizeTitle())
	h.expectToast(t, locales.StatsAiSummarizeFailed())
	if h.aiLoading {
		t.Fatal("still loading after failure")
	}
	h.expectNot(t, locales.StatsAiTitle())
}

// fakeAI 返回固定的总结，记下请求的维度。
type fakeAI struct {
	dims []string
	err  error
}

func (f *fakeAI) AISummarize(req vo.AISummaryRequest) (vo.AISummaryResponse, error) {
	f.dims = append(f.dims, req.Dimension)
	if f.err != nil {
		return vo.AISummaryResponse{}, f.err
	}
	return vo.AISummaryResponse{Summary: "本周锐评：" + req.Dimension, Dimension: req.Dimension, WebSearchUsed: true}, nil
}

func TestAISummaryCachedPerPeriod(t *testing.T) {
	h := newHarness(t, 1100, nil)
	ai := &fakeAI{}
	h.env.Svc.AI = ai
	h.click(t, locales.StatsAiSummarizeTitle())
	h.expect(t, locales.StatsAiTitle(), "本周锐评：week", locales.StatsAiWebEnhanced())

	// 总结按维度缓存：换到“月”没有总结，换回“周”还在
	h.click(t, locales.StatsPeriodMonth())
	h.expectNot(t, locales.StatsAiTitle())
	h.click(t, locales.StatsPeriodWeek())
	h.expect(t, "本周锐评：week")

	// 离开再进入页面：缓存在 store 里保留，“联网增强”是组件状态，不保留
	h.Open(time.Now())
	h.frames(2)
	h.expect(t, "本周锐评：week")
	h.expectNot(t, locales.StatsAiWebEnhanced())

	// 再次总结失败时清空这个维度的缓存
	ai.err = errors.New("boom")
	h.click(t, locales.StatsAiSummarizeTitle())
	h.expectToast(t, locales.StatsAiSummarizeFailed())
	h.expectNot(t, locales.StatsAiTitle(), "本周锐评：week")
	if !slices.Equal(ai.dims, []string{"week", "week"}) {
		t.Fatalf("dims = %q", ai.dims)
	}
}

func TestLoadFailureKeepsStats(t *testing.T) {
	h := newHarness(t, 1100, games(t, 1))
	h.expect(t, name(1))
	h.env.Svc.Stats = failingStats{}
	h.click(t, locales.StatsPeriodMonth())
	h.expectToast(t, locales.StatsToastLoadStatsFailed())
	h.expect(t, name(1))
	if h.loading {
		t.Fatal("still loading after failure")
	}
}

type failingStats struct{}

func (failingStats) GetGameStats(vo.GameStatsRequest) (vo.GameDetailStats, error) {
	return vo.GameDetailStats{}, errors.New("boom")
}

func (failingStats) GetGlobalPeriodStats(vo.PeriodStatsRequest) (vo.PeriodStats, error) {
	return vo.PeriodStats{}, errors.New("boom")
}

// TestContainerColumns：主内容区不小于 1024 时排行榜与标签分布、时段分布与总时长趋势各占一行，否则上下排列。
func TestContainerColumns(t *testing.T) {
	sameRow := func(h *harness, a, b string) bool {
		ra, okA := h.tt.Find(a)
		rb, okB := h.tt.Find(b)
		if !okA || !okB {
			t.Fatalf("missing %q or %q in %q", a, b, h.tt.Texts())
		}
		return ra.Y == rb.Y && ra.X < rb.X
	}
	wide := newHarness(t, 1200, games(t, 2))
	if !sameRow(wide, locales.StatsLeaderboardFullTitle(), locales.StatsTagDistributionTitle()) ||
		!sameRow(wide, locales.StatsTimeOfDayTitle(), locales.StatsChartsTotalTrend()) {
		t.Error("wide layout is not two columns")
	}
	narrow := newHarness(t, 900, games(t, 2))
	if sameRow(narrow, locales.StatsLeaderboardFullTitle(), locales.StatsTagDistributionTitle()) {
		t.Error("narrow layout is not one column")
	}
}

func TestDurationCompact(t *testing.T) {
	for _, tc := range []struct {
		sec  int
		want string
	}{{0, "0分"}, {59, "0分"}, {120, "2分"}, {3600, "1时"}, {3720, "1时2分"}, {-5, "0分"}} {
		if got := durationCompact(tc.sec); got != tc.want {
			t.Errorf("durationCompact(%d) = %q, want %q", tc.sec, got, tc.want)
		}
	}
}
