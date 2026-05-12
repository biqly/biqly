-- 010_add_query_fingerprint.up.sql
-- Fingerprint groups runs of the same canonical LogicalQuery under the same
-- semantic model version and permission scope. Used by audit grouping and
-- (later) cache key lookup.
ALTER TABLE query_history
    ADD COLUMN IF NOT EXISTS query_fingerprint TEXT;

CREATE INDEX IF NOT EXISTS idx_query_history_fingerprint
    ON query_history(query_fingerprint)
    WHERE query_fingerprint IS NOT NULL;

ALTER TABLE ai_query_history
    ADD COLUMN IF NOT EXISTS query_fingerprint TEXT;

CREATE INDEX IF NOT EXISTS idx_ai_query_history_fingerprint
    ON ai_query_history(query_fingerprint)
    WHERE query_fingerprint IS NOT NULL;
