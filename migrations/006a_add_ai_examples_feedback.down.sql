-- 006_add_ai_examples_feedback.down.sql

DROP VIEW IF EXISTS v_ai_usage_daily;

DROP TABLE IF EXISTS ai_feedback;
DROP TABLE IF EXISTS few_shot_examples;

ALTER TABLE ai_query_history
    DROP COLUMN IF EXISTS user_rating,
    DROP COLUMN IF EXISTS model_used,
    DROP COLUMN IF EXISTS token_count,
    DROP COLUMN IF EXISTS cost_usd,
    DROP COLUMN IF EXISTS latency_ms;
