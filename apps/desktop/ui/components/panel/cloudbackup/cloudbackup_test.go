package cloudbackup

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/backend/version"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// harness 模拟设置页：Change、PatchLive 改本地的一份配置并计数，Refresh 从服务重新读取。
type harness struct {
	svc       *host.Backend
	env       *env.Env
	cfg       appconf.AppConfig
	state     State
	hidden    bool // 不构建面板（离开设置页或收起分区）
	changes   int
	patches   int
	tt        *ui.Tester
	announced []string
}

func newHarness(t *testing.T, setup func(cfg *appconf.AppConfig)) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{svc: svc, cfg: *svc.Config}
	h.cfg.CloudBackupEnabled = true
	if setup != nil {
		setup(&h.cfg)
	}
	h.env = &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 1500)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
	return h
}

func (h *harness) props() panel.Settings {
	return panel.Settings{
		Env:       h.env,
		Config:    &h.cfg,
		Change:    func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg); h.changes++ },
		PatchLive: func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg); h.patches++ },
		Refresh:   func() { h.cfg = *h.svc.Config },
	}
}

// view 是设置页里分区内容所在的位置：CollapsibleSection 的 space-y-4 p-5 列。
func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	ui.Column(c).Fill().Padding(32).Background(t.AppBg).Children(func() {
		ui.Column(c).Width(800).Padding(20).Gap(16).Children(func() {
			if !h.hidden {
				View(c, t, &h.state, h.props())
			}
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

// typeInto 点击标签下方的输入框（label、space-y-2、py-2 的输入框），全选后输入。
func (h *harness) typeInto(t *testing.T, label, text string) {
	t.Helper()
	r, ok := h.tt.Find(label)
	if !ok {
		t.Fatalf("no %q in %q", label, h.tt.Texts())
	}
	h.tt.ClickAt(r.X+40, r.Y+r.H+8+20)
	h.frames(1)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type(text)
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

// expectToast 检查到目前为止有没有以 prefix 开头的提示（Toast 会读给屏幕阅读器）。
func (h *harness) expectToast(t *testing.T, prefix string) {
	t.Helper()
	h.announced = append(h.announced, h.tt.Announcements()...)
	if !slices.ContainsFunc(h.announced, func(s string) bool { return strings.HasPrefix(s, prefix) }) {
		t.Errorf("no toast %q in %q", prefix, h.announced)
	}
}

// selectProvider 打开提供商下拉框并选中 label。
func (h *harness) selectProvider(t *testing.T, current, label string) {
	t.Helper()
	h.click(t, current)
	h.click(t, label)
}

func TestEnableSwitchPatchesLive(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.CloudBackupEnabled = false })
	h.expect(t, locales.SettingsCloudBackupServiceEnableLabel(), locales.SettingsCloudBackupServiceEnableHint(),
		locales.SettingsCloudBackupProviderLabel())
	h.click(t, locales.SettingsCloudBackupServiceEnableLabel())
	if !h.cfg.CloudBackupEnabled || h.patches != 1 || h.changes != 0 {
		t.Fatalf("enabled %v, patches %d, changes %d", h.cfg.CloudBackupEnabled, h.patches, h.changes)
	}
	h.click(t, locales.SettingsCloudBackupServiceEnableLabel())
	if h.cfg.CloudBackupEnabled || h.patches != 2 {
		t.Fatalf("disable: enabled %v, patches %d", h.cfg.CloudBackupEnabled, h.patches)
	}
}

func TestProviderSwitching(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.CloudBackupProvider = "s3"
		cfg.LastCloudSyncStatus = "success"
		cfg.LastCloudSyncTime = "2026-10-01T00:00:00Z"
	})
	h.expect(t, locales.SettingsCloudBackupProviderS3(), locales.SettingsCloudBackupS3Section(), "S3 Endpoint", "Region", "Bucket",
		"Access Key", "Secret Key", locales.SettingsCloudBackupTestConnection(), locales.SettingsCloudBackupBackupPasswordLabel(),
		locales.SettingsCloudBackupSetPasswordBtn(), locales.SettingsCloudBackupPasswordWarning())

	// 选同一个提供商不改草稿。
	h.selectProvider(t, locales.SettingsCloudBackupProviderS3(), locales.SettingsCloudBackupProviderS3())
	if h.changes != 0 {
		t.Fatalf("selecting the same provider changed the draft %d times", h.changes)
	}

	h.selectProvider(t, locales.SettingsCloudBackupProviderS3(), "WebDAV")
	if h.cfg.CloudBackupProvider != "webdav" || h.cfg.LastCloudSyncStatus != "idle" || h.cfg.LastCloudSyncTime != "" || h.changes != 1 {
		t.Fatalf("provider %q, status %q, time %q, changes %d", h.cfg.CloudBackupProvider, h.cfg.LastCloudSyncStatus,
			h.cfg.LastCloudSyncTime, h.changes)
	}
	h.expect(t, locales.SettingsCloudBackupWebdavSection(), "URL", locales.SettingsCloudBackupWebdavUrlHint(),
		locales.SettingsCloudBackupWebdavUsername(), locales.SettingsCloudBackupWebdavPassword(), locales.SettingsCloudBackupBackupPasswordLabel())
	h.expectNot(t, locales.SettingsCloudBackupS3Section())

	h.selectProvider(t, "WebDAV", "OneDrive")
	h.expect(t, locales.SettingsCloudBackupOneDriveSection(), locales.SettingsCloudBackupOneDriveNote(),
		locales.SettingsCloudBackupOneDriveNoteItem5(), "Client ID", locales.SettingsCloudBackupClientIdChangeHint(),
		locales.SettingsCloudBackupAuthStatusLabel(), locales.SettingsCloudBackupAuthOneDriveBtn())
	// 没有授权时没有测试连接。
	h.expectNot(t, locales.SettingsCloudBackupTestConnection())

	h.selectProvider(t, "OneDrive", locales.SettingsCloudBackupProviderUmbra())
	h.expect(t, locales.SettingsCloudBackupUmbraSection(), locales.SettingsCloudBackupAuthUmbraBtn(), locales.SettingsCloudBackupAdvancedOptions())
	// Umbra 不需要备份密码，也不构建密码弹窗。
	h.expectNot(t, locales.SettingsCloudBackupBackupPasswordLabel(), locales.SettingsCloudBackupUmbraBaseURL())
	if h.cfg.CloudBackupProvider != "umbra" || h.changes != 3 {
		t.Fatalf("provider %q, changes %d", h.cfg.CloudBackupProvider, h.changes)
	}
}

func TestS3AndWebDAVInputs(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.CloudBackupProvider = "s3" })
	h.typeInto(t, "S3 Endpoint", "https://s3.test")
	h.typeInto(t, "Bucket", "saves")
	h.typeInto(t, "Secret Key", "hunter2")
	if h.cfg.S3Endpoint != "https://s3.test" || h.cfg.S3Bucket != "saves" || h.cfg.S3SecretKey != "hunter2" {
		t.Fatalf("endpoint %q, bucket %q, secret %q", h.cfg.S3Endpoint, h.cfg.S3Bucket, h.cfg.S3SecretKey)
	}
	if h.tt.HasText("hunter2") {
		t.Error("the secret key is shown in plain text")
	}

	h.cfg.CloudBackupProvider = "webdav"
	h.frames(2)
	h.typeInto(t, "URL", "https://dav.test/files")
	h.typeInto(t, locales.SettingsCloudBackupWebdavUsername(), "luna")
	if h.cfg.WebDAVURL != "https://dav.test/files" || h.cfg.WebDAVUsername != "luna" {
		t.Fatalf("url %q, user %q", h.cfg.WebDAVURL, h.cfg.WebDAVUsername)
	}
}

