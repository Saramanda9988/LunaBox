package plugins

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/api"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

type stubPluginService struct {
	api.PluginService
	list     []plugin.PluginInfo
	devPaths []string
	slots    []plugin.SlotRegistration
}

func (s *stubPluginService) List() []plugin.PluginInfo                  { return s.list }
func (s *stubPluginService) DevPaths() []string                         { return s.devPaths }
func (s *stubPluginService) Decorations() []plugin.DecorationRegistration { return nil }
func (s *stubPluginService) GetConfigValues(string) map[string]any        { return map[string]any{"focusMinutes": 25} }
func (s *stubPluginService) SetConfigValues(string, map[string]any) error { return nil }
func (s *stubPluginService) Slots(slotName string) []plugin.SlotRegistration {
	if slotName == "" {
		return s.slots
	}
	var out []plugin.SlotRegistration
	for _, sl := range s.slots {
		if sl.SlotName == slotName {
			out = append(out, sl)
		}
	}
	return out
}

func TestEmptyPluginsPage(t *testing.T) {
	e := &env.Env{
		Theme:     theme.New(false, 1),
		MainWidth: 960,
	}
	page := New(e)
	page.Open()

	tt := ui.NewTester(func(c *ui.Context) {
		page.View(c)
	}, 960, 720)
	tt.Frame()
	if _, ok := tt.Find(locales.NativeSideBarPlugins()); !ok {
		t.Fatalf("expected plugins page title %q", locales.NativeSideBarPlugins())
	}
	if _, ok := tt.Find(locales.NativePluginsEmpty()); !ok {
		t.Fatalf("expected empty state text %q", locales.NativePluginsEmpty())
	}
}

func TestPluginsListShowsLocalBadgeAndOmitsDevDirsSection(t *testing.T) {
	stub := &stubPluginService{
		list: []plugin.PluginInfo{
			{
				ID:          "pomodoro-overlay",
				Name:        "番茄钟悬浮窗",
				Description: "实时显示游戏游玩时长",
				Version:     "1.0.0",
				Icon:        "timer-outline",
				IsDev:       true,
				Enabled:     true,
				Status:      plugin.StatusActive,
			},
		},
		devPaths: []string{`D:\dev\pomodoro-overlay`},
	}
	e := &env.Env{
		Theme:     theme.New(false, 1),
		MainWidth: 960,
		Router:    ui.NewRouter(env.PathPlugins),
		Svc:       &api.Services{Plugin: stub},
	}
	page := New(e)
	page.Open()

	tt := ui.NewTester(func(c *ui.Context) {
		page.View(c)
	}, 960, 720)
	tt.Frame()
	if _, ok := tt.Find("番茄钟悬浮窗"); !ok {
		t.Fatalf("expected plugin name in list")
	}
	if _, ok := tt.Find(locales.NativePluginsStatusActive()); ok {
		t.Fatalf("did not expect status pill %q in list row", locales.NativePluginsStatusActive())
	}
	if _, ok := tt.Find(locales.NativePluginsLocalBadge()); !ok {
		t.Fatalf("expected local pill %q in list row for dev plugin", locales.NativePluginsLocalBadge())
	}
	if _, ok := tt.Find(`D:\dev\pomodoro-overlay`); ok {
		t.Fatalf("did not expect dev directory list on plugins page")
	}
}

func TestPluginsListInstalledOmitsLocalBadge(t *testing.T) {
	stub := &stubPluginService{
		list: []plugin.PluginInfo{
			{
				ID:      "installed-plugin",
				Name:    "已安装插件",
				Version: "1.0.0",
				IsDev:   false,
				Enabled: true,
				Status:  plugin.StatusActive,
			},
		},
	}
	e := &env.Env{
		Theme:     theme.New(false, 1),
		MainWidth: 960,
		Router:    ui.NewRouter(env.PathPlugins),
		Svc:       &api.Services{Plugin: stub},
	}
	page := New(e)
	page.Open()

	tt := ui.NewTester(func(c *ui.Context) {
		page.View(c)
	}, 960, 720)
	tt.Frame()
	if _, ok := tt.Find(locales.NativePluginsLocalBadge()); ok {
		t.Fatalf("did not expect local pill %q for installed plugin", locales.NativePluginsLocalBadge())
	}
}

