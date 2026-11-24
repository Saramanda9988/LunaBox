package bar

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop/ui/theme"
)

func TestTopBarFadeAttenuatesTopAndPassesThrough(t *testing.T) {
	clicked := false
	const sidebarW = 200
	tt := ui.NewTester(func(c *ui.Context) {
		th := theme.New(true, 1)
		th.TopbarHidden = true
		ui.Box(c).Fill().Background(theme.White).Children(func() {
			ui.ButtonBase(c.Key("under")).Absolute().Top(24).Left(sidebarW + 20).Size(80, 24).
				Label("under").OnClick(func() { clicked = true })
		})
		TopBarFade(c, th, sidebarW)
	}, 400, 100)
	tt.Frame()

	img := tt.Image()
	// 侧边栏区域（x < sidebarW）不受 TopBarFade 影响。
	sidebarPx := img.RGBAAt(100, 10)
	if sidebarPx.R != 255 || sidebarPx.G != 255 || sidebarPx.B != 255 {
		t.Fatalf("sidebar area tinted: got %+v, want white", sidebarPx)
	}
	// 主区顶部按钮图标高度（y = 14）被暗色底色柔和压暗，避免亮封面冲淡窗口按钮。
	topPx := img.RGBAAt(sidebarW+50, 14)
	if topPx.R > 90 {
		t.Fatalf("top strip at y=14 not sufficiently attenuated: got %+v", topPx)
	}
	// 过渡区（y = 26）应比顶部更亮、比下方正文更暗，呈渐隐效果。
	midPx := img.RGBAAt(sidebarW+50, 26)
	if midPx.R <= topPx.R || midPx.R >= 250 {
		t.Fatalf("fade transition not monotonic: top=%d mid=%d", topPx.R, midPx.R)
	}
	// 32px 以下（页面 pagePadding 起始处及正文区域）完全不受影响。
	belowPx := img.RGBAAt(sidebarW+50, 40)
	if belowPx.R != 255 || belowPx.G != 255 || belowPx.B != 255 {
		t.Fatalf("content below fade strip altered: got %+v, want white", belowPx)
	}

	// PassThrough：渐隐层不应拦截下方元素点击。
	tt.ClickAt(sidebarW+40, 30)
	tt.Frame()
	if !clicked {
		t.Fatal("TopBarFade blocked click on element underneath")
	}
}
