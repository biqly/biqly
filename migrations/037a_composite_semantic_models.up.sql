-- 037_composite_semantic_models.up.sql

CREATE TABLE IF NOT EXISTS composite_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    label TEXT,
    description TEXT,
    canonical_date JSONB,
    is_active BOOLEAN NOT NULL DEFAULT true,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published')),
    version INT NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ,
    published_by TEXT,
    draft_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(datasource_id, name)
);

CREATE INDEX IF NOT EXISTS idx_composite_models_datasource
    ON composite_models(datasource_id);

CREATE TABLE IF NOT EXISTS composite_model_components (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    composite_id UUID NOT NULL REFERENCES composite_models(id) ON DELETE CASCADE,
    model_id UUID NOT NULL REFERENCES semantic_models(id) ON DELETE CASCADE,
    alias TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'secondary'
        CHECK (role IN ('primary', 'secondary')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(composite_id, alias),
    UNIQUE(composite_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_composite_components_composite
    ON composite_model_components(composite_id);

CREATE TABLE IF NOT EXISTS composite_cross_model_joins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    composite_id UUID NOT NULL REFERENCES composite_models(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    from_alias TEXT NOT NULL,
    from_dimension TEXT NOT NULL,
    to_alias TEXT NOT NULL,
    to_dimension TEXT NOT NULL,
    join_type TEXT NOT NULL DEFAULT 'LEFT',
    relationship TEXT NOT NULL DEFAULT 'many_to_one',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(composite_id, name)
);

CREATE INDEX IF NOT EXISTS idx_composite_cross_joins_composite
    ON composite_cross_model_joins(composite_id);

CREATE TABLE IF NOT EXISTS composite_dimension_resolutions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    composite_id UUID NOT NULL REFERENCES composite_models(id) ON DELETE CASCADE,
    dimension_name TEXT NOT NULL,
    resolution TEXT NOT NULL DEFAULT 'use_primary'
        CHECK (resolution IN ('use_primary', 'rename', 'merge')),
    source_alias TEXT,
    target_alias TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(composite_id, dimension_name, source_alias)
);

CREATE INDEX IF NOT EXISTS idx_composite_dim_resolutions_composite
    ON composite_dimension_resolutions(composite_id);

CREATE TABLE IF NOT EXISTS composite_context_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    composite_id UUID NOT NULL REFERENCES composite_models(id) ON DELETE CASCADE,
    version INT NOT NULL,
    context JSONB NOT NULL,
    validation_result JSONB NOT NULL DEFAULT '{}',
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(composite_id, version)
);

CREATE INDEX IF NOT EXISTS idx_composite_context_snapshots_composite_version
    ON composite_context_snapshots(composite_id, version DESC);
