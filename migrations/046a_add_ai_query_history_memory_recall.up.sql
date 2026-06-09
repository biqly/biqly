ALTER TABLE ai_query_history
    ADD COLUMN IF NOT EXISTS memory_recall_used BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS memory_recall_hit_count INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_ai_query_history_memory_recall_used
    ON ai_query_history (memory_recall_used)
    WHERE memory_recall_used = TRUE;