// 测试连接：服务在连网之前就校验配置，空的地址立即失败，不访问网络。
func TestConnectionTestFailures(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.CloudBackupProvider = "webdav" })
	h.click(t, locales.SettingsCloudBackupTestConnection())
	h.expectToast(t, locales.SettingsCloudBackupToastWebdavTestFailed(""))
	if h.state.testingWebDAV {
		t.Error("still testing after the result")
	}
	h.expect(t, locales.SettingsCloudBackupTestConnection())

	h.cfg.CloudBackupProvider = "s3"
	h.frames(2)
	h.click(t, locales.SettingsCloudBackupTestConnection())
	h.expectToast(t, locales.SettingsCloudBackupToastS3TestFailed(""))
}

func TestOneDriveClientIDAndRevoke(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.CloudBackupProvider = "onedrive"
		cfg.OneDriveClientID = "abc"
		cfg.OneDriveRefreshToken = "tok"
	})
	h.expect(t, locales.SettingsCloudBackupAuthorized(), locales.SettingsCloudBackupRevokeAuth(), locales.SettingsCloudBackupTestConnection())
	h.expectNot(t, locales.SettingsCloudBackupAuthOneDriveBtn())

	// 改 Client ID 清除旧授权。
	h.typeInto(t, "Client ID", "xyz")
	if h.cfg.OneDriveClientID != "xyz" || h.cfg.OneDriveRefreshToken != "" {
		t.Fatalf("client id %q, token %q", h.cfg.OneDriveClientID, h.cfg.OneDriveRefreshToken)
	}
	// 只多了首尾空白时保留授权。
	h.cfg.OneDriveRefreshToken = "tok2"
	h.frames(2)
	r, _ := h.tt.Find("Client ID")
	h.tt.ClickAt(r.X+40, r.Y+r.H+28)
	h.tt.Key(0, ui.KeyEnd)
	h.tt.Type(" ")
	h.frames(2)
	if h.cfg.OneDriveClientID != "xyz " || h.cfg.OneDriveRefreshToken != "tok2" {
		t.Fatalf("client id %q, token %q", h.cfg.OneDriveClientID, h.cfg.OneDriveRefreshToken)
	}

	h.click(t, locales.SettingsCloudBackupRevokeAuth())
	if h.cfg.OneDriveRefreshToken != "" {
		t.Fatalf("token %q after revoke", h.cfg.OneDriveRefreshToken)
	}
	h.expect(t, locales.SettingsCloudBackupAuthOneDriveBtn())
	h.expectNot(t, locales.SettingsCloudBackupTestConnection())

	// 没有 Client ID 时授权按钮禁用（不会打开浏览器）。
	h.cfg.OneDriveClientID = "  "
	h.frames(2)
	h.click(t, locales.SettingsCloudBackupAuthOneDriveBtn())
	if h.state.authorizingOneDrive || len(h.tt.OpenedURLs()) != 0 {
		t.Fatalf("authorizing %v, opened %v", h.state.authorizingOneDrive, h.tt.OpenedURLs())
	}

	// 文档链接在浏览器里打开。
	h.click(t, locales.SettingsCloudBackupClientIdHintLink())
	if urls := h.tt.OpenedURLs(); len(urls) != 1 || urls[0] != oneDriveDocsURL {
		t.Errorf("opened %v", urls)
	}
}

