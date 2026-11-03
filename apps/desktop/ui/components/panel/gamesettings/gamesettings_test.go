package gamesettings

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

const width = 1024

type harness struct {
	cfg     appconf.AppConfig
	env     *env.Env
	state   State
	tt      *ui.Tester
	changes int
}

// newHarness 在真实服务上打开面板；goos 不为空时代替 GetGOOS（测 macOS / Linux 的分支）。Change 改本地的配置。
func newHarness(t *testing.T, goos string, mute bool, cfg func(*appconf.AppConfig)) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{cfg: *svc.Config}
	if cfg != nil {
		cfg(&h.cfg)
	}
	if goos != "" {
		h.state = State{probed: true, goos: goos, muteSupported: mute}
	}
	h.env = &env.Env{Svc: svc.API(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: width}
	h.tt = ui.NewTester(h.view, width, 2000)
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

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) expectNot(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if h.tt.HasText(s) {
			t.Errorf("unexpected %q", s)
		}
	}
}

// clickSwitch 点击 label 那一行右侧的开关（分区内容 p-5，开关 w-11，与左侧的文字垂直居中）。
func (h *harness) clickSwitch(t *testing.T, label, hint string) {
	t.Helper()
	r, ok := h.tt.Find(label)
	if !ok {
		t.Fatalf("missing %q", label)
	}
	top, bottom := r.Y, r.Y+r.H
	if hint != "" {
		hr, ok := h.tt.Find(hint)
		if !ok {
			t.Fatalf("missing %q", hint)
		}
		bottom = hr.Y + hr.H
	}
	h.tt.ClickAt(width-20-22, (top+bottom)/2)
	h.frames(2)
}

// stubPicker 替换文件对话框，记录调用。
func stubPicker(t *testing.T, path string, err error) *[]bool {
	t.Helper()
	var calls []bool
	old := pickFile
	pickFile = func(_ *env.Env, wine bool, _ string) (string, error) {
		calls = append(calls, wine)
		return path, err
	}
	t.Cleanup(func() { pickFile = old })
	return &calls
}

func TestProbeUsesVersionService(t *testing.T) {
	h := newHarness(t, "", false, nil)
	if !h.state.probed || h.state.goos != h.env.Svc.Version.GetGOOS() ||
		h.state.muteSupported != h.env.Svc.Version.SupportsBackgroundProcessMute() {
		t.Errorf("probe %+v", h.state)
	}
}

func TestWindowsLayout(t *testing.T) {
	h := newHarness(t, "windows", true, func(cfg *appconf.AppConfig) { cfg.ProcessDetectionTimeoutSec = 0 })
	h.expect(t, locales.SettingsGameRecordActiveOnly(), locales.SettingsGameRecordActiveOnlyHint(),
		locales.SettingsGameMuteInBackground(), locales.SettingsGameMuteInBackgroundHint(),
		locales.SettingsGameProcessDetectionTimeout(), locales.SettingsGameProcessDetectionTimeoutMinutes("1"),
		locales.SettingsGameProcessDetectionTimeoutHint(), locales.SettingsGameWarningTitle(),
		locales.SettingsGameProcessDetectionTimeoutWarning(), locales.SettingsGameLaunchTools(),
		locales.SettingsGameLePath(), locales.SettingsGameLePathHint(), locales.SettingsGameDefaultUseLe(),
		locales.SettingsGameMagpiePath(), locales.SettingsGameMagpiePathHint(), locales.SettingsGameDefaultUseMagpie())
	h.expectNot(t, locales.SettingsGameWineRunnerPath(), locales.SettingsGameCrossoverRunnerPath(), locales.SettingsGameTricksTools())
	if h.tt.HasText(locales.SettingsGameMuteInBackgroundHintMacOS()) {
		t.Error("macOS mute hint on Windows")
	}
}

func TestMuteHiddenWhenUnsupported(t *testing.T) {
	h := newHarness(t, "windows", false, nil)
	h.expectNot(t, locales.SettingsGameMuteInBackground())
}

func TestSwitches(t *testing.T) {
	h := newHarness(t, "windows", true, func(cfg *appconf.AppConfig) {
		cfg.RecordActiveTimeOnly, cfg.MuteGameInBackground, cfg.DefaultUseLocaleEmulator, cfg.DefaultUseMagpie = false, false, false, false
	})
	h.clickSwitch(t, locales.SettingsGameRecordActiveOnly(), locales.SettingsGameRecordActiveOnlyHint())
	h.clickSwitch(t, locales.SettingsGameMuteInBackground(), locales.SettingsGameMuteInBackgroundHint())
	h.clickSwitch(t, locales.SettingsGameDefaultUseLe(), "")
	h.clickSwitch(t, locales.SettingsGameDefaultUseMagpie(), "")
	c := h.cfg
	if !c.RecordActiveTimeOnly || !c.MuteGameInBackground || !c.DefaultUseLocaleEmulator || !c.DefaultUseMagpie || h.changes != 4 {
		t.Fatalf("switches %v %v %v %v after %d changes", c.RecordActiveTimeOnly, c.MuteGameInBackground,
			c.DefaultUseLocaleEmulator, c.DefaultUseMagpie, h.changes)
	}
	// 点 label 本身不切换（原版的 label 没有 htmlFor）。
	h.click(t, locales.SettingsGameRecordActiveOnly())
	if !h.cfg.RecordActiveTimeOnly {
		t.Error("clicking the label toggled the switch")
	}
	h.clickSwitch(t, locales.SettingsGameRecordActiveOnly(), locales.SettingsGameRecordActiveOnlyHint())
	if h.cfg.RecordActiveTimeOnly {
		t.Error("switch did not turn off")
	}
}

