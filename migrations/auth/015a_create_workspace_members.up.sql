CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id      UUID NOT NULL REFERENCES roles(id),
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_by   UUID REFERENCES users(id),
    PRIMARY KEY (workspace_id, user_id)
);
