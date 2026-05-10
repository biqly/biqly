-- 007_add_column_embeddings.up.sql
-- Stores semantic embeddings for individual columns so the AI router can keep
-- the most relevant column context after table routing.
ALTER TABLE columns
    ADD COLUMN IF NOT EXISTS embedding JSONB,
    ADD COLUMN IF NOT EXISTS embedding_model TEXT,
    ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_columns_embedding_model ON columns(embedding_model)
    WHERE embedding_model IS NOT NULL;
