-- Reverse agentic query runner runtime state migration.

DROP INDEX IF EXISTS idx_agent_shadow_comparisons_category_created;
DROP INDEX IF EXISTS idx_agent_shadow_comparisons_job;
DROP TABLE IF EXISTS agent_shadow_comparisons;

ALTER TABLE agent_steps
    DROP CONSTRAINT IF EXISTS ux_agent_steps_run_seq;

DROP INDEX IF EXISTS ux_agent_runs_job;

ALTER TABLE agent_runs
    DROP COLUMN IF EXISTS query_execute_started,
    DROP COLUMN IF EXISTS terminal_version,
    DROP COLUMN IF EXISTS runtime_state,
    DROP COLUMN IF EXISTS job_id;
