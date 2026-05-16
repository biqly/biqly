-- 017b_add_labels_and_schema_filter.down.sql
ALTER TABLE tables DROP COLUMN IF EXISTS label;
ALTER TABLE semantic_models DROP COLUMN IF EXISTS excluded_schemas;
