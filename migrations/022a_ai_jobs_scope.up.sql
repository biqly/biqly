ALTER TABLE ai_jobs
    ADD COLUMN IF NOT EXISTS datasource_id UUID,
    ADD COLUMN IF NOT EXISTS scope_schemas TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS progress_json JSONB;

CREATE INDEX IF NOT EXISTS idx_ai_jobs_describe_batch_active_scope
    ON ai_jobs (datasource_id)
    WHERE kind = 'describe_batch'
      AND status IN ('pending', 'queued', 'running');
