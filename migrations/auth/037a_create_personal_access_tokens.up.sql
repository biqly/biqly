-- Personal access tokens: long-lived, revocable, user-generated credentials for
-- programmatic/API access (e.g. the MCP integration), as an alternative to
-- pasting a short-lived session JWT. The plaintext value is shown to the user
-- exactly once at creation time; only its hash is ever persisted.

CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_personal_access_tokens_user_id
    ON personal_access_tokens(user_id) WHERE revoked_at IS NULL;
