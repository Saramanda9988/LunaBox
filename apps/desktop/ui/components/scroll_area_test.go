package components

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/ui/theme"
)

// TestScrollAreaFirstFrame：区域刚出现时还不知道是否超出，那一帧不绘制内容；画出来的每一帧都已经按滚动条
// 让出了宽度，不会先按没有滚动条排一次版、下一帧再变窄。
func TestScrollAreaFirstFrame(t *testing.T) {
	th := theme.New(false, 1)
	var sb ScrollBar
	show := false
	var painted []float32 // 每次绘制内容时用的右内边距
	tt := ui.NewTester(func(c *ui.Context) {
		if !show {
			return
		}
		gutter := float32(0)
		if sb.Overflowing() {
			gutter = th.Gutter()
		}
		ScrollArea(c, th, "area", &sb, func(w ui.Element) { w.Size(200, 100) }, func(sc ui.Element) {
			sc.Padding(0, gutter, 0, 0).Children(func() {
				ui.Box(c).Height(300).Draw(func(*ui.Painter, ui.Rect) { painted = append(painted, gutter) })
			})
		})
	}, 400, 300)
	tt.Frame()

	show = true
	tt.Frame()
	if !sb.Overflowing() {
		t.Fatal("overflow not measured")
	}
	if len(painted) == 0 {
		t.Fatal("content never painted")
	}
	for i, g := range painted {
		if g != th.Gutter() {
			t.Fatalf("paint %d used gutter %v, want %v (all: %v)", i, g, th.Gutter(), painted)
		}
	}
}
