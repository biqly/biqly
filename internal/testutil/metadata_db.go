package testutil

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver registration
)

//nolint:gosec // test-only default DSN for local development
const defaultMetadataDBDSN = "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"

func OpenMetadataDB(t testing.TB) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_METADATA_DB_DSN")
	if dsn == "" {
		dsn = defaultMetadataDBDSN
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("skipping database tests; DB not available: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close database: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping database tests; ping failed: %v", err)
	}
	return db
}
