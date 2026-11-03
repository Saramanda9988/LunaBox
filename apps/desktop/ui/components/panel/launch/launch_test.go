package launch

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
	"lunabox/backend/service"
	"lunabox/backend/utils/processutils"
	"lunabox/desktop-mygo/api"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/host/hosttest"
	"lunabox/desktop-mygo/ui/components/panel"
	"lunabox/desktop-mygo/ui/covers"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/locales"
	"lunabox/desktop-mygo/ui/theme"
)

const testGameID = "g1"

// fixture 像详情页一样持有游戏、面板状态与启动器：OnChange 直接替换游戏，OnSaved 从数据库重新读取。
type fixture struct {
	svc       *host.Backend
	env       *env.Env
	game      models.Game
	state     State
	launcher  *Launcher
	changes   int
	saves     int
	tt        *ui.Tester
	announced []string
	width     int
}

// newFixture 打开一个路径不存在的游戏（启动会在找可执行文件时失败，不会真的启动任何程序）。
func newFixture(t *testing.T, width, height int) *fixture {
	t.Helper()
	svc := hosttest.NewServices(t)
	hosttest.AddGame(t, svc, models.Game{ID: testGameID, Name: "千恋＊万花", Path: `Z:\lunabox-missing\Launcher.exe`})
	g, err := svc.Game.GetGameByID(testGameID)
	if err != nil {
		t.Fatal(err)
	}
	e := &env.Env{Svc: svc.API(), Covers: covers.New(), Theme: theme.New(true, 1.5), Router: ui.NewRouter("/"), MainWidth: float32(width)}
	f := &fixture{svc: svc, env: e, game: g, launcher: NewLauncher(e), width: width}
	// 默认对照 Windows 界面（管理员启动、导出快捷方式、启动增强工具、“请先退出 Steam”）：
	// 要测 Linux 界面的用 useGOOS("linux")，这样测试不依赖跑测试的机器。
	f.launcher.goos = "windows"
	f.tt = ui.NewTester(f.view, width, height)
	f.tt.SetScale(1.5)
	f.tt.SetDark(true)
	f.frames(2)
	return f
}

func (f *fixture) panel() panel.Game {
	return panel.Game{
		Env:  f.env,
		Game: &f.game,
		OnChange: func(g models.Game) {
			f.game = g
			f.changes++
		},
		OnSaved: func() {
			f.saves++
			if g, err := f.svc.Game.GetGameByID(f.game.ID); err == nil {
				f.game = g
			}
		},
	}
}

// view 是详情页里标签页所在的位置：p-8 的主内容区，标签页之后是启动器的弹窗。
func (f *fixture) view(c *ui.Context) {
	t := f.env.Theme
	ui.Column(c).Fill().Padding(32).Gap(16).Background(t.AppBg).Children(func() {
		start := ui.ButtonBase(c.Key("test-start")).Size(80, 24).Label("测试启动")
		start.OnClick(func() { f.launcher.Start(c, f.panel(), enums.LaunchModeNormal) })
		ui.ButtonBase(c.Key("test-card-start")).Size(80, 24).Label("测试卡片启动").OnClick(func() { f.launcher.StartFromCard(c, f.game) })
		View(c, t, &f.state, f.panel())
	})
	f.launcher.Modals(c, t, f.panel())
}

func (f *fixture) frames(n int) {
	for range n {
		f.tt.Frame()
	}
}

func (f *fixture) click(t *testing.T, s string) {
	t.Helper()
	if err := f.tt.Click(s); err != nil {
		t.Fatalf("%v in %q", err, f.tt.Texts())
	}
	f.frames(2)
}

func (f *fixture) expect(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !f.tt.HasText(s) {
			t.Errorf("missing %q in %q", s, f.tt.Texts())
		}
	}
}

// toasts 返回到目前为止的提示（Toast 会读给屏幕阅读器）。
func (f *fixture) toasts() []string {
	f.announced = append(f.announced, f.tt.Announcements()...)
	return f.announced
}

func (f *fixture) expectToast(t *testing.T, msg string) {
	t.Helper()
	if !slices.Contains(f.toasts(), msg) {
		t.Errorf("no toast %q in %q", msg, f.announced)
	}
}

func (f *fixture) expectNot(t *testing.T, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if f.tt.HasText(s) {
			t.Errorf("unexpected %q", s)
		}
	}
}

