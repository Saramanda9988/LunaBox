// Package ui 是 LunaBox 前端在 MyGo 原生 UI 上的一比一实现，目录结构对应原版 frontend/src：
//
//	ui/app.go、routes.go   routes/__root.tsx 与路由表（MyGo 的 ui.Router）
//	ui/pages/home          routes/index.tsx（首页）
//	ui/pages/library       routes/library.tsx
//	ui/pages/game          routes/game.tsx（游戏详情）
//	ui/pages/categories    routes/categories.tsx（收藏夹列表）
//	ui/pages/category      routes/category.tsx（收藏详情）
//	ui/pages/settings      routes/settings.tsx（设置）
//	ui/pages/downloads     routes/downloads.tsx（下载管理）
//	ui/pages/stats         routes/stats.tsx（统计）
//	ui/pages/startup       components/startup/StartupWindow.tsx（启动失败窗口）
//	ui/components          components/ui（Better* 系列等通用组件）
//	ui/components/bar      components/bar（TopBar、SideBar）
//	ui/components/card     components/card（GameCard、CategoryCard、DownloadCard）
//	ui/components/grid     components/grid（VirtualGameGrid）
//	ui/theme、ui/assets    uno.config.ts 的设计令牌、图标与图片
//	ui/env、covers、perf   页面共享的运行环境、封面缓存、性能统计
package ui

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"lunabox/backend/appconf"
	"lunabox/backend/service"
	"lunabox/desktop/api"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/bar"
	"lunabox/desktop/ui/components/importer"
	"lunabox/desktop/ui/components/modal"
	"lunabox/desktop/ui/components/panel/launch"
	"lunabox/desktop/ui/covers"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/pages/categories"
	"lunabox/desktop/ui/pages/category"
	"lunabox/desktop/ui/pages/downloads"
	"lunabox/desktop/ui/pages/game"
	"lunabox/desktop/ui/pages/home"
	"lunabox/desktop/ui/pages/library"
	"lunabox/desktop/ui/pages/settings"
	"lunabox/desktop/ui/pages/stats"
	"lunabox/desktop/ui/perf"
	"lunabox/desktop/ui/theme"
)

// Backend 是界面用到的后端能力。
type Backend = env.Backend

// Options 是界面的启动参数与调试开关。
type Options struct {
	Route     string // 启动时显示的页面，默认首页 /
	HUD       bool
	Bench     bool // 加载后自动滚动压测
	BenchExit bool
	// SkipUpdateCheck 不在启动时检查更新（截图与压测时，免得弹出更新对话框）。
	SkipUpdateCheck bool
	// ConfigSaved 在设置页保存配置、界面应用新的语言与主题之后（主线程）调用，app 包用它更新托盘菜单。
	ConfigSaved func(prev, next appconf.AppConfig)
}

// App 是整个窗口的状态，View 由它构建。
type App struct {
	env   *env.Env
	opts  Options
	perf  *perf.Perf
	bench *perf.Bench

	router     *ui.Router
	sidebar    bar.Sidebar
	launcher   *launch.Launcher // 运行状态（store 的 gameRuntimes），与首页、详情页共用
	playing    bar.PlayingIsland
	home       *home.Page
	library    *library.Page
	detail     *game.Page
	categories *categories.Page
	category   *category.Page
	settings   *settings.Page
	downloads  *downloads.Page
	stats      *stats.Page
	dragDrop   *importer.DragDrop // 根布局的拖放导入（DragDropImportModal）
	location   string             // 上一次构建时的路由位置，变化时详情页、收藏页重新加载
	scale      float64            // 窗口所在显示器的缩放比例
	maximised  bool

	showHUD bool
	hudOn   atomic.Bool

	notices     []env.Notice // 事件回调里产生、下一帧显示的提示
	quitSyncing bool         // 正在退出前同步（只做一次）

	// App.tsx 的弹窗：lunabox:// 安装确认、没有时区时（第一次打开）的初始设置向导、启动时发现的新版本（useUpdateCheck）。
	install      modal.InstallConfirm
	setup        modal.SetupWizard
	setupPreview bool // 开发托盘菜单重新打开了向导（ShowSetupWizard），已有时区也显示
	updateInfo   *service.UpdateCheckResult
	updateDialog components.UpdateDialogState

	downloadStatus map[string]service.DownloadStatus // useDownloadNotifications 的 downloadStatusRef
}

