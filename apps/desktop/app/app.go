// Package app 是 MyGo 版的应用外壳，对应 apps/desktop/main.go：启动、生命周期与退出由 lunabox/backend/bootstrap 负责
// （与 Wails 版相同），这里只做 MyGo 相关的部分：单实例、数据目录与日志、主窗口与界面、托盘、启动错误窗。
//
// 开发运行（Options.Dev）的数据在 build/data，与安装版隔离（bootstrap.Options.Isolated）：不注册开机自启、
// 不启动 CLI 的 IPC 服务与 MCP 服务、不自动云同步，以免影响同时运行的安装版（IPC 的端口文件与开机启动项都是全局的）。
package app

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/bootstrap"
	"lunabox/backend/service"
	"lunabox/desktop/host"
	"lunabox/desktop/ui"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/pages/startup"
)

// 主窗口的最小尺寸（DIP）。
const (
	minWidth  = 960
	minHeight = 600
)

// Options 是命令行决定的启动参数。
type Options struct {
	Args      []string // flag 之后的参数：系统打开 lunabox:// 链接时是这个链接
	Autostart bool     // 由开机自启动启动（--autostart）：启动后留在托盘，不显示主窗口
	Dev       bool     // 开发运行（mygo.IsDev）：见包说明
	DataDir   string   // 指定数据目录（-data-dir）
	Reseed    bool     // 开发运行时重新从安装版复制数据
	Icon      []byte   // 应用图标（PNG），窗口与任务栏使用

	Width, Height int    // 窗口尺寸（DIP）；为 0 时用配置里上次的尺寸
	Screenshot    string // 封面加载完成后把窗口截图保存到这个 PNG 并退出
	State         string // 截图调试：启动时进入的界面状态（ui.App.SetDebugState）
	Route         string // 启动时显示的页面
	Repeat        int    // 把游戏列表复制 N 份（压测大列表）
	HUD           bool
	Bench         bool
	BenchExit     bool
}

type app struct {
	o       Options
	dataDir string
	logger  *applog.FileLogger
	core    *bootstrap.App
	backend *host.Backend
	ui      *ui.App

	tray        *mygo.Tray
	relabelTray func()

	// 启动时恢复备份的进度窗（启动错误窗的恢复模式）：恢复开始时创建，启动结束后关闭；只在主线程上访问。
	restoreWin  *mygo.Window
	restorePage *startup.Page
	// 上次刷新进度窗时的阶段与时间，用于节流；进度窗出现的时间，用于最短显示时间。只在执行恢复的 goroutine 上访问。
	restoreStage   service.RestoreStage
	restoreAt      time.Time
	restoreShownAt time.Time
}

// 恢复进度窗的最短显示时间：恢复很快（备份很小）时进度窗也不会一闪而过，主窗口紧跟着弹出。
const (
	restoreMinVisible = 1500 * time.Millisecond // 从出现到恢复结束至少这么久
	restoreDoneMin    = 500 * time.Millisecond  // “恢复完成，正在启动”至少显示这么久
)

// Run 启动应用并运行到退出。
func Run(o Options) error {
	name := "LunaBox"
	if o.Dev {
		name = "LunaBox Dev" // 单实例锁按名称区分：开发版与安装版可以同时运行
	}
	mygo.App.SetName(name)

	a := &app{o: o}
	if err := a.prepare(); err != nil {
		return err
	}
	// lunabox:// 链接交给已在运行的 LunaBox（它的 IPC 服务），否则启动后自己处理。
	req, forwarded, err := bootstrap.TakeProtocolRequest(o.Args, a.logger)
	if err != nil || forwarded {
		return err
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return nil // 已有实例：命令行交给它（onSecondInstance）
	}

	a.core = bootstrap.New(bootstrap.Options{
		Logger:          a.logger,
		InitialRequest:  req,
		Isolated:        o.Dev,
		KeepWindowState: o.Screenshot != "",
		RestoreProgress: a.onRestoreProgress,
	})
	mygo.App.OnSecondInstance(a.onSecondInstance)
	if runtime.GOOS == "darwin" {
		// macOS 用 Apple 事件把 lunabox:// 交给声明了它的 .app（mygo.json 的 urlSchemes 写进 Info.plist），
		// 启动时的与运行中收到的都在这里，不在命令行里（Wails 版的 ApplicationLaunchedWithUrl）。
		// Windows 与 Linux 的链接在命令行里，由 TakeProtocolRequest 与 onSecondInstance 处理。
		mygo.App.OnOpenURL(a.core.HandleLaunchedWithURL)
	}
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		// 系统结束会话（Windows 的 WM_ENDSESSION、macOS 的 applicationWillTerminate:）时，框架在返回后
		// 随时可能结束进程：OnQuit 里的 shutdown 跳过耗时的自动备份。Windows 上通常已由 sessionend 钩子
		// 在 WM_QUERYENDSESSION 时提前开始退出，这里是它没能先完成时的兜底（如强制关机）。
		if e.SessionEnding {
			a.core.MarkSystemSessionEnding()
		}
		a.core.ForceQuit()
	})
	mygo.App.OnQuit(a.shutdown)
	mygo.App.WhenReady(a.ready)
	return mygo.App.Run()
}

