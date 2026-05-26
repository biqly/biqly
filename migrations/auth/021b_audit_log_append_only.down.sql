DROP TRIGGER IF EXISTS audit_log_no_update ON audit_log;
DROP TRIGGER IF EXISTS audit_log_no_delete ON audit_log;
DROP FUNCTION IF EXISTS audit_log_block_mutations();
DROP INDEX IF EXISTS audit_log_user_action_idx;
DROP INDEX IF EXISTS audit_log_created_at_idx;
