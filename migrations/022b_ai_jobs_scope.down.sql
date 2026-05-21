DROP INDEX IF EXISTS idx_ai_jobs_describe_batch_active_scope;

ALTER TABLE ai_jobs
    DROP COLUMN IF EXISTS progress_json,
    DROP COLUMN IF EXISTS scope_schemas,
    DROP COLUMN IF EXISTS datasource_id;
