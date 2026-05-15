ALTER TABLE ai_query_history
    ADD COLUMN IF NOT EXISTS outcome_status TEXT NOT NULL DEFAULT 'unknown'
        CHECK (outcome_status IN ('unknown', 'success', 'partial', 'failed', 'clarification')),
    ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS needs_clarification BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_ai_query_history_outcome_created
    ON ai_query_history(outcome_status, created_at DESC);

CREATE OR REPLACE VIEW v_ai_metrics_daily AS
SELECT
    DATE(created_at) AS metric_date,
    COUNT(*) AS total_queries,
    COUNT(*) FILTER (WHERE outcome_status = 'success') AS success_count,
    COUNT(*) FILTER (WHERE outcome_status = 'failed') AS failed_count,
    COUNT(*) FILTER (WHERE outcome_status = 'partial') AS partial_count,
    COUNT(*) FILTER (WHERE outcome_status = 'clarification') AS clarification_count,
    COALESCE(AVG(retry_count), 0) AS avg_retry_count,
    COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
    COALESCE(SUM(cost_usd), 0) AS total_cost,
    COALESCE(SUM(token_count), 0) AS total_tokens
FROM ai_query_history
GROUP BY DATE(created_at)
ORDER BY metric_date DESC;
