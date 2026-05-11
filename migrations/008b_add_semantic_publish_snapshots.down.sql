-- 008_add_semantic_publish_snapshots.down.sql

DROP TABLE IF EXISTS semantic_context_snapshots;

ALTER TABLE semantic_models
    DROP COLUMN IF EXISTS draft_updated_at,
    DROP COLUMN IF EXISTS published_by,
    DROP COLUMN IF EXISTS published_at,
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS status;
