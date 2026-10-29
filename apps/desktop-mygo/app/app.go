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

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/bootstrap"
	"lunabox/desktop-mygo/host"
	"lunabox/desktop-mygo/ui"
	"lunabox/desktop-mygo/ui/env"
	"lunabox/desktop-mygo/ui/pages/startup"
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
}

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
	})
	mygo.App.OnSecondInstance(a.onSecondInstance)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.core.ForceQuit() })
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
		if err != nil {
			mygo.RunOnMain(func() { a.showStartupFailure(err) })
		}
	}()
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
	w, h := a.o.Width, a.o.Height
	if w == 0 {
		w = max(cfg.WindowWidth, minWidth)
	}
	if h == 0 {
		h = max(cfg.WindowHeight, minHeight)
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
	page := &startup.Page{Message: err.Error(), OnExit: mygo.App.Quit}
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
}

// shutdown 在事件循环结束后按 bootstrap 的顺序关闭服务。
func (a *app) shutdown() {
	if a.backend != nil {
		// 窗口已经关闭：之后的服务事件不再交给界面。
		a.backend.Events.SetDispatcher(func(func()) {})
	}
	a.core.Shutdown()
}
