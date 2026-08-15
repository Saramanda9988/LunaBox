package test

import (
	"database/sql"
	"lunabox/internal/applog"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

// setupTestDB 创建测试数据库（供所有 service 测试使用）
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	applog.SetMode(applog.ModeCLI)

	// 使用内存数据库进行测试
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("无法打开测试数据库: %v", err)
	}

	// 创建测试表结构
	initTestSchema(t, db)

	// 返回清理函数
	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// initTestSchema 初始化测试表结构
func initTestSchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS categories (
			id TEXT PRIMARY KEY,
			name TEXT,
			emoji TEXT DEFAULT '',
			created_at TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			is_system BOOLEAN
		)`,
		`CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			name TEXT,
			aliases TEXT DEFAULT '[]',
			cover_url TEXT,
			cover_source_url TEXT DEFAULT '',
			company TEXT,
			summary TEXT,
			rating DOUBLE DEFAULT 0,
			release_date TEXT DEFAULT '',
			path TEXT,
			game_directory TEXT DEFAULT '',
			save_path TEXT,
			process_name TEXT DEFAULT '',
			wine_runner TEXT DEFAULT '',
			wine_args TEXT DEFAULT '',
			wine_prefix TEXT DEFAULT '',
			launch_mode TEXT DEFAULT 'normal',
			steam_launch_id TEXT DEFAULT '',
			steam_launch_kind TEXT DEFAULT '',
			steam_user_id TEXT DEFAULT '',
			steam_launch_options TEXT DEFAULT '',
			status TEXT DEFAULT 'not_started',
			source_type TEXT,
			cached_at TIMESTAMP,
			source_id TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			use_locale_emulator BOOLEAN DEFAULT FALSE,
			use_magpie BOOLEAN DEFAULT FALSE,
			is_nsfw BOOLEAN DEFAULT FALSE,
			metadata_locked BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE IF NOT EXISTS game_metadata_sources (
			game_id TEXT NOT NULL,
			source_type TEXT NOT NULL,
			source_id TEXT NOT NULL,
			cached_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (game_id, source_type)
		)`,
		`CREATE TABLE IF NOT EXISTS game_categories (
			game_id TEXT,
			category_id TEXT,
			updated_at TIMESTAMP,
			PRIMARY KEY (game_id, category_id)
		)`,
		`CREATE TABLE IF NOT EXISTS play_sessions (
			id TEXT PRIMARY KEY,
			game_id TEXT,
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			duration INTEGER,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS game_progress (
			id TEXT PRIMARY KEY,
			game_id TEXT NOT NULL,
			chapter TEXT DEFAULT '',
			route TEXT DEFAULT '',
			progress_note TEXT DEFAULT '',
			spoiler_boundary TEXT DEFAULT 'none',
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS game_reviews (
			game_id TEXT PRIMARY KEY,
			rating INTEGER,
			content TEXT NOT NULL DEFAULT '',
			is_spoiler BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (rating IS NULL OR (rating >= 1 AND rating <= 10))
		)`,
		`CREATE TABLE IF NOT EXISTS game_tags (
			id TEXT PRIMARY KEY,
			game_id TEXT NOT NULL,
			name TEXT NOT NULL,
			source TEXT NOT NULL,
			weight DOUBLE DEFAULT 1.0,
			is_spoiler BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (game_id, name, source)
		)`,
		`CREATE TABLE IF NOT EXISTS sync_tombstones (
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			parent_id TEXT DEFAULT '',
			secondary_id TEXT DEFAULT '',
			deleted_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (entity_type, entity_id, parent_id, secondary_id)
		)`,
		`CREATE TABLE IF NOT EXISTS cloud_sync_state (
			bucket_key TEXT PRIMARY KEY,
			local_hash TEXT NOT NULL,
			remote_hash TEXT NOT NULL,
			remote_revision_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			t.Fatalf("创建测试表失败: %v", err)
		}
	}
}
