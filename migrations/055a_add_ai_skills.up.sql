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
