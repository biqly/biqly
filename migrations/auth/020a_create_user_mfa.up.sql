CREATE TABLE user_mfa (
    user_id              UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    method               TEXT NOT NULL DEFAULT 'totp',
    secret_encrypted     BYTEA NOT NULL,
    recovery_codes       TEXT[] NOT NULL DEFAULT '{}',
    enabled              BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at          TIMESTAMPTZ,
    last_used_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_mfa_enabled ON user_mfa(enabled) WHERE enabled = TRUE;
