DROP INDEX IF EXISTS idx_users_active_workspace_id;
ALTER TABLE users DROP COLUMN IF EXISTS active_workspace_id;
