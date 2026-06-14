package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/apptray"
	"lunabox/internal/autostart"
	"lunabox/internal/cli"
	"lunabox/internal/cli/ipcclient"
	"lunabox/internal/cli/ipcserver"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/protocol"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/dbutils"
	"lunabox/internal/utils/imageutils"
	"lunabox/internal/utils/sessionend"
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

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var icon []byte

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/darwin/tray-template.png
var darwinTrayIcon []byte

var db *sql.DB

var config *appconf.AppConfig

var appState = newLifecycleState()
var ipcHTTPServer *http.Server
var remoteImageProxyHTTPServer *http.Server
var sessionEndHook *sessionend.Hook

const remoteImageProxyHTTPAddr = "127.0.0.1:23680"

type lifecycleState struct {
	ctxMu sync.RWMutex
	ctx   context.Context

	forceQuit               atomic.Bool
	shuttingDown            atomic.Bool
	systemSessionEnding     atomic.Bool
	quitRequestPending      atomic.Bool
	frontendQuitSyncPlanned atomic.Bool
	frontendQuitSyncRunning atomic.Bool
	frontendQuitSyncBacked  atomic.Bool

	trayReady     chan struct{}
	trayReadyOnce sync.Once
	trayExit      chan struct{}
	trayExitOnce  sync.Once
	trayQuitOnce  sync.Once
	trayAvailable atomic.Bool
}

func newLifecycleState() *lifecycleState {
	return &lifecycleState{
		trayReady: make(chan struct{}),
		trayExit:  make(chan struct{}),
	}
}

func (s *lifecycleState) SetContext(ctx context.Context) {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	s.ctx = ctx
}

func (s *lifecycleState) Context() context.Context {
	s.ctxMu.RLock()
	defer s.ctxMu.RUnlock()
	return s.ctx
}

func (s *lifecycleState) MarkTrayReady() {
	s.trayAvailable.Store(true)
	s.trayReadyOnce.Do(func() {
		close(s.trayReady)
	})
}

func (s *lifecycleState) MarkTrayExit() {
	s.trayExitOnce.Do(func() {
		close(s.trayExit)
	})
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

	ctx := s.Context()
	if ctx == nil || s.shuttingDown.Load() {
		return
	}

	s.RequestTrayQuit()
	runtime.Quit(ctx)
}

func (s *lifecycleState) HasPendingQuitRequest() bool {
	return s.quitRequestPending.Load()
}

func (s *lifecycleState) ShowMainWindow() {
	if s.shuttingDown.Load() {
		return
	}

	ctx := s.Context()
	if ctx == nil {
		return
	}

	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	runtime.EventsEmit(ctx, "app:main-window-shown")
}

func (s *lifecycleState) EmitMainWindowShown() {
	ctx := s.Context()
	if ctx == nil || s.shuttingDown.Load() {
		return
	}

	runtime.EventsEmit(ctx, "app:main-window-shown")
}

func (s *lifecycleState) QuitApplication() {
	if s.shuttingDown.Load() {
		return
	}

	ctx := s.Context()
	if ctx == nil {
		return
	}

	s.forceQuit.Store(true)
	s.shuttingDown.Store(true)
	s.RequestTrayQuit()
	runtime.Quit(ctx)
}

