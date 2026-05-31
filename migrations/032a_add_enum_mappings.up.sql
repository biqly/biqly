CREATE TABLE IF NOT EXISTS enum_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dimension_id UUID NOT NULL REFERENCES semantic_dimensions(id) ON DELETE CASCADE,
    raw_value TEXT NOT NULL,
    label TEXT NOT NULL,
    description TEXT,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(dimension_id, raw_value)
);

CREATE INDEX idx_enum_mappings_dimension ON enum_mappings(dimension_id);
