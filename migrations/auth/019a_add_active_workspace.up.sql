ALTER TABLE users ADD COLUMN active_workspace_id UUID REFERENCES workspaces(id) ON DELETE SET NULL;
CREATE INDEX idx_users_active_workspace_id ON users(active_workspace_id);