// prepare 准备数据目录与日志，必须在任何服务读取路径之前调用。
func (a *app) prepare() error {
	dir, seededFrom, err := host.PrepareDataDir(host.DataDirOptions{Dir: a.o.DataDir, Dev: a.o.Dev, Reseed: a.o.Reseed})
	if err != nil {
		return fmt.Errorf("获取应用数据目录失败: %w", err)
	}
	a.dataDir = dir
	if a.logger, err = bootstrap.NewLogger(); err != nil {
		return fmt.Errorf("准备日志目录失败: %w", err)
	}
	applog.SetMode(applog.ModeGUI)
	applog.SetLogger(a.logger)
	a.logger.Info("application startup initiated")
	if seededFrom != "" {
		a.logger.Info(fmt.Sprintf("copied LunaBox data from %s to %s", seededFrom, dir))
	}
	if a.o.Dev {
		if err := host.LoadDevEnv(); err != nil {
			a.logger.Warning("failed to load dev env files: " + err.Error())
		}
	}
	return nil
}

// ready 在应用就绪后于后台启动：打开服务，在主线程上创建界面、主窗口与托盘，再启动后台服务。
// 失败时显示启动错误窗。
func (a *app) ready() {
	if len(a.o.Icon) > 0 {
		if err := mygo.App.Dock.SetIcon(a.o.Icon); err != nil {
			a.logger.Warning("failed to set application icon: " + err.Error())
		}
	}
	go func() {
		err := a.core.Startup(context.Background(), func() (err error) {
			mygo.RunOnMain(func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("创建主窗口时发生异常: %v", r)
					}
				}()
				a.attach()
			})
			return err
		}, a.o.Autostart)
		// 主窗口（或启动错误窗）显示之后再关闭恢复进度窗，中间不会出现没有窗口的空档。
		mygo.RunOnMain(func() {
			if err != nil {
				a.showStartupFailure(err)
			}
			a.closeRestoreWindow()
		})
	}()
}

// onRestoreProgress 显示启动时恢复备份的进度（bootstrap.Options.RestoreProgress，在执行恢复的 goroutine 上调用）：
// 第一次调用时创建进度窗，之后同一阶段内最多每 50ms 刷新一次，换阶段与阶段完成时立即刷新。
// 恢复成功（service.RestoreStageDone）时在这里等到满足最短显示时间（restoreMinVisible、restoreDoneMin）
// 再返回，之后才打开数据库并显示主窗口；恢复失败时不等，直接换成启动错误窗。
func (a *app) onRestoreProgress(p service.RestoreProgress) {
	now := time.Now()
	if p.Stage == a.restoreStage && p.Done < p.Total && now.Sub(a.restoreAt) < 50*time.Millisecond {
		return
	}
	a.restoreStage, a.restoreAt = p.Stage, now
	if a.restoreShownAt.IsZero() {
		a.restoreShownAt = now
	}
	if p.Stage == service.RestoreStageDone {
		defer func() {
			time.Sleep(max(restoreMinVisible-time.Since(a.restoreShownAt), restoreDoneMin))
		}()
	}
	// 与 Open 在同一个 goroutine 上，配置已经读好（全量恢复在恢复设置后换成备份里的配置）。
	lang, theme := a.core.Config.Language, a.core.Config.Theme
	mygo.RunOnMain(func() {
		if a.restoreWin != nil {
			a.restoreWin.Update(func() { a.restorePage.Restore = &p })
			return
		}
		env.ApplyLanguage(lang)
		env.ApplyTheme(theme)
		a.restorePage = &startup.Page{Restore: &p}
		a.restoreWin = a.newStartupWindow(a.restorePage)
		// 恢复不能中途停下：拦住 Alt+F4 之类的关闭；closeRestoreWindow 用 Destroy 关闭，不经过这里。
		a.restoreWin.OnClose(func(e *mygo.CloseEvent) { e.PreventDefault() })
	})
}

