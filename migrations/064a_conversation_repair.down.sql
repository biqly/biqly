-- Reverse conversation repair migration.

ALTER TABLE ai_conversation_messages
    DROP CONSTRAINT IF EXISTS fk_ai_conv_msg_repair_run;

DROP TABLE IF EXISTS conversation_message_repair_archive;
DROP TABLE IF EXISTS conversation_repair_runs;