// New 创建应用；list 是游戏库的列表查询，svc 是其余页面用的后端服务。
func New(list Backend, svc *api.Services, opts Options) *App {
	route := opts.Route
	if route == "" {
		route = env.PathHome
	}
	router := ui.NewRouter(route)
	router.Transition = ui.TransitionNone // 原版切换页面没有过渡
	e := &env.Env{Backend: list, Svc: svc, Router: router, Covers: covers.New(), Storage: env.NewStorage(list.DataDir()),
		Theme: theme.New(true, 1)}
	cfg := list.Config()
	p := perf.New()
	lib := library.New(e)
	a := &App{
		env:      e,
		opts:     opts,
		perf:     p,
		bench:    perf.NewBench(p, opts.BenchExit),
		router:   router,
		sidebar:  bar.NewSidebar(cfg == nil || cfg.SidebarOpen, sidebarWidth(cfg)),
		launcher: launch.For(e),
		home:     home.New(e),
		library:  lib,
		detail: game.New(e, game.Links{
			SearchLibrary:  func(q string) { lib.SearchFor(q); router.Push(env.PathLibrary) },
			FilterByTag:    func(tag string) { lib.FilterByTag(tag); router.Push(env.PathLibrary) },
			LibraryChanged: lib.Reload,
		}),
		categories: categories.New(e),
		category:   category.New(e),
		settings:   settings.New(e),
		downloads:  downloads.New(e, lib.Reload),
		stats:      stats.New(e),
		dragDrop:   importer.NewDragDrop(e),
		scale:      1,
		showHUD:    opts.HUD,

		downloadStatus: map[string]service.DownloadStatus{},
	}
	e.SidebarState = func(cfg *appconf.AppConfig) {
		cfg.SidebarOpen, cfg.SidebarWidth = a.sidebar.Open, a.sidebar.OpenWidth()
	}
	e.ConfigSaved = func(prev, next appconf.AppConfig) {
		applyConfig(prev, next)
		if opts.ConfigSaved != nil {
			opts.ConfigSaved(prev, next)
		}
	}
	return a
}

// sidebarWidth 是配置里的 sidebar_width，没有配置时为 0（默认宽度）。
func sidebarWidth(cfg *appconf.AppConfig) int {
	if cfg == nil {
		return 0
	}
	return cfg.SidebarWidth
}

// applyConfig 让保存后的配置立即生效，对应原版 App.tsx 的 i18n.changeLanguage 与 useAppTheme。
// 缩放（window_zoom_factor）不用在这里处理：View 每帧按配置调用 SetZoom（见 zoom）。
func applyConfig(prev, next appconf.AppConfig) {
	if next.Language != prev.Language {
		env.ApplyLanguage(next.Language)
	}
	if next.Theme != prev.Theme {
		env.ApplyTheme(next.Theme)
	}
}

// Attach 在窗口创建后调用：开始加载数据与封面，订阅应用级事件，并跟踪最大化状态与缩放。
func (a *App) Attach(win *mygo.Window) {
	a.env.Win = win
	a.subscribe()
	a.startup()
	a.env.Covers.Start(win, a.env.Backend.DataDir())
	a.maximised = win.IsMaximized()
	a.syncScale()
	sync := func() { a.maximised = win.IsMaximized(); a.syncScale(); win.Invalidate() }
	win.OnMaximize(sync)
	win.OnUnmaximize(sync)
	win.OnRestore(sync)
	win.OnMove(func() { a.syncScale(); win.Invalidate() })
	go a.perf.SampleMemory(a.hudOn.Load, win.Invalidate)
	a.library.Reload()
	if a.opts.Bench {
		a.bench.Arm()
	}
}

