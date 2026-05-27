CREATE TABLE IF NOT EXISTS email_block_list (
    email TEXT PRIMARY KEY,
    reason TEXT NOT NULL,
    blocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT
);

CREATE INDEX IF NOT EXISTS email_block_list_blocked_at_idx ON email_block_list (blocked_at DESC);