func (s *lifecycleState) RequestFrontendQuitSync(reason string) bool {
	if s.shuttingDown.Load() {
		return false
	}

	ctx := s.Context()
	if ctx == nil {
		return false
	}

	if !s.quitRequestPending.CompareAndSwap(false, true) {
		return true
	}

	s.frontendQuitSyncPlanned.Store(true)
	s.frontendQuitSyncRunning.Store(false)
	s.frontendQuitSyncBacked.Store(false)
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	runtime.EventsEmit(ctx, "app:quit-sync-requested", map[string]string{
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

func (s *lifecycleState) WaitForTrayExit(timeout time.Duration) bool {
	select {
	case <-s.trayExit:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (s *lifecycleState) StartTray() {
	apptray.Start(apptray.Options{
		Icon:       icon,
		AppIcon:    appIcon,
		DarwinIcon: darwinTrayIcon,
		Callbacks: apptray.Callbacks{
			OnReady:                   s.MarkTrayReady,
			OnExit:                    s.MarkTrayExit,
			ShowMainWindow:            s.ShowMainWindow,
			NativeMainWindowDidShow:   s.EmitMainWindowShown,
			RequestFrontendQuitSync:   s.RequestFrontendQuitSync,
			QuitApplication:           s.QuitApplication,
			ShouldRunFrontendQuitSync: func() bool { return shouldRunFrontendQuitSync(config) },
		},
	})
}

func (s *lifecycleState) RequestTrayQuit() {
	s.trayQuitOnce.Do(func() {
		apptray.Stop()
	})
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
	appLogger logger.Logger,
) {
	if req == nil {
		return
	}

	if req.install != nil {
		downloadService.SetPendingInstall(req.install)
		if ctx := appState.Context(); ctx != nil {
			runtime.WindowUnminimise(ctx)
			runtime.WindowShow(ctx)
			runtime.EventsEmit(ctx, "install:pending", req.install)
		}
		return
	}

	if req.launch != nil {
		launchReq := *req.launch
		go func() {
			if err := startService.HandleProtocolLaunch(launchReq); err != nil {
				appLogger.Error("protocol launch failed: " + err.Error())
			}
		}()
	}
}

// isBindingsBuild 检测当前是否为生成绑定的构建（通过环境变量判断）
// FIXME: 现在这样做是因为前置恢复逻辑和数据库初始化会影响wails generate module的正常执行，这里用了很tricky的方法做了一个兼容
func isBindingsBuild() bool {
	_, hasTSPrefix := os.LookupEnv("tsprefix")
	_, hasTSSuffix := os.LookupEnv("tssuffix")
	_, hasTSOutputType := os.LookupEnv("tsoutputtype")

	return hasTSPrefix || hasTSSuffix || hasTSOutputType
}

func main() {
	// ================================================================
	// 启动参数预处理：在 Wails 初始化之前处理协议参数
	// ================================================================
	args := os.Args[1:]
	args, launchedByAutostart := autostart.ExtractLaunchFlag(args)

	// lunabox:// URL：检查 GUI 是否已运行
	var pendingProtocolReq *pendingProtocolRequest
	if len(args) == 1 && protocol.IsProtocolURL(args[0]) {
		req, err := parseProtocolRequest(args[0], goruntime.GOOS != "darwin")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing protocol URL: %v\n", err)
			os.Exit(1)
		}
		pendingProtocolReq = req

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
	appLogger := applog.NewFileLogger(filepath.Join(logDir, "app.log"))

	gameService := service.NewGameService()
	bangumiService := service.NewBangumiService()
	aiService := service.NewAiService()
	aiStatsBuilder := service.NewAIStatsBuilder()
	backupService := service.NewBackupService()
	cloudSyncService := service.NewCloudSyncService()
	homeService := service.NewHomeService()
	statsService := service.NewStatsService()
	startService := service.NewStartService()
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

	bindServices := []interface{}{
		gameService,
		bangumiService,
		aiService,
		backupService,
		cloudSyncService,
		homeService,
		statsService,
		startService,
		categoryService,
		configService,
		importService,
		versionService,
		templateService,
		updateService,
		sessionService,
		downloadService,
		gameProgressService,
		tagService,
		portableSetupService,
	}
	enumBindings := []interface{}{
		enums.AllSourceTypes,
		enums.AllPeriodTypes,
		enums.Prompts,
		enums.AllGameStatuses,
		enums.AllGameListSortByTypes,
		enums.AllSortOrderTypes,
	}

	// 如果有待安装 URL，解析并暂存到 downloadService
	if pendingProtocolReq != nil && pendingProtocolReq.install != nil {
		downloadService.SetPendingInstall(pendingProtocolReq.install)
	}

	if isBindingsBuild() {
		bootstrapErr := wails.Run(&options.App{
			Bind:     bindServices,
			EnumBind: enumBindings,
		})
		if bootstrapErr != nil {
			fmt.Fprintf(os.Stderr, "generate bindings failed: %v\n", bootstrapErr)
			os.Exit(1)
		}
		return
	}

	execPath, err := apputils.GetDataDir()
	if err != nil {
		appLogger.Fatal(err.Error())
	}
	dbPath := filepath.Join(execPath, "lunabox.db")
	db, err = dbutils.OpenDuckDBWithWALRecovery(context.Background(), dbPath, appLogger)
	if err != nil {
		appLogger.Fatal(err.Error())
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

	// Create application with options
	// 使用配置中保存的窗口尺寸，如果小于最小值则使用最小值
	initWidth := config.WindowWidth
	if initWidth < 970 {
		initWidth = 970
	}
	initHeight := config.WindowHeight
	if initHeight < 563 {
		initHeight = 563
	}

	bootstrapErr := wails.Run(&options.App{
		Title:     "LunaBox",
		Logger:    appLogger,
		LogLevel:  logger.INFO,
		Width:     initWidth,
		Height:    initHeight,
		MinWidth:  970,
		MinHeight: 563,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// 跨域处理
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

					if r.Method == "OPTIONS" {
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
		BackgroundColour: &options.RGBA{R: 18, G: 20, B: 22, A: 0},
		StartHidden:      true,
		Frameless:        goruntime.GOOS != "darwin", // macOS 需要原生 NSWindow 才有 traffic lights 和系统圆角
		// 启用拖拽文件导入功能
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
			CSSDropProperty:    "--wails-drop-target",
			CSSDropValue:       "drop",
		},
		// 样式完全交由wails前端控制
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Auto,
			Theme:                windows.SystemDefault,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHidden(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			OnUrlOpen: func(rawURL string) {
				req, err := parseProtocolRequest(rawURL, false)
				if err != nil {
					appLogger.Error("failed to handle macOS protocol URL: " + err.Error())
					return
				}
				dispatchProtocolRequest(req, downloadService, startService, appLogger)
			},
		},
		// 关闭窗口时的处理
		OnBeforeClose: func(ctx context.Context) bool {
			// 保存当前窗口大小（只在非最大化时）
			if !runtime.WindowIsMaximised(ctx) {
				config.WindowWidth, config.WindowHeight = runtime.WindowGetSize(ctx)
			}

			// 如果是从托盘强制退出，直接允许关闭
			if appState.ShouldForceQuit() {
				return false
			}
			if appState.HasPendingQuitRequest() {
				return true
			}
			if config.CloseToTray && appState.IsTrayAvailable() {
				runtime.WindowHide(ctx)
				return true
			}
			if shouldRunFrontendQuitSync(config) {
				return appState.RequestFrontendQuitSync("window-close")
			}
			return false
		},
		OnStartup: func(ctx context.Context) {
			appState.SetContext(ctx)
			applog.SetMode(applog.ModeGUI)
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

			configService.Init(ctx, db, config)
			configService.SetSuppressInitialWindowShow(launchedByAutostart)
			configService.SetQuitHandler(func() {
				appState.QuitApplication()
			})

			if err := autostart.Sync(config.LaunchAtLogin); err != nil {
				appLogger.Error("failed to sync launch-at-login: " + err.Error())
			}

			downloadService.Init(ctx, db, config)
			gameService.Init(ctx, db, config)
			bangumiService.Init(ctx, db, config)
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
			categoryService.Init(ctx, db, config)
			importService.Init(ctx, db, config, gameService)
			versionService.Init(ctx)
			templateService.Init(ctx, db, config)
			updateService.Init(ctx, configService)
			gameProgressService.Init(ctx, db, config)
			mcpReadService.Init(ctx, db, config)
			mcpServerService.Init(ctx)
			portableSetupService.Init(ctx)

			startService.SetBackupService(backupService)
			startService.SetGameService(gameService)
			startService.SetSessionService(sessionService)
			downloadService.SetGameService(gameService)
			gameService.SetImageDownloadTaskStarter(downloadService.StartCoverImageDownloadTask)
			gameService.SetTagService(tagService)
			gameService.SetBangumiService(bangumiService)
			importService.SetBangumiService(bangumiService)
			importService.SetSessionService(sessionService)
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
			if err := mcpServerService.ApplyConfig(*config); err != nil {
				appLogger.Error("failed to apply MCP server config: " + err.Error())
			}

			if pendingProtocolReq != nil && pendingProtocolReq.launch != nil {
				req := *pendingProtocolReq.launch
				go func() {
					// 等待前端完成事件订阅，确保协议启动失败时用户能看到提示。
					time.Sleep(1200 * time.Millisecond)
					if err := startService.HandleProtocolLaunch(req); err != nil {
						appLogger.Error("protocol launch failed: " + err.Error())
					}
				}()
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
			ipcHTTPServer = ipcserver.StartServer(cliApp)

			// 在 Wails 启动后初始化系统托盘
			// TODO: 升级wails v3，使用原生的托盘功能
			appState.StartTray()

			// 等待托盘初始化完成，避免竞态条件
			select {
			case <-appState.trayReady:
				appLogger.Info("system tray initialized successfully")
			case <-time.After(5 * time.Second):
				appLogger.Error("system tray initialization timed out")
			}

			if shouldRunAutomaticCloudSync(config) {
				cloudSyncService.RunStartupSync()
			}
			cloudSyncService.StartScheduledSync()
		},
		OnShutdown: func(ctx context.Context) {
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

			logShutdownStep("shutdown system tray", func() {
				appState.RequestTrayQuit()
				if appState.WaitForTrayExit(1200 * time.Millisecond) {
					appLogger.Info("system tray exited successfully")
				} else {
					appLogger.Warning("system tray exit timed out, continuing shutdown")
				}
			})

			appLogger.Info(fmt.Sprintf("shutdown completed (total elapsed: %s)", time.Since(shutdownStartedAt)))
		},
		Bind:     bindServices,
		EnumBind: enumBindings,
	})

	if bootstrapErr != nil {
		appLogger.Fatal(bootstrapErr.Error())
	}
}
