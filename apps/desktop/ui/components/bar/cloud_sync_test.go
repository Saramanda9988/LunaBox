package bar

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/common/vo"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// fakeCloudSync 记录 SyncNow 的调用；GetCloudSyncStatus 返回 status。
type fakeCloudSync struct {
	status vo.CloudSyncStatus
	err    error
	syncs  int
	reads  int
}

func (f *fakeCloudSync) GetCloudSyncStatus() vo.CloudSyncStatus { f.reads++; return f.status }

func (f *fakeCloudSync) SyncNow() (vo.CloudSyncStatus, error) {
	f.syncs++
	if f.err != nil {
		return vo.CloudSyncStatus{}, f.err
	}
	f.status.LastSyncStatus = "success"
	f.status.LastSyncTime = "2026-10-21T08:00:00Z"
	return f.status, nil
}

func newCloudSyncFixture(t *testing.T, cfg *appconf.AppConfig, svc *fakeCloudSync) (*ui.Tester, *Sidebar, *env.Env) {
	t.Helper()
	e := &env.Env{Svc: &api.Services{Config: cfg, CloudSync: svc}}
	s := NewSidebar(true)
	tt := ui.NewTester(func(c *ui.Context) {
		e.FlushToasts(c)
		SideBar(c, theme.New(true, 1.5), SideBarProps{State: &s, Router: ui.NewRouter(env.PathLibrary), Config: cfg, Env: e})
	}, 1280, 800)
	tt.SetScale(1.5)
	tt.SetDark(true)
	return tt, &s, e
}

func syncConfig() *appconf.AppConfig {
	return &appconf.AppConfig{
		CloudBackupEnabled: true, CloudSyncEnabled: true, CloudBackupProvider: "webdav",
		BackupUserID: "u", WebDAVURL: "https://dav.example.com",
	}
}

func TestCloudSyncButtonSyncsNow(t *testing.T) {
	cfg := syncConfig()
	svc := &fakeCloudSync{status: vo.CloudSyncStatus{Enabled: true, Configured: true, LastSyncStatus: "idle"}}
	tt, _, _ := newCloudSyncFixture(t, cfg, svc)
	tt.Frame()
	if svc.reads == 0 {
		t.Fatal("status not read on first build")
	}
	if err := tt.Click(locales.SideBarCloudSync()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if svc.syncs != 1 {
		t.Fatalf("SyncNow called %d times", svc.syncs)
	}
	if !slices.Contains(tt.Announcements(), locales.SettingsCloudBackupToastSyncSuccess()) {
		t.Fatalf("no success toast: %q", tt.Announcements())
	}
}

func TestCloudSyncButtonReportsFailure(t *testing.T) {
	cfg := syncConfig()
	svc := &fakeCloudSync{status: vo.CloudSyncStatus{Enabled: true, Configured: true}, err: errors.New("boom")}
	tt, _, _ := newCloudSyncFixture(t, cfg, svc)
	tt.Frame()
	_ = tt.Click(locales.SideBarCloudSync())
	tt.Frame()
	if !slices.Contains(tt.Announcements(), locales.SettingsCloudBackupToastSyncFailed("boom")) {
		t.Fatalf("no failure toast: %q", tt.Announcements())
	}
}

func TestCloudSyncButtonIgnoresClicksWhenUnavailable(t *testing.T) {
	cases := map[string]func(*appconf.AppConfig, *fakeCloudSync){
		"sync off":       func(cfg *appconf.AppConfig, _ *fakeCloudSync) { cfg.CloudSyncEnabled = false },
		"not configured": func(cfg *appconf.AppConfig, _ *fakeCloudSync) { cfg.WebDAVURL = "" },
		"already syncing": func(_ *appconf.AppConfig, f *fakeCloudSync) {
			f.status.Syncing, f.status.SyncStage = true, "uploading_files"
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := syncConfig()
			svc := &fakeCloudSync{status: vo.CloudSyncStatus{Enabled: true, Configured: true}}
			edit(cfg, svc)
			if name == "not configured" {
				svc.status.Configured = false
			}
			tt, _, _ := newCloudSyncFixture(t, cfg, svc)
			tt.Frame()
			_ = tt.Click(locales.SideBarCloudSync())
			tt.Frame()
			if svc.syncs != 0 {
				t.Fatalf("SyncNow called %d times", svc.syncs)
			}
		})
	}
}

func TestCloudSyncStatusEventShowsProgress(t *testing.T) {
	cfg := syncConfig()
	svc := &fakeCloudSync{status: vo.CloudSyncStatus{Enabled: true, Configured: true}}
	tt, s, _ := newCloudSyncFixture(t, cfg, svc)
	tt.Frame()
	// 服务一边同步一边发事件；悬停时重新读到的也是同一份状态。
	svc.status = vo.CloudSyncStatus{Enabled: true, Configured: true, Syncing: true,
		SyncStage: "uploading_covers", SyncCurrent: 3, SyncTotal: 10, LastSyncStatus: "syncing"}
	s.Cloud.StatusChanged("webdav", svc.status)
	r, ok := tt.Find(locales.SideBarCloudSync())
	if !ok {
		t.Fatal("cloud sync button not found")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if !tt.HasText(locales.SettingsCloudBackupSyncDetailUploadingCovers("3", "10")) || !tt.HasText(locales.SettingsCloudBackupSyncStatusSyncing()) {
		t.Fatalf("progress not shown: %q", tt.Texts())
	}
}
