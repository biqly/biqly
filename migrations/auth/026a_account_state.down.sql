DROP TABLE IF EXISTS account_unlock_tokens;
DROP TABLE IF EXISTS known_devices;

DROP INDEX IF EXISTS sessions_user_active_idx;
ALTER TABLE sessions
    DROP COLUMN IF EXISTS device_fingerprint,
    DROP COLUMN IF EXISTS last_active_at;

DROP INDEX IF EXISTS users_purge_after_idx;
DROP INDEX IF EXISTS users_deleted_at_idx;
ALTER TABLE users
    DROP COLUMN IF EXISTS password_changed_at,
    DROP COLUMN IF EXISTS purge_after,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS frozen_at;
