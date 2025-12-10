package main

import (
	"context"
	"database/sql"
	"embed"
	"log"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/service"

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
	metaDataService := service.NewMetaDataService()
	statsService := service.NewStatsService()
	timerService := service.NewTimerService()
	categoryService := service.NewCategoryService()
	configService := service.NewConfigService()

	// Create application with options
	bootstrapErr := wails.Run(&options.App{
		Title:  "lunabox",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			var err error
			db, err = sql.Open("duckdb", "lunabox.db")
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
			metaDataService.Init(ctx, db, config)
			statsService.Init(ctx, db, config)
			timerService.Init(ctx, db, config)
		},
		OnShutdown: func(ctx context.Context) {
			var err error
			err = appconf.SaveConfig(config)
			if err != nil {
				log.Fatal(err)
			}

			err = db.Close()
			if err != nil {
				log.Fatal(err)
			}
		},
		Bind: []interface{}{
			gameService,
			aiService,
			backupService,
			homeService,
			metaDataService,
			statsService,
			timerService,
			categoryService,
			configService,
		},
		EnumBind: []interface{}{
			enums.AllSourceTypes,
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
			user_id TEXT,
			name TEXT,
			is_system BOOLEAN
		)`,
		`CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			user_id TEXT,
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
			user_id TEXT,
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
