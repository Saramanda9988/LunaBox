package dbbackup

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
	"lunabox/desktop-mygo/ui/timefmt"
)

// fakeCloud 用真实的 BackupService，只替换云端的调用（测试不联网）。
type fakeCloud struct {
	*service.BackupService
	items      []vo.CloudBackupItem
	uploaded   []string
	listCalls  int
	restoreKey string
}

func (f *fakeCloud) GetCloudDBBackups() ([]vo.CloudBackupItem, error) {
	f.listCalls++
	return f.items, nil
}

func (f *fakeCloud) UploadDBBackupToCloud(path string) error {
	f.uploaded = append(f.uploaded, path)
	return nil
}

func (f *fakeCloud) ScheduleDBRestoreFromCloud(key string) error {
	f.restoreKey = key
	return nil
}

type harness struct {
	svc   *host.Backend
	env   *env.Env
	cfg   appconf.AppConfig
	state State
	quits int
	tt    *ui.Tester

	collapsed bool // 分区收起或离开了设置页：只调用 Background（设置页每一帧都调用它）
}

func newHarness(t *testing.T, setup func(h *harness)) *harness {
	t.Helper()
	h := &harness{svc: hosttest.NewServices(t)}
	h.cfg = *h.svc.Config
	h.env = &env.Env{Svc: h.svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.state.quit = func() { h.quits++ }
	if setup != nil {
		setup(h)
	}
	h.tt = ui.NewTester(h.view, 1024, 1400)
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

func (h *harness) backups(t *testing.T) []vo.DBBackupInfo {
	t.Helper()
	st, err := h.svc.Backup.GetDBBackups()
	if err != nil {
		t.Fatal(err)
	}
	return st.Backups
}

// clearBackups 删除数据目录里已有的数据库备份（同一个测试进程的用例共用数据目录）。
func clearBackups(t *testing.T, svc *host.Backend) {
	t.Helper()
	st, err := svc.Backup.GetDBBackups()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range st.Backups {
		if err := svc.Backup.DeleteDBBackup(b.Path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEmptyAndCloudHint(t *testing.T) {
	h := newHarness(t, func(h *harness) { clearBackups(t, h.svc) })
	h.expect(t, locales.SettingsDbBackupSectionTitle(), locales.SettingsDbBackupSectionHint(), locales.SettingsDbBackupBackupBtn(),
		locales.SettingsDbBackupLocalBackups(), locales.SettingsDbBackupNoLocalBackups(),
		locales.SettingsDbBackupCloudNotEnabled(), locales.SettingsDbBackupCloudNotEnabledHint())
	if h.tt.HasText(locales.SettingsDbBackupCloudBackups()) || h.tt.HasText(locales.SettingsDbBackupAutoBackupEnabled()) {
		t.Errorf("cloud list or auto badge shown: %q", h.tt.Texts())
	}
	if h.tt.HasText(locales.SettingsDbBackupLastBackup()) {
		t.Error("last backup shown without a backup")
	}
}

func TestCreateRestoreDelete(t *testing.T) {
	h := newHarness(t, func(h *harness) {
		clearBackups(t, h.svc)
		h.svc.Config.AutoBackupDB = true
	})
	h.expect(t, locales.SettingsDbBackupAutoBackupEnabled())

	h.click(t, locales.SettingsDbBackupBackupBtn())
	list := h.backups(t)
	if len(list) != 1 {
		t.Fatalf("backups = %v", list)
	}
	loc := timefmt.Location(h.svc.Config)
	h.expect(t, locales.SettingsDbBackupToastBackupSuccess(), timefmt.DateTime(list[0].CreatedAt, loc),
		locales.SettingsDbBackupSizeLabel()+components.FormatFileSize(float64(list[0].Size)),
		locales.SettingsDbBackupLastBackup()+" "+formatTime(h.svc.Config.LastDBBackupTime, loc))
	if h.state.backingUp || h.state.loadingLocal {
		t.Errorf("still busy: %+v", h.state)
	}
	if h.tt.HasText(locales.NativeBackupUploadToCloud()) {
		t.Error("upload button shown without cloud backup")
	}

	// 删除：取消不删，确定才删。
	h.click(t, locales.SettingsDbBackupModalDeleteTitle())
	h.expect(t, locales.SettingsDbBackupModalDeleteMsg(), locales.CommonCancel(), locales.CommonConfirm())
	h.click(t, locales.CommonCancel())
	if h.state.confirm.open || len(h.backups(t)) != 1 {
		t.Fatal("cancel deleted the backup or left the modal open")
	}

	// 恢复：安排在下次启动时执行，1.5 秒后退出。
	h.click(t, locales.SettingsDbBackupModalRestoreTitle())
	h.expect(t, locales.SettingsDbBackupModalRestoreMsg())
	h.click(t, locales.CommonConfirm())
	if got := h.svc.Config.PendingDBRestore; got != list[0].Path {
		t.Fatalf("pending restore = %q, want %q", got, list[0].Path)
	}
	h.expect(t, locales.SettingsDbBackupToastRestoreScheduled())
	if !h.state.disabled() || h.quits != 0 {
		t.Fatalf("restoring %q, quits %d", h.state.restoring, h.quits)
	}
	h.collapsed = true // 收起分区后照常退出
	h.frames(1)
	h.tt.Advance(time.Second)
	if h.quits != 0 {
		t.Fatal("quit before 1.5s")
	}
	h.tt.Advance(600 * time.Millisecond)
	h.frames(1)
	if h.quits != 1 {
		t.Fatalf("quits = %d after 1.6s", h.quits)
	}
	h.collapsed = false
	h.svc.Config.PendingDBRestore = ""

	// 重新挂载后清掉“恢复中”（原版退出前一直禁用）：这里直接删备份。
	h.state.restoring = ""
	h.frames(1)
	h.click(t, locales.SettingsDbBackupModalDeleteTitle())
	h.click(t, locales.CommonConfirm())
	if n := len(h.backups(t)); n != 0 {
		t.Fatalf("%d backups after delete", n)
	}
	h.expect(t, locales.SettingsDbBackupToastDeleteSuccess(), locales.SettingsDbBackupNoLocalBackups())
}

func TestRestoreMissingFile(t *testing.T) {
	h := newHarness(t, func(h *harness) { clearBackups(t, h.svc) })
	h.state.askRestore(panel.Settings{Env: h.env}, `Z:\missing\lunabox_backup.zip`)
	h.frames(1)
	h.click(t, locales.CommonConfirm())
	if h.state.restoring != "" || h.state.quitPending || h.svc.Config.PendingDBRestore != "" {
		t.Fatalf("restore scheduled for a missing file: %+v", h.state)
	}
	if texts := h.tt.Texts(); !hasPrefix(texts, locales.SettingsDbBackupToastRestoreScheduleFailed("")) {
		t.Errorf("missing failure toast in %q", texts)
	}
}

func TestCloudBackups(t *testing.T) {
	created := time.Date(2026, 9, 11, 13, 5, 9, 0, time.UTC)
	var fake *fakeCloud
	h := newHarness(t, func(h *harness) {
		clearBackups(t, h.svc)
		cfg := h.svc.Config
		cfg.CloudBackupEnabled, cfg.CloudBackupProvider = true, "webdav"
		cfg.BackupUserID, cfg.WebDAVURL = "user", "https://dav.example.invalid"
		fake = &fakeCloud{BackupService: h.svc.Backup, items: []vo.CloudBackupItem{
			{Key: "user/database/a.zip", Name: "lunabox_a", CreatedAt: created},
			{Key: "user/database/b.zip", CreatedAt: created},
		}}
		h.state.api = fake
	})
	loc := timefmt.Location(h.svc.Config)
	h.expect(t, locales.SettingsDbBackupCloudBackups(), "lunabox_a", timefmt.DateTime(created, loc))
	if h.tt.HasText(locales.SettingsDbBackupCloudNotEnabled()) {
		t.Error("cloud hint shown while configured")
	}
	if fake.listCalls != 1 {
		t.Errorf("cloud list loaded %d times on mount", fake.listCalls)
	}

	// 立即备份：自动上传关闭时提示本地成功，云端列表同样重新读取。
	h.click(t, locales.SettingsDbBackupBackupBtn())
	h.expect(t, locales.SettingsDbBackupToastBackupSuccess())
	if fake.listCalls != 2 {
		t.Errorf("cloud list loaded %d times after backup", fake.listCalls)
	}
	list := h.backups(t)
	if len(list) != 1 {
		t.Fatalf("backups = %v", list)
	}
	h.click(t, locales.NativeBackupUploadToCloud())
	if len(fake.uploaded) != 1 || fake.uploaded[0] != list[0].Path {
		t.Fatalf("uploaded = %v", fake.uploaded)
	}
	h.expect(t, locales.SettingsDbBackupToastUploadSuccess())
	if h.state.uploading != "" {
		t.Error("still uploading")
	}

	h.click(t, locales.NativeBackupRefreshCloud())
	if fake.listCalls != 4 {
		t.Errorf("cloud list loaded %d times after refresh", fake.listCalls)
	}

	// 从云端恢复：两个云端备份各有一个按钮，点第一个。
	if err := h.tt.Click(locales.SettingsDbBackupModalRestoreCloudTitle()); err != nil {
		t.Fatal(err)
	}
	h.frames(2)
	h.expect(t, locales.SettingsDbBackupModalRestoreCloudMsg())
	h.click(t, locales.CommonConfirm())
	if fake.restoreKey == "" || !h.state.quitPending {
		t.Fatalf("cloud restore not scheduled: key %q, %+v", fake.restoreKey, h.state)
	}
	h.expect(t, locales.SettingsDbBackupToastRestoreScheduled())

	// 关掉云备份：列表换成提示。
	h.svc.Config.CloudBackupEnabled = false
	h.frames(2)
	h.expect(t, locales.SettingsDbBackupCloudNotEnabled())
	if len(h.state.cloudBackups) != 0 {
		t.Error("cloud list kept after disabling")
	}
}

func TestLocalBackupSucceededUploadFailed(t *testing.T) {
	h := newHarness(t, func(h *harness) { clearBackups(t, h.svc) })
	h.state.api = &failingCreate{BackupService: h.svc.Backup, err: errors.New("本地备份成功，但云端上传失败: boom")} // i18n:keep：后端的错误文字
	h.click(t, locales.SettingsDbBackupBackupBtn())
	h.expect(t, locales.SettingsDbBackupToastLocalBackupSuccess(), "本地备份成功，但云端上传失败: boom") // i18n:keep：后端的错误文字

	h.state.api = &failingCreate{BackupService: h.svc.Backup, err: errors.New("disk full")}
	h.click(t, locales.SettingsDbBackupBackupBtn())
	h.expect(t, locales.SettingsDbBackupToastBackupFailed("disk full"))
	if h.state.backingUp {
		t.Error("still backing up")
	}
}

type failingCreate struct {
	*service.BackupService
	err error
}

func (f *failingCreate) CreateAndUploadDBBackup() (*vo.DBBackupInfo, error) { return nil, f.err }

func TestIsCloudProviderConfigured(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  appconf.AppConfig
		want bool
	}{
		{"disabled", appconf.AppConfig{CloudBackupProvider: "s3", S3Endpoint: "e", S3AccessKey: "k", BackupUserID: "u"}, false},
		{"umbra", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "umbra", UmbraBaseURL: "u", UmbraAuthenticated: true}, true},
		{"umbra unauthenticated", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "umbra", UmbraBaseURL: "u"}, false},
		{"no user", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "webdav", WebDAVURL: "w"}, false},
		{"onedrive", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "onedrive", BackupUserID: "u", OneDriveRefreshToken: "r"}, true},
		{"webdav", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "webdav", BackupUserID: "u", WebDAVURL: "w"}, true},
		{"s3", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "s3", BackupUserID: "u", S3Endpoint: "e", S3AccessKey: "k"}, true},
		{"s3 incomplete", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "s3", BackupUserID: "u", S3Endpoint: "e"}, false},
	} {
		if got := isCloudProviderConfigured(&tc.cfg); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if isCloudProviderConfigured(nil) {
		t.Error("nil config configured")
	}
}

func hasPrefix(texts []string, prefix string) bool {
	for _, s := range texts {
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
