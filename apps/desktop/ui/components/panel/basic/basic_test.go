package basic

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/components/panel/account"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

const width = 1024

type harness struct {
	svc     *host.Backend
	cfg     appconf.AppConfig // 草稿
	env     *env.Env
	state   State
	tt      *ui.Tester
	changes int
	live    int // PatchLive 的次数
}

// newHarness 在真实服务（临时 DuckDB 与数据目录）上打开面板。Change 与 PatchLive 改本地的草稿，
// 外面套一层分区内容的 p-5 space-y-4。
func newHarness(t *testing.T, cfg func(*appconf.AppConfig)) *harness {
	t.Helper()
	svc := hosttest.NewServices(t)
	h := &harness{svc: svc, cfg: *svc.Config}
	if cfg != nil {
		cfg(&h.cfg)
	}
	h.env = &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: width}
	h.tt = ui.NewTester(h.view, width, 2400)
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
		PatchLive: func(fn func(*appconf.AppConfig)) { fn(&h.cfg); h.live++ },
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

// clickSwitch 点击 label 那一行右侧的开关（分区内容 p-5，开关 w-11）。
func (h *harness) clickSwitch(t *testing.T, label string) {
	t.Helper()
	r, ok := h.tt.Find(label)
	if !ok {
		t.Fatalf("missing %q", label)
	}
	h.tt.ClickAt(width-20-22, r.Y+r.H/2)
	h.frames(2)
}

func TestFields(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.Theme, cfg.Language, cfg.WindowZoomFactor, cfg.TimeZone = "dark", "zh-CN", 0, ""
	})
	h.expect(t, locales.SettingsBasicAccountAuthorizationSectionLabel(), vndbTokenLabel,
		locales.SettingsBasicThemeLabel(), locales.SettingsBasicThemeDark(),
		locales.SettingsBasicLanguageLabel(), "简体中文", // i18n:keep：测试数据
		locales.SettingsBasicZoomLabel(), "100%", locales.SettingsBasicZoomHint(),
		locales.SettingsBasicTimezoneLabel(), "China Standard Time (UTC+8)", locales.SettingsBasicTimezoneHint(),
		locales.SettingsBasicGameLibraryPath(), locales.SettingsBasicGameLibraryPathHint(),
		locales.SettingsBasicCloseToTray(), locales.SettingsBasicLaunchAtLogin())
}

func TestSelects(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) {
		cfg.Theme, cfg.Language, cfg.WindowZoomFactor, cfg.TimeZone = "dark", "zh-CN", 1, "UTC"
	})
	h.click(t, locales.SettingsBasicThemeDark())
	h.click(t, locales.SettingsBasicThemeLight())
	if h.cfg.Theme != "light" || h.changes != 1 {
		t.Fatalf("theme %q after %d changes", h.cfg.Theme, h.changes)
	}

	h.click(t, "简体中文") // i18n:keep：测试数据
	h.click(t, "English")
	if h.cfg.Language != "en-US" || h.changes != 2 {
		t.Fatalf("language %q after %d changes", h.cfg.Language, h.changes)
	}

	// 界面缩放走 patchLiveConfig，不改草稿的计时。
	h.click(t, "100%")
	h.click(t, "125%")
	if h.cfg.WindowZoomFactor != 1.25 || h.live != 1 || h.changes != 2 {
		t.Fatalf("zoom %v, live %d, changes %d", h.cfg.WindowZoomFactor, h.live, h.changes)
	}
	h.expect(t, "125%")

	h.click(t, "Coordinated Universal Time (UTC)")
	h.click(t, "Japan Standard Time (UTC+9)")
	if h.cfg.TimeZone != "Asia/Tokyo" {
		t.Fatalf("time zone %q", h.cfg.TimeZone)
	}
}

