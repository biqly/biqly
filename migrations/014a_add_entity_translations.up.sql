-- Entity translations: language-specific overlay for description/label fields.
-- One row per (entity_type, entity_id, lang, field). Repository merges this
-- table with the legacy `description` columns: requested lang → 'en' → raw.
--
-- entity_type values: 'table', 'column', 'semantic_model',
-- 'semantic_dimension', 'semantic_metric'.
-- field values: 'description', 'label'.

CREATE TABLE IF NOT EXISTS entity_translations (
    entity_type TEXT NOT NULL,
    entity_id   UUID NOT NULL,
    lang        TEXT NOT NULL,
    field       TEXT NOT NULL,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id, lang, field)
);

CREATE INDEX IF NOT EXISTS idx_entity_translations_lookup
    ON entity_translations (entity_type, entity_id, lang);

CREATE INDEX IF NOT EXISTS idx_entity_translations_by_lang
    ON entity_translations (lang, entity_type);
