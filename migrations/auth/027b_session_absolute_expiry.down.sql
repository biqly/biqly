DROP INDEX IF EXISTS sessions_absolute_expires_at_idx;
ALTER TABLE sessions DROP COLUMN IF EXISTS absolute_expires_at;
