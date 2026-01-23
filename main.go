package main

import (
	"database/sql"
	"embed"
	"log/slog"
	"lunabox/internal/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"lunabox/internal/appconf"
	"lunabox/internal/migrations"
	"lunabox/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"

	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var icon []byte

var db *sql.DB

var config *appconf.AppConfig

var app *application.App

// 标记是否是从托盘强制退出
var forceQuit bool

func main() {

	logger, logErr := initLogger()
	if logErr != nil {
		// TODO: 处理日志初始化失败
		slog.Error("failed to initialize logger", "error", logErr.Error())
		os.Exit(1)
	}

	var loadErr error
	config, loadErr = appconf.LoadConfig()
	if loadErr != nil {
		logger.Error("failed to load config", "error", loadErr.Error())
		panic(loadErr)
	}

	// 创建本地文件处理器
	localFileHandler, localFileErr := utils.NewLocalFileHandler()
	if localFileErr != nil {
		slog.Warn("Failed to create local file handler", "error", localFileErr.Error())
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

	// 创建服务实例
	gameService := service.NewGameService(db, config, logger)
	aiService := service.NewAiService(db, config, logger)
	backupService := service.NewBackupService(db, config, logger)
	homeService := service.NewHomeService(db, config, logger)
	statsService := service.NewStatsService(db, config, logger)
	timerService := service.NewTimerService(db, config, logger)
	categoryService := service.NewCategoryService(db, config, logger)
	configService := service.NewConfigService(db, config, logger)
	importService := service.NewImportService(db, config, logger)
	templateService := service.NewTemplateService(db, config, logger)
	updateService := service.NewUpdateService(logger)
	versionService := service.NewVersionService()

	configService.SetQuitHandler(func() {
		forceQuit = true
		app.Quit()
	})

	// 设置 TimerService 的 BackupService 依赖
	timerService.SetBackupService(backupService)

	// 设置 ImportService 的 TimerService 依赖
	importService.SetTimerService(timerService)
	importService.SetGameService(gameService)

	// 设置 UpdateService 的 ConfigService 依赖
	updateService.SetConfigService(configService)

	// 创建 Wails v3 应用
	app = application.New(application.Options{
		Name:        "LunaBox",
		Description: "A game library manager",
		LogLevel:    slog.LevelInfo,
		Logger:      logger,
		Services: []application.Service{
			application.NewService(gameService),
			application.NewService(aiService),
			application.NewService(backupService),
			application.NewService(homeService),
			application.NewService(statsService),
			application.NewService(timerService),
			application.NewService(categoryService),
			application.NewService(configService),
			application.NewService(importService),
			application.NewService(versionService),
			application.NewService(templateService),
			application.NewService(updateService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
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
		// FIXME: 用shouldQuit和PostShutDown这两个钩子生命周期对吗
		ShouldQuit: func() bool {
			window := app.Window.Current()
			// 保存当前窗口大小（只在非最大化时）
			if !window.IsMaximised() {
				config.WindowWidth, config.WindowHeight = window.Size()
			}

			// 如果是从托盘强制退出，直接允许关闭
			if forceQuit {
				return true
			}
			if config.CloseToTray {
				window.Hide()
				return false
			}
			return true
		},
		OnShutdown: func() {
			// FIXME: 托盘的生命周期需不需要我控
			// 从 configService 获取最新配置
			latestConfig, err := configService.GetAppConfig()
			if err != nil {
				slog.Error("failed to get latest config", "error", err.Error())
			} else {
				latestConfig.WindowWidth = config.WindowWidth
				latestConfig.WindowHeight = config.WindowHeight
				config = &latestConfig
			}

			// 自动备份数据库
			if config.AutoBackupDB {
				slog.Info("performing automatic database backup...")
				_, err := backupService.CreateAndUploadDBBackup()
				if err != nil {
					slog.Error("automatic database backup failed", "error", err.Error())
				} else {
					slog.Info("automatic database backup succeeded")
				}
			}

			// 关闭数据库连接
			if db != nil {
				if err := db.Close(); err != nil {
					slog.Error("failed to close database", "error", err.Error())
				}
			}

			// 保存最终配置
			if err := appconf.SaveConfig(config); err != nil {
				slog.Error("failed to save config", "error", err.Error())
			}
		},
	})

	// 初始化数据库
	initDatabase()

	// 创建主窗口
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "LunaBox",
		Width:            initWidth,
		Height:           initHeight,
		MinWidth:         970,
		MinHeight:        563,
		BackgroundColour: application.NewRGBA(18, 20, 22, 255),
		Hidden:           true,
	})

	// 初始化系统托盘
	initSystemTray(app, mainWindow, logger)

	// 运行应用
	if err := app.Run(); err != nil {
		slog.Error("application error", "error", err.Error())
		panic(err)
	}
}

// initDatabase 初始化数据库
func initDatabase() {
	// 检查是否有待恢复的数据库备份（在打开数据库前执行）
	if config.PendingDBRestore != "" {
		restored, restoreErr := service.ExecuteDBRestore(config)
		if restoreErr != nil {
			slog.Error("fail to restore database", "error", restoreErr.Error())
		} else if restored {
			slog.Info("database restored successfully")
		}
	}

	execPath, err := utils.GetDataDir()
	if err != nil {
		slog.Error("failed to get data dir", "error", err.Error())
		panic(err)
	}
	dbPath := filepath.Join(execPath, "lunabox.db")
	db, err = sql.Open("duckdb", dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err.Error())
		panic(err)
	}

	if err := initSchema(db); err != nil {
		slog.Error("failed to init schema", "error", err.Error())
		panic(err)
	}

	// 运行数据库迁移
	slog.Info("Checking for pending database migrations...")
	if err := migrations.Run(db); err != nil {
		slog.Error("Database migration failed", "error", err.Error())
		panic(err)
	}
	slog.Info("Database migrations completed")
}

func initLogger() (*slog.Logger, error) {
	logDir, _ := utils.GetSubDir("logs")
	logFile := filepath.Join(logDir, "app.log")
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// 打开日志文件（追加模式）
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	handler := slog.NewTextHandler(file, nil)
	logger := slog.New(handler)
	return logger, nil
}

// initSystemTray 初始化系统托盘
func initSystemTray(app *application.App, mainWindow *application.WebviewWindow, logger *slog.Logger) *application.SystemTray {
	// 创建托盘菜单
	trayMenu := app.Menu.New()
	trayMenu.Add("显示主窗口").OnClick(func(ctx *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("退出").OnClick(func(ctx *application.Context) {
		forceQuit = true
		app.Quit()
	})

	// 创建系统托盘
	systray := app.SystemTray.New()
	systray.SetLabel("LunaBox")
	systray.SetIcon(icon)
	systray.SetMenu(trayMenu)

	// 托盘图标点击事件
	systray.OnClick(func() {
		mainWindow.Show()
		mainWindow.Focus()
	})
	systray.OnDoubleClick(func() {
		mainWindow.Show()
		mainWindow.Focus()
	})

	// 注册托盘以便后续管理
	systray.SetLabel("main-tray")

	logger.Info("system tray initialized successfully")
	return systray
}

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
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}
	return nil
}
