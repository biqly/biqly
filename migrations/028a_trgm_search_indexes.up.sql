-- Speed up Repository.SearchColumns and Repository.SearchTables. Both run
-- `column_or_table ILIKE '%' || term || '%'` which cannot use a B-tree
-- index. With pg_trgm + GIN, the leading-wildcard substring search becomes
-- index-backed and stays sub-millisecond on large metadata catalogs.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_columns_column_name_trgm
    ON columns USING gin (column_name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_columns_description_trgm
    ON columns USING gin (description gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_tables_table_name_trgm
    ON tables USING gin (table_name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_tables_description_trgm
    ON tables USING gin (description gin_trgm_ops);
