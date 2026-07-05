-- Recreates the legacy stores for rollback only; dropped table data is not restored.
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_confirmed_queries_uniq
    ON ai_confirmed_queries (datasource_id, question_hash, semantic_model_hash,
                             COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid));

CREATE INDEX IF NOT EXISTS idx_ai_confirmed_queries_recall
    ON ai_confirmed_queries (datasource_id, model_id, is_active, confirmed_at DESC)
    WHERE is_active = true;

CREATE TABLE IF NOT EXISTS ai_skills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    model_id UUID REFERENCES semantic_models(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL DEFAULT '',
    logical_query JSONB NOT NULL,
    parameters JSONB NOT NULL DEFAULT '[]',
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (datasource_id, name)
);

CREATE INDEX IF NOT EXISTS idx_ai_skills_datasource
    ON ai_skills (datasource_id, is_active, updated_at DESC);