func TestShowsWindowsLaunchConfig(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	f.expect(t, "进程监控", "默认启动方式", "普通启动", "可执行程序", "Launcher.exe", "实际游戏进程", "选择运行进程",
		"导出快捷启动方式", "启动增强工具", "Locale Emulator", "Magpie",
		"使用日文环境模拟启动游戏，解决乱码和区域限制问题。", "游戏启动后自动启动 Magpie 进行全屏超分辨率缩放。")
	f.expectNot(t, "Steam 启动器", "Steam 启动参数")
	cfg := f.env.Config()
	if cfg.LocaleEmulatorPath == "" {
		f.expect(t, "请先在设置中配置 LEProc.exe 路径")
	}
	if cfg.MagpiePath == "" {
		f.expect(t, "请先在设置中配置 Magpie.exe 路径")
	}

	f.game.Path = ""
	f.frames(1)
	f.expect(t, "未设置游戏路径")
}

func TestLaunchModeSelect(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	f.click(t, "普通启动")
	f.click(t, "管理员启动")
	if f.game.LaunchMode != enums.LaunchModeAdmin {
		t.Fatalf("launch mode = %q", f.game.LaunchMode)
	}
	f.expect(t, "管理员启动")
	// 兼容层启动在 Windows 上按普通启动显示。
	f.game.LaunchMode = enums.LaunchModeCompatibility
	f.frames(1)
	f.expect(t, "普通启动")
}

func TestSteamCard(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	f.game.LaunchMode = enums.LaunchModeSteam
	f.frames(2)
	f.expect(t, "Steam 启动", "Steam 启动器", "读取 Steam 的兼容工具设置，可为此游戏指定 Proton 版本。", "Steam 启动参数",
		"强制中文环境", "完整中文 Locale", "开启 Proton 日志", "Magpie")
	f.expectNot(t, "Locale Emulator")

	// 预设只改输入框里的草稿，保存到 Steam 时才写入。
	f.click(t, "完整中文 Locale")
	if d := f.state.optionsDraft; d == nil || *d != "LC_ALL=zh_CN.UTF-8 LANG=zh_CN.UTF-8 %command%" || f.game.SteamLaunchOptions != "" {
		t.Fatalf("draft %v, game options %q", d, f.game.SteamLaunchOptions)
	}
	f.state.optionsDraft = nil

	// 刷新只合并 Steam 相关字段：页面里没保存的进程名保留，数据库里的启动参数读回来。
	if _, err := f.svc.DB.Exec(`UPDATE games SET steam_launch_options = 'PROTON_LOG=1 %command%' WHERE id = ?`, testGameID); err != nil {
		t.Fatal(err)
	}
	f.game.ProcessName = "unsaved.exe"
	f.click(t, "刷新 Steam 设置")
	if f.game.SteamLaunchOptions != "PROTON_LOG=1 %command%" || f.game.ProcessName != "unsaved.exe" {
		t.Fatalf("after refresh: options %q, process %q", f.game.SteamLaunchOptions, f.game.ProcessName)
	}
}

func TestEditsProcessName(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	r, _ := f.tt.Find("实际游戏进程")
	f.tt.ClickAt(r.X+40, r.Y+r.H+4+20) // 标签 mb-1 下面 42 高的输入框
	f.frames(1)
	f.tt.Type("game.exe")
	f.frames(2)
	if f.game.ProcessName != "game.exe" {
		t.Fatalf("process name = %q", f.game.ProcessName)
	}
	// 页面的游戏从别处改变时，输入框跟着变。
	f.game.ProcessName = "other.exe"
	f.frames(2)
	f.tt.Type("!")
	f.frames(2)
	if !strings.HasPrefix(f.game.ProcessName, "other.ex") || len(f.game.ProcessName) != len("other.exe!") {
		t.Fatalf("process name after outside change = %q", f.game.ProcessName)
	}
}

