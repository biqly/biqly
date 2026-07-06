-- Persistent agent run + step trace (Agentic Runtime A1).
-- One agent_runs row per user question (spanning its clarification rounds);
-- agent_steps mirrors the in-memory RunRecorder timeline so a thread's reasoning
-- is durable, retrievable, and debuggable independent of the conversation blob.

CREATE TABLE IF NOT EXISTS agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- conversation_id is TEXT to match ai_conversations.id (TEXT); nullable for
    -- ad-hoc runs with no persisted conversation.
    conversation_id TEXT REFERENCES ai_conversations(id) ON DELETE CASCADE,
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    model_id UUID REFERENCES semantic_models(id) ON DELETE SET NULL,
    user_id TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL DEFAULT '',
    question_hash TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT 'interactive',
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'waiting_clarification', 'completed', 'failed')),
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    answer TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation_created
    ON agent_runs(conversation_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_runs_datasource_created
    ON agent_runs(datasource_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    seq INT NOT NULL,
    kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ok'
        CHECK (status IN ('ok', 'failed', 'skipped')),
    attempt INT NOT NULL DEFAULT 0,
    duration_ms INT NOT NULL DEFAULT 0,
    detail TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_steps_run_seq
    ON agent_steps(run_id, seq);