func TestUmbraUnauthorized(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.CloudBackupProvider = "umbra"
		cfg.UmbraBaseURL = ""
		cfg.UmbraAuthenticated = false
	})
	// 没有服务地址时只提示，不开始授权。
	h.click(t, locales.SettingsCloudBackupAuthUmbraBtn())
	h.expectToast(t, locales.SettingsCloudBackupToastUmbraConfigRequired())
	if h.state.authorizingUmbra {
		t.Fatal("authorization started without a base URL")
	}

	// 高级选项里的服务地址（提示里也有“服务地址”，所以按整段文字查找）。
	if _, ok := h.tt.Find(locales.SettingsCloudBackupUmbraBaseURL()); ok {
		t.Fatal("advanced options open by default")
	}
	h.click(t, locales.SettingsCloudBackupAdvancedOptions())
	h.expect(t, locales.SettingsCloudBackupUmbraBaseURL(), locales.SettingsCloudBackupUmbraBaseURLHint())
	h.cfg.UmbraAuthenticated = true // 改地址后需要重新授权
	h.typeInto(t, locales.SettingsCloudBackupUmbraBaseURL(), "https://umbra.test")
	if h.cfg.UmbraBaseURL != "https://umbra.test" || h.cfg.UmbraAuthenticated {
		t.Fatalf("base url %q, authenticated %v", h.cfg.UmbraBaseURL, h.cfg.UmbraAuthenticated)
	}
	h.click(t, locales.SettingsCloudBackupAdvancedOptions())
	h.frames(2)
	h.expectNot(t, locales.SettingsCloudBackupUmbraBaseURLHint())
}

