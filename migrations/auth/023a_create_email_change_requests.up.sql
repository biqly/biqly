CREATE TABLE email_change_requests (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    old_email              TEXT NOT NULL,
    new_email              TEXT NOT NULL,
    old_email_token TEXT NOT NULL UNIQUE,
    new_email_token TEXT NOT NULL UNIQUE,
    old_email_confirmed_at TIMESTAMPTZ,
    new_email_confirmed_at TIMESTAMPTZ,
    requested_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    not_before             TIMESTAMPTZ NOT NULL,
    expires_at             TIMESTAMPTZ NOT NULL,
    completed_at           TIMESTAMPTZ
);

CREATE INDEX idx_email_change_requests_user_pending
    ON email_change_requests (user_id)
    WHERE completed_at IS NULL;
