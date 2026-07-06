-- Agentic query runner runtime state (Agentic Runtime A4).
-- Extends agent_runs with the resumable planner/tool-loop state so a
-- NATS-redelivered job resumes from its persisted step timeline instead of
-- restarting the run, and terminal_version marks a run's outcome as
-- immutable once set. agent_shadow_comparisons records shadow-mode
-- agent-vs-legacy outcome comparisons for rollout evaluation (Task 10).

ALTER TABLE agent_runs
    ADD COLUMN IF NOT EXISTS job_id UUID,
    ADD COLUMN IF NOT EXISTS runtime_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS terminal_version INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS query_execute_started BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX IF NOT EXISTS ux_agent_runs_job
    ON agent_runs(job_id) WHERE job_id IS NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'ux_agent_steps_run_seq'
    ) THEN
        ALTER TABLE agent_steps
            ADD CONSTRAINT ux_agent_steps_run_seq UNIQUE (run_id, seq);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS agent_shadow_comparisons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,
    legacy_run_id UUID,
    agent_run_id UUID,
    category TEXT NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_shadow_comparisons_job
    ON agent_shadow_comparisons(job_id);

CREATE INDEX IF NOT EXISTS idx_agent_shadow_comparisons_category_created
    ON agent_shadow_comparisons(category, created_at DESC);
