package update

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/service"
	"lunabox/desktop/host"
	"lunabox/desktop/host/hosttest"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/panel"
	"lunabox/desktop/ui/covers"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/locales"
	"lunabox/desktop/ui/theme"
)

// fakeCheck 用真实的 UpdateService，只替换联网的检查与应用内更新。
type fakeCheck struct {
	*service.UpdateService
	result *service.UpdateCheckResult
	err    error
	calls  int

	// DownloadAndApplyUpdate 依次发出 progress 里的进度，再返回 applyResult、applyErr。
	progress    []service.UpdateProgress
	applyResult *service.UpdateApplyResult
	applyErr    error
	applied     []string
}

func (f *fakeCheck) CheckForUpdates() (*service.UpdateCheckResult, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeCheck) DownloadAndApplyUpdate(manifestURL string) (*service.UpdateApplyResult, error) {
	f.applied = append(f.applied, manifestURL)
	for _, p := range f.progress {
		components.UpdateProgressEvent(p, time.Now())
	}
	return f.applyResult, f.applyErr
}

type harness struct {
	svc     *host.Backend
	env     *env.Env
	cfg     appconf.AppConfig
	changes int
	state   State
	fake    *fakeCheck
	tt      *ui.Tester
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t)}
	h.cfg = *h.svc.Config
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.fake = &fakeCheck{UpdateService: h.svc.Update}
	h.state.api = h.fake
	h.tt = ui.NewTester(h.view, 1024, 900)
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
		Change:    func(fn func(*appconf.AppConfig)) { fn(&h.cfg); h.changes++ },
		PatchLive: func(fn func(*appconf.AppConfig)) { fn(&h.cfg) },
		Refresh:   func() {},
	}
	ui.Column(c).Fill().Padding(20).Gap(16).Background(t.AppBg).Children(func() { View(c, t, &h.state, p) })
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

func TestAutoCheckSwitch(t *testing.T) {
	h := newHarness(t)
	h.expect(t, locales.SettingsUpdateAutoCheckLabel(), locales.SettingsUpdateAutoCheckHint(), locales.SettingsUpdateManualCheckBtn())
	before := h.cfg.CheckUpdateOnStartup
	h.click(t, locales.SettingsUpdateAutoCheckLabel())
	if h.cfg.CheckUpdateOnStartup == before || h.changes != 1 {
		t.Fatalf("label click: check_update_on_startup %v, changes %d", h.cfg.CheckUpdateOnStartup, h.changes)
	}
}

func TestUpToDateAndError(t *testing.T) {
	h := newHarness(t)
	h.fake.result = &service.UpdateCheckResult{CurrentVer: "1.2.3", LatestVer: "1.2.3"}
	h.click(t, locales.SettingsUpdateManualCheckBtn())
	h.expect(t, locales.SettingsUpdateUpToDate(), "1.2.3")
	if h.state.showDialog || h.state.checking {
		t.Errorf("dialog %v, checking %v", h.state.showDialog, h.state.checking)
	}

	h.fake.result, h.fake.err = nil, errors.New("all update URLs failed")
	h.click(t, locales.SettingsUpdateManualCheckBtn())
	h.expect(t, locales.SettingsUpdateCheckFailedTitle(), "all update URLs failed")
	if h.tt.HasText(locales.SettingsUpdateUpToDate()) {
		t.Error("up-to-date hint kept after a failed check")
	}
}

