CREATE TABLE IF NOT EXISTS update_events (
    event_id TEXT PRIMARY KEY,
    transaction_id TEXT,
    installation_id TEXT,
    event_type TEXT NOT NULL,
    current_version TEXT,
    target_version TEXT NOT NULL,
    channel TEXT NOT NULL,
    architecture TEXT NOT NULL,
    build_mode TEXT NOT NULL,
    artifact TEXT,
    transferred_bytes INTEGER,
    failure_code TEXT,
    client_time TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_update_events_version_type
    ON update_events(target_version, event_type);

CREATE INDEX IF NOT EXISTS idx_update_events_created_at
    ON update_events(created_at);

CREATE TABLE IF NOT EXISTS download_requests (
    request_date TEXT NOT NULL,
    version TEXT NOT NULL,
    asset TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    requested_bytes INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (request_date, version, asset)
);