// closeRestoreWindow 在主线程上关闭恢复进度窗，没有时什么也不做。
func (a *app) closeRestoreWindow() {
	if a.restoreWin != nil {
		a.restoreWin.Destroy()
		a.restoreWin, a.restorePage = nil, nil
	}
}

// attach 创建界面、主窗口与托盘，并把窗口交给 bootstrap（主线程）。
func (a *app) attach() {
	core := a.core
	env.ApplyLanguage(core.Config.Language)
	env.ApplyTheme(core.Config.Theme)
	a.backend = &host.Backend{
		Services: core.Services, DB: core.DB, Config: core.Config,
		DataDir: a.dataDir, Events: host.NewEvents(),
	}
	var list ui.Backend = listBackend{a.backend}
	if a.o.Repeat > 1 {
		list = repeatedBackend{listBackend: listBackend{a.backend}, n: a.o.Repeat}
	}
	a.ui = ui.New(list, a.backend.API(), ui.Options{
		Route: a.o.Route, HUD: a.o.HUD, Bench: a.o.Bench, BenchExit: a.o.BenchExit,
		SkipUpdateCheck: a.o.Screenshot != "" || a.o.Bench,
		ConfigSaved: func(prev, next appconf.AppConfig) {
			if next.Language != prev.Language && a.relabelTray != nil {
				a.relabelTray()
			}
		},
	})
	a.ui.SetDebugState(a.o.State)

	win := a.newMainWindow()
	a.backend.Events.SetDispatcher(win.Update)
	rt := &host.Runtime{Events: a.backend.Events, Win: win, Autostart: !a.o.Dev}
	core.Attach(host.Window{Window: win}, rt, mygo.App.Quit)
	a.ui.Attach(win)
	win.OnClose(func(e *mygo.CloseEvent) {
		if !core.MainWindowClosing() {
			e.PreventDefault()
		}
	})
	a.setupTray()
	if a.o.Screenshot != "" {
		win.Show()
		go a.ui.Screenshot(a.o.Screenshot)
	}
}

// newMainWindow 创建隐藏的主窗口（启动完成后显示，开机自启动时留在托盘），尺寸与最大化状态取自配置。
func (a *app) newMainWindow() *mygo.Window {
	cfg := a.core.Config
	// 配置里的尺寸可能来自更大的屏幕（换了显示器、导入了别人的配置），缩到主显示器的工作区以内，
	// 否则窗口会超出屏幕。命令行指定的尺寸（截图用）不改。
	sw, sh := fitWorkArea(cfg.WindowWidth, cfg.WindowHeight, mygo.Screen.PrimaryDisplay().WorkArea)
	w, h := a.o.Width, a.o.Height
	if w == 0 {
		w = sw
	}
	if h == 0 {
		h = sh
	}
	bg := "#f3f4f6"
	if mygo.Theme.IsDark() {
		bg = "#121416"
	}
	opts := mygo.WindowOptions{
		Title:     "LunaBox",
		Width:     w,
		Height:    h,
		MinWidth:  minWidth,
		MinHeight: minHeight,
		Maximized: cfg.WindowMaximised && a.o.Screenshot == "",
		Hidden:    true,
		// 隐藏标题栏与系统的窗口按钮，顶栏自己画（bar.TopBar）。与 Frameless 相比，Windows 上保留顶边的拖动调整大小、
		// 阴影与圆角，顶栏的最大化按钮（MaximizeButton）悬停时显示 Windows 11 的贴靠布局。
		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,
		BackgroundColor:   bg,
		Content:           mygoui.View(a.ui.View),
	}
	if runtime.GOOS == "linux" {
		// Linux 的 TitleBarHidden 仍由合成器装饰窗口（KWin 会画一条自己的标题栏），与顶栏叠成两条。
		// 无边框窗口由 MyGo 在映射前 announceCSD，告诉合成器窗口自己画装饰；Wails 版在 Linux 上也是无边框。
		opts.Frameless = true
		opts.TitleBarStyle = mygo.TitleBarDefault
		opts.HideWindowButtons = false
	}
	return mygo.NewWindow(opts)
}

// fitWorkArea 返回窗口的初始尺寸：至少 minWidth × minHeight，超出工作区 wa 时缩到工作区以内（工作区比最小尺寸还小时
// 用最小尺寸）。wa 为空（拿不到显示器）时不限制。
func fitWorkArea(w, h int, wa mygo.Rectangle) (int, int) {
	w, h = max(w, minWidth), max(h, minHeight)
	if wa.Width > 0 && wa.Height > 0 {
		w, h = max(min(w, wa.Width), minWidth), max(min(h, wa.Height), minHeight)
	}
	return w, h
}

