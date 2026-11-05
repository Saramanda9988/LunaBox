// Package bootstrap 是 LunaBox 桌面端与界面框架无关的启动、生命周期与退出：读取配置与执行待恢复的备份、
// 打开数据库与迁移、构造并注入服务、启动后台服务（会话结束钩子、开机自启、MCP、CLI 的 IPC 服务、云同步、定时备份）、
// 关闭到托盘与退出前同步、lunabox:// 协议请求，以及退出时的关闭顺序。
//
// 桌面端（apps/desktop，用 MyGo）只负责界面框架相关的部分：
// 创建主窗口与托盘，实现 Window 与 appruntime.Runtime，并把框架的事件交给 App。
package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/appruntime"
	"lunabox/backend/cli"
	"lunabox/backend/cli/ipcserver"
	"lunabox/backend/service"
	"lunabox/backend/utils/apputils"
	"lunabox/backend/utils/dbutils"
	"lunabox/backend/utils/sessionend"
)

// LogLevel 是应用日志的级别。
const LogLevel = slog.LevelInfo

// NewLogger 创建写到数据目录 logs/app.log 的应用日志。
func NewLogger() (*applog.FileLogger, error) {
	logDir, err := apputils.GetSubDir("logs")
	if err != nil {
		return nil, err
	}
	return applog.NewFileLogger(filepath.Join(logDir, "app.log"), LogLevel), nil
}

// Options 是创建 App 的参数。
type Options struct {
	Logger *applog.FileLogger
	// InitialRequest 是启动参数里的 lunabox:// 请求（TakeProtocolRequest），启动完成后处理。
	InitialRequest *ProtocolRequest
	// Isolated 是开发运行：不启动 CLI 的 IPC 服务与 MCP 服务、不自动云同步，
	// 以免影响同时运行的安装版（IPC 的端口文件是全局的）。
	Isolated bool
	// KeepWindowState 为 true 时不把主窗口的尺寸写回配置（截图时用固定尺寸）。
	KeepWindowState bool
	// RestoreProgress 接收 Open 里执行设置页安排的恢复时的进度（见 RunPendingRestores），在调用 Open 的 goroutine 上调用；可为 nil。
	RestoreProgress service.RestoreProgressFunc
}

// App 持有一次运行的服务、数据库与生命周期状态。
type App struct {
	Services *Services
	DB       *sql.DB
	Config   *appconf.AppConfig

	o      Options
	logger *applog.FileLogger
	ctx    context.Context

	mu            sync.RWMutex
	window        Window
	rt            appruntime.Runtime
	quit          func()
	sessionEnd    *sessionend.Hook
	queued        []*ProtocolRequest // 启动完成前第二次启动带来的请求
	shutdownSteps []shutdownStep
	ipc           *http.Server

	forceQuit               atomic.Bool
	shuttingDown            atomic.Bool
	systemSessionEnding     atomic.Bool
	quitRequestPending      atomic.Bool
	frontendQuitSyncPlanned atomic.Bool
	frontendQuitSyncBacked  atomic.Bool
	trayAvailable           atomic.Bool

	startupReady                  atomic.Bool
	startupDone                   chan struct{}
	startupDoneOnce               sync.Once
	secondInstanceLaunchPending   atomic.Bool
	initialRequestHandled         atomic.Bool
	initialDuplicateSuppressUntil time.Time
}

type shutdownStep struct {
	name string
	fn   func()
}

// New 构造服务。之后依次调用 Startup（或 Open、Attach、Start、Ready），退出时调用 Shutdown。
func New(o Options) *App {
	a := &App{
		o:                             o,
		logger:                        o.Logger,
		ctx:                           context.Background(),
		rt:                            appruntime.Unavailable(),
		startupDone:                   make(chan struct{}),
		initialDuplicateSuppressUntil: time.Now().Add(30 * time.Second),
	}
	a.Services = NewServices(ServiceOptions{
		QuitForUpdate: func() { a.RequestQuit("application-update") },
		DisableMCP:    o.Isolated,
	})
	return a
}

// Startup 依次执行 Open、attach（桌面端创建主窗口并调用 Attach）与 Start，期间的 panic 也当作启动失败。
// 成功时调用 Ready；失败时调用 Fail 并返回错误，桌面端随后显示启动错误窗。
func (a *App) Startup(ctx context.Context, attach func() error, launchedByAutostart bool) error {
	err := func() (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("启动期间发生异常: %v\n\n%s", recovered, debug.Stack())
			}
		}()
		if err := a.Open(ctx); err != nil {
			return err
		}
		if err := attach(); err != nil {
			return err
		}
		a.Start()
		return nil
	}()
	if err != nil {
		a.Fail(err)
		return err
	}
	a.Ready(launchedByAutostart)
	return nil
}

