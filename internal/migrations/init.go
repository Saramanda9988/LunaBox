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
			rating DOUBLE DEFAULT 0,
			release_date TEXT DEFAULT '',
			path TEXT,
			save_path TEXT,
			process_name TEXT DEFAULT '',
			status TEXT DEFAULT 'not_started',
			source_type TEXT,
			cached_at TIMESTAMPTZ,
			source_id TEXT,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			use_locale_emulator BOOLEAN DEFAULT FALSE,
			use_magpie BOOLEAN DEFAULT FALSE,
			metadata_locked BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE IF NOT EXISTS game_categories (
			game_id TEXT,
			category_id TEXT,
			updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
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
		`CREATE TABLE IF NOT EXISTS sync_tombstones (
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			parent_id TEXT DEFAULT '',
			secondary_id TEXT DEFAULT '',
			deleted_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (entity_type, entity_id, parent_id, secondary_id)
		)`,
		`CREATE TABLE IF NOT EXISTS game_progress (
			id TEXT PRIMARY KEY,
			game_id TEXT NOT NULL,
			chapter TEXT DEFAULT '',
			route TEXT DEFAULT '',
			progress_note TEXT DEFAULT '',
			spoiler_boundary TEXT DEFAULT 'none',
			updated_at TIMESTAMPTZ
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
		`
		CREATE TABLE IF NOT EXISTS game_tags (
			id          TEXT PRIMARY KEY,
			game_id     TEXT NOT NULL,
			name        TEXT NOT NULL,
			source      TEXT NOT NULL,
			weight      DOUBLE DEFAULT 1.0,
			is_spoiler  BOOLEAN DEFAULT FALSE,
			created_at  TIMESTAMPTZ,
			updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (game_id, name, source)
		)
		`,
		`CREATE INDEX IF NOT EXISTS idx_games_status ON games(status)`,
		`CREATE INDEX IF NOT EXISTS idx_games_created_at ON games(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_games_rating ON games(rating)`,
		`CREATE INDEX IF NOT EXISTS idx_games_release_date ON games(release_date)`,
		`CREATE INDEX IF NOT EXISTS idx_games_path ON games(path)`,
		`CREATE INDEX IF NOT EXISTS idx_games_source_identity ON games(source_type, source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_games_name_path ON games(name, path)`,
		`CREATE INDEX IF NOT EXISTS idx_play_sessions_game_start ON play_sessions(game_id, start_time)`,
		`CREATE INDEX IF NOT EXISTS idx_game_tags_game_id ON game_tags(game_id)`,
		`CREATE INDEX IF NOT EXISTS idx_game_tags_name ON game_tags(name)`,
		`CREATE INDEX IF NOT EXISTS idx_game_tags_name_game ON game_tags(name, game_id)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}
	return nil
}
