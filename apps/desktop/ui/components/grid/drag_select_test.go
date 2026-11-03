package grid

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// dragFixture 是一个 200×300 的滚动区域，里面 20 个 200×50 的格子，间距 10：第 i 个格子在内容的 60i 到 60i+50。
// 每个格子有自己的右键菜单，用来检查批量模式下右键不弹菜单。
type dragFixture struct {
	d       DragSelection
	sel     map[string]bool
	changes int
	scroll  ui.ScrollState
	enabled bool
	c       *ui.Context
	tt      *ui.Tester
}

func newDragFixture(t *testing.T) *dragFixture {
	t.Helper()
	f := &dragFixture{sel: map[string]bool{}, enabled: true}
	f.tt = ui.NewTester(func(c *ui.Context) {
		f.c = c
		ui.Scroll(c.Key("scroll")).Size(200, 300).TrackScroll(&f.scroll).Children(func() {
			surface := ui.Column(c.Key("surface")).Gap(10)
			f.d.Surface(c, surface, DragProps{
				Enabled:  f.enabled,
				Selected: func(id string) bool { return f.sel[id] },
				OnSelectChange: func(id string, selected bool) {
					f.changes++
					f.sel[id] = selected
				},
				Scroll: &f.scroll,
			})
			surface.Children(func() {
				for i := range 20 {
					id := strconv.Itoa(i)
					f.d.Target(c.Key(id), id, func(c *ui.Context) {
						ui.Box(c).Size(200, 50).ContextMenu(func(m *ui.Menu) { m.Item("menu") })
					})
				}
			})
		})
	}, 200, 300)
	f.tt.Frame()
	return f
}

// 指针事件的坐标相对 surface：没有滚动时就是窗口坐标加上滚动距离。直接调用 input 不经过引擎，
// 之后补一帧，如同引擎在接下事件后构建的那一帧。
func (f *dragFixture) event(kind ui.InputKind, y float32) bool {
	taken := f.d.input(f.c, ui.InputEvent{Kind: kind, Button: 1, X: 100, Y: y + f.scroll.Y})
	f.tt.Frame()
	return taken
}

func (f *dragFixture) expect(t *testing.T, want ...string) {
	t.Helper()
	n := 0
	for _, s := range f.sel {
		if s {
			n++
		}
	}
	for _, id := range want {
		if !f.sel[id] {
			t.Fatalf("%s not selected: %v", id, f.sel)
		}
	}
	if n != len(want) {
		t.Fatalf("selected %v, want %v", f.sel, want)
	}
}

func TestDragSelectRightPressTogglesStart(t *testing.T) {
	f := newDragFixture(t)
	f.tt.RightClickAt(100, 25)
	f.expect(t, "0")
	if m := f.tt.Menu(); m != nil {
		t.Fatalf("context menu %v in batch mode", m)
	}
	f.tt.RightClickAt(100, 25)
	f.expect(t)

	// 格子之间的空隙不起拖，右键照常。
	f.tt.RightClickAt(100, 55)
	f.expect(t)

	// 不在批量模式时不处理，右键菜单照常。
	f.enabled = false
	f.tt.Frame()
	f.tt.RightClickAt(100, 25)
	f.expect(t)
	if f.tt.Menu() == nil {
		t.Fatal("no context menu outside batch mode")
	}
}

func TestDragSelectSetsCardsPassed(t *testing.T) {
	f := newDragFixture(t)
	f.sel["2"] = true

	// 起点没有选中：经过的都选中，已经选中的保持选中。
	if !f.event(ui.InputPointerDown, 25) {
		t.Fatal("press on a card not taken")
	}
	f.event(ui.InputPointerMove, 55) // 空隙
	f.event(ui.InputPointerMove, 85)
	f.event(ui.InputPointerMove, 145)
	f.event(ui.InputPointerMove, 25) // 回到起点：每个格子只处理一次
	f.event(ui.InputPointerUp, 25)
	f.expect(t, "0", "1", "2")
	if f.changes != 3 {
		t.Fatalf("changes = %d, want 3", f.changes)
	}

	// 起点已经选中：经过的都取消。
	f.event(ui.InputPointerDown, 145)
	f.event(ui.InputPointerMove, 85)
	f.event(ui.InputPointerUp, 85)
	f.expect(t, "0")

	// 松开以后的移动不再选择。
	f.event(ui.InputPointerMove, 205)
	f.expect(t, "0")
}

func TestDragSelectAutoScroll(t *testing.T) {
	f := newDragFixture(t)
	f.event(ui.InputPointerDown, 280) // 第 4 个格子，离底边 20
	f.expect(t, "4")
	for range 4 {
		f.tt.Advance(17 * time.Millisecond)
	}
	if f.scroll.Y < 20 {
		t.Fatalf("scrolled %v, want the pointer over the next card", f.scroll.Y)
	}
	if !f.sel["5"] {
		t.Fatalf("card under the pointer after scrolling not selected: %v (scroll %v)", f.sel, f.scroll.Y)
	}

	// 松开后停止滚动。
	f.event(ui.InputPointerUp, 280)
	y := f.scroll.Y
	f.tt.Advance(100 * time.Millisecond)
	if f.scroll.Y != y {
		t.Fatalf("kept scrolling after release: %v → %v", y, f.scroll.Y)
	}

	// 指针不在边缘区域时不滚动。
	f.event(ui.InputPointerDown, 145)
	y = f.scroll.Y
	f.tt.Advance(100 * time.Millisecond)
	if f.scroll.Y != y {
		t.Fatalf("scrolled away from the edges: %v → %v", y, f.scroll.Y)
	}
}

func TestDragSelectEndsWhenBatchModeEnds(t *testing.T) {
	f := newDragFixture(t)
	f.event(ui.InputPointerDown, 25)
	f.enabled = false
	f.tt.Frame()
	if f.d.Active() {
		t.Fatal("drag still active after leaving batch mode")
	}
	f.enabled = true
	f.tt.Frame()
	f.event(ui.InputPointerMove, 85)
	f.expect(t, "0")
}
