-- Eval run summaries (aggregate per-run stats)
CREATE TABLE IF NOT EXISTS eval_runs (
    run_id UUID PRIMARY KEY,
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    context_version INT NOT NULL DEFAULT 0,
    total_cases INT NOT NULL DEFAULT 0,
    passed INT NOT NULL DEFAULT 0,
    failed INT NOT NULL DEFAULT 0,
    avg_confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    avg_latency_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_tokens INT NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Per-case eval results
CREATE TABLE IF NOT EXISTS eval_results (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES eval_runs(run_id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    context_version INT NOT NULL DEFAULT 0,
    context_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    case_id TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL DEFAULT '',
    expected_lq JSONB,
    got_lq JSONB,
    match BOOLEAN NOT NULL DEFAULT false,
    reason TEXT NOT NULL DEFAULT '',
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    token_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_eval_results_run_id ON eval_results(run_id);
CREATE INDEX IF NOT EXISTS idx_eval_results_case_id ON eval_results(case_id);
CREATE INDEX IF NOT EXISTS idx_eval_runs_completed_at ON eval_runs(completed_at DESC);
CREATE INDEX IF NOT EXISTS idx_eval_runs_provider_model ON eval_runs(provider, model);
