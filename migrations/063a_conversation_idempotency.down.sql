-- Reverse conversation idempotency migration.

DROP TABLE IF EXISTS conversation_write_requests;
DROP INDEX IF EXISTS idx_ai_conversation_messages_active_order;
DROP INDEX IF EXISTS ux_ai_conversation_messages_remote;

ALTER TABLE ai_conversation_messages
    DROP COLUMN IF EXISTS deleted_by_repair_run_id,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS ordinal,
    DROP COLUMN IF EXISTS remote_id;

ALTER TABLE ai_conversations
    DROP COLUMN IF EXISTS snapshot_version;
