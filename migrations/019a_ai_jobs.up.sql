CREATE TABLE IF NOT EXISTS ai_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_session_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('query', 'preview', 'run')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'queued', 'running', 'succeeded', 'failed', 'cancelled')),
    phase TEXT NOT NULL DEFAULT 'queued',
    phase_message TEXT NOT NULL DEFAULT '',
    progress_pct INT NOT NULL DEFAULT 0 CHECK (progress_pct >= 0 AND progress_pct <= 100),
    request_json JSONB NOT NULL,
    result_json JSONB,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_jobs_client_session_created
    ON ai_jobs (client_session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_jobs_active
    ON ai_jobs (status, created_at DESC)
    WHERE status IN ('pending', 'queued', 'running');