func TestZoomOptions(t *testing.T) {
	var labels, values []string
	for _, o := range appZoomOptions() {
		labels, values = append(labels, o.Label), append(values, o.Value)
	}
	if got := strconv.Quote(joined(labels)); got != strconv.Quote("80%,90%,100%,110%,125%,150%") {
		t.Errorf("labels %s", got)
	}
	if got := joined(values); got != "0.8,0.9,1,1.1,1.25,1.5" {
		t.Errorf("values %s", got)
	}
}

func joined(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}

func TestVNDBToken(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.VNDBAccessToken = "" })
	r, ok := h.tt.Find(vndbTokenLabel)
	if !ok {
		t.Fatal("missing VNDB label")
	}
	h.tt.ClickAt(200, r.Y+r.H+8+20) // label 下面 space-y-2 的输入框（h-10）
	h.frames(1)
	h.tt.Type("token-123")
	h.frames(2)
	if h.cfg.VNDBAccessToken != "token-123" {
		t.Fatalf("token %q", h.cfg.VNDBAccessToken)
	}
}

// TestSwitchRows：关闭到托盘只能点开关；开机自启的 label 有 htmlFor，点文字也切换。
func TestSwitchRows(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.CloseToTray, cfg.LaunchAtLogin = false, false })
	h.click(t, locales.SettingsBasicCloseToTray())
	if h.cfg.CloseToTray {
		t.Fatal("close-to-tray label toggled the switch")
	}
	h.clickSwitch(t, locales.SettingsBasicCloseToTray())
	if !h.cfg.CloseToTray {
		t.Fatal("close-to-tray switch did not toggle")
	}
	h.click(t, locales.SettingsBasicLaunchAtLogin())
	if !h.cfg.LaunchAtLogin {
		t.Fatal("launch-at-login label did not toggle")
	}
	h.clickSwitch(t, locales.SettingsBasicLaunchAtLogin())
	if h.cfg.LaunchAtLogin {
		t.Fatal("launch-at-login switch did not toggle back")
	}
}

// TestAccountExpandChoreography：handleAccountExpand 先淡出 100ms 再切换展开的卡片，有网格过渡时 196ms 后淡入；
// 切换期间的点击被忽略。
func TestAccountExpandChoreography(t *testing.T) {
	h := newHarness(t, nil)
	s := &h.state
	if s.expanded != "" || s.contentHidden {
		t.Fatalf("initial %q hidden=%v", s.expanded, s.contentHidden)
	}
	s.expandAccount(account.Bangumi, false, true)
	h.frames(1)
	if !s.contentHidden || s.expanded != "" {
		t.Fatalf("fade out: expanded %q hidden %v", s.expanded, s.contentHidden)
	}
	s.expandAccount(account.Kungal, false, true) // 切换期间忽略
	h.tt.Advance(99 * time.Millisecond)
	h.frames(1)
	if s.expanded != "" {
		t.Fatalf("expanded before the fade ended: %q", s.expanded)
	}
	h.tt.Advance(2 * time.Millisecond)
	h.frames(1)
	if s.expanded != account.Bangumi || !s.contentHidden {
		t.Fatalf("after fade: expanded %q hidden %v", s.expanded, s.contentHidden)
	}
	h.tt.Advance(194 * time.Millisecond)
	h.frames(1)
	if !s.contentHidden {
		t.Fatal("content shown before the grid transition ended")
	}
	h.tt.Advance(3 * time.Millisecond)
	h.frames(1)
	if s.contentHidden || s.phase != accountIdle {
		t.Fatalf("content still hidden after the transition: phase %v", s.phase)
	}
	if s.expandAccount(account.Bangumi, false, true); s.contentHidden {
		t.Error("expanding the expanded card started a transition")
	}
	if got := accountColumns(s.expanded); got != [3]float32{6, 2, 2} {
		t.Errorf("columns %v", got)
	}

	// 窗口窄于 640：没有网格过渡，切换后 16ms 淡入。
	s.expandAccount(account.Hikarinagi, false, false)
	h.frames(1)
	h.tt.Advance(101 * time.Millisecond)
	h.frames(1)
	if s.expanded != account.Hikarinagi || !s.contentHidden {
		t.Fatalf("narrow: expanded %q hidden %v", s.expanded, s.contentHidden)
	}
	h.tt.Advance(17 * time.Millisecond)
	h.frames(1)
	if s.contentHidden {
		t.Error("narrow: content still hidden after 16ms")
	}

	// 减少动画：直接切换，内容不隐藏。
	s.expandAccount(account.Kungal, true, true)
	if s.expanded != account.Kungal || s.contentHidden || s.phase != accountIdle {
		t.Errorf("reduced motion: expanded %q hidden %v phase %v", s.expanded, s.contentHidden, s.phase)
	}
	h.frames(2)

	// 重新进入设置页（重新挂载面板）：三张卡片回到等宽，进行中的切换作废，之后可以照常展开。
	s.expandAccount(account.Bangumi, false, true)
	h.frames(1)
	s.ResetAccounts()
	h.tt.Advance(300 * time.Millisecond)
	h.frames(2)
	if s.expanded != "" || s.contentHidden || s.phase != accountIdle {
		t.Fatalf("after reset: expanded %q hidden %v phase %v", s.expanded, s.contentHidden, s.phase)
	}
	if s.expandAccount(account.Hikarinagi, true, true); s.expanded != account.Hikarinagi {
		t.Errorf("expand after reset: %q", s.expanded)
	}
}

