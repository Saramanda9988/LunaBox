package appdata

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// fakeOpen 用真实的 ConfigService，只替换打开资源管理器。
type fakeOpen struct {
	api.ConfigService
	opened int
	err    error
	export string
}

func (f *fakeOpen) OpenDataDirectory() (string, error) {
	f.opened++
	return "", f.err
}

func (f *fakeOpen) ExportLogsZip() (string, error) { return f.export, nil }

type harness struct {
	env   *env.Env
	cfg   appconf.AppConfig
	state State
	tt    *ui.Tester
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{cfg: *svc.Config}
	h.env = &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 600)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.tt.Frame()
	return h
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	p := panel.Settings{
		Env:       h.env,
		Config:    &h.cfg,
		Change:    func(fn func(*appconf.AppConfig)) { fn(&h.cfg) },
		PatchLive: func(fn func(*appconf.AppConfig)) { fn(&h.cfg) },
		Refresh:   func() {},
	}
	ui.Column(c).Fill().Padding(20).Gap(16).Background(t.AppBg).Children(func() { View(c, t, &h.state, p) })
}

func (h *harness) click(t *testing.T, s string) {
	t.Helper()
	if err := h.tt.Click(s); err != nil {
		t.Fatalf("%v; texts %q", err, h.tt.Texts())
	}
	h.tt.Frame()
	h.tt.Frame()
}

func (h *harness) hasPrefix(prefix string) bool {
	for _, s := range h.tt.Texts() {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func TestRenderAndExportLogs(t *testing.T) {
	h := newHarness(t)
	for _, s := range []string{locales.SettingsAppDataLogsTitle(), locales.SettingsAppDataLogsHint(), locales.SettingsAppDataExportLogs(),
		locales.SettingsAppDataDataDirTitle(), locales.SettingsAppDataDataDirHint(), locales.SettingsAppDataOpenDataDir()} {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
	// 测试环境没有窗口，保存对话框返回错误。
	h.click(t, locales.SettingsAppDataExportLogs())
	if !h.hasPrefix(locales.SettingsAppDataToastExportLogsFailed("")) {
		t.Errorf("missing failure toast in %q", h.tt.Texts())
	}
	if h.state.exporting {
		t.Error("still exporting")
	}

	fake := &fakeOpen{ConfigService: h.env.Svc.AppConfig, export: `D:\logs.zip`}
	h.state.api = fake
	h.click(t, locales.SettingsAppDataExportLogs())
	if !h.tt.HasText(locales.SettingsAppDataToastExportLogsSuccess(`D:\logs.zip`)) {
		t.Errorf("missing success toast in %q", h.tt.Texts())
	}
}

func TestOpenDataDir(t *testing.T) {
	h := newHarness(t)
	fake := &fakeOpen{ConfigService: h.env.Svc.AppConfig}
	h.state.api = fake
	h.click(t, locales.SettingsAppDataOpenDataDir())
	if fake.opened != 1 || h.state.opening {
		t.Fatalf("opened %d, opening %v", fake.opened, h.state.opening)
	}
	fake.err = errors.New("denied")
	h.click(t, locales.SettingsAppDataOpenDataDir())
	if !h.tt.HasText(locales.SettingsAppDataToastOpenDataDirFailed("denied")) {
		t.Errorf("missing failure toast in %q", h.tt.Texts())
	}
}
