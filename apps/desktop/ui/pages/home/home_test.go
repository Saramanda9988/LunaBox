package home

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
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

// game 是一个路径不存在的游戏（启动会在找可执行文件时失败，不会真的启动任何程序）。
func game(id, name string) models.Game {
	return models.Game{ID: id, Name: name, Path: `Z:\lunabox-missing\` + id + `.exe`}
}

// newHarness 写入游戏与游玩记录（setup）后打开首页。服务的事件经 host.Events 同步分发给页面。
func newHarness(t *testing.T, setup func(svc *host.Backend)) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	svc.Config.TimeZone = ""
	if setup != nil {
		setup(svc)
	}
	h := &harness{svc: svc}
	e := &env.Env{Backend: envtest.NewBackend(0), Svc: svc.API(), Router: ui.NewRouter(env.PathHome), Covers: covers.New(),
		Theme: theme.New(true, 1.5), MainWidth: 1100}
	h.Page = New(e)
	h.Open()
	h.tt = ui.NewTester(h.View, 1280, 800)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(2)
	return h
}

// twoGames 写入两个游戏：b 最近玩过（选中、显示在前），a 更早。
func twoGames(t *testing.T) func(svc *host.Backend) {
	return func(svc *host.Backend) {
		now := time.Now()
		hosttest.AddGame(t, svc, game("a", "千恋＊万花"))
		hosttest.AddGame(t, svc, game("b", "サノバウィッチ"))
		hosttest.AddSession(t, svc, "a", now.Add(-48*time.Hour), 3600)
		hosttest.AddSession(t, svc, "b", now.Add(-2*time.Hour), 300)
	}
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

// advance 推进帧时钟：Tester 在其间按页面要求的时间（c.After）与动画逐帧构建，轮播与淡入淡出都按帧时钟计时。
func (h *harness) advance(d time.Duration) {
	h.tt.Advance(d)
	h.frames(1)
}

// hero 是左下角正在显示的游戏（面板展开时封面的无障碍名称里也有游戏名，不能只看文字）。
func (h *harness) hero() string {
	if h.Page.hero.shown == nil {
		return ""
	}
	return h.Page.hero.shown.game.ID
}

// itemIDs 是轮播里的游戏。
func (h *harness) itemIDs() []string {
	var ids []string
	for _, it := range h.items() {
		ids = append(ids, it.Game.ID)
	}
	return ids
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

func TestNoPlayRecord(t *testing.T) {
	h := newHarness(t, func(svc *host.Backend) { hosttest.AddGame(t, svc, game("a", "千恋＊万花")) })
	h.expect(t, locales.HomeTitle(), locales.HomeWelcome(), locales.HomeNoPlayRecord(), locales.HomeNoPlayRecordHint())
	h.expectNot(t, "千恋＊万花", locales.HomeContinueGame())
	h.click(t, locales.HomeBrowseLibrary())
	if got := h.env.Router.Path(); got != env.PathLibrary {
		t.Errorf("browse library went to %q", got)
	}
}

func TestShowsLastPlayed(t *testing.T) {
	start := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	h := newHarness(t, func(svc *host.Backend) {
		hosttest.AddGame(t, svc, game("a", "千恋＊万花"))
		hosttest.AddSession(t, svc, "a", start, 3900)
		hosttest.AddSession(t, svc, "a", start.Add(-48*time.Hour), 300)
	})
	h.expect(t, locales.HomeTitle(), locales.HomeWelcomeBack(), "千恋＊万花", locales.HomeContinueGame(),
		locales.HomeLastPlayed(timefmt.DateTime(start, time.Local)),
		locales.HomeTotalPlayTime()+timefmt.Duration(4200))
	// 只有一个游戏时没有封面选择面板。
	h.expectNot(t, locales.HomeExpandCoverPicker())

	h.click(t, "千恋＊万花")
	if got := h.env.Router.Path(); got != env.GamePath("a") {
		t.Errorf("title click went to %q", got)
	}
}

func TestCarouselAdvances(t *testing.T) {
	h := newHarness(t, twoGames(t))
	h.expect(t, "サノバウィッチ")
	h.expectNot(t, "千恋＊万花")

	// 默认 6 秒切换；旧内容淡出 280ms、等 90ms 后换上新内容。
	h.advance(5 * time.Second)
	h.expect(t, "サノバウィッチ")
	h.advance(1500 * time.Millisecond)
	h.expect(t, "千恋＊万花")
	h.expectNot(t, "サノバウィッチ")
	h.advance(6500 * time.Millisecond)
	h.expect(t, "サノバウィッチ")
}

func TestCarouselStops(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		h := newHarness(t, twoGames(t))
		h.svc.Config.HomeGameCarouselEnabled = false
		h.advance(7 * time.Second)
		h.expect(t, "サノバウィッチ")
		h.expectNot(t, "千恋＊万花")
	})
	t.Run("picker expanded", func(t *testing.T) {
		h := newHarness(t, twoGames(t))
		h.advance(5 * time.Second)
		h.click(t, locales.HomeExpandCoverPicker())
		h.advance(time.Second) // 收起时已经计时 5 秒，展开后不再切换
		h.expect(t, locales.HomeCollapseCoverPicker())
		if h.activeID != "b" || !h.tickFrom.IsZero() {
			t.Errorf("carousel at %q, timer from %v while the picker was open", h.activeID, h.tickFrom)
		}
		// 收起后重新计时。
		h.click(t, locales.HomeCollapseCoverPicker())
		h.advance(5 * time.Second)
		if h.activeID != "b" {
			t.Errorf("carousel did not restart its timer: at %q", h.activeID)
		}
		h.advance(1500 * time.Millisecond)
		if h.hero() != "a" {
			t.Errorf("hero = %q after the restarted interval", h.hero())
		}
	})
	t.Run("game running", func(t *testing.T) {
		h := newHarness(t, twoGames(t))
		h.svc.Events.Emit(runtimeEvent, service.GameRuntimeChangedEvent{GameID: "a", SessionID: "s1", StartTime: time.Now(),
			State: service.GameRuntimeStatePlaying})
		h.advance(7 * time.Second)
		if h.activeID != "b" {
			t.Errorf("carousel moved to %q while a game was running", h.activeID)
		}
	})
}

func TestRailPicker(t *testing.T) {
	h := newHarness(t, twoGames(t))
	h.click(t, locales.HomeExpandCoverPicker())
	h.advance(400 * time.Millisecond)
	h.expect(t, locales.HomeRecentPlayed(), locales.StatsLibrarySectionTitle(), locales.StatsHeatmapTitle())

	if h.hero() != "b" {
		t.Fatalf("hero = %q", h.hero())
	}
	h.click(t, locales.HomeSelectGame("千恋＊万花"))
	h.advance(500 * time.Millisecond)
	if h.hero() != "a" {
		t.Errorf("hero = %q after selecting a cover", h.hero())
	}

	// 游戏库统计：2 个游戏、2 次游玩、共 1 小时 5 分钟。
	h.click(t, locales.StatsLibrarySectionTitle())
	h.expect(t, locales.StatsLibraryTotalGames(), locales.StatsLibraryTotalSessions(), locales.StatsLibraryCompletedGames(),
		timefmt.Duration(3900))

	h.click(t, locales.StatsHeatmapTitle())
	h.expectNot(t, locales.StatsToastLoadStatsFailed(), locales.CommonLoading())

	// 收起后内容不再构建；面板重新挂载（再次进入首页）时回到“最近游玩”。
	h.click(t, locales.HomeCollapseCoverPicker())
	h.advance(400 * time.Millisecond)
	h.expectNot(t, locales.StatsHeatmapTitle())
	h.Open()
	h.frames(2)
	if h.expanded || h.rail.Tab() != "recent" {
		t.Errorf("reopened with the picker expanded %v on tab %q", h.expanded, h.rail.Tab())
	}
}

func TestPlayingGame(t *testing.T) {
	h := newHarness(t, twoGames(t))
	h.svc.Events.Emit(runtimeEvent, service.GameRuntimeChangedEvent{GameID: "b", SessionID: "s1", StartTime: time.Now(),
		State: service.GameRuntimeStateLaunching, Reason: "launched"})
	h.frames(2)
	h.expect(t, locales.HomeGaming(), locales.HomePlayingNow())
	h.expectNot(t, locales.HomeContinueGame())

	// 刚启动时跳过一次刷新请求；游戏结束（idle）后刷新首页数据，看到新的游玩记录。
	if !h.skipLaunch {
		t.Fatal("launched event did not skip the next refresh")
	}
	h.svc.Events.Emit(refreshEvent)
	if h.skipLaunch {
		t.Error("refresh request did not clear the skip")
	}
	hosttest.AddGame(t, h.svc, game("c", "天使☆騒々"))
	hosttest.AddSession(t, h.svc, "c", time.Now().Add(-time.Minute), 60)
	h.svc.Events.Emit(runtimeEvent, service.GameRuntimeChangedEvent{GameID: "b", SessionID: "s1", State: service.GameRuntimeStateIdle})
	h.frames(2)
	h.expect(t, locales.HomeContinueGame())
	// 新玩过的游戏排在最前，原来选中的游戏还在轮播里，所以保持选中它（activeGameId 的 effect）。
	if ids := h.itemIDs(); !slices.Equal(ids, []string{"c", "b", "a"}) || h.activeID != "b" {
		t.Errorf("items %v, active %q", ids, h.activeID)
	}
}

func TestRefreshRequested(t *testing.T) {
	h := newHarness(t, twoGames(t))
	hosttest.AddGame(t, h.svc, game("c", "天使☆騒々"))
	hosttest.AddSession(t, h.svc, "c", time.Now().Add(-time.Minute), 60)
	h.svc.Events.Emit(refreshEvent)
	h.frames(2)
	if ids := h.itemIDs(); !slices.Equal(ids, []string{"c", "b", "a"}) || h.activeID != "b" {
		t.Errorf("items %v, active %q", ids, h.activeID)
	}
	// 选中的游戏不在了时回到第一个。
	if _, err := h.svc.DB.Exec(`DELETE FROM play_sessions WHERE game_id = 'b'`); err != nil {
		t.Fatal(err)
	}
	h.svc.Events.Emit(refreshEvent)
	h.advance(500 * time.Millisecond)
	if h.activeID != "c" || h.hero() != "c" {
		t.Errorf("active %q, hero %q after the selected game left", h.activeID, h.hero())
	}
	h.expect(t, "天使☆騒々")
}

func TestContinueGameFailure(t *testing.T) {
	h := newHarness(t, twoGames(t))
	h.click(t, locales.HomeContinueGame())
	h.expectToast(t, locales.HomeToastLaunchFailed())
	if h.launcher.Active() {
		t.Error("failed start left a runtime")
	}
	h.expect(t, locales.HomeContinueGame())
}

// 首页数据里正在游玩的游戏同步为运行状态（setGameRuntimeFromHome）。
func TestSyncRuntimeFromHome(t *testing.T) {
	h := newHarness(t, twoGames(t))
	if _, err := h.svc.DB.Exec(`UPDATE play_sessions SET end_time = NULL WHERE game_id = 'b'`); err != nil {
		t.Fatal(err)
	}
	h.Open()
	h.frames(2)
	if running, _ := h.launcher.Status("b"); !running {
		t.Error("playing game from home data is not running")
	}
	h.expect(t, locales.HomeGaming())
}

func TestNoData(t *testing.T) {
	h := newHarness(t, nil)
	h.data = nil
	h.frames(1)
	h.expect(t, locales.HomeNoData())
	h.click(t, locales.HomeRetry())
	h.expect(t, locales.HomeNoPlayRecord())
}
