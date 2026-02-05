package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

		runtime.LogInfof(ctx, "Running migration %d: %s", migration.Version, migration.Description)

		// 开启事务 - 确保迁移和版本记录原子执行
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		if err := migration.Up(tx); err != nil {
			tx.Rollback()
			runtime.LogErrorf(ctx, "Migration %d failed: %v", migration.Version, err)
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
			runtime.LogErrorf(ctx, "Failed to commit migration %d: %v", migration.Version, err)
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		runtime.LogInfof(ctx, "Migration %d completed successfully", migration.Version)
	}

	return nil
}
