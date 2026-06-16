DROP INDEX IF EXISTS idx_ai_confirmed_queries_uniq;

CREATE INDEX IF NOT EXISTS idx_ai_confirmed_queries_dedup
    ON ai_confirmed_queries (datasource_id, model_id, question_hash, semantic_model_hash);
