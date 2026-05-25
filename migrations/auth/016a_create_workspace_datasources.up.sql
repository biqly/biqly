CREATE TABLE workspace_datasources (
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    datasource_id UUID NOT NULL,
    access_level  TEXT NOT NULL DEFAULT 'read',
    attached_by   UUID REFERENCES users(id),
    attached_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, datasource_id)
);
