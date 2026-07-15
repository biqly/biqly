-- Public share links for dashboards (embedded analytics phase 1).
-- Token plaintext lives only in the share URL; we persist its SHA-256 hex.
CREATE TABLE IF NOT EXISTS dashboard_public_shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dashboard_id UUID NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ
);

-- At most one active (non-revoked) share per dashboard: "rotate" revokes then inserts.
CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_public_shares_active
    ON dashboard_public_shares(dashboard_id) WHERE revoked_at IS NULL;
