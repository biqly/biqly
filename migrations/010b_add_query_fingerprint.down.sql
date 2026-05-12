-- 010_add_query_fingerprint.down.sql
DROP INDEX IF EXISTS idx_ai_query_history_fingerprint;
ALTER TABLE ai_query_history DROP COLUMN IF EXISTS query_fingerprint;

DROP INDEX IF EXISTS idx_query_history_fingerprint;
ALTER TABLE query_history DROP COLUMN IF EXISTS query_fingerprint;
