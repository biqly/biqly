-- Structured connection columns for datasources. Legacy `dsn_encrypted` stays
-- intact for old rows. New rows are written with dsn_mode='structured' and the
-- structured columns populated; the handler composes the runtime DSN from them
-- per driver. Existing raw-DSN rows default to dsn_mode='raw' and continue to
-- be read via dsn_encrypted.

ALTER TABLE datasources
    ADD COLUMN IF NOT EXISTS host               TEXT,
    ADD COLUMN IF NOT EXISTS port               INTEGER,
    ADD COLUMN IF NOT EXISTS username           TEXT,
    ADD COLUMN IF NOT EXISTS password_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS database_name      TEXT,
    ADD COLUMN IF NOT EXISTS ssl_mode           TEXT,
    ADD COLUMN IF NOT EXISTS connection_params  JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS dsn_mode           TEXT NOT NULL DEFAULT 'raw'
        CHECK (dsn_mode IN ('structured', 'raw'));

CREATE INDEX IF NOT EXISTS idx_datasources_dsn_mode
    ON datasources (dsn_mode);
