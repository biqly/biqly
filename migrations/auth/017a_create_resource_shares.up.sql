CREATE TABLE resource_shares (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_type TEXT NOT NULL,
    resource_id   UUID NOT NULL,
    owner_id      UUID NOT NULL REFERENCES users(id),
    shared_with   UUID REFERENCES users(id),
    workspace_id  UUID REFERENCES workspaces(id),
    permission    TEXT NOT NULL DEFAULT 'view',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX resource_shares_unique_idx ON resource_shares (
    resource_type, 
    resource_id, 
    COALESCE(shared_with, '00000000-0000-0000-0000-000000000000'), 
    COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000')
);

