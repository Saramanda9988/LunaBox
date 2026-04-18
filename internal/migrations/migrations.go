package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/applog"
	"time"
)

// Migration 表示一个数据库迁移
type Migration struct {
	Version     int
	Description string
	Up          func(tx *sql.Tx) error // 改为接收事务
}

// migration131 添加 Locale Emulator 和 Magpie 支持列
func migration131(tx *sql.Tx) error {
	// DuckDB 支持 IF NOT EXISTS，列已存在时会静默成功
	// 添加 use_locale_emulator 列
	_, err := tx.Exec(`
		ALTER TABLE games 
		ADD COLUMN IF NOT EXISTS use_locale_emulator BOOLEAN DEFAULT FALSE
	`)
	if err != nil {
		return fmt.Errorf("failed to add use_locale_emulator column: %w", err)
	}

	// 添加 use_magpie 列
	_, err = tx.Exec(`
		ALTER TABLE games 
		ADD COLUMN IF NOT EXISTS use_magpie BOOLEAN DEFAULT FALSE
	`)
	if err != nil {
		return fmt.Errorf("failed to add use_magpie column: %w", err)
	}

	return nil
}

// migration134 将所有表的时间戳字段从 TIMESTAMP 改为 TIMESTAMPTZ
//
// 关键理解：TIMESTAMP 和 TIMESTAMPTZ 存储格式完全相同（都是 INT64 微秒数）
// 区别只在查询时的行为：
// - TIMESTAMP: 按 UTC 处理，start_time::DATE 会得到 UTC 日期（可能与用户本地日期不符）
// - TIMESTAMPTZ: 按配置的时区处理，start_time::DATE 会得到本地日期（正确）
//
// 迁移策略：重建表（CREATE AS SELECT -> DROP -> RENAME）
func migration134(tx *sql.Tx) error {
	// 迁移 play_sessions 表
	if err := migrateTableTimestamps(tx, "play_sessions", []string{"start_time"}, `
		id TEXT PRIMARY KEY,
		game_id TEXT,
		start_time TIMESTAMPTZ,
		end_time TIMESTAMPTZ,
		duration INTEGER
	`, "id, game_id, start_time, end_time, duration"); err != nil {
		return fmt.Errorf("failed to migrate play_sessions table: %w", err)
	}

	// 迁移 users 表
	if err := migrateTableTimestamps(tx, "users", []string{"created_at"},
		"id TEXT PRIMARY KEY, created_at TIMESTAMPTZ, default_backup_target TEXT",
		"id, created_at, default_backup_target"); err != nil {
		return fmt.Errorf("failed to migrate users table: %w", err)
	}

	// 迁移 categories 表
	if err := migrateTableTimestamps(tx, "categories", []string{"created_at", "updated_at"},
		"id TEXT PRIMARY KEY, name TEXT, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, is_system BOOLEAN",
		"id, name, created_at, updated_at, is_system"); err != nil {
		return fmt.Errorf("failed to migrate categories table: %w", err)
	}

	// 迁移 games 表 - 显式指定列名，排除可能存在的 process_name 列
	if err := migrateTableTimestamps(tx, "games", []string{"cached_at", "created_at"}, `
		id TEXT PRIMARY KEY,
		name TEXT,
		cover_url TEXT,
		company TEXT,
		summary TEXT,
		path TEXT,
		save_path TEXT,
		status TEXT DEFAULT 'not_started',
		source_type TEXT,
		cached_at TIMESTAMPTZ,
		source_id TEXT,
		created_at TIMESTAMPTZ,
		use_locale_emulator BOOLEAN DEFAULT FALSE,
		use_magpie BOOLEAN DEFAULT FALSE
	`, "id, name, cover_url, company, summary, path, save_path, status, source_type, cached_at, source_id, created_at, use_locale_emulator, use_magpie"); err != nil {
		return fmt.Errorf("failed to migrate games table: %w", err)
	}

	return nil
}

