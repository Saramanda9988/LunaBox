// Package hosttest 为测试提供真实服务：每个用例一个临时 DuckDB，已跑完迁移。
// 导入它的测试会链接 DuckDB；只需要游戏列表的测试用 ui/env/envtest 的内存后端。
package hosttest

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/bootstrap"
	"lunabox/backend/models"
	"lunabox/backend/utils/dbutils"
	"lunabox/desktop-mygo/host"
)

var (
	dataDirOnce sync.Once
	dataDir     string
	logger      *applog.FileLogger
	dataDirErr  error
)

// NewServices 返回真实服务，数据库是本用例独占的临时 DuckDB（已跑完迁移）。
// apputils 的数据目录是进程级的，同一个测试进程的用例共用一个临时数据目录。
func NewServices(tb testing.TB) *host.Backend {
	tb.Helper()
	dataDirOnce.Do(func() {
		var dir string
		if dir, dataDirErr = os.MkdirTemp("", "lunabox-mygo-test-"); dataDirErr == nil {
			dataDir, dataDirErr = host.UseDataDir(dir)
		}
		if dataDirErr == nil {
			logger, dataDirErr = bootstrap.NewLogger()
		}
		if dataDirErr == nil {
			applog.SetMode(applog.ModeGUI)
			applog.SetLogger(logger)
		}
	})
	if dataDirErr != nil {
		tb.Fatal(dataDirErr)
	}
	ctx := context.Background()
	config, err := appconf.LoadConfig()
	if err != nil {
		tb.Fatal(err)
	}
	db, err := bootstrap.OpenDatabase(ctx, filepath.Join(tb.TempDir(), "lunabox.db"), config.TimeZone, logger)
	if db != nil {
		tb.Cleanup(func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = dbutils.SafeCloseDuckDB(closeCtx, db, logger)
		})
	}
	if err != nil {
		tb.Fatal(err)
	}
	svc := bootstrap.NewServices(bootstrap.ServiceOptions{DisableMCP: true})
	svc.Init(ctx, db, config)
	return &host.Backend{Services: svc, DB: db, Config: config, DataDir: dataDir, Events: host.NewEvents()}
}

// AddGame 直接写入一条游戏记录（主程序只有从元数据添加的入口），以及它的元数据来源。
func AddGame(tb testing.TB, svc *host.Backend, g models.Game) {
	tb.Helper()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	if g.SourceType == "" {
		g.SourceType = "local"
	}
	if g.Status == "" {
		g.Status = "not_started"
	}
	_, err := svc.DB.Exec(`INSERT INTO games (id, name, cover_url, company, summary, rating, release_date, path, game_directory,
		save_path, status, source_type, source_id, created_at, updated_at, cached_at, is_nsfw, launch_mode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'normal')`,
		g.ID, g.Name, g.CoverURL, g.Company, g.Summary, g.Rating, g.ReleaseDate, g.Path, g.GameDirectory,
		g.SavePath, g.Status, g.SourceType, g.SourceID, g.CreatedAt, g.CreatedAt, g.CreatedAt, g.IsNSFW)
	if err != nil {
		tb.Fatal(err)
	}
	for _, s := range g.MetadataSources {
		if _, err := svc.DB.Exec(`INSERT INTO game_metadata_sources (game_id, source_type, source_id) VALUES (?, ?, ?)`,
			g.ID, s.SourceType, s.SourceID); err != nil {
			tb.Fatal(err)
		}
	}
}

// AddTag 写入一个标签；source 为 "user" 时是用户标签。
func AddTag(tb testing.TB, svc *host.Backend, gameID, id, name, source string, spoiler bool) {
	tb.Helper()
	if _, err := svc.DB.Exec(`INSERT INTO game_tags (id, game_id, name, source, is_spoiler, created_at) VALUES (?, ?, ?, ?, ?, now())`,
		id, gameID, name, source, spoiler); err != nil {
		tb.Fatal(err)
	}
}

// AddSession 写入一条游玩记录。
func AddSession(tb testing.TB, svc *host.Backend, gameID string, start time.Time, seconds int) models.PlaySession {
	tb.Helper()
	s, err := svc.Session.AddPlaySession(gameID, start, seconds)
	if err != nil {
		tb.Fatal(err)
	}
	return s
}
