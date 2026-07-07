package main

import (
	"strings"
	"testing"
)

func TestConversationIdempotencyMigrationFiles(t *testing.T) {
	up := readMigrationForTest(t, "migrations/063a_conversation_idempotency.up.sql")
	down := readMigrationForTest(t, "migrations/063a_conversation_idempotency.down.sql")

	upMustContain := []string{
		"ALTER TABLE ai_conversations",
		"ADD COLUMN snapshot_version BIGINT NOT NULL DEFAULT 0",
		"ALTER TABLE ai_conversation_messages",
		"ADD COLUMN remote_id TEXT",
		"ADD COLUMN ordinal INTEGER",
		"ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now()",
		"ADD COLUMN deleted_at TIMESTAMPTZ",
		"ADD COLUMN deleted_by_repair_run_id UUID",
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_ai_conversation_messages_remote",
		"ON ai_conversation_messages(conversation_id, remote_id)",
		"WHERE remote_id IS NOT NULL",
		"CREATE INDEX IF NOT EXISTS idx_ai_conversation_messages_active_order",
		"CREATE TABLE IF NOT EXISTS conversation_write_requests",
		"idempotency_key TEXT PRIMARY KEY",
		"user_id TEXT NOT NULL",
		"conversation_id TEXT NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE",
		"payload_hash TEXT NOT NULL",
		"CHECK (status IN ('processing', 'completed', 'failed'))",
		"response_status INTEGER",
		"response_body JSONB",
		"completed_at TIMESTAMPTZ",
	}
	for _, want := range upMustContain {
		if !strings.Contains(up, want) {
			t.Errorf("up migration missing %q", want)
		}
	}

	downMustContain := []string{
		"DROP TABLE IF EXISTS conversation_write_requests",
		"DROP INDEX IF EXISTS idx_ai_conversation_messages_active_order",
		"DROP INDEX IF EXISTS ux_ai_conversation_messages_remote",
		"ai_conversation_messages",
		"ai_conversations",
	}
	for _, want := range downMustContain {
		if !strings.Contains(down, want) {
			t.Errorf("down migration missing %q", want)
		}
	}
}
