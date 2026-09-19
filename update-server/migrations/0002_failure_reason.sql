ALTER TABLE update_events ADD COLUMN failure_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_update_events_failure
    ON update_events(event_type, failure_code, failure_reason);