// migrateTableTimestamps 辅助函数：迁移表的时间戳字段
func migrateTableTimestamps(tx *sql.Tx, tableName string, timestampColumns []string, newSchema string, columnList string) error {
	// 检查是否需要迁移（检查第一个时间戳列是否已经是 TIMESTAMPTZ）
	if len(timestampColumns) > 0 {
		var columnType string
		err := tx.QueryRow(`
			SELECT data_type 
			FROM information_schema.columns 
			WHERE table_name = ? AND column_name = ?
		`, tableName, timestampColumns[0]).Scan(&columnType)
		if err != nil {
			return fmt.Errorf("failed to check column type: %w", err)
		}

		// 如果已经是 TIMESTAMP WITH TIME ZONE，跳过迁移
		if columnType == "TIMESTAMP WITH TIME ZONE" {
			return nil
		}
	}

	newTableName := tableName + "_new"

	// 步骤 1: 创建新表
	_, err := tx.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", newTableName, newSchema))
	if err != nil {
		return fmt.Errorf("failed to create new table: %w", err)
	}

	// 步骤 2: 复制数据 - 使用显式列名避免列数不匹配
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", newTableName, columnList, columnList, tableName)
	_, err = tx.Exec(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// 步骤 3: 删除旧表
	_, err = tx.Exec(fmt.Sprintf("DROP TABLE %s", tableName))
	if err != nil {
		return fmt.Errorf("failed to drop old table: %w", err)
	}

	// 步骤 4: 重命名新表
	_, err = tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", newTableName, tableName))
	if err != nil {
		return fmt.Errorf("failed to rename new table: %w", err)
	}

	return nil
}

// migration140 添加 process_name 列，用于记录实际监控的进程名
// 某些汉化补丁需要启动启动器，但实际运行的游戏进程与启动器不同
func migration140(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE games 
		ADD COLUMN IF NOT EXISTS process_name TEXT DEFAULT ''
	`)
	if err != nil {
		return fmt.Errorf("failed to add process_name column: %w", err)
	}
	return nil
}

// migration150 添加 categories.emoji 列，用于自定义分类图标
func migration150(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE categories
		ADD COLUMN IF NOT EXISTS emoji TEXT DEFAULT ''
	`)
	if err != nil {
		return fmt.Errorf("failed to add emoji column to categories: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE categories
		SET emoji = ''
		WHERE emoji IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to normalize categories emoji values: %w", err)
	}

	return nil
}

// migration151 新增 game_progress 表，记录玩家手动游玩点，供防剧透 AI 总结使用
func migration151(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS game_progress (
			id          TEXT PRIMARY KEY,
			game_id     TEXT NOT NULL,
			chapter     TEXT DEFAULT '',
			route       TEXT DEFAULT '',
			progress_note TEXT DEFAULT '',
			spoiler_boundary TEXT DEFAULT 'none',
			updated_at  TIMESTAMPTZ
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create game_progress table: %w", err)
	}
	return nil
}

// migration154 新增 game_tags 表，存储来自 Bangumi/VNDB/用户的 tag 元数据
func migration154(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS game_tags (
			id          TEXT PRIMARY KEY,
			game_id     TEXT NOT NULL,
			name        TEXT NOT NULL,
			source      TEXT NOT NULL,
			weight      DOUBLE DEFAULT 1.0,
			is_spoiler  BOOLEAN DEFAULT FALSE,
			created_at  TIMESTAMPTZ,
			UNIQUE (game_id, name, source)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create game_tags table: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_game_tags_game_id ON game_tags(game_id)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_game_tags_game_id: %w", err)
	}

	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_game_tags_name ON game_tags(name)`)
	if err != nil {
		return fmt.Errorf("failed to create idx_game_tags_name: %w", err)
	}

	return nil
}

// migration155 添加 games.rating 和 games.release_date 列，用于存储刮削得到的评分与发售日期
func migration155(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS rating DOUBLE DEFAULT 0
	`)
	if err != nil {
		return fmt.Errorf("failed to add rating column to games: %w", err)
	}

	_, err = tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS release_date TEXT DEFAULT ''
	`)
	if err != nil {
		return fmt.Errorf("failed to add release_date column to games: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE games
		SET rating = 0
		WHERE rating IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to normalize games rating values: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE games
		SET release_date = ''
		WHERE release_date IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to normalize games release_date values: %w", err)
	}

	return nil
}

// migration156 将 game_progress 升级为历史链模型
func migration156(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE INDEX IF NOT EXISTS idx_game_progress_game_timeline
		ON game_progress(game_id, updated_at)
	`)
	if err != nil {
		return fmt.Errorf("failed to create idx_game_progress_game_timeline: %w", err)
	}

	return nil
}

