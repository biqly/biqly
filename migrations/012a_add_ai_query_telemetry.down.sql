DROP VIEW IF EXISTS v_ai_metrics_daily;
DROP INDEX IF EXISTS idx_ai_query_history_outcome_created;
ALTER TABLE ai_query_history
    DROP COLUMN IF EXISTS needs_clarification,
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS outcome_status;
