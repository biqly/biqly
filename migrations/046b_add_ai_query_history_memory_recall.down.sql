DROP INDEX IF EXISTS idx_ai_query_history_memory_recall_used;

ALTER TABLE ai_query_history
    DROP COLUMN IF EXISTS memory_recall_hit_count,
    DROP COLUMN IF EXISTS memory_recall_used;
