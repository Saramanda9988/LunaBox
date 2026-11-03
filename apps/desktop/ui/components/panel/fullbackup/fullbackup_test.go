package fullbackup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// fakeDialogs 用真实的 BackupService，只替换文件对话框。
type fakeDialogs struct {
	*service.BackupService
	save, open string
}

func (f *fakeDialogs) SelectBackupSavePath() (string, error)    { return f.save, nil }
func (f *fakeDialogs) SelectBackupRestorePath() (string, error) { return f.open, nil }

type harness struct {
	svc   *host.Backend
	env   *env.Env
	cfg   appconf.AppConfig
	state State
	quits int
	tt    *ui.Tester

	collapsed bool // 分区收起或离开了设置页：只调用 Background（设置页每一帧都调用它）
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t)}
	h.cfg = *h.svc.Config
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.state.quit = func() { h.quits++ }
	h.tt = ui.NewTester(h.view, 1024, 800)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
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
	if h.collapsed {
		Background(c, &h.state, p)
		return
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

func (h *harness) expectPrefix(t *testing.T, prefix string) {
	t.Helper()
	for _, s := range h.tt.Texts() {
		if strings.HasPrefix(s, prefix) {
			return
		}
	}
	t.Errorf("no text starting with %q in %q", prefix, h.tt.Texts())
}

func TestRender(t *testing.T) {
	h := newHarness(t)
	h.expect(t, locales.SettingsFullDataBackupHint(), locales.SettingsFullDataBackupExportBtn(), locales.SettingsFullDataBackupImportBtn())
	if h.tt.HasText(locales.SettingsFullDataBackupLastBackup()) {
		t.Error("last backup shown without a backup")
	}
}

// 测试环境没有窗口，服务的文件对话框返回错误：导出提示失败，导入提示选择文件失败。
func TestDialogsUnavailable(t *testing.T) {
	h := newHarness(t)
	h.click(t, locales.SettingsFullDataBackupExportBtn())
	h.expectPrefix(t, locales.SettingsFullDataBackupToastExportFailed(""))
	h.click(t, locales.SettingsFullDataBackupImportBtn())
	h.expectPrefix(t, locales.SettingsFullDataBackupToastSelectFileFailed(""))
	if h.state.exporting || h.state.restoring || h.state.picking || h.state.confirm.open {
		t.Errorf("busy after failed dialogs: %+v", h.state)
	}
}

func TestExportAndRestore(t *testing.T) {
	h := newHarness(t)
	fake := &fakeDialogs{BackupService: h.svc.Backup}
	h.state.api = fake

	// 取消保存对话框：什么都不做。
	h.click(t, locales.SettingsFullDataBackupExportBtn())
	if h.state.exporting || len(h.state.toasts) != 0 {
		t.Fatalf("cancelled export did something: %+v", h.state)
	}

	fake.save = filepath.Join(t.TempDir(), "full.zip")
	h.click(t, locales.SettingsFullDataBackupExportBtn())
	if _, err := os.Stat(fake.save); err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	h.expect(t, locales.SettingsFullDataBackupToastExportSuccess(fake.save))
	tm, err := time.Parse(time.RFC3339, h.svc.Config.LastFullBackupTime)
	if err != nil {
		t.Fatal(err)
	}
	h.expect(t, locales.SettingsFullDataBackupLastBackup()+timefmt.DateTime(tm, timefmt.Location(h.svc.Config)))

	// 导入：取消确认不安排恢复。
	fake.open = fake.save
	h.click(t, locales.SettingsFullDataBackupImportBtn())
	h.expect(t, locales.SettingsFullDataBackupModalRestoreTitle(), locales.SettingsFullDataBackupModalRestoreMsg(fake.save))
	h.click(t, locales.CommonCancel())
	if h.svc.Config.PendingFullRestore != "" || h.state.restoring {
		t.Fatal("cancel scheduled the restore")
	}

	h.svc.Config.PendingDBRestore = "stale.zip"
	h.click(t, locales.SettingsFullDataBackupImportBtn())
	h.click(t, locales.CommonConfirm())
	if got := h.svc.Config.PendingFullRestore; got != fake.save {
		t.Fatalf("pending full restore = %q", got)
	}
	if h.svc.Config.PendingDBRestore != "" {
		t.Error("pending DB restore kept")
	}
	h.expect(t, locales.SettingsFullDataBackupToastRestoreScheduled(), locales.SettingsFullDataBackupImporting())
	h.collapsed = true // 收起分区后照常退出
	h.frames(1)
	h.tt.Advance(1400 * time.Millisecond)
	if h.quits != 0 {
		t.Fatal("quit before 1.5s")
	}
	h.tt.Advance(200 * time.Millisecond)
	h.frames(1)
	if h.quits != 1 {
		t.Fatalf("quits = %d", h.quits)
	}
	h.collapsed = false
	h.svc.Config.PendingFullRestore = ""
}

func TestRestoreScheduleFailed(t *testing.T) {
	h := newHarness(t)
	h.state.api = &fakeDialogs{BackupService: h.svc.Backup, open: filepath.Join(t.TempDir(), "missing.zip")}
	h.click(t, locales.SettingsFullDataBackupImportBtn())
	h.click(t, locales.CommonConfirm())
	h.expectPrefix(t, locales.SettingsFullDataBackupToastRestoreScheduleFailed(""))
	if h.state.restoring || h.state.quitPending {
		t.Errorf("restoring after failure: %+v", h.state)
	}
}