// Open 读取配置，执行设置页安排的恢复，打开数据库并迁移，再初始化服务。
func (a *App) Open(ctx context.Context) error {
	if ctx != nil {
		a.ctx = ctx
	}
	config, err := appconf.LoadConfig()
	if err != nil {
		return fmt.Errorf("读取应用配置失败: %w", err)
	}
	a.Config = config

	repairStaleAppImageProtocolRegistration(a.logger)
	if err := RunPendingRestores(config, a.logger, a.o.RestoreProgress); err != nil {
		return err
	}

	dataDir, err := apputils.GetDataDir()
	if err != nil {
		return fmt.Errorf("获取应用数据目录失败: %w", err)
	}
	a.DB, err = OpenDatabase(a.ctx, filepath.Join(dataDir, "lunabox.db"), config.TimeZone, a.logger)
	if err != nil {
		return err
	}

	s := a.Services
	s.Init(a.ctx, a.DB, config)
	// 首次显示由 Ready 控制，主窗口不会在前端加载时盖住启动状态。
	s.Config.SetSuppressInitialWindowShow(true)
	s.Config.SetQuitHandler(func() {
		if a.HasPendingQuitRequest() {
			a.QuitApplication()
			return
		}
		a.RequestQuit("frontend-request")
	})
	service.ConfigureBackupServiceQuitSyncDBBackupHooks(s.Backup, nil, func() { a.frontendQuitSyncBacked.Store(true) }, nil)
	return nil
}

// Attach 在桌面端创建主窗口后调用：quit 让框架退出应用，rt 交给各服务。
func (a *App) Attach(window Window, rt appruntime.Runtime, quit func()) {
	a.mu.Lock()
	a.window, a.rt, a.quit = window, rt, quit
	a.mu.Unlock()
	a.Services.SetRuntime(rt)
}

func (a *App) runtime() appruntime.Runtime {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rt
}

func (a *App) emit(name string, data ...any) {
	a.runtime().Emit(name, data...)
}

// Start 启动后台服务：清理未结束的游玩记录，会话结束钩子，同步开机自启，MCP 服务，CLI 的 IPC 服务，云同步与定时数据库备份。
func (a *App) Start() {
	s := a.Services
	if err := s.Session.CleanupUnfinishedSessions(); err != nil {
		a.logger.Error("startup cleanup unfinished sessions failed: " + err.Error())
	}
	hook, err := sessionend.Start(sessionend.Options{
		Reason:            "LunaBox 正在保存数据并退出",
		OnQueryEndSession: a.QuitForSystemSessionEnd,
	})
	if err != nil {
		a.logger.Error("failed to start Windows session-end hook: " + err.Error())
	}
	a.mu.Lock()
	a.sessionEnd = hook
	a.mu.Unlock()

	rt := a.runtime()
	if err := rt.SetAutostart(a.Config.LaunchAtLogin); err != nil {
		a.logger.Error("failed to sync launch-at-login: " + err.Error())
	}
	if !a.o.Isolated {
		if err := s.MCPServer.ApplyConfig(*a.Config); err != nil {
			a.logger.Error("failed to apply MCP server config: " + err.Error())
		}
		cliApp := &cli.CoreApp{
			Config: a.Config, DB: a.DB, Ctx: a.ctx, GameService: s.Game,
			StartService: s.Start, SessionService: s.Session,
			BackupService: s.Backup, VersionService: s.Version,
		}
		a.ipc = ipcserver.StartServer(cliApp, rt)
		if shouldRunAutomaticCloudSync(a.Config) {
			s.CloudSync.RunStartupSync()
		}
		s.CloudSync.StartScheduledSync()
	}
	s.Backup.StartScheduledDBBackups()
}

// Ready 在启动完成后调用：显示主窗口（由开机自启动启动时留在托盘），再处理启动参数与启动期间收到的协议请求。
func (a *App) Ready(launchedByAutostart bool) {
	a.mu.Lock()
	a.startupReady.Store(true)
	queued := a.queued
	a.queued = nil
	a.mu.Unlock()
	a.closeStartupDone()

	if a.secondInstanceLaunchPending.Swap(false) || !launchedByAutostart || strings.TrimSpace(a.Config.TimeZone) == "" {
		a.ShowMainWindow()
	}
	if a.o.InitialRequest != nil && a.initialRequestHandled.CompareAndSwap(false, true) {
		a.logger.Info("dispatching initial protocol request from startup arguments")
		a.DispatchProtocolRequest(a.o.InitialRequest)
	}
	for _, req := range queued {
		a.DispatchProtocolRequest(req)
	}
}

// Fail 记录启动失败，交给 StartupService（Wails 版的启动错误窗读取它）。
func (a *App) Fail(err error) {
	a.logger.Error("application startup failed: " + err.Error())
	a.Services.Startup.ReportFailure(err.Error())
	a.closeStartupDone()
}

func (a *App) closeStartupDone() {
	a.startupDoneOnce.Do(func() { close(a.startupDone) })
}

