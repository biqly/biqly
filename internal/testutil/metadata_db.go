package testutil

import (
	"database/sql"
	"testing"
)

//nolint:gosec // test-only default DSN for local development
const defaultMetadataDBDSN = "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"

func OpenMetadataDB(t testing.TB) *sql.DB {
	t.Helper()
	return openPingDB(t, "BI_METADATA_DB_DSN", defaultMetadataDBDSN)
}