func TestNarrowWindowStacksAccounts(t *testing.T) {
	h := newHarness(t, nil)
	h.tt.SetSize(600, 2400)
	h.frames(2)
	h.expect(t, locales.SettingsBasicAccountAuthorizationSectionLabel())
}

// libraryFixture 是一个游戏库目录 old，里面两个游戏；new 里只有 GameA，GameB 的目标不存在。
type libraryFixture struct {
	oldDir, newDir string
}

func newLibrary(t *testing.T, h *harness) libraryFixture {
	t.Helper()
	root := t.TempDir()
	f := libraryFixture{oldDir: filepath.Join(root, "old"), newDir: filepath.Join(root, "new")}
	for _, d := range []string{filepath.Join(f.oldDir, "GameA"), filepath.Join(f.oldDir, "GameB"), filepath.Join(f.newDir, "GameA")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, g := range []struct{ id, name string }{{"lib-a", "GameA"}, {"lib-b", "GameB"}} {
		dir := filepath.Join(f.oldDir, g.name)
		hosttest.AddGame(t, h.svc, models.Game{ID: g.id, Name: g.name, Path: filepath.Join(dir, "game.exe"), GameDirectory: dir})
	}
	cfg := *h.svc.Config
	cfg.GameLibraryPath = f.oldDir
	if err := h.svc.Services.Config.UpdateAppConfig(cfg); err != nil {
		t.Fatal(err)
	}
	// GameA 已经移到新目录（目录与可执行文件都在），GameB 还没有。
	if err := os.WriteFile(filepath.Join(f.newDir, "GameA", "game.exe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	h.cfg.GameLibraryPath = f.oldDir
	h.frames(1)
	return f
}

// typeLibraryPath 在游戏库目录输入框里输入 path 并按 Enter。
func (h *harness) typeLibraryPath(t *testing.T, path string) {
	t.Helper()
	h.click(t, locales.SettingsBasicGameLibraryPathPlaceholder())
	h.tt.Key(ui.Cmd, ui.KeyA)
	h.tt.Type(path)
	h.frames(1)
	h.tt.Key(0, ui.KeyEnter)
	h.frames(3)
}

func TestLibraryChangeModal(t *testing.T) {
	h := newHarness(t, nil)
	f := newLibrary(t, h)

	h.typeLibraryPath(t, f.newDir)
	if h.state.libPreview == nil {
		t.Fatalf("modal not opened; texts %q", h.tt.Texts())
	}
	h.expect(t, locales.SettingsBasicLibraryChangeTitle(), locales.SettingsBasicLibraryChangeAffectedGamesDescription("2"),
		locales.SettingsBasicLibraryChangeMissingWarning("1"), locales.SettingsBasicLibraryChangePreviewTitle(),
		"GameA", "GameB", locales.SettingsBasicLibraryChangeSkipPathUpdate(), locales.SettingsBasicLibraryChangeConfirmChange())
	if got := len(h.state.libChanges); got != 2 {
		t.Fatalf("grouped changes %d", got)
	}
	if !h.state.libPending || h.state.libInput != f.newDir {
		t.Errorf("pending input %q (%v)", h.state.libInput, h.state.libPending)
	}

	// Esc 关闭弹窗并放弃输入。
	h.tt.Key(0, ui.KeyEscape)
	h.frames(2)
	if h.state.libPreview != nil || h.state.libPending {
		t.Fatalf("escape: preview %v pending %v", h.state.libPreview != nil, h.state.libPending)
	}
	if h.tt.HasText(locales.SettingsBasicLibraryChangeTitle()) {
		t.Error("modal still shown")
	}

	// 关闭按钮同样关闭。
	h.typeLibraryPath(t, f.newDir)
	h.click(t, locales.SettingsBasicLibraryChangeClose())
	if h.state.libPreview != nil {
		t.Fatal("close button did not close the modal")
	}

	// 确认更改：同步游戏路径，草稿与生效的配置都换成新目录。
	h.typeLibraryPath(t, f.newDir)
	h.click(t, locales.SettingsBasicLibraryChangeConfirmChange())
	h.frames(1)
	h.expect(t, locales.SettingsBasicLibraryChangeChangeSuccess("2"))
	if h.state.libPreview != nil || h.state.libPending || h.state.applying || h.state.previewLoading {
		t.Errorf("state after apply %+v", h.state)
	}
	if h.cfg.GameLibraryPath != f.newDir || h.svc.Config.GameLibraryPath != f.newDir || h.live != 1 {
		t.Errorf("library path draft %q, live %q, patches %d", h.cfg.GameLibraryPath, h.svc.Config.GameLibraryPath, h.live)
	}
	g, err := h.svc.Game.GetGameByID("lib-a")
	if err != nil || g.GameDirectory != filepath.Join(f.newDir, "GameA") || g.Path != filepath.Join(f.newDir, "GameA", "game.exe") {
		t.Errorf("game not rebased: %q %q %v", g.GameDirectory, g.Path, err)
	}
}

// 弹窗比窗口还高时整个遮罩滚动（overflow-y-auto）：点在弹窗外面仍然关闭，底部的按钮可以滚到窗口里。
func TestLibraryChangeModalScrollsInShortWindow(t *testing.T) {
	h := newHarness(t, nil)
	f := newLibrary(t, h)
	const winH = 360
	open := func() {
		h.tt.SetSize(width, 2400)
		h.frames(1)
		h.typeLibraryPath(t, f.newDir)
		h.tt.SetSize(width, winH)
		h.frames(3)
	}

	open()
	h.tt.ClickAt(4, winH/2) // 遮罩的左边缘
	h.frames(2)
	if h.state.libPreview != nil {
		t.Fatal("clicking the backdrop did not close the modal")
	}

	open()
	confirm := locales.SettingsBasicLibraryChangeConfirmChange()
	r, ok := h.tt.Find(confirm)
	if !ok || r.Y+r.H <= winH {
		t.Fatalf("confirm button %v (found %v) fits in a %d DIP window; the test needs a taller modal", r, ok, winH)
	}
	title, _ := h.tt.Find(locales.SettingsBasicLibraryChangeTitle())
	h.tt.Scroll(title.X, title.Y, 0, 400)
	h.frames(2)
	if r, _ = h.tt.Find(confirm); r.Y+r.H > winH {
		t.Fatalf("confirm button %v still below the window after scrolling", r)
	}
	h.click(t, confirm)
	h.frames(1)
	if h.state.libPreview != nil || h.cfg.GameLibraryPath != f.newDir {
		t.Fatalf("confirm after scrolling: preview %v, library %q", h.state.libPreview != nil, h.cfg.GameLibraryPath)
	}
}

func TestLibraryChangeSkipSync(t *testing.T) {
	h := newHarness(t, nil)
	f := newLibrary(t, h)
	h.typeLibraryPath(t, f.newDir)
	h.click(t, locales.SettingsBasicLibraryChangeSkipPathUpdate())
	h.frames(1)
	h.expect(t, locales.SettingsBasicLibraryChangeChangeWithoutSyncSuccess())
	if h.cfg.GameLibraryPath != f.newDir || h.svc.Config.GameLibraryPath != f.newDir {
		t.Errorf("library path draft %q, live %q", h.cfg.GameLibraryPath, h.svc.Config.GameLibraryPath)
	}
	g, err := h.svc.Game.GetGameByID("lib-a")
	if err != nil || g.GameDirectory != filepath.Join(f.oldDir, "GameA") {
		t.Errorf("game paths changed without sync: %q %v", g.GameDirectory, err)
	}
}

// TestLibraryNoAffectedGames：没有受影响的游戏时，Enter 直接保存新目录；“扫描路径”只提示。
func TestLibraryNoAffectedGames(t *testing.T) {
	h := newHarness(t, nil)
	root := t.TempDir()
	oldDir, newDir := filepath.Join(root, "empty-old"), filepath.Join(root, "empty-new")
	cfg := *h.svc.Config
	cfg.GameLibraryPath = oldDir
	if err := h.svc.Services.Config.UpdateAppConfig(cfg); err != nil {
		t.Fatal(err)
	}
	h.cfg.GameLibraryPath = oldDir
	h.frames(1)

	h.click(t, locales.SettingsBasicLibraryChangeScanPaths())
	h.frames(1)
	h.expect(t, locales.SettingsBasicLibraryChangeNoAffectedRecords())
	if h.state.libPreview != nil || h.live != 0 {
		t.Fatalf("scan opened the modal or saved: %v %d", h.state.libPreview != nil, h.live)
	}

	h.typeLibraryPath(t, newDir)
	h.frames(1)
	h.expect(t, locales.SettingsBasicLibraryChangeChangeWithoutAffectedGamesSuccess())
	if h.cfg.GameLibraryPath != newDir || h.svc.Config.GameLibraryPath != newDir || h.state.libPending {
		t.Errorf("draft %q, live %q, pending %v", h.cfg.GameLibraryPath, h.svc.Config.GameLibraryPath, h.state.libPending)
	}
}

func TestLibraryEscapeDiscardsInput(t *testing.T) {
	h := newHarness(t, func(cfg *appconf.AppConfig) { cfg.GameLibraryPath = `C:\Games` })
	h.click(t, locales.SettingsBasicGameLibraryPathPlaceholder())
	h.tt.Type("x")
	h.frames(1)
	if !h.state.libPending {
		t.Fatal("typing did not set the pending input")
	}
	h.tt.Key(0, ui.KeyEscape)
	h.frames(1)
	if h.state.libPending || h.cfg.GameLibraryPath != `C:\Games` {
		t.Errorf("escape: pending %v, path %q", h.state.libPending, h.cfg.GameLibraryPath)
	}
}

func TestLibraryPreviewFailed(t *testing.T) {
	h := newHarness(t, nil)
	file := filepath.Join(t.TempDir(), "not-a-dir.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.typeLibraryPath(t, file) // 游戏库地址指向文件
	h.frames(1)
	h.expect(t, locales.SettingsBasicLibraryChangePreviewFailed())
	if h.state.libPreview != nil || h.state.previewLoading {
		t.Errorf("state after failure %+v", h.state)
	}
}

func TestSelectLibraryFolder(t *testing.T) {
	h := newHarness(t, nil)
	f := newLibrary(t, h)
	old := selectDirectory
	t.Cleanup(func() { selectDirectory = old })
	var titles []string
	selectDirectory = func(_ *env.Env, title string) (string, error) {
		titles = append(titles, title)
		return f.newDir, nil
	}
	h.click(t, locales.SettingsBasicSelectGameLibraryTitle())
	if len(titles) != 1 || titles[0] != locales.SettingsBasicSelectGameLibraryTitle() {
		t.Fatalf("dialog titles %q", titles)
	}
	if h.state.libPreview == nil || h.state.libInput != f.newDir {
		t.Fatalf("selected folder not previewed: %q", h.state.libInput)
	}

	selectDirectory = func(*env.Env, string) (string, error) { return "", errors.New("boom") }
	h.tt.Key(0, ui.KeyEscape)
	h.frames(2)
	h.click(t, locales.SettingsBasicSelectGameLibraryTitle())
	h.frames(1)
	h.expect(t, locales.SettingsBasicSelectGameLibraryFailed())
}

func TestBlockedChangeDisablesConfirm(t *testing.T) {
	h := newHarness(t, nil)
	h.state.showLibraryChange(service.GameLibraryPathChangePreview{
		NewConfiguredPath: `D:\New`, OldLibraryPath: `C:\Old`, NewLibraryPath: `D:\New`,
		AffectedGameCount: 1, BlockingDownloadTaskCount: 2, SteamGameCount: 1,
		Changes: []service.GameLibraryPathChangeItem{
			{RecordType: "game", RecordID: "1", RecordName: "Steam Game", Field: "path", OldPath: `C:\Old\S\s.exe`, NewPath: `D:\New\S\s.exe`, TargetExists: true},
			{RecordType: "game", RecordID: "1", RecordName: "Steam Game", Field: "game_directory", OldPath: `C:\Old\S`, NewPath: `D:\New\S`, TargetExists: true},
			{RecordType: "download_task", RecordID: "t", RecordName: "task", Field: "file_path", OldPath: `C:\Old\t.zip`, NewPath: `D:\New\t.zip`},
		},
	})
	h.frames(2)
	h.expect(t, locales.SettingsBasicLibraryChangeDownloadBlocked("2"), locales.SettingsBasicLibraryChangeSteamWarning("1"), "Steam Game")
	if h.tt.HasText(locales.SettingsBasicLibraryChangeMissingWarning("1")) {
		t.Error("missing warning shown while every target exists")
	}
	h.click(t, locales.SettingsBasicLibraryChangeConfirmChange())
	if h.state.libPreview == nil || h.live != 0 {
		t.Error("confirm worked while downloads block the change")
	}
	if got := h.state.libChanges; len(got) != 1 || got[0].oldPath != `C:\Old\S` {
		t.Errorf("grouped %+v", got)
	}
}

func TestSplitRebasedPath(t *testing.T) {
	for _, tc := range []struct{ old, new, oldLib, newLib, wantOld, wantNew, wantSuffix string }{
		{`C:\Games\A\a.exe`, `D:\Lib\A\a.exe`, `C:\Games`, `D:\Lib`, `C:\Games`, `D:\Lib`, `\A\a.exe`},
		{`c:\games\A`, `D:\Lib\a`, `C:\Games`, `D:\Lib`, `C:\Games`, `D:\Lib`, `\a`},
		{`C:\GamesX\A`, `D:\Lib\A`, `C:\Games`, `D:\Lib`, `C:\GamesX\A`, `D:\Lib\A`, ""},
		{`C:\Games\A`, `D:\Lib\B`, `C:\Games`, `D:\Lib`, `C:\Games\A`, `D:\Lib\B`, ""},
	} {
		o, n, s := splitRebasedPath(tc.old, tc.new, tc.oldLib, tc.newLib)
		if o != tc.wantOld || n != tc.wantNew || s != tc.wantSuffix {
			t.Errorf("splitRebasedPath(%q, %q) = %q %q %q", tc.old, tc.new, o, n, s)
		}
	}
}