// HandleSecondInstance 在又启动了一次时调用：req 是它带来的 lunabox:// 请求（没有时为 nil）。
// 启动完成前记下，完成后显示主窗口并处理请求。
func (a *App) HandleSecondInstance(req *ProtocolRequest) {
	a.logger.Info("second application launch received")
	a.mu.Lock()
	if !a.startupReady.Load() {
		a.secondInstanceLaunchPending.Store(true)
		if req != nil {
			a.queued = append(a.queued, req)
		}
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	if req != nil {
		a.DispatchProtocolRequest(req)
		return
	}
	a.ShowMainWindow()
}

// OnShutdown 添加桌面端自己的关闭步骤，在关闭 MCP 服务之后、关闭数据库之前执行。
func (a *App) OnShutdown(name string, fn func()) {
	a.mu.Lock()
	a.shutdownSteps = append(a.shutdownSteps, shutdownStep{name: name, fn: fn})
	a.mu.Unlock()
}

// Shutdown 在应用退出时按顺序关闭：停止定时任务、IPC 与 MCP 服务，清理待选择的进程，退出时的自动数据库备份
// （系统会话结束或退出前同步已备份时跳过），关闭数据库，保存配置，最后释放会话结束钩子。
func (a *App) Shutdown() {
	a.shuttingDown.Store(true)
	a.mu.RLock()
	extra := append([]shutdownStep(nil), a.shutdownSteps...)
	a.mu.RUnlock()

	if !a.startupReady.Load() {
		for _, step := range extra {
			step.fn()
		}
		a.stopSessionEnd()
		a.closeDatabase(3 * time.Second)
		return
	}

	s := a.Services
	isSystemSessionEnding := a.systemSessionEnding.Load()
	shutdownMode := "normal"
	if isSystemSessionEnding {
		shutdownMode = "system-session-ending"
	}
	s.CloudSync.StopScheduledSync()
	s.Backup.StopScheduledDBBackups()

	shutdownStartedAt := time.Now()
	a.logger.Info("shutdown mode: " + shutdownMode)
	logShutdownStep := func(step string, fn func()) {
		stepStartedAt := time.Now()
		a.logger.Info("shutdown step started: " + step)
		fn()
		a.logger.Info(fmt.Sprintf("shutdown step finished: %s (elapsed: %s)", step, time.Since(stepStartedAt)))
	}

	logShutdownStep("shutdown IPC server", func() {
		if a.o.Isolated {
			return // 没有启动 IPC 服务：ShutdownServer 会删掉安装版的端口文件
		}
		if err := ipcserver.ShutdownServer(a.ipc); err != nil {
			a.logger.Error("failed to shutdown IPC server: " + err.Error())
		}
	})
	logShutdownStep("shutdown MCP server", func() {
		if err := s.MCPServer.Shutdown(); err != nil {
			a.logger.Error("failed to shutdown MCP server: " + err.Error())
		}
	})
	for _, step := range extra {
		logShutdownStep(step.name, step.fn)
	}
	config := a.Config
	logShutdownStep("refresh latest config", func() {
		latestConfig, err := s.Config.GetAppConfig()
		if err != nil {
			a.logger.Error("failed to get latest config: " + err.Error())
			return
		}
		latestConfig.WindowWidth = config.WindowWidth
		latestConfig.WindowHeight = config.WindowHeight
		latestConfig.WindowMaximised = config.WindowMaximised
		config = &latestConfig
	})
	logShutdownStep("cleanup pending process selections", func() {
		s.Start.CleanupPendingSessions()
	})
	logShutdownStep("automatic database backup", func() {
		if isSystemSessionEnding || !config.AutoBackupDB {
			return
		}
		if a.frontendQuitSyncPlanned.Load() && a.frontendQuitSyncBacked.Load() {
			return
		}
		if _, err := s.Backup.CreateDBBackupForShutdown(); err != nil {
			a.logger.Error("automatic local database backup failed: " + err.Error())
		}
	})
	logShutdownStep("checkpoint and close database connection", func() {
		a.closeDatabase(5 * time.Second)
	})
	logShutdownStep("save final config", func() {
		if err := appconf.SaveConfig(config); err != nil {
			a.logger.Error("failed to save config: " + err.Error())
		}
	})
	logShutdownStep("shutdown Windows session-end hook", a.stopSessionEnd)
	a.logger.Info(fmt.Sprintf("shutdown completed (total elapsed: %s)", time.Since(shutdownStartedAt)))
}

func (a *App) closeDatabase(timeout time.Duration) {
	if a.DB == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := dbutils.SafeCloseDuckDB(closeCtx, a.DB, a.logger); err != nil {
		a.logger.Error("database shutdown completed with error: " + err.Error())
	}
	a.DB = nil
}

func (a *App) stopSessionEnd() {
	a.mu.Lock()
	hook := a.sessionEnd
	a.sessionEnd = nil
	a.mu.Unlock()
	if hook == nil {
		return
	}
	hook.ReleaseShutdownBlockReason()
	if err := hook.Stop(); err != nil {
		a.logger.Error("failed to shutdown Windows session-end hook: " + err.Error())
	}
}

func shouldRunAutomaticCloudSync(config *appconf.AppConfig) bool {
	if config == nil {
		return false
	}
	return config.CloudSyncEnabled && config.AutoCloudSyncEnabled
}
