package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/applog"
	"os"
	"path/filepath"
	"strings"
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

// migration159 adds indexes used by paginated library and category list queries.
func migration159(tx *sql.Tx) error {
	indexes := []struct {
		name  string
		query string
	}{
		{"idx_games_status", `CREATE INDEX IF NOT EXISTS idx_games_status ON games(status)`},
		{"idx_games_created_at", `CREATE INDEX IF NOT EXISTS idx_games_created_at ON games(created_at)`},
		{"idx_games_rating", `CREATE INDEX IF NOT EXISTS idx_games_rating ON games(rating)`},
		{"idx_games_release_date", `CREATE INDEX IF NOT EXISTS idx_games_release_date ON games(release_date)`},
		{"idx_play_sessions_game_start", `CREATE INDEX IF NOT EXISTS idx_play_sessions_game_start ON play_sessions(game_id, start_time)`},
		{"idx_game_tags_name_game", `CREATE INDEX IF NOT EXISTS idx_game_tags_name_game ON game_tags(name, game_id)`},
	}

	for _, index := range indexes {
		if _, err := tx.Exec(index.query); err != nil {
			return fmt.Errorf("failed to create %s: %w", index.name, err)
		}
	}
	return nil
}

// migration160 adds indexes used by import duplicate checks.
func migration160(tx *sql.Tx) error {
	indexes := []struct {
		name  string
		query string
	}{
		{"idx_games_path", `CREATE INDEX IF NOT EXISTS idx_games_path ON games(path)`},
		{"idx_games_source_identity", `CREATE INDEX IF NOT EXISTS idx_games_source_identity ON games(source_type, source_id)`},
		{"idx_games_name_path", `CREATE INDEX IF NOT EXISTS idx_games_name_path ON games(name, path)`},
	}

	for _, index := range indexes {
		if _, err := tx.Exec(index.query); err != nil {
			return fmt.Errorf("failed to create %s: %w", index.name, err)
		}
	}
	return nil
}

// migration161 adds a per-game metadata lock so bulk remote refresh can skip user-edited games.
func migration161(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS metadata_locked BOOLEAN DEFAULT FALSE
	`)
	if err != nil {
		return fmt.Errorf("failed to add metadata_locked column to games: %w", err)
	}
	return nil
}

// migration162 adds macOS Wine launch configuration columns to games.
func migration162(tx *sql.Tx) error {
	columns := []struct {
		name string
		sql  string
	}{
		{"wine_runner", `ALTER TABLE games ADD COLUMN IF NOT EXISTS wine_runner TEXT DEFAULT ''`},
		{"wine_args", `ALTER TABLE games ADD COLUMN IF NOT EXISTS wine_args TEXT DEFAULT ''`},
		{"wine_prefix", `ALTER TABLE games ADD COLUMN IF NOT EXISTS wine_prefix TEXT DEFAULT ''`},
	}

	for _, column := range columns {
		if _, err := tx.Exec(column.sql); err != nil {
			return fmt.Errorf("failed to add %s column to games: %w", column.name, err)
		}
	}
	return nil
}

// migration163 adds per-game default launch mode.
func migration163(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS launch_mode TEXT DEFAULT 'normal'
	`); err != nil {
		return fmt.Errorf("failed to add launch_mode column to games: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE games
		SET launch_mode = 'normal'
		WHERE launch_mode IS NULL OR TRIM(launch_mode) = ''
	`); err != nil {
		return fmt.Errorf("failed to normalize games launch_mode values: %w", err)
	}

	return nil
}

// migration164 adds the user-editable NSFW flag. Existing games remain SFW.
func migration164(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS is_nsfw BOOLEAN DEFAULT FALSE
	`); err != nil {
		return fmt.Errorf("failed to add is_nsfw column to games: %w", err)
	}

	return nil
}

