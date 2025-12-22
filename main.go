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

	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

var db *sql.DB

var config *appconf.AppConfig

func main() {
	appLogger := logger.NewFileLogger("app.log")

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

	// 创建本地文件处理器
	localFileHandler, err := utils.NewLocalFileHandler()
	if err != nil {
		appLogger.Error("Warning: Failed to create local file handler: " + err.Error())
	}

	// Create application with options
	bootstrapErr := wails.Run(&options.App{
		Title:     "lunabox",
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
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		// 样式完全交由wails前端控制
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.SystemDefault,
		},
		OnStartup: func(ctx context.Context) {
			var err error

			// 检查是否有待恢复的数据库备份（在打开数据库前执行）
			if config.PendingDBRestore != "" {
				restored, restoreErr := service.ExecuteDBRestore(config)
				if restoreErr != nil {
					appLogger.Error("恢复数据库失败: " + restoreErr.Error())
				} else if restored {
					appLogger.Info("数据库恢复成功")
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
			gameService.Init(ctx, db, config)
			aiService.Init(ctx, db, config)
			backupService.Init(ctx, db, config)
			homeService.Init(ctx, db, config)
			statsService.Init(ctx, db, config)
			timerService.Init(ctx, db, config)
			categoryService.Init(ctx, db, config)
			importService.Init(ctx, db, config, gameService)
		},
		OnShutdown: func(ctx context.Context) {
			// 关闭数据库连接
			if err := db.Close(); err != nil {
				appLogger.Error("关闭数据库失败: " + err.Error())
			}

			// 保存配置
			if err := appconf.SaveConfig(config); err != nil {
				appLogger.Error("保存配置失败: " + err.Error())
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
