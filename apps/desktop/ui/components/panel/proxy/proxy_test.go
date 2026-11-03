package proxy

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

type harness struct {
	cfg     appconf.AppConfig
	env     *env.Env
	state   State
	tt      *ui.Tester
	changes int
}

// newHarness 在真实服务上打开面板；Change 改本地的配置，外面套一层分区内容的 p-5 space-y-4。
func newHarness(t *testing.T, cfg func(*appconf.AppConfig)) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{cfg: *svc.Config}
	if cfg != nil {
		cfg(&h.cfg)
	}
	h.env = &env.Env{Svc: svc.API(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 800)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
	return h
}

func (h *harness) props() panel.Settings {
	return panel.Settings{
		Env:       h.env,
		Config:    &h.cfg,
		Change:    func(fn func(*appconf.AppConfig)) { fn(&h.cfg); h.changes++ },
		PatchLive: func(fn func(*appconf.AppConfig)) { fn(&h.cfg) },
		Refresh:   func() {},
	}
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	ui.Column(c).Fill().Background(t.AppBg).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(16).Children(func() {
			View(c, t, &h.state, h.props())
		})
	})
}

func (h *harness) frames(n int) {
	for range n {
		h.tt.Frame()
	}
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.frames(2)
}

func TestRendersDefaults(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.NetworkProxyMode, cfg.NetworkProxyURL = "", "" })
	for _, s := range []string{
		locales.SettingsProxyGlobalMode(), locales.SettingsProxyGlobalModeHint(), locales.SettingsProxyModeSystem(),
		locales.SettingsProxyManualProxyURL(), locales.SettingsProxyManualProxyURLHint(), locales.SettingsProxyManualProxyURLIdle(),
	} {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
	if _, ok := h.tt.Find(locales.SettingsProxyManualProxyURLPlaceholder()); !ok {
		t.Error("proxy URL input not labelled by its placeholder")
	}
}

// TestModeSelect：选择“手动代理”后写进草稿，“暂不生效”的说明消失。
func TestModeSelect(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.NetworkProxyMode = "direct" })
	h.click(t, locales.SettingsProxyModeDirect())
	h.click(t, locales.SettingsProxyModeManual())
	if h.cfg.NetworkProxyMode != "manual" || h.changes != 1 {
		t.Fatalf("mode %q after %d changes", h.cfg.NetworkProxyMode, h.changes)
	}
	if h.tt.HasText(locales.SettingsProxyManualProxyURLIdle()) {
		t.Error("idle hint still shown in manual mode")
	}
}

// TestProxyURLInput：输入的地址直接写进草稿。
func TestProxyURLInput(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.NetworkProxyMode, cfg.NetworkProxyURL = "manual", "" })
	h.click(t, locales.SettingsProxyManualProxyURLPlaceholder())
	h.tt.Type("127.0.0.1:7890")
	h.frames(2)
	if h.cfg.NetworkProxyURL != "127.0.0.1:7890" {
		t.Fatalf("proxy url %q", h.cfg.NetworkProxyURL)
	}
	if h.tt.HasText(locales.SettingsProxyManualProxyURLIdle()) {
		t.Error("idle hint shown in manual mode")
	}
}
