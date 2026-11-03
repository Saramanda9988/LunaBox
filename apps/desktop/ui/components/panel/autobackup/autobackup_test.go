package autobackup

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

// harness 模拟设置页：Change 改本地的一份配置并计数。
type harness struct {
	env     *env.Env
	cfg     appconf.AppConfig
	state   State
	changes int
	patches int // 面板只改草稿，不应调用 PatchLive
	tt      *ui.Tester
}

func newHarness(t *testing.T, setup func(cfg *appconf.AppConfig)) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{cfg: *svc.Config}
	h.cfg.CloudBackupEnabled = false
	h.cfg.ScheduledDBBackupEnabled = false
	if setup != nil {
		setup(&h.cfg)
	}
	h.env = &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: 1024}
	h.tt = ui.NewTester(h.view, 1024, 1600)
	h.tt.SetScale(1.5)
	h.tt.SetDark(true)
	h.frames(3)
	return h
}

func (h *harness) view(c *ui.Context) {
	t := h.env.Theme
	ui.Column(c).Fill().Padding(32).Background(t.AppBg).Children(func() {
		ui.Column(c).Width(800).Padding(20).Gap(16).Children(func() {
			View(c, t, &h.state, panel.Settings{
				Env:       h.env,
				Config:    &h.cfg,
				Change:    func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg); h.changes++ },
				PatchLive: func(fn func(cfg *appconf.AppConfig)) { fn(&h.cfg); h.patches++ },
				Refresh:   func() {},
			})
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

// typeNumber 点击 label 那一行右侧的 BetterNumberInput（size="sm"：w-25 h-9，与左侧的 label 与提示垂直居中），
// 全选后输入。面板内容区的右边缘是外层 p-8、内层 p-5 加宽 760；label 下面是一行或两行提示，行的中线在
// label 顶部往下 22 到 30 之间，点 label 顶部往下 22 总在输入框里。
func (h *harness) typeNumber(t *testing.T, label, text string) {
	t.Helper()
	r, ok := h.tt.Find(label)
	if !ok {
		t.Fatalf("no %q in %q", label, h.tt.Texts())
	}
	right := float32(32 + 20 + 760)
	if unit := locales.SettingsAutoBackupMinutesUnit(); label == locales.SettingsAutoBackupScheduleInterval() {
		u, _ := h.tt.Find(unit)
		right = u.X - 8
	} else if label == locales.SettingsCloudBackupSyncIntervalLabel() {
		u, _ := h.tt.Find(locales.SettingsCloudBackupSyncIntervalUnit())
		right = u.X - 8
	}
	h.tt.ClickAt(right-80, r.Y+22)
	h.frames(1)
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type(text)
	h.frames(2)
}

func TestSectionsAndDefaults(t *testing.T) {
	h := newHarness(t, nil)
	h.expect(t, locales.SettingsAutoBackupLocalSection(), locales.SettingsAutoBackupBackupDbOnExit(), locales.SettingsAutoBackupScheduledDbBackup(),
		locales.SettingsAutoBackupBackupSaveOnExit(), locales.SettingsAutoBackupLocalGameRetention(), locales.SettingsAutoBackupLocalDbRetention(),
		locales.SettingsAutoBackupCloudBackupSection(), locales.SettingsAutoBackupAutoUploadDb(), locales.SettingsAutoBackupRestoreCloudSaveBeforeLaunch(),
		locales.SettingsAutoBackupCloudGameRetention(), locales.SettingsAutoBackupCloudDbRetention(),
		locales.SettingsAutoBackupCloudSyncSection(), locales.SettingsCloudBackupSyncEnableLabel(), locales.SettingsCloudBackupAutoSyncEnableLabel(),
		locales.SettingsCloudBackupSyncIntervalLabel(), locales.SettingsCloudBackupSyncIntervalUnit())
	h.expectNot(t, locales.SettingsAutoBackupScheduleMode())
	// 云服务未启用：两处提示需要先启用云服务。
	if n := countText(h.tt, locales.SettingsAutoBackupCloudServiceRequiredHint()); n != 2 {
		t.Errorf("service required hint shown %d times", n)
	}

	h.click(t, locales.SettingsAutoBackupBackupDbOnExit())
	h.click(t, locales.SettingsAutoBackupBackupSaveOnExit())
	if !h.cfg.AutoBackupDB || !h.cfg.AutoBackupGameSave || h.changes != 2 || h.patches != 0 {
		t.Fatalf("db %v, save %v, changes %d, patches %d", h.cfg.AutoBackupDB, h.cfg.AutoBackupGameSave, h.changes, h.patches)
	}
}

func countText(tt *ui.Tester, s string) int {
	n := 0
	for _, x := range tt.Texts() {
		if x == s {
			n++
		}
	}
	return n
}

func TestScheduleControls(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.ScheduledDBBackupMode = ""
		cfg.ScheduledDBBackupIntervalMinutes = 0
	})
	h.click(t, locales.SettingsAutoBackupScheduledDbBackup())
	if !h.cfg.ScheduledDBBackupEnabled {
		t.Fatal("scheduled backup not enabled")
	}
	h.expect(t, locales.SettingsAutoBackupScheduleMode(), locales.SettingsAutoBackupScheduleModeInterval(),
		locales.SettingsAutoBackupScheduleInterval(), locales.SettingsAutoBackupMinutesUnit())

	// 间隔 15 到 10080 分钟：范围外的数字不提交，按 Enter 收回到范围内。
	h.typeNumber(t, locales.SettingsAutoBackupScheduleInterval(), "5")
	if h.cfg.ScheduledDBBackupIntervalMinutes != 0 {
		t.Fatalf("out-of-range draft committed: %d", h.cfg.ScheduledDBBackupIntervalMinutes)
	}
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if h.cfg.ScheduledDBBackupIntervalMinutes != 15 {
		t.Fatalf("interval %d after Enter, want 15", h.cfg.ScheduledDBBackupIntervalMinutes)
	}
	h.typeNumber(t, locales.SettingsAutoBackupScheduleInterval(), "99999")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if h.cfg.ScheduledDBBackupIntervalMinutes != 10080 {
		t.Fatalf("interval %d, want 10080", h.cfg.ScheduledDBBackupIntervalMinutes)
	}
	// ↓ 按步长 15 减少。
	h.tt.Key(0, ui.KeyDown)
	h.frames(2)
	if h.cfg.ScheduledDBBackupIntervalMinutes != 10065 {
		t.Fatalf("interval %d after Down, want 10065", h.cfg.ScheduledDBBackupIntervalMinutes)
	}

	// 换成每日：显示默认的 03:00，滚轮选择后写回草稿。
	h.click(t, locales.SettingsAutoBackupScheduleModeInterval())
	h.click(t, locales.SettingsAutoBackupScheduleModeDaily())
	if h.cfg.ScheduledDBBackupMode != appconf.ScheduledDBBackupModeDaily {
		t.Fatalf("mode %q", h.cfg.ScheduledDBBackupMode)
	}
	h.expect(t, locales.SettingsAutoBackupScheduleDailyTime(), locales.SettingsAutoBackupScheduleDailyTimeHint(), "03:00")
	h.expectNot(t, locales.SettingsAutoBackupScheduleInterval())
	h.click(t, "03:00")
	// 滚轮的数字是画出来的，不是文字：点小时列的中间一项（当前值，不改变）让它获得焦点，再按两次 ↓（等滚动停下再按下一次）。
	colon, ok := h.tt.Find(":")
	if !ok {
		t.Fatalf("time wheel not open: %q", h.tt.Texts())
	}
	h.tt.ClickAt(colon.X-60, colon.Y+colon.H/2)
	h.frames(1)
	for range 2 {
		h.tt.Key(0, ui.KeyDown)
		for range 20 {
			h.tt.Advance(50 * time.Millisecond)
			h.tt.Frame()
		}
	}
	if h.cfg.ScheduledDBBackupTime != "05:00" {
		t.Fatalf("time %q, want 05:00", h.cfg.ScheduledDBBackupTime)
	}
}

func TestLocalRetentionBounds(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.LocalDBBackupRetention = 3 })
	// 数据库至少保留 1 份：输入 0 后收回到 1。
	h.typeNumber(t, locales.SettingsAutoBackupLocalDbRetention(), "0")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if h.cfg.LocalDBBackupRetention != 1 {
		t.Fatalf("db retention %d, want 1", h.cfg.LocalDBBackupRetention)
	}
	h.typeNumber(t, locales.SettingsAutoBackupLocalDbRetention(), "7")
	if h.cfg.LocalDBBackupRetention != 7 {
		t.Fatalf("db retention %d, want 7", h.cfg.LocalDBBackupRetention)
	}
}

