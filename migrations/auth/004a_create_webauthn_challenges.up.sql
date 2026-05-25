CREATE TABLE webauthn_challenges (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge  BYTEA NOT NULL,
    user_id    UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