// 已授权的 Umbra 挂载时读取用户信息。测试构建没有注入 OAuth Client ID，服务在连网之前就返回错误。
func TestUmbraAuthorizedProfileError(t *testing.T) {
	if version.UmbraOAuthClientID != "" {
		t.Skip("built with an Umbra client ID; the profile request would hit the network")
	}
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.CloudBackupProvider = "umbra"
		cfg.UmbraBaseURL = "https://umbra.test"
		cfg.UmbraAuthenticated = true
	})
	if h.state.profileError == "" || h.state.loadingProfile {
		t.Fatalf("profile error %q, loading %v", h.state.profileError, h.state.loadingProfile)
	}
	h.expect(t, locales.SettingsCloudBackupUmbraProfileLoadFailed(h.state.profileError), locales.SettingsCloudBackupRetry(),
		locales.SettingsCloudBackupRefresh(), locales.SettingsCloudBackupTestConnection(), locales.SettingsCloudBackupRevokeAuth())
	h.expectNot(t, locales.SettingsCloudBackupAuthUmbraBtn())

	// 依赖没变时不重复读取；离开再回来（重新挂载）时重新读取。
	h.state.profileError = ""
	h.frames(2)
	if h.state.profileError != "" {
		t.Fatal("profile reloaded without a dependency change")
	}
	h.hidden = true
	h.frames(2)
	h.hidden = false
	h.frames(2)
	if h.state.profileError == "" {
		t.Fatal("profile not reloaded after remounting")
	}

	// 重试同样失败。
	h.state.profileError = ""
	h.frames(1)
	h.click(t, locales.SettingsCloudBackupRefresh())
	if h.state.profileError == "" {
		t.Fatal("refresh did not reload the profile")
	}

	// 登出失败时提示，保持已授权。
	h.click(t, locales.SettingsCloudBackupRevokeAuth())
	h.expectToast(t, locales.SettingsCloudBackupToastUmbraLogoutFailed(""))
	if !h.cfg.UmbraAuthenticated || h.state.revokingUmbra {
		t.Fatalf("authenticated %v, revoking %v", h.cfg.UmbraAuthenticated, h.state.revokingUmbra)
	}

	// 换掉提供商时作废用户信息的读取。
	h.cfg.CloudBackupProvider = "s3"
	h.frames(2)
	if h.state.loadingProfile {
		t.Error("still loading after switching provider")
	}
}

// 用户信息卡片：直接放入状态（测试环境不访问网络）。
func TestUmbraProfileCard(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.CloudBackupProvider = "umbra" })
	h.cfg.UmbraBaseURL, h.cfg.UmbraAuthenticated = "", true // 没有地址：effect 不读取
	h.state.profile = &vo.UmbraUserProfile{Username: " luna", UsedBytes: 512 << 20, QuotaBytes: 2 << 30, AvailableBytes: 1536 << 20}
	h.frames(2)
	h.expect(t, "L", "luna", locales.SettingsCloudBackupAuthorized(), locales.SettingsCloudBackupUmbraStorage(),
		"512.0 MB / 2.0 GB", locales.SettingsCloudBackupUmbraStorageAvailable("1.5 GB"))
	if got := storagePercent(512<<20, 2<<30); got != 25 {
		t.Errorf("percent %v", got)
	}
	if got := storagePercent(5, 0); got != 0 {
		t.Errorf("percent without quota %v", got)
	}
	if got := storagePercent(10, 5); got != 100 {
		t.Errorf("percent over quota %v", got)
	}

	h.state.profile, h.state.loadingProfile = nil, true
	h.frames(2)
	h.expect(t, locales.SettingsCloudBackupUmbraProfileLoading())
}