func (a *App) syncScale() {
	d := mygo.Screen.DisplayMatching(a.env.Win.Bounds())
	if d.ScaleFactor > 0 {
		a.scale = d.ScaleFactor
	}
}

// SetScale 供测试在没有窗口时指定缩放。
func (a *App) SetScale(scale float64) { a.scale = scale }

// View 构建整个窗口，对应 routes/__root.tsx：顶栏，下面是侧边栏与路由出口。
func (a *App) View(c *ui.Context) {
	start := time.Now()
	// 缩放后一个 DIP 占 a.scale × zoom 个物理像素：发丝线、吸附到设备像素的尺寸与背景图都按它计算。
	scale := a.scale * a.zoom(c)
	t := theme.New(c.Theme().Dark, scale)
	a.env.Theme = t
	applyTheme(c, t)

	if c.Shortcut(0, ui.KeyF2) {
		a.showHUD = !a.showHUD
	}
	a.hudOn.Store(a.showHUD)
	if c.Shortcut(0, ui.KeyF5) {
		a.bench.Arm()
	}
	total, _ := a.library.Stats()
	a.bench.Step(c, perf.Target{Scroll: &a.library.Scroll().State, Ready: !a.library.Loading(), Items: total})

	winW, winH := c.Size()
	root := c.Root().Background(t.AppBg).TextColor(t.Text)
	// 背景层：absolute inset-0 bg-cover bg-center，filter: blur(background_blur px)，scale(1.1)。图在后台裁好、模糊好，
	// 画在根元素的底色之上、所有子元素之下；还没算好时先露出底色。
	cfg := a.env.Config()
	bgReq := covers.BackgroundRequest{W: winW, H: winH, Scale: scale}
	if cfg != nil && cfg.BackgroundEnabled && cfg.BackgroundImage != "" {
		t.SetGlass(cfg.BackgroundOpacity)
		bgReq.Src, bgReq.Blur = cfg.BackgroundImage, float32(cfg.BackgroundBlur)
	}
	if bmp := a.env.Covers.Background(bgReq); bmp != nil {
		root.Draw(func(p *ui.Painter, r ui.Rect) { p.Image(bmp, r, ui.Cover) })
	}
	layout := ui.Column(c.Key("layout")).Fill()
	dragOver := a.fileDrop(c, layout)
	layout.Children(func() {
		ui.Box(c.Key("topbar-space")).Height(env.TopbarHeight).Shrink(0) // 顶栏在灵动岛之后构建，这里占住它的位置
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			bar.SideBar(c, t, bar.SideBarProps{State: &a.sidebar, Router: a.router, Config: a.env.Config(), Env: a.env})
			a.env.MainWidth = winW - a.sidebar.Width()
			// <main>：flex-1 overflow-hidden，玻璃模式下 rgba(var(--main-bg-rgb), background_opacity)
			a.routes(c).Basis(0).Clip().Background(t.MainBg())
		})
	})
	if dragOver {
		importer.DropOverlay(c, t, env.TopbarHeight)
	}
	a.dragDrop.View(c, t, func() { // handleImportComplete
		a.library.Reload()
		a.home.Refresh()
	})
	for _, n := range a.notices {
		if n.Type == toastDismiss {
			c.CloseToast(n.ID)
			continue
		}
		a.env.Toast(c, n)
	}
	a.notices = nil
	a.settings.Background(c) // 设置页的备份面板收起或离开设置页后，后台结果的提示与安排恢复后的退出照常进行
	a.env.FlushToasts(c)
	a.playingIsland(c, t)
	// TopBar 是 relative z-50，盖在灵动岛（z-45）之上：收起的岛藏在顶栏后面，只在顶栏下方露出 14px。
	win := a.env.Win
	ui.Column(c.Key("topbar")).Absolute().Top(0).Left(0).Right(0).Children(func() {
		bar.TopBar(c, t, bar.TopBarProps{
			Maximised:  a.maximised,
			OnMinimise: func() { win.Minimize() },
			OnMaximise: func() { win.ToggleMaximize() },
			OnClose:    func() { win.Close() },
		})
	})
	a.appModals(c, t)
	// <AppToaster topOffset={(TOPBAR_HEIGHT + 12) * zoomFactor} />
	components.Toaster(c, t, env.TopbarHeight+12)
	if a.showHUD {
		a.hud(c)
	}
	a.env.Covers.EndFrame()
	a.perf.ViewCalled(time.Now(), time.Since(start))
}

