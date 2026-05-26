-- §18.3: absolute session timeout. Refresh-token rotation must not extend a
-- session forever — the absolute cap is fixed at the time of the original
-- authentication and copied across every rotation.

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS absolute_expires_at TIMESTAMPTZ;

UPDATE sessions
SET absolute_expires_at = created_at + INTERVAL '30 days'
WHERE absolute_expires_at IS NULL;

ALTER TABLE sessions
    ALTER COLUMN absolute_expires_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS sessions_absolute_expires_at_idx
    ON sessions(absolute_expires_at)
    WHERE revoked_at IS NULL;
