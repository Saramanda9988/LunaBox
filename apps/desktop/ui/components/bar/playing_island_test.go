package bar

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// islandFixture 像 App 一样持有灵动岛的状态，运行状态与动作由测试直接改。
type islandFixture struct {
	island   PlayingIsland
	runtime  *IslandRuntime
	count    int
	switches int
	ended    []string
	endErr   error
	endDone  func(error) // 不为 nil 时由测试决定何时结束
	tt       *ui.Tester
	env      *env.Env
	toasts   []string
}

func newIslandFixture(t *testing.T) *islandFixture {
	t.Helper()
	f := &islandFixture{count: 1, env: &env.Env{}}
	f.tt = ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1.5)
		f.env.FlushToasts(c)
		f.island.View(c, th, PlayingIslandProps{
			Env: f.env, Runtime: f.runtime, Count: f.count,
			OnSwitch: func() { f.switches++ },
			OnEnd: func(id string, done func(error)) {
				f.ended = append(f.ended, id)
				f.endDone = done
			},
		})
	}, 1280, 800)
	f.tt.SetScale(1.5)
	f.tt.SetDark(true)
	return f
}

func (f *islandFixture) frames(n int) {
	for range n {
		f.tt.Frame()
	}
}

func (f *islandFixture) announced() []string {
	f.toasts = append(f.toasts, f.tt.Announcements()...)
	return f.toasts
}

func islandRuntime(id, name string, state service.GameRuntimeState, start time.Time) *IslandRuntime {
	return &IslandRuntime{Game: models.Game{ID: id, Name: name}, SessionID: "s-" + id, StartTime: start, State: state}
}

func TestPlayingIslandStatesAndAutoCollapse(t *testing.T) {
	f := newIslandFixture(t)
	f.frames(1)
	if f.tt.HasText(locales.PlayingIslandEnd()) {
		t.Fatal("island shown without a runtime")
	}

	f.runtime = islandRuntime("g1", "千恋＊万花", service.GameRuntimeStateLaunching, f.tt.Now())
	f.frames(2)
	if !f.tt.HasText("千恋＊万花") || !f.tt.HasText(locales.PlayingIslandLaunching()) {
		t.Fatalf("launching island: %q", f.tt.Texts())
	}

	f.runtime = islandRuntime("g1", "千恋＊万花", service.GameRuntimeStatePlaying, f.tt.Now().Add(-65*time.Minute))
	f.frames(2)
	if want := locales.PlayingIslandElapsed(timefmt.DurationCompact(65 * 60)); !f.tt.HasText(want) {
		t.Fatalf("missing %q in %q", want, f.tt.Texts())
	}
	// 活跃时长计时：显示后端报告的秒数。
	secs := 125
	f.runtime.ActiveSeconds = &secs
	f.frames(2)
	if want := locales.PlayingIslandElapsed(timefmt.DurationCompact(secs)); !f.tt.HasText(want) {
		t.Fatalf("missing %q in %q", want, f.tt.Texts())
	}

	// 开始游玩 5 秒后自动收起，只收起一次。
	f.tt.Advance(4 * time.Second)
	if f.island.collapsed {
		t.Fatal("collapsed too early")
	}
	f.tt.Advance(time.Second + 100*time.Millisecond)
	if !f.island.collapsed {
		t.Fatal("not collapsed after 5s of playing")
	}
	f.tt.Advance(time.Second)
	if err := f.tt.Click(locales.PlayingIslandExpand()); err != nil {
		t.Fatalf("%v in %q", err, f.tt.Texts())
	}
	f.frames(2)
	if f.island.collapsed {
		t.Fatal("click did not expand the collapsed island")
	}
	f.tt.Advance(6 * time.Second)
	if f.island.collapsed {
		t.Fatal("collapsed again for the same runtime")
	}

	// 向上拖动超过阈值收起。
	f.tt.Advance(time.Second)
	r, ok := f.tt.Find("千恋＊万花")
	if !ok {
		t.Fatal("title not found")
	}
	f.tt.Press(r.X+4, r.Y+4)
	f.tt.Move(r.X+4, r.Y-10)
	f.tt.Move(r.X+4, r.Y-30)
	f.frames(1)
	f.tt.Release(r.X+4, r.Y-30)
	f.frames(2)
	if !f.island.collapsed {
		t.Fatal("dragging up did not collapse the island")
	}

	// 运行状态消失后播放退出动画，220ms 后不再显示。
	f.runtime = nil
	f.frames(1)
	if !f.tt.HasText("千恋＊万花") {
		t.Fatal("island removed without the exit animation")
	}
	f.tt.Advance(300 * time.Millisecond)
	if f.tt.HasText("千恋＊万花") {
		t.Fatal("island still shown after the exit animation")
	}
	if f.island.collapsed {
		t.Fatal("state kept after the island went away")
	}
}

