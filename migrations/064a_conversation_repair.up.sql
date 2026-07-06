-- Conversation repair: detect and reversible remove replay-chain duplicates.

CREATE TABLE IF NOT EXISTS conversation_repair_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mode TEXT NOT NULL CHECK (mode IN ('detect', 'apply', 'restore', 'purge')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'completed', 'failed')),
    candidate_count INTEGER NOT NULL DEFAULT 0,
    repaired_count INTEGER NOT NULL DEFAULT 0,
    skipped_count INTEGER NOT NULL DEFAULT 0,
    canonical_hash TEXT NOT NULL DEFAULT '',
    report_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS conversation_message_repair_archive (
    repair_run_id UUID NOT NULL REFERENCES conversation_repair_runs(id) ON DELETE CASCADE,
    original_message_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    remote_id TEXT,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    ordinal INTEGER,
    created_at TIMESTAMPTZ NOT NULL,
    full_row_json JSONB NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (repair_run_id, original_message_id)
);

-- Deferred FK: message soft-delete links back to the repair run that caused it.
-- Added separately so the constraint can be added after the referenced table exists.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_ai_conv_msg_repair_run'
          AND table_name = 'ai_conversation_messages'
    ) THEN
        ALTER TABLE ai_conversation_messages
            ADD CONSTRAINT fk_ai_conv_msg_repair_run
            FOREIGN KEY (deleted_by_repair_run_id)
            REFERENCES conversation_repair_runs(id) ON DELETE SET NULL;
    END IF;
END $$;
