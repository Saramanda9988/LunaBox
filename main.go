package main

import (
	"context"
	"database/sql"
	"embed"
	"lunabox/internal/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"log"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/service"

	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

var db *sql.DB

var config *appconf.AppConfig

func main() {
	var loadErr error
	config, loadErr = appconf.LoadConfig()
	if loadErr != nil {
		log.Fatal(loadErr)
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
		log.Printf("Warning: Failed to create local file handler: %v", err)
	}

	// Create application with options
	bootstrapErr := wails.Run(&options.App{
		Title:  "lunabox",
		Width:  1230,
		Height: 768,
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
					log.Printf("恢复数据库失败: %v", restoreErr)
				} else if restored {
					log.Println("数据库恢复成功")
				}
			}

			execPath, err := os.Executable()
			if err != nil {
				log.Fatal(err)
			}
			dbPath := filepath.Join(filepath.Dir(execPath), "lunabox.db")
			db, err = sql.Open("duckdb", dbPath)
			if err != nil {
				log.Fatal(err)
			}

			if err := initSchema(db); err != nil {
				log.Fatal(err)
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
				log.Printf("关闭数据库失败: %v", err)
			}

			// 保存配置
			if err := appconf.SaveConfig(config); err != nil {
				log.Printf("保存配置失败: %v", err)
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
		},
	})

	if bootstrapErr != nil {
		println("Bootstrap Error:", bootstrapErr.Error())
		log.Fatal(bootstrapErr)
	}

	log.Println("Bootstrap completed")
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
		// 添加 save_path 列（如果不存在）
		`ALTER TABLE games ADD COLUMN IF NOT EXISTS save_path TEXT`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}
	return nil
}
