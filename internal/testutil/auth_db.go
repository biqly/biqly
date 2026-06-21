// Package testutil provides shared PostgreSQL helpers for integration tests,
// including opening auth and metadata databases and running test-only SQL setup.
package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

//nolint:gosec // test-only default DSN for local development
const defaultAuthDBDSN = "postgres://bi_user:bi_password@localhost:5432/bi_auth?sslmode=disable"

const authTestAdvisoryLockKey int64 = 0x6269716c79417574 // biqly_Auth test lock

func OpenAuthDB(t testing.TB) *sql.DB {
	t.Helper()
	db := openPingDB(t, "BI_AUTH_DB_DSN", defaultAuthDBDSN)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, authTestAdvisoryLockKey); err != nil {
		t.Fatalf("acquire auth test advisory lock: %v", err)
	}
	t.Cleanup(func() {
		releaseAdvisoryLock(t, db, authTestAdvisoryLockKey)
	})
	return db
}

func releaseAdvisoryLock(t testing.TB, db *sql.DB, key int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return
	}
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, key); err != nil {
		t.Errorf("release advisory lock %d: %v", key, err)
	}
}

func ExecAuthSQL(ctx context.Context, t testing.TB, db *sql.DB, statements ...string) {
	t.Helper()
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec auth test sql %q: %v", stmt, err)
		}
	}
}

func purgeAuthAuditLogForTests(ctx context.Context, t testing.TB, db *sql.DB) {
	t.Helper()
	ExecAuthSQL(ctx, t, db,
		"ALTER TABLE audit_log DISABLE TRIGGER audit_log_no_delete",
		"DELETE FROM audit_log",
		"ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_delete",
	)
}

func ResetAuthUserTables(ctx context.Context, t testing.TB, db *sql.DB) {
	t.Helper()
	ExecAuthSQL(ctx, t, db,
		"DELETE FROM email_verification_tokens",
		"DELETE FROM password_reset_tokens",
		"DELETE FROM sessions",
		"DELETE FROM resource_shares",
		"DELETE FROM workspace_members",
		"DELETE FROM workspaces",
		"DELETE FROM user_roles",
	)
	purgeAuthAuditLogForTests(ctx, t, db)
	ExecAuthSQL(ctx, t, db, "DELETE FROM users")
}

func PurgeAuthUserByID(ctx context.Context, t testing.TB, db *sql.DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		stmts := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM user_mfa WHERE user_id = $1`, []any{id}},
			{`DELETE FROM sessions WHERE user_id = $1`, []any{id}},
			{`DELETE FROM resource_shares WHERE owner_id = $1 OR shared_with = $1`, []any{id}},
			{`DELETE FROM workspace_members WHERE user_id = $1`, []any{id}},
			{`DELETE FROM workspaces WHERE created_by = $1`, []any{id}},
			{`DELETE FROM user_roles WHERE user_id = $1`, []any{id}},
			{`DELETE FROM oauth_accounts WHERE user_id = $1`, []any{id}},
			{`DELETE FROM users WHERE id = $1`, []any{id}},
		}
		for _, stmt := range stmts {
			if _, err := db.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
				t.Fatalf("purge auth user id %q: exec %q: %v", id, stmt.query, err)
			}
		}
	}
}

func PurgeAuthUsersByEmail(ctx context.Context, t testing.TB, db *sql.DB, emails ...string) {
	t.Helper()
	for _, email := range emails {
		stmts := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, []any{email}},
			{`DELETE FROM resource_shares WHERE owner_id IN (SELECT id FROM users WHERE email = $1) OR shared_with IN (SELECT id FROM users WHERE email = $1)`, []any{email}},
			{`DELETE FROM workspace_members WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, []any{email}},
			{`DELETE FROM workspaces WHERE created_by IN (SELECT id FROM users WHERE email = $1)`, []any{email}},
			{`DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, []any{email}},
			{`DELETE FROM oauth_accounts WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, []any{email}},
			{`DELETE FROM users WHERE email = $1`, []any{email}},
		}
		for _, stmt := range stmts {
			if _, err := db.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
				t.Fatalf("purge auth user %q: exec %q: %v", email, stmt.query, err)
			}
		}
	}
}

func ResetAuthIntegrationTables(ctx context.Context, t testing.TB, db *sql.DB, extra ...string) {
	t.Helper()
	stmts := make([]string, 0, len(extra)+5)
	stmts = append(stmts, extra...)
	stmts = append(stmts,
		"DELETE FROM sessions",
		"DELETE FROM resource_shares",
		"DELETE FROM workspace_members",
		"DELETE FROM workspaces",
		"DELETE FROM user_roles",
	)
	ExecAuthSQL(ctx, t, db, stmts...)
	purgeAuthAuditLogForTests(ctx, t, db)
	ExecAuthSQL(ctx, t, db, "DELETE FROM users")
}
