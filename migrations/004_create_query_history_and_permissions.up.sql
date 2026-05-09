-- 004_create_query_history_and_permissions.up.sql
CREATE TABLE IF NOT EXISTS query_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id),
    model_id UUID REFERENCES semantic_models(id),
    user_id TEXT,
    logical_query JSONB NOT NULL,
    compiled_sql TEXT,
    sql_args JSONB,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, running, success, failed, cancelled
    row_count INT,
    duration_ms INT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_query_history_datasource ON query_history(datasource_id);
CREATE INDEX idx_query_history_user ON query_history(user_id);
CREATE INDEX idx_query_history_created ON query_history(created_at DESC);

CREATE TABLE IF NOT EXISTS query_saved (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id),
    model_id UUID REFERENCES semantic_models(id),
    name TEXT NOT NULL,
    description TEXT,
    logical_query JSONB NOT NULL,
    tags TEXT[] DEFAULT '{}',
    created_by TEXT NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_query_saved_created_by ON query_saved(created_by);

CREATE TABLE IF NOT EXISTS ai_query_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id),
    model_id UUID REFERENCES semantic_models(id),
    user_id TEXT,
    question TEXT NOT NULL,
    prompt_context JSONB,
    ai_response JSONB,
    logical_query JSONB,
    confidence_score FLOAT,
    warnings TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_query_history_user ON ai_query_history(user_id);
CREATE INDEX idx_ai_query_history_created ON ai_query_history(created_at DESC);

CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    datasource_id UUID REFERENCES datasources(id),
    allowed_models TEXT[] DEFAULT '{}',
    denied_fields TEXT[] DEFAULT '{}',
    row_filters JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, datasource_id)
);

CREATE INDEX idx_permissions_user ON permissions(user_id);
