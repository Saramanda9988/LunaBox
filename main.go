package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"lunabox/internal/applog"
	"lunabox/internal/cli"
	"lunabox/internal/cli/ipcclient"
	"lunabox/internal/cli/ipcserver"
	"lunabox/internal/common/vo"
	"lunabox/internal/protocol"
	"lunabox/internal/utils"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/dbutils"
	"lunabox/internal/utils/imageutils"
	"lunabox/internal/utils/sessionend"
	"lunabox/internal/wailsruntime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/migrations"
	_ "lunabox/internal/platform"
	"lunabox/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/darwin/appicon.png
var darwinAppIcon []byte

//go:embed build/darwin/tray-template.png
var darwinTrayIcon []byte

//go:embed build/windows/tray.png
var windowsTrayIcon []byte

var db *sql.DB

var config *appconf.AppConfig

var appState = newLifecycleState()
var ipcHTTPServer *http.Server
var remoteImageProxyHTTPServer *http.Server
var sessionEndHook *sessionend.Hook

const remoteImageProxyHTTPAddr = "127.0.0.1:23680"

type lifecycleState struct {
	ctxMu  sync.RWMutex
	ctx    context.Context
	app    *application.App
	window *application.WebviewWindow

	forceQuit               atomic.Bool
	shuttingDown            atomic.Bool
	systemSessionEnding     atomic.Bool
	quitRequestPending      atomic.Bool
	frontendQuitSyncPlanned atomic.Bool
	frontendQuitSyncRunning atomic.Bool
	frontendQuitSyncBacked  atomic.Bool

	trayAvailable atomic.Bool
}

func newLifecycleState() *lifecycleState {
	return &lifecycleState{}
}

func (s *lifecycleState) SetContext(ctx context.Context) {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	s.ctx = ctx
}

func (s *lifecycleState) SetRuntime(app *application.App, window *application.WebviewWindow) {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	s.app = app
	s.window = window
}

func (s *lifecycleState) Runtime() (*application.App, *application.WebviewWindow) {
	s.ctxMu.RLock()
	defer s.ctxMu.RUnlock()
	return s.app, s.window
}

func (s *lifecycleState) Context() context.Context {
	s.ctxMu.RLock()
	defer s.ctxMu.RUnlock()
	return s.ctx
}

func (s *lifecycleState) IsTrayAvailable() bool {
	return s.trayAvailable.Load() && !s.shuttingDown.Load()
}

func (s *lifecycleState) ShouldForceQuit() bool {
	return s.forceQuit.Load() || s.shuttingDown.Load()
}

func (s *lifecycleState) BeginShutdown() {
	s.shuttingDown.Store(true)
}

func (s *lifecycleState) MarkSystemSessionEnding() {
	s.systemSessionEnding.Store(true)
	s.forceQuit.Store(true)
}

func (s *lifecycleState) IsSystemSessionEnding() bool {
	return s.systemSessionEnding.Load()
}

func (s *lifecycleState) QuitForSystemSessionEnd() {
	s.MarkSystemSessionEnding()

	app, _ := s.Runtime()
	if app == nil || s.shuttingDown.Load() {
		return
	}

	app.Quit()
}

func (s *lifecycleState) HasPendingQuitRequest() bool {
	return s.quitRequestPending.Load()
}

func (s *lifecycleState) ShowMainWindow() {
	if s.shuttingDown.Load() {
		return
	}

	app, window := s.Runtime()
	if app == nil || window == nil {
		return
	}

	window.Restore()
	window.Show()
	window.Focus()
	app.Event.Emit("app:main-window-shown")
}

func (s *lifecycleState) QuitApplication() {
	if s.shuttingDown.Load() {
		return
	}

	app, _ := s.Runtime()
	if app == nil {
		return
	}

	s.forceQuit.Store(true)
	s.shuttingDown.Store(true)
	app.Quit()
}

func (s *lifecycleState) ShouldQuitApplication(config *appconf.AppConfig) bool {
	if s.ShouldForceQuit() {
		return true
	}
	if s.HasPendingQuitRequest() {
		return false
	}
	if shouldRunFrontendQuitSync(config) && s.RequestFrontendQuitSync("application-quit") {
		return false
	}

	// Native application quit (for example Cmd+Q) must bypass the window-close
	// hook, which may otherwise interpret shutdown as a close-to-tray request.
	s.forceQuit.Store(true)
	return true
}

