-- 003_create_semantic_layer.up.sql
CREATE TABLE IF NOT EXISTS semantic_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    label TEXT,
    description TEXT,
    base_schema TEXT NOT NULL,
    base_table TEXT NOT NULL,
    synonyms TEXT[] DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(datasource_id, name)
);

CREATE INDEX idx_semantic_models_datasource ON semantic_models(datasource_id);
CREATE INDEX idx_semantic_models_name ON semantic_models(name);

CREATE TABLE IF NOT EXISTS semantic_dimensions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES semantic_models(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    label TEXT,
    column_ref TEXT NOT NULL, -- can be "table.column" for joined tables
    type TEXT NOT NULL DEFAULT 'text', -- text, number, date, boolean, geo
    synonyms TEXT[] DEFAULT '{}',
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(model_id, name)
);

CREATE INDEX idx_semantic_dimensions_model ON semantic_dimensions(model_id);

CREATE TABLE IF NOT EXISTS semantic_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES semantic_models(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    label TEXT,
    expression TEXT NOT NULL, -- e.g. "orders.id"
    aggregation TEXT NOT NULL, -- count, sum, avg, min, max, count_distinct
    format TEXT, -- e.g. "$#,##0" for currency
    synonyms TEXT[] DEFAULT '{}',
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(model_id, name)
);

CREATE INDEX idx_semantic_metrics_model ON semantic_metrics(model_id);

CREATE TABLE IF NOT EXISTS semantic_joins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES semantic_models(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    from_table TEXT NOT NULL,
    from_column TEXT NOT NULL,
    to_table TEXT NOT NULL,
    to_column TEXT NOT NULL,
    join_type TEXT NOT NULL DEFAULT 'LEFT', -- LEFT, INNER, RIGHT
    relationship TEXT NOT NULL DEFAULT 'many_to_one',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(model_id, name)
);

CREATE INDEX idx_semantic_joins_model ON semantic_joins(model_id);
