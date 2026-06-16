-- 007_add_column_embeddings.down.sql
DROP INDEX IF EXISTS idx_columns_embedding_model;
ALTER TABLE columns
    DROP COLUMN IF EXISTS embedding_updated_at,
    DROP COLUMN IF EXISTS embedding_model,
    DROP COLUMN IF EXISTS embedding;
