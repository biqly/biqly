CREATE INDEX IF NOT EXISTS idx_audit_events_history
    ON audit_events ((details->>'history_id'))
    WHERE details ? 'history_id';
