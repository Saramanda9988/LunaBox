package portable

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// fakeSetup 不碰注册表与 PATH：GetStatus 返回给定的状态，注册操作只记下调用。
type fakeSetup struct {
	status  service.PortableSetupStatus
	loadErr error
	actErr  error
	calls   []string
}

func (f *fakeSetup) GetStatus() (service.PortableSetupStatus, error) { return f.status, f.loadErr }

func (f *fakeSetup) act(name string, edit func(*service.PortableSetupStatus)) (service.PortableSetupStatus, error) {
	f.calls = append(f.calls, name)
	if f.actErr != nil {
		return service.PortableSetupStatus{}, f.actErr
	}
	edit(&f.status)
	return f.status, nil
}

func (f *fakeSetup) RegisterProtocol() (service.PortableSetupStatus, error) {
	return f.act("registerProtocol", func(s *service.PortableSetupStatus) {
		s.Protocol.Registered, s.Protocol.UpToDate, s.Protocol.RegisteredPath = true, true, s.ExecutablePath
	})
}

func (f *fakeSetup) UnregisterProtocol() (service.PortableSetupStatus, error) {
	return f.act("unregisterProtocol", func(s *service.PortableSetupStatus) { s.Protocol = service.PortableProtocolStatus{} })
}

func (f *fakeSetup) RegisterCLIPath() (service.PortableSetupStatus, error) {
	return f.act("registerCli", func(s *service.PortableSetupStatus) { s.CLI.Registered = true })
}

func (f *fakeSetup) UnregisterCLIPath() (service.PortableSetupStatus, error) {
	return f.act("unregisterCli", func(s *service.PortableSetupStatus) { s.CLI.Registered = false })
}

type harness struct {
	env   *env.Env
	cfg   appconf.AppConfig
	state State
	tt    *ui.Tester
}

func newHarness(t *testing.T, api setupAPI) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{cfg: *svc.Config}
	h.env = &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.state.api = api
	h.tt = ui.NewTester(h.view, 1024, 900)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.tt.Frame()
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

func (h *harness) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !h.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, h.tt.Texts())
		}
	}
}

func (h *harness) absent(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if h.tt.HasText(s) {
			t.Errorf("unexpected %q", s)
		}
	}
}

// 真实的 GetStatus 只读注册表与文件，不写任何东西。
func TestRealStatus(t *testing.T) {
	h := newHarness(t, nil)
	if h.state.loading || h.state.status == nil {
		t.Fatalf("status not loaded: %+v; texts %q", h.state, h.tt.Texts())
	}
	st := h.state.status
	h.expect(t, locales.SettingsPortableSetupProtocolTitle(), locales.SettingsPortableSetupCliTitle(),
		locales.SettingsPortableSetupCurrentPathLabel(), st.ExecutablePath)
}

func TestStatusVariantsAndActions(t *testing.T) {
	fake := &fakeSetup{status: service.PortableSetupStatus{
		Platform: "windows", ExecutablePath: `D:\LunaBox\lunabox.exe`,
		Protocol: service.PortableProtocolStatus{Registered: true, RegisteredPath: `C:\Old\lunabox.exe`},
		CLI:      service.PortableCLIStatus{Available: true, CLIPath: `D:\LunaBox\lunacli.exe`, InstallPath: `C:\Users\u\bin\lunacli.exe`},
	}}
	h := newHarness(t, fake)
	h.expect(t, locales.SettingsPortableSetupDescription(), locales.SettingsPortableSetupStatusStalePath(), `C:\Old\lunabox.exe`,
		`D:\LunaBox\lunabox.exe`, locales.SettingsPortableSetupReregisterProtocol(), locales.SettingsPortableSetupUnregisterProtocol(),
		locales.SettingsPortableSetupStatusNotRegistered(), `D:\LunaBox\lunacli.exe`, `C:\Users\u\bin\lunacli.exe`,
		locales.SettingsPortableSetupRegisterCli(), locales.SettingsPortableSetupPathReopenHint())
	h.absent(t, locales.SettingsPortableSetupCliMissingHint(), locales.SettingsPortableSetupUnregisterCli())

	h.click(t, locales.SettingsPortableSetupReregisterProtocol())
	h.expect(t, locales.SettingsPortableSetupToastProtocolRegistered(), locales.SettingsPortableSetupStatusRegistered(),
		locales.SettingsPortableSetupRegisterProtocol())
	h.click(t, locales.SettingsPortableSetupRegisterCli())
	h.expect(t, locales.SettingsPortableSetupToastCliRegistered(), locales.SettingsPortableSetupReregisterCli(),
		locales.SettingsPortableSetupUnregisterCli())
	h.click(t, locales.SettingsPortableSetupUnregisterCli())
	h.expect(t, locales.SettingsPortableSetupToastCliUnregistered())

	fake.actErr = errors.New("access denied")
	h.click(t, locales.SettingsPortableSetupUnregisterProtocol())
	h.expect(t, locales.SettingsPortableSetupToastProtocolUnregisterFailed("access denied"), locales.SettingsPortableSetupUnregisterProtocol())
	if want := []string{"registerProtocol", "registerCli", "unregisterCli", "unregisterProtocol"}; len(fake.calls) != len(want) {
		t.Errorf("calls = %q, want %q", fake.calls, want)
	}
	if h.state.busy != "" {
		t.Errorf("busy = %q", h.state.busy)
	}
}

func TestMissingCLIAndMac(t *testing.T) {
	fake := &fakeSetup{status: service.PortableSetupStatus{Platform: "windows", ExecutablePath: `D:\x.exe`}}
	h := newHarness(t, fake)
	h.expect(t, locales.SettingsPortableSetupStatusCliMissing(), locales.SettingsPortableSetupCliMissingHint(), "-",
		locales.SettingsPortableSetupRegisterProtocol())
	h.absent(t, locales.SettingsPortableSetupUnregisterProtocol(), locales.SettingsPortableSetupRegisteredPathLabel())
	// 没有 lunacli 时“添加到 PATH”禁用。
	h.click(t, locales.SettingsPortableSetupRegisterCli())
	if len(fake.calls) != 0 {
		t.Fatalf("disabled button ran %q", fake.calls)
	}

	mac := &fakeSetup{status: service.PortableSetupStatus{Platform: "darwin", ExecutablePath: "/Applications/LunaBox.app",
		Protocol: service.PortableProtocolStatus{Registered: true, UpToDate: true, RegisteredPath: "LaunchServices / Info.plist"},
		CLI:      service.PortableCLIStatus{Available: true, Registered: true, CLIPath: "/Applications/LunaBox.app/bin/lunacli"}}}
	h = newHarness(t, mac)
	h.expect(t, locales.SettingsPortableSetupDescriptionMac(), locales.SettingsPortableSetupProtocolHintMac(),
		locales.SettingsPortableSetupCliHintMac(), locales.SettingsPortableSetupPathReopenHintMac(), "LaunchServices / Info.plist")
	h.absent(t, locales.SettingsPortableSetupRegisterProtocol(), locales.SettingsPortableSetupUnregisterProtocol())
}

func TestLoadFailed(t *testing.T) {
	h := newHarness(t, &fakeSetup{loadErr: errors.New("registry unavailable")})
	h.expect(t, locales.SettingsPortableSetupLoading(), locales.SettingsPortableSetupToastLoadFailed("registry unavailable"))
}