func TestNewVersionDialog(t *testing.T) {
	h := newHarness(t)
	h.fake.result = &service.UpdateCheckResult{
		HasUpdate: true, CurrentVer: "1.2.3", LatestVer: "1.3.0", ReleaseDate: "2026-10-01",
		Changelog:         []string{"新增设置页", "修复若干问题"}, // i18n:keep：测试数据
		Downloads:         map[string]string{"github": "https://github.com/x", "gitee": "https://gitee.com/x"},
		UpdateManifestURL: "https://example.invalid/manifest.json",
	}
	h.click(t, locales.SettingsUpdateManualCheckBtn())
	if !h.state.showDialog {
		t.Fatal("dialog not shown")
	}
	h.tt.Advance(400 * time.Millisecond)
	h.expect(t, locales.UpdateDialogNewVersion(), locales.UpdateDialogAvailable(), "v1.2.3", "v1.3.0", "2026-10-01",
		locales.UpdateDialogChangelog(), "新增设置页", "修复若干问题", locales.UpdateDialogSkipVersion()) // i18n:keep：测试数据
	if h.tt.HasText(locales.SettingsUpdateNewVersion() + " ") {
		t.Error("hint shown while the dialog is open")
	}
	// Windows 是应用内更新，macOS、Linux 才有手动下载按钮。
	if runtime.GOOS == "windows" {
		if h.tt.HasText(locales.UpdateDialogGithubDownload()) {
			t.Error("manual download shown on Windows")
		}
		// 失败：显示错误，可以再试。
		h.fake.applyErr = errors.New("network down")
		h.click(t, locales.UpdateDialogUpdateNow())
		h.expect(t, locales.UpdateDialogUpdateFailed(), "network down")
		// 没有要更新的文件同样是失败。
		h.fake.applyErr, h.fake.applyResult = nil, &service.UpdateApplyResult{}
		h.click(t, locales.UpdateDialogUpdateNow())
		h.expect(t, locales.UpdateDialogNoFilesToUpdate())
		if h.state.dialog.Updating || len(h.fake.applied) != 2 || h.fake.applied[0] != "https://example.invalid/manifest.json" {
			t.Fatalf("after failures: updating %v, applied %q", h.state.dialog.Updating, h.fake.applied)
		}
		// 开始更新：进度事件交给这个对话框，之后后端退出应用，对话框保持更新中。
		h.fake.applyResult = &service.UpdateApplyResult{Started: true, FileCount: 1}
		h.fake.progress = []service.UpdateProgress{{Phase: "downloading", File: "LunaBox.exe", Downloaded: 50, Total: 100, Percent: 50}}
		h.click(t, locales.UpdateDialogUpdateNow())
		h.expect(t, locales.UpdateDialogDownloading(), "50%", "LunaBox.exe", locales.UpdateDialogDownloadingButton())
		if !h.state.dialog.Updating || h.state.dialog.Error != "" {
			t.Fatalf("after start: %+v", h.state.dialog)
		}
		h.state.dialog.Updating = false // 测试里应用不会退出，放开关闭按钮
		h.frames(1)
	}

	// 关闭：淡出 200ms 后关闭，面板里留下新版本提示，点击再次打开。
	h.click(t, locales.UpdateDialogClose())
	if !h.state.showDialog {
		t.Fatal("dialog closed before fading out")
	}
	h.tt.Advance(300 * time.Millisecond)
	h.frames(1)
	if h.state.showDialog {
		t.Fatalf("dialog still open after fading out: %+v %q", h.state.dialog, h.tt.Texts())
	}
	h.expect(t, locales.SettingsUpdateNewVersion(), "v1.3.0")
	h.click(t, locales.SettingsUpdateNewVersion())
	if !h.state.showDialog {
		t.Fatal("hint did not reopen the dialog")
	}
	h.tt.Advance(400 * time.Millisecond)

	// 点击遮罩同样关闭；再从提示打开。
	h.tt.ClickAt(4, 100)
	h.tt.Advance(300 * time.Millisecond)
	h.frames(1)
	if h.state.showDialog {
		t.Fatal("backdrop click did not close the dialog")
	}
	h.click(t, locales.SettingsUpdateNewVersion())
	h.tt.Advance(400 * time.Millisecond)

	// 跳过此版本：写进配置，提示换成已是最新。
	h.click(t, locales.UpdateDialogSkipVersion())
	if got := h.svc.Config.SkipVersion; got != "1.3.0" {
		t.Errorf("skip_version = %q", got)
	}
	h.tt.Advance(300 * time.Millisecond)
	h.frames(1)
	if h.state.showDialog || h.state.info.HasUpdate {
		t.Errorf("dialog %v, has update %v after skip", h.state.showDialog, h.state.info.HasUpdate)
	}
	h.expect(t, locales.SettingsUpdateUpToDate(), "1.2.3")
}
