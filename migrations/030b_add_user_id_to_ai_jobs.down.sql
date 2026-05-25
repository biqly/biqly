DROP INDEX IF EXISTS idx_ai_jobs_user_pending_position;
DROP INDEX IF EXISTS idx_ai_jobs_user_status;
ALTER TABLE ai_jobs DROP COLUMN IF EXISTS user_id;
