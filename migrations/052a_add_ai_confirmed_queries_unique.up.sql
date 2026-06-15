-- Replaces the non-unique idx_ai_confirmed_queries_dedup with a unique index that
-- also handles NULL model_id (COALESCE to sentinel UUID). This lets UpsertConfirmedQuery
-- use a single atomic INSERT … ON CONFLICT DO UPDATE instead of the non-atomic
-- UPDATE-then-INSERT pattern that could produce duplicate rows.

-- 1. Remove any latent duplicates (keep most-recent row per logical key).
WITH dups AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY datasource_id, question_hash, semantic_model_hash,
                            COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid)
               ORDER BY confirmed_at DESC
           ) AS rn
    FROM ai_confirmed_queries
)
DELETE FROM ai_confirmed_queries
WHERE id IN (SELECT id FROM dups WHERE rn > 1);

-- 2. Drop the old non-unique index.
DROP INDEX IF EXISTS idx_ai_confirmed_queries_dedup;

-- 3. Create the unique index. COALESCE treats NULL model_id as a sentinel UUID so
--    that the unique constraint covers rows where model_id IS NULL as well.
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_confirmed_queries_uniq
    ON ai_confirmed_queries (datasource_id, question_hash, semantic_model_hash,
                             COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid));