func TestPlayingIslandEnd(t *testing.T) {
	f := newIslandFixture(t)
	f.runtime = islandRuntime("g1", "千恋＊万花", service.GameRuntimeStatePlaying, f.tt.Now())
	f.frames(2)
	if err := f.tt.Click(locales.PlayingIslandEnd()); err != nil {
		t.Fatal(err)
	}
	f.frames(2)
	if !slices.Equal(f.ended, []string{"g1"}) || !f.tt.HasText(locales.PlayingIslandEnding()) {
		t.Fatalf("ended %q, texts %q", f.ended, f.tt.Texts())
	}
	// 结束中再点不会重复结束。
	_ = f.tt.Click(locales.PlayingIslandEnd())
	f.frames(1)
	if len(f.ended) != 1 {
		t.Fatalf("ended twice: %q", f.ended)
	}

	f.endDone(errors.New("boom"))
	f.frames(2)
	if !slices.Contains(f.announced(), locales.PlayingIslandToastEndFailed()) || f.island.ending {
		t.Fatalf("toasts %q, ending %v", f.toasts, f.island.ending)
	}

	_ = f.tt.Click(locales.PlayingIslandEnd())
	f.frames(2)
	f.endDone(nil)
	f.frames(2)
	if !slices.Contains(f.announced(), locales.PlayingIslandToastEndSuccess()) {
		t.Fatalf("toasts %q", f.toasts)
	}
}

func TestPlayingIslandSwitch(t *testing.T) {
	f := newIslandFixture(t)
	f.runtime = islandRuntime("g1", "千恋＊万花", service.GameRuntimeStatePlaying, f.tt.Now())
	f.count = 2
	f.frames(2)
	if !f.tt.HasText("2") {
		t.Fatalf("no runtime count badge in %q", f.tt.Texts())
	}
	if err := f.tt.Click(locales.PlayingIslandSwitchGame("2")); err != nil {
		t.Fatal(err)
	}
	f.frames(1)
	if f.switches != 1 {
		t.Fatalf("switches = %d", f.switches)
	}

	// 换到另一个游戏：新内容淡入，旧内容淡出后移除。
	f.runtime = islandRuntime("g2", "サノバウィッチ", service.GameRuntimeStatePlaying, f.tt.Now())
	f.frames(1)
	if !f.tt.HasText("サノバウィッチ") || f.island.outgoing == nil {
		t.Fatalf("switch: outgoing %v, texts %q", f.island.outgoing, f.tt.Texts())
	}
	f.tt.Advance(300 * time.Millisecond)
	if f.island.outgoing != nil || f.tt.HasText("千恋＊万花") {
		t.Fatalf("outgoing content kept: %q", f.tt.Texts())
	}

	// 只有一个游戏时封面不切换。
	f.count = 1
	f.frames(1)
	_ = f.tt.Click(locales.PlayingIslandSwitchGame("1"))
	f.frames(1)
	if f.switches != 1 {
		t.Fatalf("switched with a single runtime: %d", f.switches)
	}
}