// onSecondInstance 在又启动了一次时调用：处理它带来的 lunabox:// 链接（没有 IPC 服务转交时），否则显示主窗口。
func (a *app) onSecondInstance(args []string, _ string) {
	var req *bootstrap.ProtocolRequest
	if raw, ok := bootstrap.ProtocolArg(args); ok {
		var err error
		if req, err = bootstrap.ParseProtocolRequest(raw); err != nil {
			a.logger.Error("failed to handle protocol URL: " + err.Error())
		}
	}
	a.core.HandleSecondInstance(req)
}

// showStartupFailure 显示启动错误窗，关闭它即退出。
func (a *app) showStartupFailure(err error) {
	a.newStartupWindow(&startup.Page{Message: err.Error(), OnExit: mygo.App.Quit})
}

// showStartupErrorPreview 显示一个示例启动错误窗，对应 Wails 版托盘里开发运行才有的“开发：预览启动错误窗”。
// 只用来检查界面样式：“退出”只关闭这个窗口，不退出应用；可以同时打开多个。
func (a *app) showStartupErrorPreview() {
	var win *mygo.Window
	win = a.newStartupWindow(&startup.Page{
		Message: "开发预览：数据库启动失败\n\n" +
			"打开数据库失败: IO Error: 无法打开 lunabox.db，文件可能正由另一个进程使用\n\n" +
			"此信息仅用于检查启动错误窗的界面样式。",
		OnExit: func() { win.Close() },
	})
}

// showRestorePreview 显示一个示例恢复进度窗，按启动时恢复的阶段走一遍假的进度（解压 3 秒、导入数据库 2 秒、
// 替换 1 秒、正在启动 1.5 秒）后自动关闭。只用来检查恢复进度窗的界面样式，不碰数据；可以同时打开多个，Alt+F4 可提前关闭。
func (a *app) showRestorePreview() {
	const total = 384 << 20 // 示例备份解压后的大小
	page := &startup.Page{Restore: &service.RestoreProgress{Stage: service.RestoreStageExtract, Step: 1, Steps: 3, Total: total}}
	win := a.newStartupWindow(page)
	go func() {
		set := func(p service.RestoreProgress) bool {
			if win.IsDestroyed() {
				return false
			}
			win.Update(func() { page.Restore = &p })
			return true
		}
		wait := func(d time.Duration, p service.RestoreProgress) bool {
			if !set(p) {
				return false
			}
			time.Sleep(d)
			return true
		}
		const frames = 60
		for i := 0; i <= frames; i++ {
			if !wait(50*time.Millisecond, service.RestoreProgress{Stage: service.RestoreStageExtract, Step: 1, Steps: 3, Done: total * int64(i) / frames, Total: total}) {
				return
			}
		}
		if wait(2*time.Second, service.RestoreProgress{Stage: service.RestoreStageDatabase, Step: 2, Steps: 3}) &&
			wait(time.Second, service.RestoreProgress{Stage: service.RestoreStageApply, Step: 3, Steps: 3}) &&
			wait(1500*time.Millisecond, service.RestoreProgress{Stage: service.RestoreStageDone}) {
			win.Destroy()
		}
	}()
}

// newStartupWindow 用 page 创建并显示启动错误窗（或恢复进度窗）：固定尺寸、置顶、无边框。
func (a *app) newStartupWindow(page *startup.Page) *mygo.Window {
	bg := "#ffffff"
	if mygo.Theme.IsDark() {
		bg = "#1c1e1f"
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:           "LunaBox",
		Width:           startup.Width,
		Height:          startup.Height,
		MinWidth:        startup.Width,
		MinHeight:       startup.Height,
		DisableResize:   true,
		DisableMaximize: true,
		AlwaysOnTop:     true,
		Frameless:       true,
		BackgroundColor: bg,
		Content:         mygoui.View(page.View),
	})
	page.Scale = mygo.Screen.DisplayMatching(win.Bounds()).ScaleFactor
	win.Center()
	win.Focus()
	return win
}

// shutdown 在事件循环结束后按 bootstrap 的顺序关闭服务。
func (a *app) shutdown() {
	if a.backend != nil {
		// 窗口已经关闭：之后的服务事件不再交给界面。
		a.backend.Events.SetDispatcher(func(func()) {})
	}
	a.core.Shutdown()
}
