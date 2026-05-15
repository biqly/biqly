-- Curated business glossary terms (external to dimension/metric synonym arrays).
CREATE TABLE IF NOT EXISTS business_glossary_terms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    model_id UUID REFERENCES semantic_models(id) ON DELETE CASCADE,
    term TEXT NOT NULL,
    definition TEXT,
    maps_to_type TEXT NOT NULL CHECK (maps_to_type IN ('dimension', 'metric', 'model')),
    maps_to_name TEXT NOT NULL,
    aliases TEXT[] DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (datasource_id, term)
);

CREATE INDEX idx_business_glossary_datasource ON business_glossary_terms(datasource_id);
CREATE INDEX idx_business_glossary_model ON business_glossary_terms(model_id);
CREATE INDEX idx_business_glossary_term ON business_glossary_terms USING GIN (to_tsvector('simple', term));