func TestCloudControlsDisabledWithoutService(t *testing.T) {
	h := newHarness(t, nil)
	for _, label := range []string{locales.SettingsAutoBackupAutoUploadDb(), locales.SettingsAutoBackupAutoUploadSave(),
		locales.SettingsAutoBackupRestoreCloudSaveBeforeLaunch(), locales.SettingsCloudBackupSyncEnableLabel()} {
		h.click(t, label)
	}
	if h.changes != 0 {
		t.Fatalf("disabled switches changed the draft %d times", h.changes)
	}
	// 云服务启用但没配置好：启动前同步存档仍禁用，同步间隔下提示先完成配置。
	h.cfg.CloudBackupEnabled = true
	h.cfg.CloudBackupProvider, h.cfg.UmbraAuthenticated = "umbra", false
	h.frames(2)
	h.expectNot(t, locales.SettingsAutoBackupCloudServiceRequiredHint())
	h.expect(t, locales.SettingsAutoBackupCloudSyncConfigRequiredHint())
	h.click(t, locales.SettingsAutoBackupRestoreCloudSaveBeforeLaunch())
	if h.cfg.AutoRestoreCloudSave {
		t.Fatal("restore before launch enabled without a configured provider")
	}
	h.click(t, locales.SettingsAutoBackupAutoUploadDb())
	if !h.cfg.AutoUploadDBToCloud {
		t.Fatal("auto upload not enabled with the cloud service on")
	}
	// 配置好以后可以开启。
	h.cfg.UmbraBaseURL, h.cfg.UmbraAuthenticated = "https://umbra.test", true
	h.frames(2)
	h.expectNot(t, locales.SettingsAutoBackupCloudSyncConfigRequiredHint())
	h.click(t, locales.SettingsAutoBackupRestoreCloudSaveBeforeLaunch())
	if !h.cfg.AutoRestoreCloudSave {
		t.Fatal("restore before launch not enabled")
	}
}

