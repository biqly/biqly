CREATE TABLE IF NOT EXISTS ab_experiments (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    template_name TEXT NOT NULL,
    locale TEXT NOT NULL DEFAULT 'en',
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'running', 'paused', 'completed')),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ab_exp_status
    ON ab_experiments (status);

CREATE TABLE IF NOT EXISTS ab_variants (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    experiment_id TEXT NOT NULL REFERENCES ab_experiments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    template_version INT NOT NULL,
    traffic_pct INT NOT NULL DEFAULT 50 CHECK (traffic_pct >= 0 AND traffic_pct <= 100),
    is_control BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (experiment_id, name)
);

ALTER TABLE ai_query_history
    ADD COLUMN IF NOT EXISTS ab_experiment_id TEXT,
    ADD COLUMN IF NOT EXISTS ab_variant_id TEXT;