func TestTimeoutSelect(t *testing.T) {
	h := newHarness(t, "windows", false, func(cfg *appconf.AppConfig) { cfg.ProcessDetectionTimeoutSec = 120 })
	h.click(t, locales.SettingsGameProcessDetectionTimeoutMinutes("2"))
	h.expect(t, locales.SettingsGameProcessDetectionTimeoutMinutes("10"))
	h.click(t, locales.SettingsGameProcessDetectionTimeoutMinutes("5"))
	if h.cfg.ProcessDetectionTimeoutSec != 300 {
		t.Fatalf("timeout %d", h.cfg.ProcessDetectionTimeoutSec)
	}
}

func TestSelectExecutable(t *testing.T) {
	h := newHarness(t, "windows", false, func(cfg *appconf.AppConfig) { cfg.LocaleEmulatorPath, cfg.MagpiePath = "", "" })
	calls := stubPicker(t, `C:\LE\LEProc.exe`, nil)
	h.click(t, locales.SettingsGameSelectBtn()) // 第一个是 Locale Emulator
	if h.cfg.LocaleEmulatorPath != `C:\LE\LEProc.exe` || len(*calls) != 1 || (*calls)[0] {
		t.Fatalf("le path %q, calls %v", h.cfg.LocaleEmulatorPath, *calls)
	}

	// 取消选择（空路径）不改动。
	stubPicker(t, "", nil)
	h.cfg.LocaleEmulatorPath = "keep"
	h.frames(1)
	h.click(t, locales.SettingsGameSelectBtn())
	if h.cfg.LocaleEmulatorPath != "keep" {
		t.Errorf("cancelled selection changed the path to %q", h.cfg.LocaleEmulatorPath)
	}

	// 失败时提示。
	stubPicker(t, "", errors.New("boom"))
	h.click(t, locales.SettingsGameSelectBtn())
	h.frames(1)
	h.expect(t, locales.SettingsGameToastLeSelectFailed())
}

func TestTypePath(t *testing.T) {
	h := newHarness(t, "windows", false, func(cfg *appconf.AppConfig) { cfg.MagpiePath = "" })
	h.click(t, locales.SettingsGameMagpiePathPlaceholder())
	h.tt.Type(`D:\Magpie.exe`)
	h.frames(2)
	if h.cfg.MagpiePath != `D:\Magpie.exe` {
		t.Fatalf("magpie path %q", h.cfg.MagpiePath)
	}
}

func TestLinuxLayout(t *testing.T) {
	h := newHarness(t, "linux", false, func(cfg *appconf.AppConfig) { cfg.WineRunnerPath, cfg.WinePrefix = "", "" })
	h.expect(t, locales.SettingsGameWineRunnerPath(), locales.SettingsGameWineRunnerPathHint(), locales.SettingsGameWinePrefix(),
		locales.SettingsGameWinePrefixHint(), locales.SettingsGameTricksTools(), locales.SettingsGameWinetricksPath(),
		locales.SettingsGameProtontricksPath())
	h.expectNot(t, locales.SettingsGameLePath(), locales.SettingsGameMagpiePath(), locales.SettingsGameCrossoverRunnerPath())

	calls := stubPicker(t, "/usr/bin/wine", nil)
	h.click(t, locales.SettingsGameSelectBtn())
	if h.cfg.WineRunnerPath != "/usr/bin/wine" || len(*calls) != 1 || !(*calls)[0] {
		t.Fatalf("wine path %q, calls %v", h.cfg.WineRunnerPath, *calls)
	}
	h.click(t, locales.SettingsGameWinePrefixPlaceholder())
	h.tt.Type("/home/me/.wine")
	h.frames(2)
	if h.cfg.WinePrefix != "/home/me/.wine" {
		t.Fatalf("prefix %q", h.cfg.WinePrefix)
	}
}

func TestDarwinLayout(t *testing.T) {
	h := newHarness(t, "darwin", true, func(cfg *appconf.AppConfig) { cfg.CrossOverBottle = "" })
	h.expect(t, locales.SettingsGameMuteInBackgroundHintMacOS(), locales.SettingsGameWineRunnerPath(),
		locales.SettingsGameCrossoverRunnerPath(), locales.SettingsGameCrossoverRunnerPathHint(),
		locales.SettingsGameCrossoverBottle(), locales.SettingsGameCrossoverBottleHint())
	h.expectNot(t, locales.SettingsGameTricksTools(), locales.SettingsGameLePath())
	h.click(t, locales.SettingsGameCrossoverBottlePlaceholder())
	h.tt.Type("Steam")
	h.frames(2)
	if h.cfg.CrossOverBottle != "Steam" {
		t.Fatalf("bottle %q", h.cfg.CrossOverBottle)
	}
}