// migration165 preserves remote cover sources and separates the game root
// directory from the launch path.
func migration165(tx *sql.Tx) error {
	columns := []struct {
		name string
		sql  string
	}{
		{"cover_source_url", `ALTER TABLE games ADD COLUMN IF NOT EXISTS cover_source_url TEXT DEFAULT ''`},
		{"game_directory", `ALTER TABLE games ADD COLUMN IF NOT EXISTS game_directory TEXT DEFAULT ''`},
	}
	for _, column := range columns {
		if _, err := tx.Exec(column.sql); err != nil {
			return fmt.Errorf("failed to add %s column to games: %w", column.name, err)
		}
	}

	if _, err := tx.Exec(`
		UPDATE games
		SET cover_source_url = cover_url
		WHERE (cover_source_url IS NULL OR TRIM(cover_source_url) = '')
		  AND (
			LOWER(TRIM(COALESCE(cover_url, ''))) LIKE 'http://%'
			OR LOWER(TRIM(COALESCE(cover_url, ''))) LIKE 'https://%'
		  )
	`); err != nil {
		return fmt.Errorf("failed to backfill games cover_source_url: %w", err)
	}

	rows, err := tx.Query(`
		SELECT id, COALESCE(path, '')
		FROM games
		WHERE game_directory IS NULL OR TRIM(game_directory) = ''
	`)
	if err != nil {
		return fmt.Errorf("failed to query games for game_directory backfill: %w", err)
	}

	type directoryBackfill struct {
		id        string
		directory string
	}
	var backfills []directoryBackfill
	for rows.Next() {
		var id string
		var launchPath string
		if err := rows.Scan(&id, &launchPath); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan game directory backfill row: %w", err)
		}
		if directory := migrationGameDirectory(launchPath); directory != "" {
			backfills = append(backfills, directoryBackfill{id: id, directory: directory})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to iterate game directory backfill rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("failed to close game directory backfill rows: %w", err)
	}

	for _, backfill := range backfills {
		if _, err := tx.Exec(`
			UPDATE games
			SET game_directory = ?
			WHERE id = ? AND (game_directory IS NULL OR TRIM(game_directory) = '')
		`, backfill.directory, backfill.id); err != nil {
			return fmt.Errorf("failed to backfill game_directory for game %s: %w", backfill.id, err)
		}
	}

	return nil
}

func migrationGameDirectory(launchPath string) string {
	launchPath = strings.TrimSpace(launchPath)
	if launchPath == "" {
		return ""
	}

	cleanPath := filepath.Clean(launchPath)
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return cleanPath
	}
	directory := filepath.Dir(cleanPath)
	if directory == "." {
		return ""
	}
	return directory
}

// migration166 stores the device-local Steam launch identity separately from
// remote metadata identity.
func migration166(tx *sql.Tx) error {
	columns := []struct {
		name string
		sql  string
	}{
		{"steam_launch_id", `ALTER TABLE games ADD COLUMN IF NOT EXISTS steam_launch_id TEXT DEFAULT ''`},
		{"steam_launch_kind", `ALTER TABLE games ADD COLUMN IF NOT EXISTS steam_launch_kind TEXT DEFAULT ''`},
		{"steam_user_id", `ALTER TABLE games ADD COLUMN IF NOT EXISTS steam_user_id TEXT DEFAULT ''`},
	}
	for _, column := range columns {
		if _, err := tx.Exec(column.sql); err != nil {
			return fmt.Errorf("failed to add %s column to games: %w", column.name, err)
		}
	}
	return nil
}

func migrateCompatibilityLaunchMode(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		UPDATE games
		SET launch_mode = 'compatibility'
		WHERE COALESCE(TRIM(wine_runner), '') <> ''
		  AND COALESCE(NULLIF(TRIM(launch_mode), ''), 'normal') = 'normal'
	`); err != nil {
		return fmt.Errorf("failed to migrate Wine games to compatibility launch mode: %w", err)
	}

	if _, err := tx.Exec(`
		UPDATE games
		SET wine_runner = 'system'
		WHERE LOWER(TRIM(COALESCE(wine_runner, ''))) = 'custom'
	`); err != nil {
		return fmt.Errorf("failed to normalize custom Wine runners: %w", err)
	}
	return nil
}

// migration167 makes Wine/CrossOver an explicit launch mode and folds the
// former custom runner into the equivalent Wine runner option.
func migration167(tx *sql.Tx) error {
	return migrateCompatibilityLaunchMode(tx)
}

// migration168 stores Steam LaunchOptions for device-local shortcut launches.
// It also re-runs the compatibility migration so databases that already used
// the old Linux branch migration167 still receive the main-branch backfill.
func migration168(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS steam_launch_options TEXT DEFAULT ''
	`); err != nil {
		return fmt.Errorf("failed to add steam_launch_options column to games: %w", err)
	}
	return migrateCompatibilityLaunchMode(tx)
}

// migration169 stores user-defined game aliases as a JSON string array.
func migration169(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		ALTER TABLE games
		ADD COLUMN IF NOT EXISTS aliases TEXT DEFAULT '[]'
	`); err != nil {
		return fmt.Errorf("failed to add aliases column to games: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE games
		SET aliases = '[]'
		WHERE aliases IS NULL OR TRIM(aliases) = ''
	`); err != nil {
		return fmt.Errorf("failed to initialize game aliases: %w", err)
	}
	return nil
}

