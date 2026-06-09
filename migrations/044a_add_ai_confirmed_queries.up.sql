CREATE TABLE IF NOT EXISTS ai_confirmed_queries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    model_id UUID REFERENCES semantic_models(id) ON DELETE SET NULL,
    user_id TEXT,
    question_hash TEXT NOT NULL,
    nl_query TEXT NOT NULL,
    sql_query TEXT NOT NULL,
    semantic_model_hash TEXT NOT NULL,
    question_embedding JSONB,
    is_active BOOLEAN NOT NULL DEFAULT true,
    confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_confirmed_queries_dedup
    ON ai_confirmed_queries (datasource_id, model_id, question_hash, semantic_model_hash);

CREATE INDEX IF NOT EXISTS idx_ai_confirmed_queries_recall
    ON ai_confirmed_queries (datasource_id, model_id, is_active, confirmed_at DESC)
    WHERE is_active = true;
