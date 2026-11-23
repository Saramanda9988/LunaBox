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