// migration170 introduces first-class per-provider metadata identities.
func migration170(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS game_metadata_sources (
			game_id TEXT NOT NULL,
			source_type TEXT NOT NULL,
			source_id TEXT NOT NULL,
			cached_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (game_id, source_type)
		)
	`); err != nil {
		return fmt.Errorf("failed to create game_metadata_sources table: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO game_metadata_sources (
			game_id, source_type, source_id, cached_at, created_at, updated_at
		)
		SELECT
			id,
			LOWER(TRIM(source_type)),
			TRIM(source_id),
			cached_at,
			COALESCE(created_at, CURRENT_TIMESTAMP),
			COALESCE(updated_at, cached_at, created_at, CURRENT_TIMESTAMP)
		FROM games
		WHERE LOWER(TRIM(COALESCE(source_type, ''))) NOT IN ('', 'local', 'mixed')
		  AND TRIM(COALESCE(source_id, '')) <> ''
		ON CONFLICT (game_id, source_type) DO UPDATE SET
			source_id = EXCLUDED.source_id,
			cached_at = EXCLUDED.cached_at,
			updated_at = EXCLUDED.updated_at
	`); err != nil {
		return fmt.Errorf("failed to backfill game metadata sources: %w", err)
	}

	indexes := []struct {
		name string
		sql  string
	}{
		{"idx_game_metadata_sources_identity", `CREATE INDEX IF NOT EXISTS idx_game_metadata_sources_identity ON game_metadata_sources(source_type, source_id)`},
		{"idx_game_metadata_sources_game_id", `CREATE INDEX IF NOT EXISTS idx_game_metadata_sources_game_id ON game_metadata_sources(game_id)`},
	}
	for _, index := range indexes {
		if _, err := tx.Exec(index.sql); err != nil {
			return fmt.Errorf("failed to create %s: %w", index.name, err)
		}
	}
	return nil
}

// migration171 adds persistent game filter presets.
func migration171(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS game_filter_presets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			tags TEXT NOT NULL DEFAULT '[]',
			exclude_tags BOOLEAN NOT NULL DEFAULT FALSE,
			status TEXT NOT NULL DEFAULT '',
			exclude_status BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create game_filter_presets table: %w", err)
	}
	return nil
}

// migration172 adds one user-authored review per game.
func migration172(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS game_reviews (
			game_id TEXT PRIMARY KEY,
			rating INTEGER,
			content TEXT NOT NULL DEFAULT '',
			is_spoiler BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (rating IS NULL OR (rating >= 1 AND rating <= 10))
		)
	`); err != nil {
		return fmt.Errorf("failed to create game_reviews table: %w", err)
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
	{
		Version:     159,
		Description: "Add library list performance indexes",
		Up:          migration159,
	},
	{
		Version:     160,
		Description: "Add import duplicate-check indexes",
		Up:          migration160,
	},
	{
		Version:     161,
		Description: "Add per-game metadata lock for remote refresh",
		Up:          migration161,
	},
	{
		Version:     162,
		Description: "Add wine_runner, wine_args, wine_prefix columns to games table",
		Up:          migration162,
	},
	{
		Version:     163,
		Description: "Add per-game default launch mode",
		Up:          migration163,
	},
	{
		Version:     164,
		Description: "Add per-game NSFW flag with SFW default",
		Up:          migration164,
	},
	{
		Version:     165,
		Description: "Add remote cover source URL and game directory to games",
		Up:          migration165,
	},
	{
		Version:     166,
		Description: "Add device-local Steam launch identity to games",
		Up:          migration166,
	},
	{
		Version:     167,
		Description: "Migrate Wine games to explicit compatibility launch mode",
		Up:          migration167,
	},
	{
		Version:     168,
		Description: "Add Steam launch options and repair legacy Linux compatibility migration",
		Up:          migration168,
	},
	{
		Version:     169,
		Description: "Add JSON aliases column to games",
		Up:          migration169,
	},
	{
		Version:     170,
		Description: "Add first-class per-provider metadata identities",
		Up:          migration170,
	},
	{
		Version:     171,
		Description: "Add persistent game filter presets",
		Up:          migration171,
	},
	{
		Version:     172,
		Description: "Add user-authored game reviews",
		Up:          migration172,
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