func TestEnhancementSwitches(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	cfg := f.env.Config()
	oldLE, oldMagpie := cfg.LocaleEmulatorPath, cfg.MagpiePath
	t.Cleanup(func() { cfg.LocaleEmulatorPath, cfg.MagpiePath = oldLE, oldMagpie })

	clickSwitch := func(label string) {
		t.Helper()
		r, ok := f.tt.Find(label)
		if !ok {
			t.Fatalf("missing %q", label)
		}
		// 开关在卡片右侧（p-6），与左侧两行文字（20 + 4 + 16）垂直居中。
		f.tt.ClickAt(float32(f.width)-32-24-22, r.Y+20)
		f.frames(2)
	}

	cfg.LocaleEmulatorPath, cfg.MagpiePath = "", ""
	f.frames(1)
	clickSwitch("Locale Emulator")
	if f.game.UseLocaleEmulator {
		t.Fatal("disabled switch toggled")
	}

	cfg.LocaleEmulatorPath, cfg.MagpiePath = `C:\LE\LEProc.exe`, `C:\Magpie\Magpie.exe`
	f.frames(2)
	f.expectNot(t, "请先在设置中配置 LEProc.exe 路径", "请先在设置中配置 Magpie.exe 路径")
	clickSwitch("Locale Emulator")
	clickSwitch("Magpie")
	if !f.game.UseLocaleEmulator || !f.game.UseMagpie {
		t.Fatalf("le %v magpie %v", f.game.UseLocaleEmulator, f.game.UseMagpie)
	}
}

func TestExportShortcutFailureToast(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	// 测试里服务没有窗口运行时，保存对话框直接报错。
	// 第一个“导出快捷启动方式”是说明框的标题，按钮在说明框右侧，与两行文字垂直居中。
	r, _ := f.tt.Find("导出快捷启动方式")
	f.tt.ClickAt(float32(f.width)-32-24-16-40, r.Y+22)
	f.frames(2)
	if !slices.ContainsFunc(f.toasts(), func(s string) bool { return strings.HasPrefix(s, "导出快捷启动方式失败: ") }) {
		t.Errorf("no failure toast in %q", f.announced)
	}
}

func TestProcessSelectModal(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	f.click(t, "选择运行进程")
	f.expect(t, "选择游戏进程", "请从当前列表中选择实际运行的游戏进程。保存选择后，重新启动游戏即可按该进程监控。", "刷新列表", "取消", "确认选择")
	if !f.state.process.open || f.state.process.loading {
		t.Fatalf("modal open %v loading %v", f.state.process.open, f.state.process.loading)
	}

	f.state.process.processes = []processutils.ProcessInfo{{Name: "explorer.exe", PID: 100}, {Name: "SenrenBanka.exe", PID: 4242}}
	f.frames(1)
	f.expect(t, "explorer.exe", "PID:100", "SenrenBanka.exe", "PID:4242")

	f.click(t, "搜索进程...")
	f.tt.Type("senren")
	f.frames(2)
	f.expectNot(t, "explorer.exe")
	f.tt.Type("zzz")
	f.frames(2)
	f.expect(t, "未找到匹配的进程")
	for range 3 {
		f.tt.Key(0, ui.KeyBackspace)
	}
	f.frames(2)

	f.click(t, "SenrenBanka.exe")
	if f.state.process.selected != "SenrenBanka.exe" {
		t.Fatalf("selected %q", f.state.process.selected)
	}
	f.click(t, "确认选择")
	if f.state.process.open || f.game.ProcessName != "SenrenBanka.exe" {
		t.Fatalf("open %v, process %q", f.state.process.open, f.game.ProcessName)
	}
	f.expectToast(t, "已设置监控进程: SenrenBanka.exe")
	saved, err := f.svc.Game.GetGameByID(testGameID)
	if err != nil || saved.ProcessName != "SenrenBanka.exe" {
		t.Fatalf("saved process %q, %v", saved.ProcessName, err)
	}

	f.click(t, "选择运行进程")
	f.click(t, "取消")
	if f.state.process.open {
		t.Fatal("cancel did not close")
	}
}

func TestStartFailure(t *testing.T) {
	f := newFixture(t, 1024, 1400)
	f.click(t, "测试启动")
	f.expectToast(t, "千恋＊万花 启动失败, 查询日志获得帮助")
	if running, _ := f.launcher.Status(testGameID); running {
		t.Error("failed start left a runtime")
	}
	if f.saves != 0 {
		t.Errorf("reloaded %d times after a failed start", f.saves)
	}
}

