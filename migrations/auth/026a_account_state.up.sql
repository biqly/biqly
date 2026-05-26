-- §18.1 Account Security: freeze, GDPR soft-delete, password aging,
-- new-device detection, concurrent session limit, lockout notifications.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS frozen_at           TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at          TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS purge_after         TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;

UPDATE users SET password_changed_at = created_at WHERE password_changed_at IS NULL AND password_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS users_deleted_at_idx  ON users(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS users_purge_after_idx ON users(purge_after) WHERE purge_after IS NOT NULL;

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS last_active_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS device_fingerprint TEXT;

CREATE INDEX IF NOT EXISTS sessions_user_active_idx
    ON sessions(user_id, last_active_at DESC)
    WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS known_devices (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fingerprint  TEXT NOT NULL,
    user_agent   TEXT,
    ip_address   INET,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, fingerprint)
);

CREATE TABLE IF NOT EXISTS account_unlock_tokens (
    token       TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS account_unlock_tokens_user_idx ON account_unlock_tokens(user_id);
