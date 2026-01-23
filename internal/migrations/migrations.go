package migrations

import (
	"database/sql"
	"fmt"
	"log/slog"
)

// Migration 表示一个数据库迁移
type Migration struct {
	Version     int
	Description string
	Up          func(tx *sql.Tx) error // 改为接收事务
}

// 所有迁移按版本号顺序排列
var migrations = []Migration{
	// {
	// 	Version:     114,
	// 	Description: "Convert UTC timestamps to local time (+8 hours for historical data)",
	// 	Up:          migrateUTCToLocalTime,
	// },
}

// migrateUTCToLocalTime 将历史 UTC 时间转换为本地时间
func migrateUTCToLocalTime(tx *sql.Tx) error {
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
func Run(db *sql.DB) error {
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

		slog.Info("Running migration", "version", migration.Version, "description", migration.Description)

		// 开启事务 - 确保迁移和版本记录原子执行
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
		}

		if err := migration.Up(tx); err != nil {
			tx.Rollback()
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
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}

		slog.Info("Migration completed successfully", "version", migration.Version)
	}

	return nil
}
