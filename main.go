package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/cli"
	"lunabox/internal/cli/ipcclient"
	"lunabox/internal/cli/ipcserver"
	"lunabox/internal/protocol"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/vo"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/migrations"
	"lunabox/internal/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/energye/systray"

	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var icon []byte

var db *sql.DB

var config *appconf.AppConfig

var appState = newLifecycleState()
var ipcHTTPServer *http.Server

type lifecycleState struct {
	ctxMu sync.RWMutex
	ctx   context.Context

	forceQuit    atomic.Bool
	shuttingDown atomic.Bool

	trayReady     chan struct{}
	trayReadyOnce sync.Once
	trayExit      chan struct{}
	trayExitOnce  sync.Once
	trayQuitOnce  sync.Once
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
	s.trayReadyOnce.Do(func() {
		close(s.trayReady)
	})
}

func (s *lifecycleState) MarkTrayExit() {
	s.trayExitOnce.Do(func() {
		close(s.trayExit)
	})
}

func (s *lifecycleState) ShouldForceQuit() bool {
	return s.forceQuit.Load() || s.shuttingDown.Load()
}

func (s *lifecycleState) BeginShutdown() {
	s.shuttingDown.Store(true)
}

func (s *lifecycleState) ShowMainWindow() {
	if s.shuttingDown.Load() {
		return
	}

	ctx := s.Context()
	if ctx == nil {
		return
	}

	runtime.WindowShow(ctx)
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

func (s *lifecycleState) StartTray() {
	go func() {
		goruntime.LockOSThread()
		defer goruntime.UnlockOSThread()
		systray.Run(onSystrayReady, onSystrayExit)
	}()
}

func (s *lifecycleState) RequestTrayQuit() {
	s.trayQuitOnce.Do(func() {
		systray.Quit()
	})
}

func (s *lifecycleState) WaitForTrayExit(timeout time.Duration) bool {
	select {
	case <-s.trayExit:
		return true
	case <-time.After(timeout):
		return false
	}
}

func main() {
	// ================================================================
	// 启动参数预处理：在 Wails 初始化之前处理协议参数
	// ================================================================
	args := os.Args[1:]

	// lunabox:// URL：检查 GUI 是否已运行
	var pendingURL string
	var pendingInstallReq *vo.InstallRequest
	if len(args) == 1 && protocol.IsProtocolURL(args[0]) {
		pendingURL = args[0]
		req, err := protocol.ParseURL(pendingURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing URL: %v\n", err)
			os.Exit(1)
		}
		pendingInstallReq = req

		// 如果 GUI 已运行，转发安装请求给它并退出
		if ipcclient.IsServerRunning() {
			if err := ipcclient.RemoteInstall(req); err != nil {
				fmt.Fprintf(os.Stderr, "Error forwarding to LunaBox: %v\n", err)
				os.Exit(1)
			}
			return
		}
		// GUI 未运行，当前进程继续启动 GUI
	}

	// ================================================================
	logDir, _ := apputils.GetSubDir("logs")
	appLogger := applog.NewFileLogger(filepath.Join(logDir, "app.log"))

	var loadErr error
	config, loadErr = appconf.LoadConfig()
	if loadErr != nil {
		appLogger.Fatal(loadErr.Error())
	}

	gameService := service.NewGameService()
	aiService := service.NewAiService()
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

	// 如果有待安装 URL，解析并暂存到 downloadService
	if pendingURL != "" {
		if pendingInstallReq != nil {
			downloadService.SetPendingInstall(pendingInstallReq)
		}
	}

	// 创建本地文件处理器
	localFileHandler, err := apputils.NewLocalFileHandler()
	if err != nil {
		appLogger.Error("Warning: Failed to create local file handler: " + err.Error())
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

					if strings.HasPrefix(r.URL.Path, "/local/") {
						localFileHandler.ServeHTTP(w, r)
						return
					}

					next.ServeHTTP(w, r)
				})
			},
		},
		BackgroundColour: &options.RGBA{R: 18, G: 20, B: 22, A: 255},
		StartHidden:      true,
		Frameless:        true, // 启用无边框模式
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
			if config.CloseToTray {
				runtime.WindowHide(ctx)
				return true
			}
			return false
		},
		OnStartup: func(ctx context.Context) {
			appState.SetContext(ctx)
			var err error

			// 检查是否有待恢复的全量数据备份（在打开数据库前执行）
			if config.PendingFullRestore != "" {
				restored, restoreErr := service.ExecuteFullDataRestore(config)
				if restoreErr != nil {
					appLogger.Error("fail to restore full data: " + restoreErr.Error())
				} else if restored {
					appLogger.Info("full data restored successfully")
				}
			}

			// 检查是否有待恢复的数据库备份（在打开数据库前执行）
			if config.PendingDBRestore != "" {
				restored, restoreErr := service.ExecuteDBRestore(config)
				if restoreErr != nil {
					appLogger.Error("fail to restore database: " + restoreErr.Error())
				} else if restored {
					appLogger.Info("database restored successfully")
				}
			}

			execPath, err := apputils.GetDataDir()
			if err != nil {
				appLogger.Fatal(err.Error())
			}
			dbPath := filepath.Join(execPath, "lunabox.db")
			db, err = sql.Open("duckdb", dbPath)
			if err != nil {
				appLogger.Fatal(err.Error())
			}

			// 设置时区为本地时区，确保 TIMESTAMPTZ 的聚合操作使用正确的日界线
			// 这对于按日期统计游戏时长非常重要
			// 注意：需要用户在前端设置时区（使用 Intl.DateTimeFormat().resolvedOptions().timeZone）
			timeZone := config.TimeZone
			if timeZone == "" {
				// 未配置时区，使用 UTC 作为默认值
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

			// 运行数据库迁移（安全、只执行一次）
			appLogger.Info("Checking for pending database migrations...")
			if err := migrations.Run(ctx, db); err != nil {
				appLogger.Fatal("Database migration failed: " + err.Error())
			}
			appLogger.Info("Database migrations completed")

			configService.Init(ctx, db, config)
			// 设置安全退出回调
			configService.SetQuitHandler(func() {
				appState.QuitApplication()
			})
			downloadService.Init(ctx, db, config)
			gameService.Init(ctx, db, config)
			tagService.Init(ctx, db, config)
			aiService.Init(ctx, db, config)
			backupService.Init(ctx, db, config)
			cloudSyncService.Init(ctx, db, config)
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

			// 设置 StartService 的 BackupService GameService SessionService依赖
			startService.SetBackupService(backupService)
			startService.SetGameService(gameService)
			startService.SetSessionService(sessionService)
			downloadService.SetGameService(gameService)
			gameService.SetTagService(tagService)
			gameService.SetCloudSyncService(cloudSyncService)
			categoryService.SetCloudSyncService(cloudSyncService)
			sessionService.SetCloudSyncService(cloudSyncService)

			// 设置 ImportService 的 SessionService 依赖（用于导入游玩记录）
			importService.SetSessionService(sessionService)

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

			cloudSyncService.RunStartupSync()
		},
		OnShutdown: func(ctx context.Context) {
			appState.BeginShutdown()

			// 先关闭 IPC Server，避免退出过程中还有外部请求进入。
			if err := ipcserver.ShutdownServer(ipcHTTPServer); err != nil {
				appLogger.Error("failed to shutdown IPC server: " + err.Error())
			}

			// 关闭系统托盘
			appState.RequestTrayQuit()
			if appState.WaitForTrayExit(1200 * time.Millisecond) {
				appLogger.Info("system tray exited successfully")
			} else {
				appLogger.Warning("system tray exit timed out, continuing shutdown")
			}

			// 从 configService 获取最新配置（避免使用启动时的旧配置覆盖文件）
			latestConfig, err := configService.GetAppConfig()
			if err != nil {
				appLogger.Error("failed to get latest config: " + err.Error())
			} else {
				// 更新窗口大小到最新配置
				latestConfig.WindowWidth = config.WindowWidth
				latestConfig.WindowHeight = config.WindowHeight
				config = &latestConfig
			}

			// 清理所有待定的进程选择会话（防止遗留临时会话）
			appLogger.Info("cleaning up pending process selections...")
			startService.CleanupPendingSessions()

			// 自动备份数据库（在关闭数据库前）
			if config.AutoBackupDB {
				appLogger.Info("performing automatic database backup...")
				_, err := backupService.CreateAndUploadDBBackup()
				if err != nil {
					appLogger.Error("automatic database backup failed: " + err.Error())
				} else {
					appLogger.Info("automatic database backup succeeded")
				}
			}

			// 关闭数据库连接
			if err := db.Close(); err != nil {
				appLogger.Error("failed to close database: " + err.Error())
			}

			// 保存最终配置
			if err := appconf.SaveConfig(config); err != nil {
				appLogger.Error("failed to save config: " + err.Error())
			}
		},
		Bind: []interface{}{
			gameService,
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
		},
		EnumBind: []interface{}{
			enums.AllSourceTypes,
			enums.AllPeriodTypes,
			enums.Prompts,
			enums.AllGameStatuses,
		},
	})

	if bootstrapErr != nil {
		appLogger.Fatal(bootstrapErr.Error())
	}
}

// 系统托盘初始化
func onSystrayReady() {
	// 先设置托盘的基本属性
	systray.SetIcon(icon)
	systray.SetTitle("LunaBox")
	systray.SetTooltip("LunaBox")

	// 点击托盘图标时显示窗口
	systray.SetOnClick(func(menu systray.IMenu) {
		appState.ShowMainWindow()
	})

	// 双击托盘图标时也显示窗口
	systray.SetOnDClick(func(menu systray.IMenu) {
		appState.ShowMainWindow()
	})

	mShow := systray.AddMenuItem("显示主窗口", "显示 LunaBox 主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 LunaBox")

	// energye/systray 使用 Click 方法设置回调，而不是 ClickedCh
	mShow.Click(func() {
		appState.ShowMainWindow()
	})

	mQuit.Click(func() {
		appState.QuitApplication()
	})

	// 通知主线程托盘已经准备就绪
	appState.MarkTrayReady()
}

func onSystrayExit() {
	appState.MarkTrayExit()
}