// migration157 添加云同步所需的时间戳/墓碑结构，并归一系统分类 ID
func migration157(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	`); err != nil {
		return fmt.Errorf("failed to add updated_at column to games: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE games
		SET updated_at = COALESCE(updated_at, created_at, cached_at, CURRENT_TIMESTAMP)
		WHERE updated_at IS NULL
	`); err != nil {
		return fmt.Errorf("failed to normalize games updated_at values: %w", err)
	}

	if _, err := tx.Exec(`
		ALTER TABLE play_sessions
		ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	`); err != nil {
		return fmt.Errorf("failed to add updated_at column to play_sessions: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE play_sessions
		SET updated_at = COALESCE(updated_at, end_time, start_time, CURRENT_TIMESTAMP)
		WHERE updated_at IS NULL
	`); err != nil {
		return fmt.Errorf("failed to normalize play_sessions updated_at values: %w", err)
	}

	if _, err := tx.Exec(`
		ALTER TABLE game_categories
		ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	`); err != nil {
		return fmt.Errorf("failed to add updated_at column to game_categories: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE game_categories
		SET updated_at = COALESCE(updated_at, CURRENT_TIMESTAMP)
		WHERE updated_at IS NULL
	`); err != nil {
		return fmt.Errorf("failed to normalize game_categories updated_at values: %w", err)
	}

	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS sync_tombstones (
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			parent_id TEXT DEFAULT '',
			secondary_id TEXT DEFAULT '',
			deleted_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (entity_type, entity_id, parent_id, secondary_id)
		)
	`); err != nil {
		return fmt.Errorf("failed to create sync_tombstones table: %w", err)
	}

	if _, err := tx.Exec(`
		ALTER TABLE sync_tombstones
		ADD COLUMN IF NOT EXISTS parent_id TEXT DEFAULT ''
	`); err != nil {
		return fmt.Errorf("failed to add parent_id column to sync_tombstones: %w", err)
	}

	if _, err := tx.Exec(`
		ALTER TABLE sync_tombstones
		ADD COLUMN IF NOT EXISTS secondary_id TEXT DEFAULT ''
	`); err != nil {
		return fmt.Errorf("failed to add secondary_id column to sync_tombstones: %w", err)
	}

	const stableFavoritesID = "system:favorites"
	const favoritesName = "最喜欢的游戏"

	var stableCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM categories WHERE id = ?`, stableFavoritesID).Scan(&stableCount); err != nil {
		return fmt.Errorf("failed to check stable favorites category: %w", err)
	}

	var legacyID string
	err := tx.QueryRow(`
		SELECT id
		FROM categories
		WHERE is_system = TRUE AND name = ? AND id <> ?
		ORDER BY created_at ASC, id ASC
		LIMIT 1
	`, favoritesName, stableFavoritesID).Scan(&legacyID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to query legacy favorites category: %w", err)
	}

	switch {
	case stableCount == 0 && legacyID != "":
		if _, err := tx.Exec(`UPDATE categories SET id = ? WHERE id = ?`, stableFavoritesID, legacyID); err != nil {
			return fmt.Errorf("failed to normalize favorites category id: %w", err)
		}
	case stableCount > 0 && legacyID != "":
		if _, err := tx.Exec(`
			INSERT INTO game_categories (game_id, category_id, updated_at)
			SELECT game_id, ?, COALESCE(updated_at, CURRENT_TIMESTAMP)
			FROM game_categories
			WHERE category_id = ?
			ON CONFLICT (game_id, category_id) DO UPDATE SET updated_at = EXCLUDED.updated_at
		`, stableFavoritesID, legacyID); err != nil {
			return fmt.Errorf("failed to merge legacy favorites relations: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM game_categories WHERE category_id = ?`, legacyID); err != nil {
			return fmt.Errorf("failed to delete legacy favorites relations: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM categories WHERE id = ?`, legacyID); err != nil {
			return fmt.Errorf("failed to delete legacy favorites category: %w", err)
		}
	case stableCount == 0 && legacyID == "":
		now := time.Now()
		if _, err := tx.Exec(`
			INSERT INTO categories (id, name, emoji, is_system, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, stableFavoritesID, favoritesName, "❤️", true, now, now); err != nil {
			return fmt.Errorf("failed to seed stable favorites category: %w", err)
		}
	}

	return nil
}

// migration158 为 game_tags 添加 updated_at，供云同步进行冲突解决
func migration158(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		ALTER TABLE game_tags
		ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	`); err != nil {
		return fmt.Errorf("failed to add updated_at column to game_tags: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE game_tags
		SET updated_at = COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		WHERE updated_at IS NULL
	`); err != nil {
		return fmt.Errorf("failed to normalize game_tags updated_at values: %w", err)
	}

	return nil
}

// 所有迁移按版本号顺序排列
var migrations = []Migration{
	{
		Version:     131,
		Description: "Add use_locale_emulator and use_magpie columns to games table",
		Up:          migration131,
	},
	{
		Version:     134,
		Description: "Migrate all tables (play_sessions, users, categories, games) timestamps from TIMESTAMP to TIMESTAMPTZ for correct timezone handling",
		Up:          migration134,
	},
	{
		Version:     140,
		Description: "Add process_name column to games table for tracking actual game process",
		Up:          migration140,
	},
	{
		Version:     150,
		Description: "Add emoji column to categories table for custom category icons",
		Up:          migration150,
	},
	{
		Version:     151,
		Description: "Add game_progress table for spoiler-aware AI summary",
		Up:          migration151,
	},
	{
		Version:     154,
		Description: "Add game_tags table for Bangumi/VNDB/user tag metadata",
		Up:          migration154,
	},
	{
		Version:     155,
		Description: "Add rating and release_date columns to games table for scraped metadata",
		Up:          migration155,
	},
	{
		Version:     156,
		Description: "Add game_progress timeline index for append-only history reads",
		Up:          migration156,
	},
	{
		Version:     157,
		Description: "Add cloud sync metadata columns and tombstones, normalize system favorites category identity",
		Up:          migration157,
	},
	{
		Version:     158,
		Description: "Add updated_at to game_tags for cloud sync conflict resolution",
		Up:          migration158,
	},
	// {
	// 	Version:     114,
	// 	Description: "Convert UTC timestamps to local time (+8 hours for historical data)",
	// 	Up:          migration114,
	// },
}

// migration114 将历史 UTC 时间转换为本地时间
func migration114(tx *sql.Tx) error {
	var count int
	err := tx.QueryRow("SELECT COUNT(*) FROM play_sessions WHERE start_time < '2026-01-19 00:00:00'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count records: %w", err)
	}

	if count == 0 {
		return nil
	}

	_, err = tx.Exec(`
		UPDATE play_sessions 
		SET start_time = start_time + INTERVAL 8 HOUR,
		    end_time = end_time + INTERVAL 8 HOUR
		WHERE start_time < '2026-01-19 00:00:00'
	`)
	if err != nil {
		return fmt.Errorf("failed to migrate timestamps: %w", err)
	}

	return nil
}

// Run 执行所有未运行的迁移
func Run(ctx context.Context, db *sql.DB) error {
	// 创建迁移版本表
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// 获取已应用的迁移版本
	appliedVersions := make(map[int]bool)
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		appliedVersions[version] = true
	}

	// 执行未应用的迁移
	for _, migration := range migrations {
		if appliedVersions[migration.Version] {
			continue
		}

		applog.LogInfof(ctx, "Running migration %d: %s", migration.Version, migration.Description)

		// 开启事务 - 确保迁移和版本记录原子执行
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		if err := migration.Up(tx); err != nil {
			tx.Rollback()
			applog.LogErrorf(ctx, "Migration %d failed: %v", migration.Version, err)
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
			migration.Version,
			migration.Description,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		// 提交事务 - 迁移和版本记录一起提交，保证原子性
		if err := tx.Commit(); err != nil {
			applog.LogErrorf(ctx, "Failed to commit migration %d: %v", migration.Version, err)
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		applog.LogInfof(ctx, "Migration %d completed successfully", migration.Version)
	}

	return nil
}
