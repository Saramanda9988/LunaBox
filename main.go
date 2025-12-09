package main

import (
	"context"
	"database/sql"
	"embed"
	"log"
	"lunabox/internal/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	_ "database/sql"

	_ "github.com/duckdb/duckdb-go/v2"
)

//go:embed all:frontend/dist
var assets embed.FS

var db *sql.DB

func main() {

	gameService := service.NewGameService()
	aiService := service.NewAiService()
	backupService := service.NewBackupService()
	homeService := service.NewHomeService()
	metaDataService := service.NewMetaDataService()
	statsService := service.NewStatsService()
	timerService := service.NewTimerService()

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
			db, err := sql.Open("duckdb", "")
			if err != nil {
				log.Fatal(err)
			}

			gameService.Init(ctx, db)
			aiService.Init(ctx, db)
			backupService.Init(ctx, db)
			homeService.Init(ctx, db)
			metaDataService.Init(ctx, db)
			statsService.Init(ctx, db)
			timerService.Init(ctx, db)
		},
		OnShutdown: func(ctx context.Context) {
			err := db.Close()
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
		},
	})

	if bootstrapErr != nil {
		println("Bootstrap Error:", bootstrapErr.Error())
	}

	println("App started")
}