func TestPluginDetailShowsSettingsAndSourceInfo(t *testing.T) {
	stub := &stubPluginService{
		list: []plugin.PluginInfo{
			{
				ID:          "pomodoro-overlay",
				Name:        "番茄钟悬浮窗",
				Description: "实时显示游戏游玩时长",
				Version:     "1.0.0",
				Author:      "LunaBox",
				Icon:        "timer-outline",
				IsDev:       true,
				Enabled:     true,
				Status:      plugin.StatusActive,
			},
		},
		slots: []plugin.SlotRegistration{
			{
				SlotName: plugin.SlotSettingsSection,
				PluginID: "pomodoro-overlay",
				ID:       "__config__",
				Title:    "番茄钟悬浮窗设置",
				ConfigSchema: &plugin.ConfigSchema{
					Fields: []plugin.ConfigFieldSchema{
						{Key: "focusMinutes", Type: "number", Label: "专注时长（分钟）", Default: 25},
					},
				},
			},
		},
	}
	e := &env.Env{
		Theme:     theme.New(false, 1),
		MainWidth: 960,
		Router:    ui.NewRouter(env.PluginDetailPath("pomodoro-overlay")),
		Svc:       &api.Services{Plugin: stub},
	}
	page := New(e)
	page.OpenDetail("pomodoro-overlay")

	tt := ui.NewTester(func(c *ui.Context) {
		page.ViewDetail(c, "pomodoro-overlay")
	}, 960, 720)
	tt.Frame()
	if _, ok := tt.Find(locales.CommonBack()); !ok {
		t.Fatalf("expected back button %q matching game detail", locales.CommonBack())
	}
	if _, ok := tt.Find("v1.0.0"); ok {
		t.Fatalf("did not expect redundant version pill in detail header")
	}
	if _, ok := tt.Find(locales.NativePluginsLocalBadge()); !ok {
		t.Fatalf("expected local pill %q in detail header for dev plugin", locales.NativePluginsLocalBadge())
	}
	if _, ok := tt.Find(locales.NativePluginsDetailSettings()); !ok {
		t.Fatalf("expected plugin settings heading %q", locales.NativePluginsDetailSettings())
	}
	if _, ok := tt.Find("专注时长（分钟）"); !ok {
		t.Fatalf("expected config field label in plugin detail page")
	}
	if _, ok := tt.Find(locales.NativePluginsDetailSourceInfo()); !ok {
		t.Fatalf("expected source info heading %q", locales.NativePluginsDetailSourceInfo())
	}
	if _, ok := tt.Find("LunaBox"); !ok {
		t.Fatalf("expected author in plugin detail page")
	}
}

func TestPluginsToolbarPlacedBelowTitleAlignedWithBody(t *testing.T) {
	stub := &stubPluginService{
		list: []plugin.PluginInfo{
			{
				ID:          "pomodoro-overlay",
				Name:        "番茄钟悬浮窗",
				Description: "实时显示游戏游玩时长",
				Version:     "1.0.0",
				Enabled:     true,
				Status:      plugin.StatusActive,
			},
		},
	}
	const mainW = 1400
	e := &env.Env{
		Theme:     theme.New(false, 1),
		MainWidth: mainW,
		Router:    ui.NewRouter(env.PathPlugins),
		Svc:       &api.Services{Plugin: stub},
	}
	page := New(e)
	page.Open()

	tt := ui.NewTester(func(c *ui.Context) {
		page.View(c)
	}, mainW, 720)
	tt.Frame()

	titleRect, ok := tt.Find(locales.NativeSideBarPlugins())
	if !ok {
		t.Fatalf("expected title %q", locales.NativeSideBarPlugins())
	}
	installRect, ok := tt.Find(locales.NativePluginsInstallZip())
	if !ok {
		t.Fatalf("expected install button %q", locales.NativePluginsInstallZip())
	}
	addDevRect, ok := tt.Find(locales.NativePluginsAddDevDir())
	if !ok {
		t.Fatalf("expected add-dev button %q", locales.NativePluginsAddDevDir())
	}
	openDirRect, ok := tt.Find(locales.NativePluginsOpenDir())
	if !ok {
		t.Fatalf("expected open-dir button %q", locales.NativePluginsOpenDir())
	}
	cardRect, ok := tt.Find("番茄钟悬浮窗")
	if !ok {
		t.Fatalf("expected plugin row in list")
	}

	// Toolbar buttons sit below the page title and above the plugin list.
	for name, r := range map[string]ui.Rect{
		"install": installRect,
		"add-dev": addDevRect,
		"open":    openDirRect,
	} {
		if r.Y < titleRect.Y+titleRect.H {
			t.Fatalf("%s button (y=%.1f) should be below title (bottom=%.1f)", name, r.Y, titleRect.Y+titleRect.H)
		}
		if r.Y+r.H > cardRect.Y {
			t.Fatalf("%s button (bottom=%.1f) should be above plugin list (y=%.1f)", name, r.Y+r.H, cardRect.Y)
		}
	}

	// On wide viewports where body is centered (listMaxW < contentW), toolbar aligns with body rather than full page edges.
	contentW := mainW - 2*pagePadding - e.Theme.Gutter()
	bodyLeft := pagePadding + (contentW-listMaxW)/2
	bodyRight := bodyLeft + listMaxW
	if installRect.X < bodyLeft-1 {
		t.Fatalf("install button (x=%.1f) should align inside centered body left (%.1f)", installRect.X, bodyLeft)
	}
	if openDirRect.X+openDirRect.W > bodyRight+1 {
		t.Fatalf("open-dir button (right=%.1f) should align inside centered body right (%.1f)", openDirRect.X+openDirRect.W, bodyRight)
	}
}