func TestPasswordModal(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.CloudBackupProvider = "s3"
		cfg.BackupUserID = ""
	})
	// 保存密码后原版把服务的整份配置交给 onChange，所以服务里的提供商也是 S3（真实的页面里草稿已经保存过）。
	provider := h.svc.Config.CloudBackupProvider
	h.svc.Config.CloudBackupProvider = "s3"
	t.Cleanup(func() {
		// 配置文件在同一个测试进程的数据目录里，恢复成原来的样子。
		h.svc.Config.BackupUserID = ""
		h.svc.Config.CloudBackupProvider = provider
		_ = appconf.SaveConfig(h.svc.Config)
	})
	h.click(t, locales.SettingsCloudBackupSetPasswordBtn())
	h.expect(t, locales.SettingsPasswordModalTitle(), locales.SettingsPasswordModalDesc1(), locales.SettingsPasswordModalDesc2(),
		locales.SettingsPasswordModalPasswordLabel(), locales.SettingsPasswordModalConfirmLabel(), locales.CommonCancel(), locales.CommonConfirm())

	h.typeInto(t, locales.SettingsPasswordModalPasswordLabel(), "abc")
	h.expect(t, locales.SettingsPasswordModalPasswordTooShort())
	// 太短时确认按钮禁用。
	h.click(t, locales.CommonConfirm())
	if !h.state.modal.open {
		t.Fatal("confirm closed the modal with a short password")
	}
	// 取消关闭并清空。
	h.click(t, locales.CommonCancel())
	if h.state.modal.open || h.state.modal.password != "" {
		t.Fatalf("after cancel: %+v", h.state.modal)
	}
	h.expectNot(t, locales.SettingsPasswordModalDesc1())

	h.click(t, locales.SettingsCloudBackupSetPasswordBtn())
	h.typeInto(t, locales.SettingsPasswordModalPasswordLabel(), "secret1")
	h.typeInto(t, locales.SettingsPasswordModalConfirmLabel(), "secret2")
	h.expect(t, locales.SettingsPasswordModalPasswordMismatch())
	h.expectNot(t, locales.SettingsPasswordModalPasswordTooShort())
	h.click(t, locales.NativeSettingsCloudBackupShowPassword())
	if !h.state.modal.show {
		t.Fatal("eye button did not show the password")
	}
	h.typeInto(t, locales.SettingsPasswordModalConfirmLabel(), "secret1")
	h.expectNot(t, locales.SettingsPasswordModalPasswordMismatch())
	h.click(t, locales.CommonConfirm())

	if h.state.modal.open || h.state.modal.password != "" || h.state.modal.show {
		t.Fatalf("modal after confirm: %+v", h.state.modal)
	}
	id := h.svc.Config.BackupUserID
	if id == "" || h.cfg.BackupUserID != id {
		t.Fatalf("backup user id: service %q, draft %q", id, h.cfg.BackupUserID)
	}
	h.expectToast(t, locales.SettingsCloudBackupToastPasswordSetSuccess(id[:8]))
	h.expect(t, "********", locales.SettingsCloudBackupPasswordSet()+" "+id[:8]+"...")
	h.expectNot(t, locales.SettingsCloudBackupSetPasswordBtn())
}

// 离开设置页再回来时弹窗关闭（原版重新挂载面板）。
func TestModalClosedOnRemount(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.CloudBackupProvider = "webdav" })
	h.click(t, locales.SettingsCloudBackupSetPasswordBtn())
	h.typeInto(t, locales.SettingsPasswordModalPasswordLabel(), "abcdef")
	h.hidden = true
	h.frames(2)
	h.hidden = false
	h.frames(2)
	if h.state.modal.open || h.state.modal.password != "" {
		t.Fatalf("modal after remount: %+v", h.state.modal)
	}
}

