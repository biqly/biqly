package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:gosec // test-only default DSN for local development
const defaultMetadataDBDSN = "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"

const metadataTestAdvisoryLockKey int64 = 0x6269716c794d6574 // biqly_Met test lock

func OpenMetadataDB(t testing.TB) *sql.DB {
	t.Helper()
	db := openPingDB(t, "BI_METADATA_DB_DSN", defaultMetadataDBDSN)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, metadataTestAdvisoryLockKey); err != nil {
		t.Fatalf("acquire metadata test advisory lock: %v", err)
	}
	t.Cleanup(func() {
		releaseAdvisoryLock(t, db, metadataTestAdvisoryLockKey)
	})
	return db
}

// EnsureMetadataTestDatasource inserts a minimal datasource row for FK constraints.
func EnsureMetadataTestDatasource(ctx context.Context, t testing.TB, db *sql.DB, id, name string) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO datasources (id, name, type, dsn_encrypted)
		VALUES ($1::uuid, $2, 'postgres', 'test')
		ON CONFLICT (id) DO NOTHING
	`, id, name)
	require.NoError(t, err)
}

// EnsureMetadataTestSemanticModel inserts a minimal semantic model row for FK constraints.
func EnsureMetadataTestSemanticModel(ctx context.Context, t testing.TB, db *sql.DB, id, datasourceID, name string) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO semantic_models (id, datasource_id, name, base_schema, base_table)
		VALUES ($1::uuid, $2::uuid, $3, 'public', 't')
		ON CONFLICT (id) DO NOTHING
	`, id, datasourceID, name)
	require.NoError(t, err)
}