func (s *lifecycleState) RequestFrontendQuitSync(reason string) bool {
	if s.shuttingDown.Load() {
		return false
	}

	app, window := s.Runtime()
	if app == nil || window == nil {
		return false
	}

	if !s.quitRequestPending.CompareAndSwap(false, true) {
		return true
	}

	s.frontendQuitSyncPlanned.Store(true)
	s.frontendQuitSyncRunning.Store(false)
	s.frontendQuitSyncBacked.Store(false)
	window.Restore()
	window.Show()
	app.Event.Emit("app:quit-sync-requested", map[string]string{
		"reason": reason,
	})
	return true
}

func (s *lifecycleState) BeginFrontendQuitSyncBackup() {
	s.frontendQuitSyncRunning.Store(true)
}

func (s *lifecycleState) MarkFrontendQuitSyncLocalBackupCreated() {
	s.frontendQuitSyncBacked.Store(true)
}

func (s *lifecycleState) FinishFrontendQuitSyncBackup() {
	s.frontendQuitSyncRunning.Store(false)
}

func (s *lifecycleState) WaitForFrontendQuitSyncBackup(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for s.frontendQuitSyncRunning.Load() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
	return true
}

func (s *lifecycleState) ConfigureTray() {
	app, _ := s.Runtime()
	if app == nil {
		return
	}

	menu := app.NewMenu()
	menu.Add("显示主窗口").OnClick(func(_ *application.Context) {
		s.ShowMainWindow()
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(_ *application.Context) {
		if shouldRunFrontendQuitSync(config) && s.RequestFrontendQuitSync("tray-menu") {
			return
		}
		s.QuitApplication()
	})

	tray := app.SystemTray.New()
	if goruntime.GOOS == "linux" {
		tray.SetLabel("LunaBox")
	}
	tray.SetMenu(menu)
	tray.SetTooltip("LunaBox")
	if goruntime.GOOS == "darwin" {
		tray.SetTemplateIcon(darwinTrayIcon)
	} else {
		// Wails v3 alpha passes a complete ICO container to an API that expects
		// one image resource. Use the extracted 32x32 ICO frame for the tray.
		tray.SetIcon(windowsTrayIcon)
		if goruntime.GOOS == "windows" {
			tray.OnClick(s.ShowMainWindow)
			tray.OnDoubleClick(s.ShowMainWindow)
		}
	}
	s.trayAvailable.Store(true)
}

func shouldRunFrontendQuitSync(config *appconf.AppConfig) bool {
	if config == nil {
		return false
	}

	return config.AutoBackupDB &&
		config.CloudBackupEnabled &&
		config.BackupUserID != "" &&
		config.AutoUploadDBToCloud
}

func shouldRunAutomaticCloudSync(config *appconf.AppConfig) bool {
	if config == nil {
		return false
	}

	return config.CloudSyncEnabled && config.AutoCloudSyncEnabled
}

type pendingProtocolRequest struct {
	rawURL  string
	install *vo.InstallRequest
	launch  *vo.ProtocolLaunchRequest
}

func parseProtocolRequest(rawURL string, allowLaunch bool) (*pendingProtocolRequest, error) {
	action, err := protocol.ParseAction(rawURL)
	if err != nil {
		return nil, err
	}

	req := &pendingProtocolRequest{rawURL: rawURL}
	switch action {
	case protocol.ActionInstall:
		installReq, err := protocol.ParseInstallURL(rawURL)
		if err != nil {
			return nil, err
		}
		req.install = installReq
	case protocol.ActionLaunch:
		if !allowLaunch {
			return nil, fmt.Errorf("lunabox://launch is not supported on macOS yet")
		}
		launchReq, err := protocol.ParseLaunchURL(rawURL)
		if err != nil {
			return nil, err
		}
		req.launch = launchReq
	default:
		return nil, fmt.Errorf("unsupported URL action: %s", action)
	}

	return req, nil
}

func forwardProtocolRequestToRunningInstance(req *pendingProtocolRequest) error {
	switch {
	case req == nil:
		return nil
	case req.install != nil:
		return ipcclient.RemoteInstall(req.install)
	case req.launch != nil:
		return ipcclient.RemoteLaunch(req.launch)
	default:
		return fmt.Errorf("unsupported protocol request: %s", req.rawURL)
	}
}

func dispatchProtocolRequest(
	req *pendingProtocolRequest,
	downloadService *service.DownloadService,
	startService *service.StartService,
	runtime wailsruntime.Runtime,
	appLogger *applog.FileLogger,
) {
	if req == nil {
		return
	}

	if req.install != nil {
		downloadService.SetPendingInstall(req.install)
		appState.ShowMainWindow()
		runtime.Emit("install:pending", req.install)
		return
	}

	if req.launch != nil {
		launchReq := *req.launch
		go func() {
			// The Wails launch event may arrive before the frontend subscribes to
			// protocol-launch:error, so give the runtime a moment to become ready.
			time.Sleep(1200 * time.Millisecond)
			if err := startService.HandleProtocolLaunch(launchReq); err != nil {
				appLogger.Error("protocol launch failed: " + err.Error())
			}
		}()
	}
}

type startupCoordinator struct {
	startup func(context.Context)
}

func (s *startupCoordinator) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.startup(ctx)
	return nil
}