func TestRuntimeEvents(t *testing.T) {
	f := newFixture(t, 1024, 600)
	l := f.launcher
	emit := func(session string, state service.GameRuntimeState) {
		f.svc.Events.Emit(runtimeEvent, service.GameRuntimeChangedEvent{
			GameID: testGameID, SessionID: session, StartTime: time.Now(), State: state, Game: &f.game,
		})
	}
	check := func(wantRunning, wantEnding bool) {
		t.Helper()
		if running, ending := l.Status(testGameID); running != wantRunning || ending != wantEnding {
			t.Fatalf("status = %v, %v; want %v, %v", running, ending, wantRunning, wantEnding)
		}
	}
	check(false, false)
	emit("s1", service.GameRuntimeStateLaunching)
	check(true, false)
	emit("s1", service.GameRuntimeStatePlaying)
	check(true, false)
	emit("s1", service.GameRuntimeStateEnding)
	check(true, true)
	emit("s1", service.GameRuntimeStatePlaying) // 同一会话的迟到事件不回退
	check(true, true)
	emit("s2", service.GameRuntimeStateIdle) // 别的会话结束不影响
	check(true, true)
	emit("s1", service.GameRuntimeStateIdle)
	check(false, false)

	// 正在运行时启动按钮什么也不做。
	emit("s3", service.GameRuntimeStatePlaying)
	f.click(t, "测试启动")
	if len(f.toasts()) != 0 {
		t.Errorf("toasts %q", f.announced)
	}
	// 没有游戏 ID 的 idle 清空全部。
	f.svc.Events.Emit(runtimeEvent, &service.GameRuntimeChangedEvent{State: service.GameRuntimeStateIdle})
	check(false, false)
}

// 乐观的 launching 只在启动失败且仍是自己写入的那一条时回滚：后端事件已经到来时保留。
func TestOptimisticRuntimeRollback(t *testing.T) {
	f := newFixture(t, 1024, 600)
	l := f.launcher
	l.runtimes[testGameID] = &runtimeInfo{state: service.GameRuntimeStateLaunching, optimistic: 7}
	l.applyEvent(service.GameRuntimeChangedEvent{GameID: testGameID, SessionID: "s1", StartTime: time.Now(), State: service.GameRuntimeStateLaunching})
	if r := l.runtimes[testGameID]; r == nil || r.optimistic != 0 || r.sessionID != "s1" {
		t.Fatalf("runtime = %+v", r)
	}
}

func TestSteamImportModal(t *testing.T) {
	f := newFixture(t, 1024, 900)
	l := f.launcher
	open := func(state string, action steamAction) {
		l.steam = steamFlow{open: true, status: &service.SteamLaunchStatus{State: state}, action: action, gameID: testGameID}
		f.frames(2)
	}
	open("needs_import", steamLaunch)
	f.expect(t, "添加到 Steam", "《千恋＊万花》尚未添加到当前 Steam 账号。LunaBox 可以将该游戏写入 Steam 的非 Steam 游戏列表。",
		"本机 Steam 关联与游戏元数据来源相互独立，并且仅保存在此设备。原有封面、标签和元数据来源保持原样。", "取消")
	f.click(t, "取消")
	if l.steam.open || l.steam.action != steamNone {
		t.Fatalf("after cancel: %+v", l.steam)
	}

	open("steam_running", steamSaveDefault)
	f.expect(t, "请先退出 Steam", "重新检查")
	f.expectNot(t, "导入并重启 Steam")
	open("executable_required", steamLaunch)
	f.expect(t, "需要游戏可执行程序", "选择可执行程序")
	open("user_unavailable", steamLaunch)
	f.expect(t, "无法识别 Steam 账号", "重新检查")
	open("steam_not_installed", steamLaunch)
	f.expect(t, "未检测到 Steam", "请先安装 Steam，或确认当前系统账号能够访问 Steam 安装信息。")
	open("weird", steamLaunch)
	f.expect(t, "Steam 返回了无法识别的状态，请稍后重试。")

	// 换到另一个游戏时弹窗关闭（原版随页面卸载）。
	f.game.ID = "other"
	f.frames(2)
	if l.steam.open {
		t.Fatal("modal stayed open for another game")
	}
}

// Steam 关联就绪后改默认启动方式：合并启动身份，交给页面保存。
func TestSaveSteamAsDefault(t *testing.T) {
	f := newFixture(t, 1024, 600)
	l := f.launcher
	g := f.game
	g.SteamLaunchID, g.SteamLaunchKind, g.SteamUserID = "123", "shortcut", "42"
	l.saveSteamAsDefault(g)
	if f.game.LaunchMode != enums.LaunchModeSteam || f.game.SteamLaunchID != "123" || f.game.SteamUserID != "42" {
		t.Fatalf("game = %+v", f.game)
	}
}

