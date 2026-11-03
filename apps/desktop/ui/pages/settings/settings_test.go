package settings

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

type testPage struct {
	*Page
	svc   *host.Backend
	tt    *ui.Tester
	saved []appconf.AppConfig // ConfigSaved 收到的配置
}

func newTestPage(t *testing.T) *testPage {
	t.Helper()
	svc := hosttest.NewServices(t)
	tp := &testPage{svc: svc}
	e := &env.Env{Backend: envtest.NewBackend(0), Svc: svc.API(), Router: ui.NewRouter(env.PathSettings), Covers: covers.New(),
		Theme: theme.New(true, 1.5), MainWidth: 1024}
	e.ConfigSaved = func(_, next appconf.AppConfig) { tp.saved = append(tp.saved, next) }
	tp.Page = New(e)
	tp.Open()
	tp.tt = ui.NewTester(tp.View, 1280, 800)
	tp.tt.SetScale(1.5)
	tp.tt.SetDark(true)
	tp.frames(3)
	return tp
}

func (tp *testPage) frames(n int) {
	for range n {
		tp.tt.Frame()
	}
}

func (tp *testPage) expectTexts(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !tp.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tp.tt.Texts())
		}
	}
}

func TestSectionsAndFooter(t *testing.T) {
	tp := newTestPage(t)
	tp.expectTexts(t, locales.SettingsTitle(), locales.SettingsSectionsBasic(), locales.SettingsSectionsMetadata(),
		locales.SettingsSectionsAppData(), locales.SettingsGithub(), "Lunabox made with LunaRain_079 & Contributors.")
	// 便携版与 AppImage 才有“本地集成设置”；没有用 ldflags 指定时 version.BuildMode 是 portable。
	mode := tp.version["buildMode"]
	if got, want := tp.tt.HasText(locales.SettingsSectionsPortableSetup()), mode == "portable" || mode == "appimage"; got != want {
		t.Errorf("build mode %q: portable setup shown %v, want %v", mode, got, want)
	}
	if v := tp.version; v == nil || !tp.tt.HasText("Version "+v["version"]+" ("+v["commit"]+") | "+v["buildMode"]+" | Built at"+v["buildTime"]) {
		t.Errorf("version line missing: %v in %q", v, tp.tt.Texts())
	}
	tp.tt.SetSize(1280, 4000) // 页脚在一屏以下
	tp.frames(2)
	if err := tp.tt.Click(locales.SettingsGithub()); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if urls := tp.tt.OpenedURLs(); len(urls) != 1 || urls[0] != githubURL {
		t.Errorf("opened %v", urls)
	}
}

func TestSectionsToggle(t *testing.T) {
	tp := newTestPage(t)
	if !tp.open[sectionBasic] || tp.open[sectionMetadata] {
		t.Fatalf("default open states %v", tp.open)
	}
	tp.tt.SetSize(1280, 4000) // 基础设置展开后，下面的分区在一屏以下
	tp.frames(2)
	if err := tp.tt.Click(locales.SettingsSectionsMetadata()); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if !tp.open[sectionMetadata] {
		t.Error("clicking the header opens the section")
	}
	if err := tp.tt.Click(locales.SettingsSectionsBasic()); err != nil {
		t.Fatal(err)
	}
	tp.frames(1)
	if tp.open[sectionBasic] {
		t.Error("clicking the header closes the section")
	}
	tp.Open()
	if !tp.open[sectionBasic] || tp.open[sectionMetadata] {
		t.Errorf("entering the page again resets the sections: %v", tp.open)
	}
}

