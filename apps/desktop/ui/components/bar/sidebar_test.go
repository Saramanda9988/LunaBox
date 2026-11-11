package bar

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

func newSidebarTester(s *Sidebar, w int) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		// 与根布局一样，侧边栏在一行里撑满高度。
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			SideBar(c, theme.New(false, 1), SideBarProps{State: s, Router: ui.NewRouter(env.PathHome)})
		})
	}, w, 800)
	tt.Frame()
	return tt
}

// toggle 点击标题栏的展开/收起按钮，等动画结束。
func toggle(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.SideBarToggle()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
}

// dragEdge 从侧边栏右边缘按下，水平拖动 dx 后松开。
func dragEdge(tt *ui.Tester, s *Sidebar, dx float32) {
	x := s.Width() - 2
	tt.Press(x, 400)
	tt.Move(x+dx/2, 400)
	tt.Move(x+dx, 400)
	tt.Frame()
	tt.Release(x+dx, 400)
	tt.Frame()
}

func TestSidebarResize(t *testing.T) {
	s := NewSidebar(true, 0)
	tt := newSidebarTester(&s, 1280)
	if s.Width() != sidebarOpenWidth {
		t.Fatalf("default width %v", s.Width())
	}

	dragEdge(tt, &s, 50)
	if s.Width() != sidebarOpenWidth+50 || s.OpenWidth() != sidebarOpenWidth+50 {
		t.Fatalf("width %v (open %d) after dragging 50 to the right", s.Width(), s.OpenWidth())
	}
	dragEdge(tt, &s, 500)
	if s.Width() != sidebarMaxWidth {
		t.Fatalf("width %v not clamped to the maximum", s.Width())
	}
	dragEdge(tt, &s, -500)
	if s.Width() != sidebarOpenWidth {
		t.Fatalf("width %v not clamped to the minimum", s.Width())
	}

	// 收起后再展开，回到调整过的宽度。
	dragEdge(tt, &s, 40)
	toggle(t, tt)
	if s.Width() != sidebarClosedWidth {
		t.Fatalf("collapsed width %v", s.Width())
	}
	dragEdge(tt, &s, 100) // 收起时没有拖动条
	toggle(t, tt)
	if s.Width() != sidebarOpenWidth+40 {
		t.Fatalf("reopened at %v, want %v", s.Width(), sidebarOpenWidth+40)
	}
}

func TestSidebarSavedWidth(t *testing.T) {
	// 配置里的宽度超出范围时限制在范围内。
	if s := NewSidebar(true, 1000); s.Width() != sidebarMaxWidth {
		t.Fatalf("saved 1000 opened at %v", s.Width())
	}
	if s := NewSidebar(true, 100); s.Width() != sidebarOpenWidth {
		t.Fatalf("saved 100 opened at %v", s.Width())
	}

	// 窗口太窄时让出主内容区，但不小于最小宽度；保存的宽度保留到窗口变宽。
	s := NewSidebar(true, 340)
	tt := newSidebarTester(&s, 900)
	tt.Advance(time.Second)
	tt.Frame()
	if want := float32(900 - sidebarMainMin); s.Width() != want {
		t.Fatalf("width %v in a 900 window, want %v", s.Width(), want)
	}
	if s.OpenWidth() != 340 {
		t.Fatalf("saved width changed to %d", s.OpenWidth())
	}
}
