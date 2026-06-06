package testutil

import (
	"context"
	"database/sql"
	"testing"
)

//nolint:gosec // test-only default DSN for local development
const defaultAuthDBDSN = "postgres://bi_user:bi_password@localhost:5432/bi_auth?sslmode=disable"

func OpenAuthDB(t testing.TB) *sql.DB {
	t.Helper()
	return openPingDB(t, "BI_AUTH_DB_DSN", defaultAuthDBDSN)
}

func ExecAuthSQL(ctx context.Context, t testing.TB, db *sql.DB, statements ...string) {
	t.Helper()
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec auth test sql %q: %v", stmt, err)
		}
	}
}

func ResetAuthUserTables(ctx context.Context, t testing.TB, db *sql.DB) {
	t.Helper()
	ExecAuthSQL(ctx, t, db,
		"DELETE FROM sessions",
		"DELETE FROM workspace_members",
		"DELETE FROM workspaces",
		"DELETE FROM user_roles",
		"DELETE FROM users",
	)
}

func ResetAuthIntegrationTables(ctx context.Context, t testing.TB, db *sql.DB, extra ...string) {
	t.Helper()
	stmts := make([]string, 0, len(extra)+5)
	stmts = append(stmts, extra...)
	stmts = append(stmts,
		"DELETE FROM sessions",
		"DELETE FROM workspace_members",
		"DELETE FROM workspaces",
		"DELETE FROM user_roles",
		"DELETE FROM users",
	)
	ExecAuthSQL(ctx, t, db, stmts...)
}
