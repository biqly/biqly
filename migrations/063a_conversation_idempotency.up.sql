-- Conversation idempotency: stable client identity, versioned snapshots, and a
-- write-request ledger so replayed POSTs never create duplicate message rows.

ALTER TABLE ai_conversations
    ADD COLUMN snapshot_version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE ai_conversation_messages
    ADD COLUMN remote_id TEXT,
    ADD COLUMN ordinal INTEGER,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN deleted_at TIMESTAMPTZ,
    ADD COLUMN deleted_by_repair_run_id UUID;

-- One active message per client remote_id within a conversation.
CREATE UNIQUE INDEX IF NOT EXISTS ux_ai_conversation_messages_remote
    ON ai_conversation_messages(conversation_id, remote_id)
    WHERE remote_id IS NOT NULL;

-- Efficient active-row ordering for conversation replay reads.
CREATE INDEX IF NOT EXISTS idx_ai_conversation_messages_active_order
    ON ai_conversation_messages(conversation_id, ordinal, created_at)
    WHERE deleted_at IS NULL;

-- Idempotent write ledger: a retried request with the same key and payload
-- returns the stored response; a different payload for the same key is a conflict.
CREATE TABLE IF NOT EXISTS conversation_write_requests (
    idempotency_key TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    payload_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('processing', 'completed', 'failed')),
    response_status INTEGER,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);
