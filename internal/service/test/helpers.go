package test

import (
	"database/sql"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

// setupTestDB 创建测试数据库（供所有 service 测试使用）
func setupTestDB(t *testing.T) (*sql.DB, func()) {
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
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			duration INTEGER
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			t.Fatalf("创建测试表失败: %v", err)
		}
	}
}
