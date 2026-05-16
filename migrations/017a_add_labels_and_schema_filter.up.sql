-- 017a_add_labels_and_schema_filter.up.sql
-- Human-friendly label for metadata.tables so the modeling UI can show a
-- display name alongside the technical schema.table identifier.
ALTER TABLE tables ADD COLUMN IF NOT EXISTS label TEXT;

-- Schemas the modeler chose to hide from the semantic model so they never
-- appear on the canvas or in AI prompts.
ALTER TABLE semantic_models ADD COLUMN IF NOT EXISTS excluded_schemas TEXT[] NOT NULL DEFAULT '{}';