// appModals 是 App.tsx 在路由之后渲染的 UpdateDialog、TimezoneSelectModal（这里换成了初始设置向导）与
// InstallConfirmModal，都在 #app-modal-root（顶栏下方）里。
func (a *App) appModals(c *ui.Context, t *theme.Theme) {
	if a.env.Svc == nil {
		return
	}
	if info := a.updateInfo; info != nil && info.HasUpdate {
		components.UpdateDialog(c, t, "app-update-dialog", &a.updateDialog, components.UpdateDialogProps{
			Info:      info,
			TopOffset: env.TopbarHeight,
			OnClose:   func() { a.updateInfo = nil },
			OnSkip:    a.skipVersion,
			OnInAppUpdate: func(manifestURL string) {
				update := a.env.Svc.Update
				a.updateDialog.StartInAppUpdate(a.env.Async, func() (*service.UpdateApplyResult, error) {
					return update.DownloadAndApplyUpdate(manifestURL)
				})
			},
		})
	}
	cfg := a.env.Config()
	modal.SetupWizardView(c, t, &a.setup, modal.SetupWizardProps{
		Env:       a.env,
		Open:      a.setupPreview || (cfg != nil && strings.TrimSpace(cfg.TimeZone) == ""),
		Detected:  a.env.Svc.SystemTimeZone,
		TopOffset: env.TopbarHeight,
		OnDone:    func() { a.setupPreview = false },
	})
	modal.InstallConfirmView(c, t, &a.install, modal.InstallConfirmProps{Env: a.env, TopOffset: env.TopbarHeight})
}

// playingIsland 对应根布局的 <PlayingIsland />：显示 activeGameRuntimeId 的运行状态。
func (a *App) playingIsland(c *ui.Context, t *theme.Theme) {
	l := a.launcher
	rt, count, ok := l.Current()
	p := bar.PlayingIslandProps{Env: a.env, Count: count, OnSwitch: l.SelectNext, OnEnd: l.EndSession}
	if ok {
		p.Runtime = &bar.IslandRuntime{Game: rt.Game, SessionID: rt.SessionID, StartTime: rt.StartTime, State: rt.State, ActiveSeconds: rt.ActiveSeconds}
	}
	a.playing.View(c, t, p)
}

// applyTheme 让 MyGo 自带控件（文本框光标、选区、提示等）跟随原版配色。
func applyTheme(c *ui.Context, t *theme.Theme) {
	mt := *c.Theme()
	// MyGo 用 Accent 画文本框光标（没有单独的光标颜色）。原版 caret-color 是 currentColor，
	// 跟输入框文字（TextStrong）一致；neutral-600 在暗色输入框底色上几乎看不见。
	// 应用里没用到其它靠 Accent 着色的 MyGo 自带控件（ui.Link 自己设了颜色）。
	mt.Accent = t.TextStrong
	// MyGo 0.8 起非活动窗口的 Accent 默认褪成灰色；这里的 Accent 只是光标色，保持原样。
	mt.InactiveAccent = mt.Accent
	mt.Text = t.Text
	mt.TextMuted = t.Placeholder
	mt.Background = t.AppBg
	// 所有滚动容器都用 MyGo 原生滚动条，样式还原 ::-webkit-scrollbar。components.ScrollArea 让它常显；
	// 直接用 ui.Scroll 的地方悬停时浮现。
	mt.ScrollbarStyle = t.ScrollbarStyle()
	mt.Scrollbar = mt.ScrollbarStyle.Thumb
	mt.ScrollbarWidth = mt.ScrollbarStyle.ThumbWidth
	mt.Inverse, mt.InverseText = t.TooltipBg, t.TooltipText
	mt.FontSize = theme.TextBase.Size
	if theme.UIFont != "" {
		mt.Font = theme.UIFont
	}
	c.SetTheme(&mt)
}

