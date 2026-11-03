package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/vo"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/env/envtest"
	"lunabox/desktop-mygo/ui/locales"
)

// newEventApp 返回订阅了应用级事件的 App；没有窗口时事件在发出它的 goroutine 上同步处理。
func newEventApp(t *testing.T) (*App, *ui.Tester, *host.Backend) {
	t.Helper()
	svc := newServices(t)
	a := New(envtest.NewBackend(0), svc.API(), Options{})
	a.SetScale(1.5)
	a.subscribe()
	tt := ui.NewTester(a.View, 1280, 800)
	tt.SetScale(1.5)
	tt.Frame()
	return a, tt, svc
}

func hasTextContaining(tt *ui.Tester, s string) bool {
	for _, text := range tt.Texts() {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

// newServices 是 hosttest.NewServices，配置像已经选过时区的用户：否则根布局一直显示时区选择弹窗，挡住其他界面。
func newServices(t *testing.T) *host.Backend {
	t.Helper()
	svc := hosttest.NewServices(t)
	svc.Config.TimeZone = "Asia/Shanghai"
	return svc
}

// hasToast 报告 msg 的提示正在显示：Toaster 把“标题：详情”拆开，只显示标题，详情折叠。
func hasToast(tt *ui.Tester, msg string) bool {
	for _, text := range tt.Texts() {
		if len([]rune(text)) >= 4 && strings.HasPrefix(msg, text) {
			return true
		}
	}
	return false
}

func TestQuitSyncBacksUpThenQuits(t *testing.T) {
	_, tt, svc := newEventApp(t)
	quit := make(chan struct{}, 2)
	svc.Services.Config.SetQuitHandler(func() { quit <- struct{}{} })

	svc.Events.Emit(eventQuitSyncRequested, map[string]string{"reason": "window-close"})
	tt.Frame()
	if !hasTextContaining(tt, locales.ExitSyncToastSuccessTitle()) {
		t.Fatalf("missing success toast in %q", tt.Texts())
	}
	select {
	case <-quit:
	case <-time.After(5 * time.Second):
		t.Fatal("SafeQuit was not called after the backup")
	}
	status, err := svc.Backup.GetDBBackups()
	if err != nil {
		t.Fatal(err)
	}
	if status == nil || len(status.Backups) == 0 {
		t.Fatalf("no database backup created: %+v", status)
	}

	// 再次请求不会重复备份与退出。
	svc.Events.Emit(eventQuitSyncRequested, nil)
	select {
	case <-quit:
		t.Fatal("quit sync ran twice")
	case <-time.After(exitSuccessDelay + 300*time.Millisecond):
	}
}

func TestProtocolLaunchErrorToast(t *testing.T) {
	_, tt, svc := newEventApp(t)
	svc.Events.Emit(eventProtocolLaunchError, vo.ProtocolLaunchErrorEvent{Message: "快捷启动失败", Detail: "未找到游戏"})
	tt.Frame()
	if !hasTextContaining(tt, "快捷启动失败") {
		t.Fatalf("missing protocol launch error toast in %q", tt.Texts())
	}
}

func TestProtocolLaunchMissingWineOpensLaunchSettings(t *testing.T) {
	a, tt, svc := newEventApp(t)
	svc.Events.Emit(eventProtocolLaunchError, vo.ProtocolLaunchErrorEvent{
		Message: "快捷启动失败", Detail: "缺少 Wine", GameID: "g1", Kind: "missing-config", ConfigKey: "wine_runner",
	})
	tt.Frame()
	if err := tt.Click(locales.GameLaunchOpenLaunchConfig()); err != nil {
		t.Fatalf("no launch config action in %q: %v", tt.Texts(), err)
	}
	tt.Frame()
	if got := a.router.Location(); got != env.GamePath("g1") {
		t.Fatalf("location = %q", got)
	}
	if got := a.detail.Tab(); got != "launch" {
		t.Fatalf("tab = %q, want launch", got)
	}
}

func TestInstallPendingShowsConfirmModal(t *testing.T) {
	a, tt, svc := newEventApp(t)
	svc.Events.Emit(eventInstallPending, &vo.InstallRequest{Title: "Test Game", DownloadSource: "Shionlib", MetaSource: "vndb", MetaID: "v17", Size: 3 << 30})
	tt.Frame()
	for _, want := range []string{locales.InstallModalTitle(), "Test Game", "Shionlib", "v17", "3.00 GB", locales.InstallModalMissingChecksumWarning()} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("v17"); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) == 0 || urls[len(urls)-1] != "https://vndb.org/v17" {
		t.Fatalf("opened %q", urls)
	}
	// 请求不完整：StartDownload 失败时提示原因并关闭弹窗。
	if err := tt.Click(locales.InstallModalConfirm()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.install.Request() != nil || tt.HasText(locales.InstallModalTitle()) {
		t.Fatalf("modal still open: %q", tt.Texts())
	}

	// 启动期间收到、还没处理的请求（GetPendingInstall）。
	svc.Download.SetPendingInstall(&vo.InstallRequest{Title: "Pending Game"})
	a.startup()
	tt.Frame()
	if !tt.HasText("Pending Game") {
		t.Fatalf("pending install not shown: %q", tt.Texts())
	}
	if err := tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.install.Request() != nil {
		t.Fatal("cancel did not close the modal")
	}
}

func TestDownloadNotifications(t *testing.T) {
	_, tt, svc := newEventApp(t)
	done := service.DownloadProgressEvent{ID: "t1", Request: vo.InstallRequest{Title: "Some Game"}, Status: service.DownloadStatusDone}
	svc.Events.Emit(eventDownloadProgress, done)
	tt.Frame()
	if !hasTextContaining(tt, locales.DownloadsToastDownloadCompleted("Some Game")) {
		t.Fatalf("missing done toast in %q", tt.Texts())
	}
	failed := service.DownloadProgressEvent{ID: "t2", Status: service.DownloadStatusError, Error: "boom"}
	svc.Events.Emit(eventDownloadProgress, failed)
	tt.Frame()
	if !hasTextContaining(tt, locales.DownloadsToastDownloadFailed(locales.DownloadsUnknownTitle())) {
		t.Fatalf("missing failed toast in %q", tt.Texts())
	}
	svc.Events.Emit(eventDownloadImportFailed, map[string]string{"task_id": "t3", "title": "Other", "error": "no exe"})
	tt.Frame()
	if !hasToast(tt, locales.DownloadsToastAutoImportFailed("Other", "no exe")) {
		t.Fatalf("missing import failed toast in %q", tt.Texts())
	}
}

func TestCoverImageDownloadToasts(t *testing.T) {
	_, tt, svc := newEventApp(t)
	svc.Events.Emit(eventCoverImageDownload, map[string]string{"game_id": "g1", "status": "started"})
	tt.Frame()
	if !hasTextContaining(tt, locales.CoverImageDownloadToastStarted()) {
		t.Fatalf("missing started toast in %q", tt.Texts())
	}
	svc.Events.Emit(eventCoverImageDownload, map[string]string{"game_id": "g1", "status": "cancelled"})
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if hasTextContaining(tt, locales.CoverImageDownloadToastStarted()) {
		t.Fatalf("cancelled toast still shown: %q", tt.Texts())
	}
	svc.Events.Emit(eventCoverImageDownload, map[string]string{"game_id": "g2", "status": "failed", "error": "404"})
	tt.Frame()
	if !hasTextContaining(tt, locales.CoverImageDownloadToastFailed()) {
		t.Fatalf("missing failed toast in %q", tt.Texts())
	}
}

func TestAccountStatusPushFailedToast(t *testing.T) {
	_, tt, svc := newEventApp(t)
	svc.Events.Emit("kungal:status-push-failed", vo.KungalStatusPushFailureEvent{GameID: "g1", GameName: "Game A", Error: "401"})
	tt.Frame()
	if !hasToast(tt, locales.SettingsBasicKungalStatusPushFailed("Game A", "401")) {
		t.Fatalf("missing push failed toast in %q", tt.Texts())
	}
}

func TestScheduledDBBackupToast(t *testing.T) {
	_, tt, svc := newEventApp(t)
	svc.Events.Emit(eventScheduledDBBackup, map[string]string{"status": "failed", "error": "disk full"})
	tt.Frame()
	if !hasToast(tt, locales.SettingsAutoBackupScheduledDbBackupFailed("disk full")) {
		t.Fatalf("missing scheduled backup toast in %q", tt.Texts())
	}
}

func TestTimezoneModalSavesAndQuits(t *testing.T) {
	a, tt, svc := newEventApp(t)
	if tt.HasText(locales.TimezoneModalTitle()) {
		t.Fatal("timezone modal shown although time_zone is set")
	}
	quit := make(chan struct{}, 1)
	svc.Services.Config.SetQuitHandler(func() { quit <- struct{}{} })
	svc.Config.TimeZone = ""
	tt.Frame()
	if !tt.HasText(locales.TimezoneModalTitle()) {
		t.Fatalf("timezone modal not shown: %q", tt.Texts())
	}
	want := a.env.Svc.SystemTimeZone
	if want == "" {
		want = "Asia/Shanghai"
	}
	if err := tt.Click(locales.TimezoneModalConfirmBtn()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.Config.TimeZone != want {
		t.Fatalf("time_zone = %q, want %q", svc.Config.TimeZone, want)
	}
	if tt.HasText(locales.TimezoneModalTitle()) {
		t.Fatal("timezone modal still shown after confirming")
	}
	select {
	case <-quit:
	case <-time.After(5 * time.Second):
		t.Fatal("SafeQuit was not called after choosing the time zone")
	}
}
