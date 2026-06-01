DROP INDEX IF EXISTS idx_ai_query_history_user_model_created;
DROP INDEX IF EXISTS idx_ai_query_history_model_used_created;
ALTER TABLE ai_query_history
    DROP COLUMN IF EXISTS completion_tokens,
    DROP COLUMN IF EXISTS prompt_tokens;
