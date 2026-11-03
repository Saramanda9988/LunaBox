package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/locales"
)

// newTestApp 从游戏库开始（这些用例测的是游戏库与从它出发的导航）；默认的首页见 TestDefaultRouteIsHome。
func newTestApp(t *testing.T, n int) (*App, *ui.Tester) {
	t.Helper()
	return newTestAppWith(t, n, Options{Route: env.PathLibrary})
}

func newTestAppWith(t *testing.T, n int, opts Options) (*App, *ui.Tester) {
	t.Helper()
	b := envtest.NewBackend(n)
	svc := newServices(t)
	for _, g := range b.Games {
		hosttest.AddGame(t, svc, g)
	}
	a := New(b, svc.API(), opts)
	a.SetScale(1.5)
	a.library.Reload()
	tt := ui.NewTester(a.View, 1280, 800)
	tt.SetScale(1.5)
	tt.SetDark(true)
	for range 3 {
		tt.Frame()
	}
	return a, tt
}

func TestDefaultRouteIsHome(t *testing.T) {
	a, tt := newTestAppWith(t, 3, Options{})
	if got := a.router.Path(); got != env.PathHome {
		t.Fatalf("default route = %q", got)
	}
	// 测试数据没有游玩记录。
	if !tt.HasText(locales.HomeWelcome()) || !tt.HasText(locales.HomeNoPlayRecord()) {
		t.Errorf("home page not shown: %q", tt.Texts())
	}
	if err := tt.Click(locales.HomeBrowseLibrary()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.router.Path() != env.PathLibrary || !tt.HasText("共 3 个游戏") {
		t.Errorf("browse library: path %q, texts %q", a.router.Path(), tt.Texts())
	}
}

func TestSidebarToggle(t *testing.T) {
	a, tt := newTestApp(t, 3)
	if err := tt.Click("切换侧边栏"); err != nil {
		t.Fatal(err)
	}
	if a.sidebar.Open {
		t.Fatal("sidebar still open after toggle")
	}
}

func TestSidebarNavigates(t *testing.T) {
	a, tt := newTestApp(t, 3)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := a.router.Path(); got != env.PathSettings {
		t.Fatalf("path after clicking 设置 = %q", got)
	}
	if !tt.HasText(locales.SettingsSectionsBasic()) {
		t.Errorf("settings page not shown: %q", tt.Texts())
	}
	a.router.Push(env.PathStats)
	tt.Frame()
	if !tt.HasText(locales.StatsTitle()) || !tt.HasText(locales.StatsSummaryTotalPlayCount()) {
		t.Errorf("stats page not shown: %q", tt.Texts())
	}
	a.router.Back()
	tt.Frame()
	a.router.Back()
	tt.Frame()
	if !tt.HasText("游戏库") || a.router.Path() != env.PathLibrary {
		t.Errorf("back did not return to the library: %q", a.router.Path())
	}
}

func TestDownloadsRoute(t *testing.T) {
	a, tt := newTestApp(t, 3)
	a.router.Push(env.PathDownloads)
	tt.Frame()
	if !tt.HasText(locales.DownloadsTitle()) || !tt.HasText(locales.DownloadsEmpty()) {
		t.Errorf("downloads page not shown: %q", tt.Texts())
	}
}

func TestGameDetailNavigation(t *testing.T) {
	a, tt := newTestApp(t, 3)
	a.router.Push(env.GamePath("1"))
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Summer Pockets") || !tt.HasText("游戏统计") {
		t.Fatalf("detail page not shown: %q", tt.Texts())
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.router.Path() != env.PathLibrary {
		t.Fatalf("back went to %q", a.router.Path())
	}

	// 再次进入时重新加载；点击开发商回到游戏库并按它搜索。
	a.router.Push(env.GamePath("2"))
	tt.Frame()
	tt.Frame()
	if !tt.HasText("星空鉄道とシロの旅") {
		t.Fatalf("second detail page not shown: %q", tt.Texts())
	}
	if err := tt.Click("ゆずソフト"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.router.Path() != env.PathLibrary || !tt.HasText("共 0 个游戏") {
		t.Errorf("company link: path %q, texts %q", a.router.Path(), tt.Texts())
	}
}

// DESKTOP_MYGO_LAYOUT=1 时打印关键文字的位置（DIP），用于与 dump-styles.js 导出的原版坐标对照；
// DESKTOP_MYGO_PNG=<目录> 时另存一张 Tester 渲染的截图。
func TestLayoutDump(t *testing.T) {
	if os.Getenv("DESKTOP_MYGO_LAYOUT") == "" {
		t.Skip("DESKTOP_MYGO_LAYOUT not set")
	}
	_, tt := newTestApp(t, 12)
	for _, s := range []string{"游戏库", "共 12 个游戏", "首页", "筛选", "添加游戏", "橘光柠水随想曲", "ゆずソフト", "2026/09/11"} {
		r, ok := tt.Find(s)
		t.Logf("%-16s found=%v x=%.1f y=%.1f w=%.1f h=%.1f", s, ok, r.X, r.Y, r.W, r.H)
	}
	if dir := os.Getenv("DESKTOP_MYGO_PNG"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "tester.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, tt.Image())
	}
}

func TestCategoriesNavigation(t *testing.T) {
	a, tt := newTestApp(t, 3)
	if err := tt.Click("收藏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.router.Path() != env.PathCategories || !tt.HasText("最喜欢的游戏") || !tt.HasText("新建收藏夹") {
		t.Fatalf("categories page not shown at %q: %q", a.router.Path(), tt.Texts())
	}
	if err := tt.Click("最喜欢的游戏"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !tt.HasText("系统") || !tt.HasText("这个收藏夹还没有游戏") {
		t.Fatalf("category detail not shown at %q: %q", a.router.Path(), tt.Texts())
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.router.Path() != env.PathCategories {
		t.Fatalf("back went to %q", a.router.Path())
	}
}
