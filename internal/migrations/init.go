package migrations

import "database/sql"

func InitSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			created_at TIMESTAMPTZ,
			default_backup_target TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id TEXT PRIMARY KEY,
			name TEXT,
			emoji TEXT DEFAULT '',
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
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
			status TEXT DEFAULT 'not_started',
			source_type TEXT,
			cached_at TIMESTAMPTZ,
			source_id TEXT,
			created_at TIMESTAMPTZ,
			use_locale_emulator BOOLEAN DEFAULT FALSE,
			use_magpie BOOLEAN DEFAULT FALSE
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
		`CREATE TABLE IF NOT EXISTS download_tasks (
			id TEXT PRIMARY KEY,
			request_json TEXT,
			status TEXT,
			progress DOUBLE,
			downloaded BIGINT,
			total BIGINT,
			error TEXT,
			file_path TEXT,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
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
