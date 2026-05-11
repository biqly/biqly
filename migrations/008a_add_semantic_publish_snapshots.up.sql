-- 008_add_semantic_publish_snapshots.up.sql

ALTER TABLE semantic_models
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'published'
        CHECK (status IN ('draft', 'published')),
    ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS published_by TEXT,
    ADD COLUMN IF NOT EXISTS draft_updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS semantic_context_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES semantic_models(id) ON DELETE CASCADE,
    version INT NOT NULL,
    context JSONB NOT NULL,
    validation_result JSONB NOT NULL DEFAULT '{}',
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(model_id, version)
);

CREATE INDEX IF NOT EXISTS idx_semantic_context_snapshots_model_version
    ON semantic_context_snapshots(model_id, version DESC);
