-- 005_add_table_embeddings.down.sql
DROP INDEX IF EXISTS idx_tables_embedding_model;
ALTER TABLE tables
    DROP COLUMN IF EXISTS embedding_updated_at,
    DROP COLUMN IF EXISTS embedding_model,
    DROP COLUMN IF EXISTS embedding;