// TestDraftSavedAfterDelay：草稿停止修改 250ms 后保存一次，保存后生效的配置与草稿相同。
func TestDraftSavedAfterDelay(t *testing.T) {
	tp := newTestPage(t)
	props := tp.props()
	props.Change(func(cfg *appconf.AppConfig) { cfg.NetworkProxyURL = "127.0.0.1:7890" })
	tp.frames(1)
	tp.tt.Advance(200 * time.Millisecond)
	props.Change(func(cfg *appconf.AppConfig) { cfg.NetworkProxyMode = "manual" })
	tp.frames(1)
	tp.tt.Advance(200 * time.Millisecond)
	tp.frames(1)
	if len(tp.saved) != 0 {
		t.Fatalf("saved before the delay: %d", len(tp.saved))
	}
	tp.tt.Advance(100 * time.Millisecond)
	tp.frames(2)
	if len(tp.saved) != 1 {
		t.Fatalf("saved %d times", len(tp.saved))
	}
	cfg := tp.svc.Config
	if cfg.NetworkProxyURL != "127.0.0.1:7890" || cfg.NetworkProxyMode != "manual" {
		t.Errorf("config not saved: %q %q", cfg.NetworkProxyURL, cfg.NetworkProxyMode)
	}
	if tp.dirty {
		t.Error("draft still dirty after saving")
	}
}

// TestPatchLive：patchLiveConfig 立即保存，不等草稿的计时。
func TestPatchLive(t *testing.T) {
	tp := newTestPage(t)
	tp.props().PatchLive(func(cfg *appconf.AppConfig) { cfg.WindowZoomFactor = 1.25 })
	if tp.svc.Config.WindowZoomFactor != 1.25 || tp.draft.WindowZoomFactor != 1.25 {
		t.Errorf("zoom %v / draft %v", tp.svc.Config.WindowZoomFactor, tp.draft.WindowZoomFactor)
	}
	if len(tp.saved) != 1 || tp.dirty {
		t.Errorf("saved %d, dirty %v", len(tp.saved), tp.dirty)
	}
}

// TestApplyZoom：缩放快捷键保存后，草稿里的缩放跟着更新，草稿里其余还没保存的修改照常保存，不把缩放覆盖回旧值。
func TestApplyZoom(t *testing.T) {
	tp := newTestPage(t)
	tp.props().Change(func(cfg *appconf.AppConfig) { cfg.NetworkProxyURL = "127.0.0.1:7890" })
	tp.svc.Config.WindowZoomFactor = 1.25 // App 已经用 PatchConfig 保存
	tp.ApplyZoom(1.25)
	tp.frames(1)
	tp.tt.Advance(300 * time.Millisecond)
	tp.frames(2)
	if cfg := tp.svc.Config; cfg.WindowZoomFactor != 1.25 || cfg.NetworkProxyURL != "127.0.0.1:7890" {
		t.Errorf("zoom %v, proxy %q", cfg.WindowZoomFactor, cfg.NetworkProxyURL)
	}
}

// TestSaveFailureRevertsDraft：保存失败（这里是空的主题）时提示，草稿恢复成生效的配置。
func TestSaveFailureRevertsDraft(t *testing.T) {
	tp := newTestPage(t)
	theme := tp.svc.Config.Theme
	tp.props().Change(func(cfg *appconf.AppConfig) { cfg.Theme = "" })
	tp.frames(1)
	tp.tt.Advance(300 * time.Millisecond)
	tp.frames(2)
	if tp.draft.Theme != theme || tp.svc.Config.Theme != theme {
		t.Errorf("draft %q, config %q, want %q", tp.draft.Theme, tp.svc.Config.Theme, theme)
	}
	if len(tp.saved) != 0 {
		t.Errorf("ConfigSaved called after a failed save")
	}
}

// TestFlushSavesPendingDraft：离开页面时补存还没保存的草稿。
func TestFlushSavesPendingDraft(t *testing.T) {
	tp := newTestPage(t)
	tp.props().Change(func(cfg *appconf.AppConfig) { cfg.NetworkProxyURL = "socks5://127.0.0.1:1080" })
	tp.Flush()
	if tp.svc.Config.NetworkProxyURL != "socks5://127.0.0.1:1080" {
		t.Errorf("proxy %q", tp.svc.Config.NetworkProxyURL)
	}
}
