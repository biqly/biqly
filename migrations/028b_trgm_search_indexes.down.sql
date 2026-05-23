DROP INDEX IF EXISTS idx_columns_column_name_trgm;
DROP INDEX IF EXISTS idx_columns_description_trgm;
DROP INDEX IF EXISTS idx_tables_table_name_trgm;
DROP INDEX IF EXISTS idx_tables_description_trgm;
-- pg_trgm extension intentionally left installed; other indexes may depend on it.