func extractAutostartLaunchFlag(args []string) ([]string, bool) {
	cleanArgs := make([]string, 0, len(args))
	launchedByAutostart := false

	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), wailsruntime.AutostartLaunchArgument) {
			launchedByAutostart = true
			continue
		}
		cleanArgs = append(cleanArgs, arg)
	}

	return cleanArgs, launchedByAutostart
}

func main() {
	// ================================================================
	// 启动参数预处理：在 Wails 初始化之前处理协议参数
	// ================================================================
	args := os.Args[1:]
	args, launchedByAutostart := extractAutostartLaunchFlag(args)

	// lunabox:// URL：检查 GUI 是否已运行
	if len(args) == 1 && protocol.IsProtocolURL(args[0]) {
		req, err := parseProtocolRequest(args[0], goruntime.GOOS != "darwin")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing protocol URL: %v\n", err)
			os.Exit(1)
		}
		if ipcclient.IsServerRunning() {
			if err := forwardProtocolRequestToRunningInstance(req); err != nil {
				fmt.Fprintf(os.Stderr, "Error forwarding protocol request to LunaBox: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// ================================================================
	applog.SetMode(applog.ModeCLI)

	var loadErr error
	config, loadErr = appconf.LoadConfig()
	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", loadErr)
		os.Exit(1)
	}

	if config.PendingFullRestore != "" {
		restored, restoreErr := service.ExecuteFullDataRestore(config)
		if restoreErr != nil {
			fmt.Fprintf(os.Stderr, "fail to restore full data: %v\n", restoreErr)
		} else if restored {
			fmt.Fprintln(os.Stdout, "full data restored successfully")
		}
	}

	if config.PendingDBRestore != "" {
		restored, restoreErr := service.ExecuteDBRestore(config)
		if restoreErr != nil {
			fmt.Fprintf(os.Stderr, "fail to restore database: %v\n", restoreErr)
		} else if restored {
			fmt.Fprintln(os.Stdout, "database restored successfully")
		}
	}

	logDir, _ := apputils.GetSubDir("logs")
	const applicationLogLevel = slog.LevelInfo
	appLogger := applog.NewFileLogger(filepath.Join(logDir, "app.log"), applicationLogLevel)

	gameService := service.NewGameService()
	bangumiService := service.NewBangumiService()
	hikarinagiService := service.NewHikarinagiService()
	aiService := service.NewAiService()
	aiStatsBuilder := service.NewAIStatsBuilder()
	backupService := service.NewBackupService()
	cloudSyncService := service.NewCloudSyncService()
	homeService := service.NewHomeService()
	statsService := service.NewStatsService()
	startService := service.NewStartService()
	integrationService := service.NewIntegrationService()
	categoryService := service.NewCategoryService()
	configService := service.NewConfigService()
	importService := service.NewImportService()
	versionService := service.NewVersionService()
	templateService := service.NewTemplateService()
	updateService := service.NewUpdateService()
	sessionService := service.NewSessionService()
	downloadService := service.NewDownloadService()
	gameProgressService := service.NewGameProgressService()
	tagService := service.NewTagService()
	mcpReadService := service.NewMCPReadService()
	mcpServerService := service.NewMCPServerService()
	portableSetupService := service.NewPortableSetupService()

	execPath, err := apputils.GetDataDir()
	if err != nil {
		appLogger.Fatal(err.Error())
	}
	dbPath := filepath.Join(execPath, "lunabox.db")
	db, err = dbutils.OpenDuckDBWithWALRecovery(context.Background(), dbPath, appLogger)
	if err != nil {
		appLogger.Fatal(err.Error())
	}
	if _, err = db.Exec("SET GLOBAL checkpoint_threshold = '4 MiB'"); err != nil {
		appLogger.Warning("Failed to set DuckDB automatic checkpoint threshold; using the default: " + err.Error())
	} else {
		appLogger.Info("DuckDB automatic checkpoint threshold set to 4 MiB")
	}

	timeZone := config.TimeZone
	if timeZone == "" {
		timeZone = "UTC"
		appLogger.Warning("TimeZone not configured, using UTC. Please set timezone in settings.")
	}

	_, err = db.Exec(fmt.Sprintf("SET TimeZone = '%s'", timeZone))
	if err != nil {
		appLogger.Warning("Failed to set timezone: " + err.Error())
	} else {
		appLogger.Info("Database timezone set to: " + timeZone)
	}

	if err := migrations.InitSchema(db); err != nil {
		appLogger.Fatal(err.Error())
	}

	appLogger.Info("Checking for pending database migrations...")
	if err := migrations.Run(context.Background(), db); err != nil {
		appLogger.Fatal("Database migration failed: " + err.Error())
	}
	appLogger.Info("Database migrations completed")
	if err := dbutils.CheckpointDuckDB(context.Background(), db); err != nil {
		appLogger.Warning("Database checkpoint after schema initialization failed; committed changes remain in WAL: " + err.Error())
	} else {
		appLogger.Info("Database checkpoint after schema initialization completed")
	}

	initBoundServices := func(ctx context.Context) {
		configService.Init(ctx, db, config)
		configService.SetSuppressInitialWindowShow(launchedByAutostart)
		configService.SetQuitHandler(func() {
			appState.QuitApplication()
		})

		downloadService.Init(ctx, db, config)
		gameService.Init(ctx, db, config)
		bangumiService.Init(ctx, db, config)
		hikarinagiService.Init(ctx, db, config)
		tagService.Init(ctx, db, config)
		aiService.Init(ctx, db, config)
		aiStatsBuilder.Init(ctx, db, config)
		backupService.Init(ctx, db, config)
		cloudSyncService.Init(ctx, db, config)
		service.ConfigureBackupServiceQuitSyncDBBackupHooks(
			backupService,
			func() {
				appState.BeginFrontendQuitSyncBackup()
			},
			func() {
				appState.MarkFrontendQuitSyncLocalBackupCreated()
			},
			func() {
				appState.FinishFrontendQuitSyncBackup()
			},
		)
		homeService.Init(ctx, db, config)
		statsService.Init(ctx, db, config)
		sessionService.Init(ctx, db, config)
		startService.Init(ctx, db, config)
		integrationService.Init(ctx, db, config)
		categoryService.Init(ctx, db, config)
		importService.Init(ctx, db, config)
		versionService.Init(ctx)
		templateService.Init(ctx, db, config)
		updateService.Init(ctx)
		gameProgressService.Init(ctx, db, config)
		mcpReadService.Init(ctx, db, config)
		mcpServerService.Init(ctx)
		portableSetupService.Init(ctx)

		startService.SetBackupService(backupService)
		startService.SetGameService(gameService)
		startService.SetIntegrationService(integrationService)
		startService.SetSessionService(sessionService)
		downloadService.SetGameService(gameService)
		gameService.SetImageDownloadTaskStarter(downloadService.StartCoverImageDownloadTask)
		gameService.SetTagService(tagService)
		gameService.SetBangumiService(bangumiService)
		gameService.SetHikarinagiService(hikarinagiService)
		importService.SetGameService(gameService)
		integrationService.SetGameService(gameService)
		importService.SetBangumiService(bangumiService)
		importService.SetHikarinagiService(hikarinagiService)
		importService.SetSessionService(sessionService)
		updateService.SetConfigService(configService)
		mcpReadService.SetGameService(gameService)
		mcpReadService.SetStartService(startService)
		mcpReadService.SetSessionService(sessionService)
		mcpReadService.SetGameProgressService(gameProgressService)
		mcpReadService.SetTagService(tagService)
		mcpReadService.SetStatsProvider(aiStatsBuilder)
		mcpServerService.SetReadService(mcpReadService)
		configService.SetConfigUpdateHook(func(updatedConfig appconf.AppConfig) error {
			return mcpServerService.ApplyConfig(updatedConfig)
		})
	}

	// 创建本地文件处理器
	localFileHandler, err := apputils.NewLocalFileHandler()
	if err != nil {
		appLogger.Error("Warning: Failed to create local file handler: " + err.Error())
	}
	remoteImageProxyHandler := imageutils.NewRemoteImageProxyHandler(config)
	remoteImageProxyListener, err := net.Listen("tcp", remoteImageProxyHTTPAddr)
	if err != nil {
		appLogger.Warning("Warning: Failed to start remote image proxy server: " + err.Error())
	} else {
		imageProxyMux := http.NewServeMux()
		imageProxyMux.Handle("/proxy/image", remoteImageProxyHandler)
		remoteImageProxyHTTPServer = &http.Server{
			Handler:           imageProxyMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		appLogger.Info("Remote image proxy server listening on http://" + remoteImageProxyHTTPAddr + "/proxy/image")
		go func() {
			if serveErr := remoteImageProxyHTTPServer.Serve(remoteImageProxyListener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				appLogger.Error("Remote image proxy server failed: " + serveErr.Error())
			}
		}()
	}

	// 使用配置中保存的窗口尺寸，如果小于最小值则使用最小值
	initWidth := config.WindowWidth
	if initWidth < 970 {
		initWidth = 970
	}
	initHeight := config.WindowHeight
	if initHeight < 563 {
		initHeight = 563
	}
	guiRuntime := wailsruntime.Unavailable()

	coordinator := &startupCoordinator{
		startup: func(ctx context.Context) {
			appState.SetContext(ctx)
			applog.SetMode(applog.ModeGUI)
			applog.SetLogger(appLogger)
			if os.Getenv("FRONTEND_DEVSERVER_URL") != "" {
				if err := utils.LoadEnvFilesIfExists(".env.build", ".env"); err != nil {
					appLogger.Warning("failed to load dev env files: " + err.Error())
				}
				utils.ApplyDevBuildEnvFallbacks()
			}
			initBoundServices(ctx)
		},
	}

	applicationStarted := func() {
		ctx := appState.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if err := sessionService.CleanupUnfinishedSessions(); err != nil {
			appLogger.Error("startup cleanup unfinished sessions failed: " + err.Error())
		} else {
			appLogger.Info("startup cleanup unfinished sessions completed")
		}

		var sessionHookErr error
		sessionEndHook, sessionHookErr = sessionend.Start(sessionend.Options{
			Reason: "LunaBox 正在保存数据并退出",
			OnQueryEndSession: func() {
				appLogger.Warning("Windows session end requested; starting short shutdown")
				appState.QuitForSystemSessionEnd()
			},
		})
		if sessionHookErr != nil {
			appLogger.Error("failed to start Windows session-end hook: " + sessionHookErr.Error())
		} else {
			appLogger.Info("Windows session-end hook started")
		}

		if err := guiRuntime.SetAutostart(config.LaunchAtLogin); err != nil {
			appLogger.Error("failed to sync launch-at-login: " + err.Error())
		}

		if err := mcpServerService.ApplyConfig(*config); err != nil {
			appLogger.Error("failed to apply MCP server config: " + err.Error())
		}

		// 启动 IPC Server (用于 CLI 通信)
		// 构造 CLI CoreApp 以共享 GUI 的服务实例
		cliApp := &cli.CoreApp{
			Config:         config,
			DB:             db,
			Ctx:            ctx,
			GameService:    gameService,
			StartService:   startService,
			SessionService: sessionService,
			BackupService:  backupService,
			VersionService: versionService,
		}
		ipcHTTPServer = ipcserver.StartServer(cliApp, guiRuntime)

		if shouldRunAutomaticCloudSync(config) {
			cloudSyncService.RunStartupSync()
		}
		cloudSyncService.StartScheduledSync()
	}

	shutdownApplication := func() {
		appState.BeginShutdown()
		isSystemSessionEnding := appState.IsSystemSessionEnding()
		shutdownMode := "normal"
		if isSystemSessionEnding {
			shutdownMode = "system-session-ending"
		}

		cloudSyncService.StopScheduledSync()

		shutdownStartedAt := time.Now()
		appLogger.Info("shutdown mode: " + shutdownMode)
		logShutdownStep := func(step string, fn func()) {
			stepStartedAt := time.Now()
			appLogger.Info("shutdown step started: " + step)
			fn()
			appLogger.Info(fmt.Sprintf("shutdown step finished: %s (elapsed: %s)", step, time.Since(stepStartedAt)))
		}

		logShutdownStep("shutdown IPC server", func() {
			// 先关闭 IPC Server，避免退出过程中还有外部请求进入。
			if err := ipcserver.ShutdownServer(ipcHTTPServer); err != nil {
				appLogger.Error("failed to shutdown IPC server: " + err.Error())
			}
		})

		logShutdownStep("shutdown MCP server", func() {
			if err := mcpServerService.Shutdown(); err != nil {
				appLogger.Error("failed to shutdown MCP server: " + err.Error())
			}
		})

		logShutdownStep("shutdown remote image proxy server", func() {
			if remoteImageProxyHTTPServer == nil {
				return
			}
			closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := remoteImageProxyHTTPServer.Shutdown(closeCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				appLogger.Error("failed to shutdown remote image proxy server: " + err.Error())
			}
			remoteImageProxyHTTPServer = nil
		})

		// 从 configService 获取最新配置（避免使用启动时的旧配置覆盖文件）
		logShutdownStep("refresh latest config", func() {
			latestConfig, err := configService.GetAppConfig()
			if err != nil {
				appLogger.Error("failed to get latest config: " + err.Error())
			} else {
				// 更新窗口大小到最新配置
				latestConfig.WindowWidth = config.WindowWidth
				latestConfig.WindowHeight = config.WindowHeight
				config = &latestConfig
			}
		})

		// 清理所有待定的进程选择会话（防止遗留临时会话）
		logShutdownStep("cleanup pending process selections", func() {
			appLogger.Info("cleaning up pending process selections...")
			startService.CleanupPendingSessions()
		})

		logShutdownStep("automatic database backup", func() {
			// 退出流程只做本地数据库备份，避免网络上传拖慢或阻塞应用退出。
			if isSystemSessionEnding {
				appLogger.Info("system session ending, skipping automatic database backup")
				return
			}
			if !config.AutoBackupDB {
				appLogger.Info("automatic database backup disabled, skipping")
				return
			}
			if appState.frontendQuitSyncPlanned.Load() {
				if appState.WaitForFrontendQuitSyncBackup(3 * time.Second) {
					appLogger.Info("frontend quit sync backup flow settled before shutdown fallback check")
				} else {
					appLogger.Warning("frontend quit sync backup flow still running after grace period, checking fallback backup state")
				}
				if appState.frontendQuitSyncBacked.Load() {
					appLogger.Info("automatic database backup already produced a local backup in frontend quit sync flow, skipping shutdown backup")
					return
				}
			}

			appLogger.Info("performing automatic local database backup...")
			if _, err := backupService.CreateDBBackup(); err != nil {
				appLogger.Error("automatic local database backup failed: " + err.Error())
			} else {
				appLogger.Info("automatic local database backup succeeded")
			}
		})

		// 关闭数据库连接
		logShutdownStep("checkpoint and close database connection", func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := dbutils.SafeCloseDuckDB(closeCtx, db, appLogger); err != nil {
				appLogger.Error("database shutdown completed with error: " + err.Error())
			}
		})

		// 保存最终配置
		logShutdownStep("save final config", func() {
			if err := appconf.SaveConfig(config); err != nil {
				appLogger.Error("failed to save config: " + err.Error())
			}
		})

		logShutdownStep("shutdown Windows session-end hook", func() {
			if sessionEndHook == nil {
				return
			}
			sessionEndHook.ReleaseShutdownBlockReason()
			if err := sessionEndHook.Stop(); err != nil {
				appLogger.Error("failed to shutdown Windows session-end hook: " + err.Error())
			}
		})

		appLogger.Info(fmt.Sprintf("shutdown completed (total elapsed: %s)", time.Since(shutdownStartedAt)))
	}

	applicationServices := []application.Service{
		application.NewService(coordinator),
		application.NewService(gameService),
		application.NewService(bangumiService),
		application.NewService(hikarinagiService),
		application.NewService(aiService),
		application.NewService(backupService),
		application.NewService(cloudSyncService),
		application.NewService(homeService),
		application.NewService(statsService),
		application.NewService(startService),
		application.NewService(integrationService),
		application.NewService(categoryService),
		application.NewService(configService),
		application.NewService(importService),
		application.NewService(versionService),
		application.NewService(templateService),
		application.NewService(updateService),
		application.NewService(sessionService),
		application.NewService(downloadService),
		application.NewService(gameProgressService),
		application.NewService(tagService),
		application.NewService(portableSetupService),
	}
	applicationIcon := appIcon
	if goruntime.GOOS == "darwin" {
		applicationIcon = darwinAppIcon
	}

	wailsApp := application.New(application.Options{
		Name:        "LunaBox",
		Description: "LunaBox game library manager",
		Icon:        applicationIcon,
		Logger:      appLogger.Slog(),
		LogLevel:    applicationLogLevel,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusOK)
						return
					}
					if localFileHandler != nil && strings.HasPrefix(r.URL.Path, "/local/") {
						localFileHandler.ServeHTTP(w, r)
						return
					}
					if remoteImageProxyHandler != nil && r.URL.Path == "/proxy/image" {
						remoteImageProxyHandler.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			ProgramName: "io.github.saramanda9988.lunabox",
		},
		Services:   applicationServices,
		OnShutdown: shutdownApplication,
		ShouldQuit: func() bool {
			if goruntime.GOOS != "darwin" {
				return true
			}
			return appState.ShouldQuitApplication(config)
		},
	})

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "LunaBox",
		URL:              "/",
		Width:            initWidth,
		Height:           initHeight,
		MinWidth:         970,
		MinHeight:        563,
		Hidden:           true,
		Frameless:        goruntime.GOOS != "darwin",
		EnableFileDrop:   true,
		BackgroundType:   application.BackgroundTypeTranslucent,
		BackgroundColour: application.NewRGBA(18, 20, 22, 0),
		Windows: application.WindowsWindow{
			BackdropType: application.Auto,
			Theme:        application.SystemDefault,
		},
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHidden,
			Backdrop: application.MacBackdropTranslucent,
		},
	})
	appState.SetRuntime(wailsApp, mainWindow)
	guiRuntime = wailsruntime.New(wailsApp, mainWindow)
	backupService.SetRuntime(guiRuntime)
	bangumiService.SetRuntime(guiRuntime)
	hikarinagiService.SetRuntime(guiRuntime)
	cloudSyncService.SetRuntime(guiRuntime)
	configService.SetRuntime(guiRuntime)
	downloadService.SetRuntime(guiRuntime)
	gameService.SetRuntime(guiRuntime)
	importService.SetRuntime(guiRuntime)
	startService.SetRuntime(guiRuntime)
	statsService.SetRuntime(guiRuntime)
	templateService.SetRuntime(guiRuntime)
	updateService.SetRuntime(guiRuntime)
	appState.ConfigureTray()

	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		applicationStarted()
	})
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(event *application.ApplicationEvent) {
		rawURL := event.Context().URL()
		req, err := parseProtocolRequest(rawURL, goruntime.GOOS != "darwin")
		if err != nil {
			appLogger.Error("failed to handle protocol URL: " + err.Error())
			return
		}
		dispatchProtocolRequest(req, downloadService, startService, guiRuntime, appLogger)
	})
	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		wailsApp.Event.Emit("files-dropped", event.Context().DroppedFiles())
	})
	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if !mainWindow.IsMaximised() {
			config.WindowWidth, config.WindowHeight = mainWindow.Size()
		}
		if appState.ShouldForceQuit() {
			return
		}
		if appState.HasPendingQuitRequest() {
			event.Cancel()
			return
		}
		if config.CloseToTray && appState.IsTrayAvailable() {
			mainWindow.Hide()
			event.Cancel()
			return
		}
		if shouldRunFrontendQuitSync(config) && appState.RequestFrontendQuitSync("window-close") {
			event.Cancel()
			return
		}
		if goruntime.GOOS == "darwin" {
			// Do not rely on Wails v3 alpha's last-window termination path. Route
			// the close through App.Quit so OnShutdown always tears down Go services.
			event.Cancel()
			go appState.QuitApplication()
		}
	})

	if err := wailsApp.Run(); err != nil {
		appLogger.Fatal(err.Error())
	}
}
