-- 005_add_table_embeddings.up.sql
-- Stores semantic embeddings of each table for vector-based table retrieval.
-- We use JSONB rather than a vector type so deployments without pgvector keep
-- working; cosine similarity is computed in-process by the AI router.
ALTER TABLE tables
    ADD COLUMN IF NOT EXISTS embedding JSONB,
    ADD COLUMN IF NOT EXISTS embedding_model TEXT,
    ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_tables_embedding_model ON tables(embedding_model)
    WHERE embedding_model IS NOT NULL;