func (a *App) hud(c *ui.Context) {
	total, loaded := a.library.Stats()
	n, avg := a.env.Covers.Decoded()
	cached, cachedBytes := a.env.Covers.Cached()
	perf.HUD(c, a.perf, a.bench, []string{
		fmt.Sprintf("route %s   scale %.2f   zoom %.2f", a.router.Location(), a.scale, c.Zoom()),
		fmt.Sprintf("games %d   loaded %d   covers %d decoded, %d pending", total, loaded, n, a.env.Covers.Pending()),
		fmt.Sprintf("cover cache %d (%.1f MB)", cached, float64(cachedBytes)/(1<<20)),
		fmt.Sprintf("cover load avg %.1f ms", float64(avg.Microseconds())/1000),
	})
}

// Screenshot 等可见封面解码完成后截图并退出，用于和原版逐像素对比（wails-final 标签里的 tools/parity）。
func (a *App) Screenshot(path string) {
	time.Sleep(800 * time.Millisecond)
	for range 60 {
		pending := make(chan int, 1)
		a.env.Win.Update(func() {
			n := a.env.Covers.Pending()
			if a.home.Loading() || a.library.Loading() || a.detail.Loading() || a.categories.Loading() || a.category.Loading() || a.settings.Loading() {
				n++
			}
			pending <- n
		})
		if <-pending == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(400 * time.Millisecond)
	data, err := a.env.Win.CapturePage()
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		log.Printf("screenshot: %v", err)
	}
	mygo.App.Quit()
}

// ShowSetupWizard 从第一步重新打开初始设置向导（开发运行的托盘菜单用），已经设置过时区也显示。
// 走完会照常保存所选的设置。可以在任何线程调用。
func (a *App) ShowSetupWizard() {
	open := func() { a.setup, a.setupPreview = modal.SetupWizard{}, true }
	if a.env.Win == nil {
		open()
		return
	}
	a.env.Win.Update(open)
}

// SetDebugState 让截图直接进入某个界面状态：drawer、addmenu、batch、collapsed，picker（首页展开封面选择面板），setup（初始设置向导），
// tab:<key>（详情页的标签页：stats、edit、launch、backup、progress、review），
// 或 settings:<分区>（设置页只展开这个分区并滚到顶部，settings:collapsed 全部收起）。
func (a *App) SetDebugState(state string) {
	switch {
	case state == "picker":
		a.home.SetDebugState(state)
	case state == "setup":
		a.ShowSetupWizard()
	case strings.HasPrefix(state, "settings:"):
		a.settings.SetDebugState(strings.TrimPrefix(state, "settings:"))
	case strings.HasPrefix(state, "tab:"):
		a.detail.SetTab(strings.TrimPrefix(state, "tab:"))
	case state == "collapsed":
		a.sidebar.SetOpen(false)
	default:
		a.library.SetDebugState(state)
	}
}

// fileDrop 对应根布局 data-file-drop-target 与 files-dropped 事件：窗口接受拖进来的文件与文件夹，放下后打开
// DragDropImportModal。返回是否正在拖着文件经过窗口（isDragOver）；导入弹窗打开期间不显示提示，也不接受新的拖放。
func (a *App) fileDrop(c *ui.Context, target ui.Element) bool {
	if a.env.Svc == nil || a.env.Svc.Import == nil {
		return false
	}
	opts := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}, Operations: transfer.Copy}
	_, over := ui.DataDragOver(target, opts)
	if drop, ok := ui.DropData(target, opts); ok && !a.dragDrop.Active() {
		paths, err := drop.Data.Files()
		if err != nil {
			log.Printf("drop: read files: %v", err)
		} else {
			a.dragDrop.Drop(c, paths)
		}
	}
	return over && !a.dragDrop.Active()
}