func TestCloudSyncSwitches(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.CloudBackupEnabled = true
		cfg.CloudSyncEnabled, cfg.AutoCloudSyncEnabled = false, false
		cfg.CloudSyncIntervalSec = 0
	})
	// 云同步关闭时自动同步禁用。
	h.click(t, locales.SettingsCloudBackupAutoSyncEnableLabel())
	if h.cfg.AutoCloudSyncEnabled {
		t.Fatal("auto sync enabled while sync is off")
	}
	h.click(t, locales.SettingsCloudBackupSyncEnableLabel())
	if !h.cfg.CloudSyncEnabled {
		t.Fatal("sync not enabled")
	}
	h.expect(t, locales.SettingsAutoBackupCloudSyncManualRecommendedHint())
	h.click(t, locales.SettingsCloudBackupAutoSyncEnableLabel())
	if !h.cfg.AutoCloudSyncEnabled {
		t.Fatal("auto sync not enabled")
	}
	h.expectNot(t, locales.SettingsAutoBackupCloudSyncManualRecommendedHint())

	// 间隔最少 15 秒（未设置时显示 60）。
	h.typeNumber(t, locales.SettingsCloudBackupSyncIntervalLabel(), "3")
	h.tt.Key(0, ui.KeyEnter)
	h.frames(2)
	if h.cfg.CloudSyncIntervalSec != 15 {
		t.Fatalf("interval %d, want 15", h.cfg.CloudSyncIntervalSec)
	}

	// 关闭云同步时一并关闭自动同步。
	h.click(t, locales.SettingsCloudBackupSyncEnableLabel())
	if h.cfg.CloudSyncEnabled || h.cfg.AutoCloudSyncEnabled {
		t.Fatalf("sync %v, auto %v", h.cfg.CloudSyncEnabled, h.cfg.AutoCloudSyncEnabled)
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把面板存成图片，用于目测。
func TestScreenshotDump(t *testing.T) {
	out := os.Getenv("DESKTOP_MYGO_PNG")
	if out == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.ScheduledDBBackupEnabled = true
		cfg.CloudBackupEnabled = true
		cfg.CloudSyncEnabled = true
	})
	h.frames(2)
	f, err := os.Create(filepath.Join(out, "autobackup.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, h.tt.Image()); err != nil {
		t.Fatal(err)
	}
}
