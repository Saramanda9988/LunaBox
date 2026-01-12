package main

import (
	"context"
	"database/sql"
	"embed"
	"lunabox/internal/utils"
	"net/http"
	"path/filepath"
	"strings"

	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/service"

	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
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

var appCtx context.Context

// 用于通知 systray 退出
var systrayQuit chan struct{}

// 用于同步托盘就绪状态
var systrayReady chan struct{}

// 标记是否是从托盘强制退出（绕过 OnBeforeClose 的最小化逻辑）
var forceQuit bool

func main() {
	logDir, _ := utils.GetSubDir("logs")
	appLogger := logger.NewFileLogger(filepath.Join(logDir, "app.log"))

	var loadErr error
	config, loadErr = appconf.LoadConfig()
	if loadErr != nil {
		appLogger.Fatal(loadErr.Error())
	}

	gameService := service.NewGameService()
	aiService := service.NewAiService()
	backupService := service.NewBackupService()
	homeService := service.NewHomeService()
	statsService := service.NewStatsService()
	timerService := service.NewTimerService()
	categoryService := service.NewCategoryService()
	configService := service.NewConfigService()
	importService := service.NewImportService()
	versionService := service.NewVersionService()
	templateService := service.NewTemplateService()

	// 创建本地文件处理器
	localFileHandler, err := utils.NewLocalFileHandler()
	if err != nil {
		appLogger.Error("Warning: Failed to create local file handler: " + err.Error())
	}

	// Create application with options
	bootstrapErr := wails.Run(&options.App{
		Title:     "LunaBox",
		Logger:    appLogger,
		LogLevel:  logger.INFO,
		Width:     1230,
		Height:    800,
		MinWidth:  1230,
		MinHeight: 800,
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
		// 样式完全交由wails前端控制
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.SystemDefault,
		},
		// 关闭窗口时的处理
		OnBeforeClose: func(ctx context.Context) bool {
			// 如果是从托盘强制退出，直接允许关闭
			if forceQuit {
				return false
			}
			if config.CloseToTray {
				runtime.WindowHide(ctx)
				return true
			}
			return false
		},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
			var err error

			// 检查是否有待恢复的数据库备份（在打开数据库前执行）
			if config.PendingDBRestore != "" {
				restored, restoreErr := service.ExecuteDBRestore(config)
				if restoreErr != nil {
					appLogger.Error("fail to restore database: " + restoreErr.Error())
				} else if restored {
					appLogger.Info("database restored successfully")
				}
			}

			execPath, err := utils.GetDataDir()
			if err != nil {
				appLogger.Fatal(err.Error())
			}
			dbPath := filepath.Join(execPath, "lunabox.db")
			db, err = sql.Open("duckdb", dbPath)
			if err != nil {
				appLogger.Fatal(err.Error())
			}

			if err := initSchema(db); err != nil {
				appLogger.Fatal(err.Error())
			}

			configService.Init(ctx, db, config)
			// 设置安全退出回调
			configService.SetQuitHandler(func() {
				forceQuit = true
				runtime.Quit(ctx)
			})
			gameService.Init(ctx, db, config)
			aiService.Init(ctx, db, config)
			backupService.Init(ctx, db, config)
			homeService.Init(ctx, db, config)
			statsService.Init(ctx, db, config)
			timerService.Init(ctx, db, config)
			categoryService.Init(ctx, db, config)
			importService.Init(ctx, db, config, gameService)
			versionService.Init(ctx)
			templateService.Init(ctx, db, config)
			// 设置 TimerService 的 BackupService 依赖
			timerService.SetBackupService(backupService)
			// 设置 ImportService 的 TimerService 依赖（用于导入游玩记录）
			importService.SetTimerService(timerService)

			// 在 Wails 启动后初始化系统托盘
			// TODO: 升级wails v3，使用原生的托盘功能
			systrayQuit = make(chan struct{})
			systrayReady = make(chan struct{})
			go systray.Run(onSystrayReady, onSystrayExit)

			// 等待托盘初始化完成，避免竞态条件
			<-systrayReady
			appLogger.Info("system tray initialized successfully")
		},
		OnShutdown: func(ctx context.Context) {
			// 关闭系统托盘
			if systrayQuit != nil {
				systray.Quit()
				<-systrayQuit // 等待 systray 完全退出
			}

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

			// 保存配置
			if err := appconf.SaveConfig(config); err != nil {
				appLogger.Error("failed to save config: " + err.Error())
			}
		},
		Bind: []interface{}{
			gameService,
			aiService,
			backupService,
			homeService,
			statsService,
			timerService,
			categoryService,
			configService,
			importService,
			versionService,
			templateService,
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

// TODO: 实现专门的migration机制
func initSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			created_at TIMESTAMP,
			default_backup_target TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id TEXT PRIMARY KEY,
			name TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			is_system BOOLEAN
		)`,
		`CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			name TEXT,
			cover_url TEXT,
			company TEXT,
			summary TEXT,
			path TEXT,
			save_path TEXT,
			status TEXT DEFAULT 'not_started',
			source_type TEXT,
			cached_at TIMESTAMP,
			source_id TEXT,
			created_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS game_categories (
			game_id TEXT,
			category_id TEXT,
			PRIMARY KEY (game_id, category_id)
		)`,
		`CREATE TABLE IF NOT EXISTS play_sessions (
			id TEXT PRIMARY KEY,
			game_id TEXT,
			start_time TIMESTAMP,
			end_time TIMESTAMP,
			duration INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS game_backups (
			id TEXT PRIMARY KEY,
			game_id TEXT,
			backup_path TEXT,
			size INTEGER,
			created_at TIMESTAMP
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}
	return nil
}

// 系统托盘初始化
func onSystrayReady() {
	// 先设置托盘的基本属性
	systray.SetIcon(icon)
	systray.SetTitle("LunaBox")
	systray.SetTooltip("LunaBox")

	// 点击托盘图标时显示窗口
	systray.SetOnClick(func(menu systray.IMenu) {
		// 确保 appCtx 已经初始化且有效
		if appCtx != nil {
			runtime.WindowShow(appCtx)
		}
	})

	// 双击托盘图标时也显示窗口
	systray.SetOnDClick(func(menu systray.IMenu) {
		// 确保 appCtx 已经初始化且有效
		if appCtx != nil {
			runtime.WindowShow(appCtx)
		}
	})

	mShow := systray.AddMenuItem("显示主窗口", "显示 LunaBox 主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 LunaBox")

	// energye/systray 使用 Click 方法设置回调，而不是 ClickedCh
	mShow.Click(func() {
		// 确保 appCtx 已经初始化且有效
		if appCtx != nil {
			runtime.WindowShow(appCtx)
		}
	})

	mQuit.Click(func() {
		// 通过托盘退出时，设置强制退出标志，绕过 OnBeforeClose 的最小化逻辑
		forceQuit = true
		if appCtx != nil {
			runtime.Quit(appCtx)
		}
	})

	// 通知主线程托盘已经准备就绪
	if systrayReady != nil {
		close(systrayReady)
	}
}

func onSystrayExit() {
	if systrayQuit != nil {
		close(systrayQuit)
	}
}
