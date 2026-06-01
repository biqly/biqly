ALTER TABLE ai_query_history
    ADD COLUMN IF NOT EXISTS prompt_tokens INT,
    ADD COLUMN IF NOT EXISTS completion_tokens INT;

CREATE INDEX IF NOT EXISTS idx_ai_query_history_model_used_created
    ON ai_query_history (model_used, created_at DESC)
    WHERE model_used IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_query_history_user_model_created
    ON ai_query_history (user_id, model_used, created_at DESC)
    WHERE user_id IS NOT NULL AND model_used IS NOT NULL;
