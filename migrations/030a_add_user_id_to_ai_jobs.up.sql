ALTER TABLE ai_jobs ADD COLUMN IF NOT EXISTS user_id TEXT;

CREATE INDEX IF NOT EXISTS idx_ai_jobs_user_status
    ON ai_jobs (user_id, status, created_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_jobs_user_pending_position
    ON ai_jobs (created_at)
    WHERE status IN ('pending', 'queued');