func TestProviderConfigured(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  appconf.AppConfig
		want bool
	}{
		{"disabled", appconf.AppConfig{CloudBackupProvider: "umbra", UmbraBaseURL: "u", UmbraAuthenticated: true}, false},
		{"umbra", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "umbra", UmbraBaseURL: "u", UmbraAuthenticated: true}, true},
		{"umbra unauthenticated", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "umbra", UmbraBaseURL: "u"}, false},
		{"onedrive without password", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "onedrive", OneDriveRefreshToken: "t"}, false},
		{"onedrive", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "onedrive", BackupUserID: "id", OneDriveRefreshToken: "t"}, true},
		{"webdav", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "webdav", BackupUserID: "id", WebDAVURL: "w"}, true},
		{"s3", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "s3", BackupUserID: "id", S3Endpoint: "e", S3AccessKey: "k"}, true},
		{"s3 without key", appconf.AppConfig{CloudBackupEnabled: true, CloudBackupProvider: "s3", BackupUserID: "id", S3Endpoint: "e"}, false},
		{"empty provider is s3", appconf.AppConfig{CloudBackupEnabled: true, BackupUserID: "id", S3Endpoint: "e", S3AccessKey: "k"}, true},
	} {
		if got := ProviderConfigured(&tc.cfg); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
	if ProviderConfigured(nil) {
		t.Error("nil config is configured")
	}
	for in, want := range map[int]int{0: 60, -3: 60, 5: 15, 15: 15, 90: 90} {
		if got := SyncIntervalSeconds(&appconf.AppConfig{CloudSyncIntervalSec: in}); got != want {
			t.Errorf("SyncIntervalSeconds(%d) = %d, want %d", in, got, want)
		}
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把各提供商的配置与密码弹窗存成图片，用于目测。
func TestScreenshotDump(t *testing.T) {
	out := os.Getenv("DESKTOP_MYGO_PNG")
	if out == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.CloudBackupProvider = "s3" })
	save := func(name string) {
		h.frames(2)
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	save("cloudbackup-s3.png")
	h.cfg.CloudBackupProvider, h.cfg.BackupUserID = "webdav", "0123456789abcdef"
	save("cloudbackup-webdav.png")
	h.cfg.CloudBackupProvider, h.cfg.OneDriveRefreshToken = "onedrive", "tok"
	save("cloudbackup-onedrive.png")
	h.cfg.CloudBackupProvider, h.cfg.UmbraBaseURL, h.cfg.UmbraAuthenticated = "umbra", "", true
	h.state.profile = &vo.UmbraUserProfile{Username: "luna", UsedBytes: 512 << 20, QuotaBytes: 2 << 30, AvailableBytes: 1536 << 20}
	h.state.profileError = "boom"
	save("cloudbackup-umbra.png")
	h.cfg.CloudBackupProvider, h.cfg.BackupUserID = "s3", ""
	h.state.modal.open = true
	save("cloudbackup-password.png")
}

// 离开设置页时取消进行中的 Umbra 授权（面板卸载时的 effect cleanup），再次进入时按钮回到未授权的状态。
// 测试里服务调用是同步的，授权不会停在进行中，所以直接设置进行中的状态。
func TestLeaveCancelsUmbraAuth(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.CloudBackupProvider = "umbra"
		cfg.UmbraBaseURL = "https://umbra.test"
		cfg.UmbraAuthenticated = false
	})
	Leave(&h.state, h.props()) // 没有进行中的授权：什么都不做
	if h.state.umbraAuthGen != 0 {
		t.Fatal("leave without an authorization invalidated it")
	}

	h.state.umbraAuthActive, h.state.authorizingUmbra, h.state.cancellingUmbra = true, true, true
	Leave(&h.state, h.props())
	if h.state.umbraAuthGen != 1 || h.state.umbraAuthActive || h.state.authorizingUmbra || h.state.cancellingUmbra {
		t.Fatalf("state after leave: %+v", h.state)
	}
	h.frames(2)
	h.expect(t, locales.SettingsCloudBackupAuthUmbraBtn())
	h.expectNot(t, locales.SettingsCloudBackupCancelAuth())
}