// DESKTOP_MYGO_PNG=<目录> 时把标签页与弹窗渲染成图片（深色，150%），用于与原版目测对照。
func TestScreenshotDump(t *testing.T) {
	dir := os.Getenv("DESKTOP_MYGO_PNG")
	if dir == "" {
		t.Skip("DESKTOP_MYGO_PNG not set")
	}
	save := func(f *fixture, name string) {
		f.frames(3)
		out, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		if err := png.Encode(out, f.tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	f := newFixture(t, 1024, 1100)
	save(f, "launch-normal.png")

	f.game.LaunchMode = enums.LaunchModeSteam
	f.game.SteamLaunchOptions = "LANG=zh_CN.UTF-8 %command%"
	save(f, "launch-steam.png")

	f.game.LaunchMode = enums.LaunchModeNormal
	f.click(t, "选择运行进程")
	f.state.process.processes = []processutils.ProcessInfo{
		{Name: "explorer.exe", PID: 1204}, {Name: "SenrenBanka.exe", PID: 4242}, {Name: "steam.exe", PID: 8812},
		{Name: "Code.exe", PID: 10240}, {Name: "chrome.exe", PID: 13020}, {Name: "Discord.exe", PID: 15872},
		{Name: "WeChat.exe", PID: 16384},
	}
	f.state.process.selected = "SenrenBanka.exe"
	save(f, "launch-process-modal.png")
	f.click(t, "取消")

	f.launcher.steam = steamFlow{open: true, status: &service.SteamLaunchStatus{State: "needs_import"}, action: steamLaunch, gameID: testGameID}
	save(f, "launch-steam-modal.png")
	f.launcher.closeSteam()

	f.state.applyConfirm = true
	save(f, "launch-apply-confirm.png")
	f.state.applyConfirm = false

	fake := f.useGOOS("linux")
	f.game.LaunchMode = enums.LaunchModeSteam
	save(f, "launch-linux-steam.png")
	f.game.LaunchMode = enums.LaunchModeNormal
	f.game.WineRunner = "proton:ge"
	save(f, "launch-linux-wine.png")
	f.launcher.steam = steamFlow{open: true, status: &service.SteamLaunchStatus{State: "steam_running"}, action: steamLaunch, gameID: testGameID, compat: &fake.compat}
	save(f, "launch-linux-steam-modal.png")
}

// fakeIntegration 替换 Steam 与兼容层相关的服务：测试在 Windows 上运行，Linux 的 Steam、Proton 与兼容层工具不可用。
type fakeIntegration struct {
	api.IntegrationService
	compat   service.SteamCompatibilityInfo
	tools    service.GameCompatibilityToolsInfo
	proton   []service.LocalProtonTool
	applied  []appliedSteam
	imported []appliedSteam
	opened   []string
	compatN  int
	toolsN   int
}

type appliedSteam struct {
	options string
	tool    *string
}

func (f *fakeIntegration) GetGameSteamCompatibility(string) (service.SteamCompatibilityInfo, error) {
	f.compatN++
	return f.compat, nil
}

func (f *fakeIntegration) GetLocalProtonTools() []service.LocalProtonTool { return f.proton }

func (f *fakeIntegration) GetGameCompatibilityTools(string) (service.GameCompatibilityToolsInfo, error) {
	f.toolsN++
	return f.tools, nil
}

func (f *fakeIntegration) OpenGameCompatibilityTool(_ string, action string) (string, error) {
	f.opened = append(f.opened, action)
	return "", nil
}

func (f *fakeIntegration) ApplyGameSteamSettings(_ string, tool *string, options string) error {
	f.applied = append(f.applied, appliedSteam{options, tool})
	return nil
}

func (f *fakeIntegration) ImportGameToSteamWithOptions(_ string, options, tool string) (service.SteamImportResult, error) {
	f.imported = append(f.imported, appliedSteam{options, &tool})
	return service.SteamImportResult{Status: service.SteamLaunchStatus{State: "ready", Ready: true, LaunchID: "9", LaunchKind: "shortcut"}, Imported: true}, nil
}

// useGOOS 让面板按另一个平台显示，Steam 与兼容层服务换成 fakeIntegration。
func (f *fixture) useGOOS(goos string) *fakeIntegration {
	fake := &fakeIntegration{
		IntegrationService: f.env.Svc.Integration,
		compat: service.SteamCompatibilityInfo{
			Supported: true, SteamInstalled: true, SteamRoot: "/home/luna/.steam/steam", AppID: "3001", DefaultTool: "proton_9",
			ProtonPrefix: "/home/luna/.steam/steam/steamapps/compatdata/3001/pfx",
			Tools:        []service.SteamCompatibilityTool{{Name: "proton_9", DisplayName: "Proton 9.0"}, {Name: "GE-Proton10", DisplayName: "GE-Proton10"}},
		},
		tools:  service.GameCompatibilityToolsInfo{Supported: true, RunnerKind: "proton", PrefixPath: "/home/luna/prefix", Actions: []string{"prefix_dir", "winecfg"}},
		proton: []service.LocalProtonTool{{ID: "ge", Name: "GE-Proton10", DisplayName: "GE-Proton10", Source: "steam"}},
	}
	f.env.Svc.Integration = fake
	f.launcher.goos = goos
	f.state = State{}
	f.frames(2)
	return fake
}

func TestLaunchOptionsByPlatform(t *testing.T) {
	l := &Launcher{goos: "linux"}
	if o := l.launchOptions(enums.LaunchModeAdmin); *o.RunAsAdmin || *o.UseCompatibility || *o.UseSteam {
		t.Errorf("admin on linux = %+v", o)
	}
	if o := l.launchOptions(enums.LaunchModeCompatibility); !*o.UseCompatibility || *o.RunAsAdmin {
		t.Errorf("compatibility = %+v", o)
	}
	native := &models.Game{SteamLaunchKind: "native", SteamLaunchID: "1"}
	for _, c := range []struct {
		goos        string
		game        *models.Game
		mode, wants enums.LaunchMode
	}{
		{"windows", nil, enums.LaunchModeAdmin, enums.LaunchModeAdmin},
		{"windows", nil, enums.LaunchModeCompatibility, enums.LaunchModeNormal},
		{"linux", nil, enums.LaunchModeAdmin, enums.LaunchModeNormal},
		{"linux", nil, enums.LaunchModeSteam, enums.LaunchModeSteam},
		{"darwin", nil, enums.LaunchModeCompatibility, enums.LaunchModeCompatibility},
		{"darwin", &models.Game{}, enums.LaunchModeSteam, enums.LaunchModeNormal},
		{"darwin", native, enums.LaunchModeSteam, enums.LaunchModeSteam},
		{"darwin", nil, "", enums.LaunchModeNormal},
	} {
		l.goos = c.goos
		if got := l.EffectiveMode(c.mode, c.game); got != c.wants {
			t.Errorf("%s %q = %q, want %q", c.goos, c.mode, got, c.wants)
		}
	}
}

// Linux：没有导出快捷方式、增强工具与选择进程，Windows 程序默认用系统 Wine，有兼容层快捷工具。
func TestLinuxWineConfig(t *testing.T) {
	f := newFixture(t, 1024, 1600)
	fake := f.useGOOS("linux")
	f.expect(t, locales.GameLaunchCompatibilityTools(), locales.GameLaunchCompatibilityEngine(), locales.GameLaunchWineRunnerSystem(),
		locales.GameLaunchWineArgs(), locales.GameLaunchWinePrefix(), locales.GameLaunchProcessHintDarwin(),
		locales.GameLaunchCompatibilityQuickTools(), "/home/luna/prefix", locales.GameLaunchCompatibilityActionProtoncfg(),
		locales.GameLaunchCompatibilityQuickToolsRunner("proton"))
	f.expectNot(t, locales.GameLaunchExportShortcut(), locales.GameLaunchEnhancementTools(), locales.GameLaunchSelectRunningProcess(),
		locales.GameLaunchLaunchModeAdmin(), "Locale Emulator")
	if fake.toolsN != 1 || fake.compatN != 1 {
		t.Errorf("tools loaded %d times, compat %d", fake.toolsN, fake.compatN)
	}

	f.click(t, locales.Key("gameLaunch.steamLaunchOptionsPresets.chineseLocale"))
	if f.game.WineArgs != "LANG=zh_CN.UTF-8 %command%" {
		t.Fatalf("wine args = %q", f.game.WineArgs)
	}
	f.expectNot(t, locales.Key("gameLaunch.steamLaunchOptionsPresets.protonLog"))

	// 换成本机的 Proton：prefix 清空，标签换成 compatdata，多了 Proton 日志预设；依赖变了，快捷工具重新读取。
	f.game.WinePrefix = "/old/prefix"
	f.click(t, locales.GameLaunchWineRunnerSystem())
	f.click(t, "GE-Proton10 · steam")
	if f.game.WineRunner != "proton:ge" || f.game.WinePrefix != "" {
		t.Fatalf("runner %q prefix %q", f.game.WineRunner, f.game.WinePrefix)
	}
	f.expect(t, locales.GameLaunchProtonCompatDataPath(), locales.GameLaunchWineRunnerProtonHint(), locales.Key("gameLaunch.steamLaunchOptionsPresets.protonLog"))
	if fake.toolsN < 2 {
		t.Errorf("tools not reloaded after the runner changed")
	}

	f.click(t, locales.GameLaunchCompatibilityActionProtoncfg())
	if !slices.Equal(fake.opened, []string{"winecfg"}) {
		t.Fatalf("opened %q", fake.opened)
	}
	f.expectToast(t, locales.GameLaunchToastCompatibilityToolOpened())

	// CrossOver 只有 macOS 有：Linux 上改回默认兼容层。
	f.game.WineRunner, f.game.WinePrefix = "crossover", "bottle"
	f.frames(2)
	if f.game.WineRunner != "system" || f.game.WinePrefix != "" {
		t.Fatalf("crossover on linux: runner %q prefix %q", f.game.WineRunner, f.game.WinePrefix)
	}
}

// Linux 的 Steam 启动：Proton 选择是草稿，保存时一起写入启动参数；Steam 正在运行时先确认。
func TestLinuxSteamSettings(t *testing.T) {
	f := newFixture(t, 1024, 1600)
	fake := f.useGOOS("linux")
	f.game.LaunchMode = enums.LaunchModeSteam
	f.frames(2)
	def := locales.GameLaunchSteamProtonDefault("proton_9")
	f.expect(t, locales.GameLaunchCompatibilityTools(), locales.GameLaunchSteamProton(), def, locales.GameLaunchSteamRestart(),
		"/home/luna/.steam/steam", locales.GameLaunchSteamProtonPrefix(), fake.compat.ProtonPrefix, locales.GameLaunchSteamProtonPrefixOpen())
	f.expectNot(t, locales.GameLaunchSteamToolsHint(), locales.GameLaunchWineRunnerHint())

	f.click(t, def)
	f.click(t, "GE-Proton10")
	f.expect(t, locales.GameLaunchSteamProtonUnsaved())
	f.click(t, locales.GameLaunchSteamProtonSave())
	if len(fake.applied) != 1 || fake.applied[0].tool == nil || *fake.applied[0].tool != "GE-Proton10" {
		t.Fatalf("applied %+v", fake.applied)
	}
	f.expectToast(t, locales.GameLaunchToastSteamSettingsSaved())
	if f.state.toolDraft != nil || f.state.saving {
		t.Fatalf("draft %v saving %v after save", f.state.toolDraft, f.state.saving)
	}

	fake.compat.SteamRunning = true
	f.click(t, locales.GameLaunchSteamLaunchOptionsSave())
	f.expect(t, locales.GameLaunchSteamApplyConfirmTitle())
	if len(fake.applied) != 1 {
		t.Fatal("applied before confirmation")
	}
	f.click(t, locales.GameLaunchSteamApplyConfirmAction())
	if len(fake.applied) != 2 || fake.applied[1].tool != nil {
		t.Fatalf("applied %+v", fake.applied)
	}
	f.expectToast(t, locales.GameLaunchToastSteamSettingsSavedRestarted())
}

// Linux 的导入弹窗：Steam 正在运行时可以导入并重启，带上弹窗里的启动参数与 Proton。
func TestLinuxSteamImport(t *testing.T) {
	f := newFixture(t, 1024, 900)
	fake := f.useGOOS("linux")
	f.game.SteamLaunchOptions = "PROTON_LOG=1 %command%"
	f.launcher.steam = steamFlow{action: steamLaunch, gameID: testGameID}
	f.launcher.openSteamModal(f.game)
	f.launcher.steam.status = &service.SteamLaunchStatus{State: "steam_running"}
	f.frames(2)
	f.expect(t, locales.SteamImportRestartRequiredTitle(), locales.SteamImportApplyOptionsHint(), locales.GameLaunchSteamProton(), locales.SteamImportImportAndRestart())
	if f.launcher.steam.launchOptions != "PROTON_LOG=1 %command%" || f.launcher.steam.compat == nil {
		t.Fatalf("modal %+v", f.launcher.steam)
	}
	f.launcher.steam.tool = "GE-Proton10"
	f.click(t, locales.SteamImportImportAndRestart())
	if len(fake.imported) != 1 || fake.imported[0].options != "PROTON_LOG=1 %command%" || *fake.imported[0].tool != "GE-Proton10" {
		t.Fatalf("imported %+v", fake.imported)
	}
	if f.launcher.steam.open {
		t.Fatal("modal stayed open after import")
	}
	f.expectToast(t, locales.SteamImportImportSuccess())
}

// macOS：兼容层启动，选它时兼容层是系统 Wine；CrossOver 没有“打开目录”。
func TestDarwinCompatibilityLaunch(t *testing.T) {
	f := newFixture(t, 1024, 1600)
	f.useGOOS("darwin")
	f.expectNot(t, locales.GameLaunchWineRunnerHint(), locales.GameLaunchLaunchModeSteam(), locales.GameLaunchEnhancementTools())
	f.click(t, locales.GameLaunchLaunchModeNormal())
	f.click(t, locales.GameLaunchLaunchModeCompatibility())
	if f.game.LaunchMode != enums.LaunchModeCompatibility || f.game.WineRunner != "system" {
		t.Fatalf("mode %q runner %q", f.game.LaunchMode, f.game.WineRunner)
	}
	f.expect(t, locales.GameLaunchCompatibilityTools(), locales.GameLaunchWineRunnerSystem(), locales.GameLaunchWinePrefix())
	f.expectNot(t, locales.GameLaunchCompatibilityQuickTools())

	f.click(t, locales.GameLaunchWineRunnerSystem())
	f.click(t, locales.GameLaunchWineRunnerCrossover())
	f.expect(t, locales.GameLaunchCrossoverBottle())
	if _, ok := f.tt.Find(locales.GameEditOpenInExplorer()); ok {
		t.Error("CrossOver bottle has an open-folder action")
	}
}

func TestStartFromCard(t *testing.T) {
	f := newFixture(t, 1024, 600)
	f.click(t, "测试卡片启动")
	if !slices.ContainsFunc(f.toasts(), func(s string) bool {
		return s == locales.GameCardStartFailedLog(f.game.Name) || s == locales.GameCardStartFailedNotLaunched(f.game.Name)
	}) {
		t.Errorf("no start failure toast in %q", f.announced)
	}
	if running, _ := f.launcher.Status(testGameID); running {
		t.Error("failed start left a runtime")
	}
}

// 灵动岛显示的运行状态（activeGameRuntimeId）：新启动的游戏优先；当前的结束后换到最晚启动的；SelectNext 轮换。
func TestActiveRuntimeSelection(t *testing.T) {
	f := newFixture(t, 1024, 600)
	l := f.launcher
	base := time.Now()
	emit := func(id string, state service.GameRuntimeState, start time.Time, active *int) {
		g := models.Game{ID: id, Name: id}
		l.applyEvent(service.GameRuntimeChangedEvent{GameID: id, SessionID: "s-" + id, StartTime: start, State: state, Game: &g,
			TimingMode: service.GameRuntimeTimingModeActive, ActiveSeconds: active})
	}
	current := func() string {
		rt, _, ok := l.Current()
		if !ok {
			return ""
		}
		return rt.Game.ID
	}
	secs := 42
	emit("a", service.GameRuntimeStatePlaying, base, &secs)
	if rt, n, ok := l.Current(); !ok || rt.Game.ID != "a" || n != 1 || rt.ActiveSeconds == nil || *rt.ActiveSeconds != 42 {
		t.Fatalf("current = %+v, %d, %v", rt, n, ok)
	}
	emit("b", service.GameRuntimeStatePlaying, base.Add(time.Minute), nil)
	if current() != "a" {
		t.Fatalf("playing event switched the island to %q", current())
	}
	emit("c", service.GameRuntimeStateLaunching, base.Add(2*time.Minute), nil)
	if current() != "c" {
		t.Fatalf("launching game not shown: %q", current())
	}
	l.SelectNext()
	if current() != "a" {
		t.Fatalf("next after c = %q", current())
	}
	l.SelectNext()
	if current() != "b" {
		t.Fatalf("next after a = %q", current())
	}
	emit("b", service.GameRuntimeStateIdle, time.Time{}, nil)
	if current() != "c" {
		t.Fatalf("after b ended = %q, want the latest started", current())
	}
}
