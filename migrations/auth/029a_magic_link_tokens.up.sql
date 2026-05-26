CREATE TABLE IF NOT EXISTS magic_link_tokens (
    token_hash TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    user_id TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip_address TEXT
);

CREATE INDEX IF NOT EXISTS magic_link_tokens_email_idx ON magic_link_tokens (email);
CREATE INDEX IF NOT EXISTS magic_link_tokens_expires_at_idx ON magic_link_tokens (expires_at);
