package handlers

import (
	"context"
	"database/sql"
	"testing"

	"github.com/biqly/biqly/internal/core"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMaskingTestTables creates the metadata tables the PII policy resolver
// reads, mirroring the migration schema (datasources, schemas, tables, columns,
// permissions) so the test runs against a real Postgres regardless of whether
// migrations have been applied to the test database.
func createMaskingTestTables(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS datasources (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			dsn_encrypted TEXT NOT NULL,
			config JSONB DEFAULT '{}',
			is_active BOOLEAN NOT NULL DEFAULT true,
			last_sync_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS schemas (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
			schema_name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(datasource_id, schema_name)
		)`,
		`CREATE TABLE IF NOT EXISTS tables (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
			schema_id UUID NOT NULL REFERENCES schemas(id) ON DELETE CASCADE,
			schema_name TEXT NOT NULL,
			table_name TEXT NOT NULL,
			table_type TEXT NOT NULL DEFAULT 'BASE TABLE',
			row_estimate BIGINT,
			description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(datasource_id, schema_name, table_name)
		)`,
		`CREATE TABLE IF NOT EXISTS columns (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
			table_id UUID NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
			schema_name TEXT NOT NULL,
			table_name TEXT NOT NULL,
			column_name TEXT NOT NULL,
			data_type TEXT NOT NULL,
			nullable BOOLEAN NOT NULL DEFAULT true,
			ordinal_position INT,
			character_maximum_length INT,
			numeric_precision INT,
			numeric_scale INT,
			column_default TEXT,
			description TEXT,
			is_primary_key BOOLEAN NOT NULL DEFAULT false,
			is_foreign_key BOOLEAN NOT NULL DEFAULT false,
			referenced_schema TEXT,
			referenced_table TEXT,
			referenced_column TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			pii_type TEXT,
			pii_confidence DOUBLE PRECISION,
			pii_detected_at TIMESTAMPTZ,
			pii_reviewed_by TEXT,
			pii_masking_strategy TEXT,
			UNIQUE(datasource_id, schema_name, table_name, column_name)
		)`,
		`CREATE TABLE IF NOT EXISTS permissions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id TEXT NOT NULL,
			datasource_id UUID REFERENCES datasources(id),
			allowed_models TEXT[] DEFAULT '{}',
			denied_fields TEXT[] DEFAULT '{}',
			row_filters JSONB DEFAULT '[]',
			pii_policy JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(user_id, datasource_id)
		)`,
	}
	for _, s := range stmts {
		_, err := db.ExecContext(ctx, s)
		require.NoError(t, err)
	}
}

// TestPublicWidgetQuery_AnonymousExecutionAppliesCreatorPolicy is the
// regression proof for the public-sharing PII/RLS leak. It exercises the REAL
// core.PIIPolicyService (wired to real Postgres exactly as production wires it
// via jwtIdentity) and proves the mechanism the anonymous-path fix depends on:
//
//   - With NO identity in context (the pre-fix anonymous path), QueryPolicy
//     returns no masking config and no row filters — the query would run fully
//     unmasked and unfiltered. This is the vulnerability.
//   - With the share creator's user ID injected (what the fix does via
//     bimw.WithUserID), QueryPolicy resolves the creator's masking config AND
//     row-level-security filters — so the anonymous viewer sees exactly the
//     masked/filtered data the creator would, never the raw data.
func TestPublicWidgetQuery_AnonymousExecutionAppliesCreatorPolicy(t *testing.T) {
	ctx := context.Background()
	db := testutil.OpenMetadataDB(t)
	createMaskingTestTables(ctx, t, db)

	const (
		dsID     = "aaaaaaaa-0000-0000-0000-000000000001"
		schemaID = "aaaaaaaa-0000-0000-0000-000000000002"
		tableID  = "aaaaaaaa-0000-0000-0000-000000000003"
		creator  = "aaaaaaaa-0000-0000-0000-00000000000c"
	)

	// Clean any leftovers from a previous run (advisory-locked, but be defensive).
	_, err := db.ExecContext(ctx, `DELETE FROM datasources WHERE id = $1::uuid`, dsID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO datasources (id, name, type, dsn_encrypted)
		VALUES ($1::uuid, $2, 'postgres', 'test')
	`, dsID, "masking-integration-ds")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO schemas (id, datasource_id, schema_name)
		VALUES ($1::uuid, $2::uuid, 'public')
	`, schemaID, dsID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO tables (id, datasource_id, schema_id, schema_name, table_name)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'public', 'customers')
	`, tableID, dsID, schemaID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO columns (datasource_id, table_id, schema_name, table_name, column_name, data_type, pii_type, pii_masking_strategy)
		VALUES ($1::uuid, $2::uuid, 'public', 'customers', 'email', 'text', 'email', 'partial')
	`, dsID, tableID)
	require.NoError(t, err)

	// The creator's security policy: an explicit row-level-security filter that
	// must be re-applied for anonymous viewers of the shared dashboard.
	repo := metadata.NewRepository(db)
	require.NoError(t, repo.UpsertSecurityPolicy(ctx, &metadata.SecurityPolicy{
		ID:           "aaaaaaaa-0000-0000-0000-00000000000d",
		UserID:       creator,
		DatasourceID: dsID,
		RowFilters: []metadata.PermissionRowFilter{
			{Field: "region", Operator: "eq", Value: "EU"},
		},
	}))

	// Identity resolver wired exactly like production (app.jwtIdentity).
	identity := func(ctx context.Context) (string, []string) {
		return bimw.UserID(ctx), bimw.UserRoles(ctx)
	}
	svc := core.NewPIIPolicyService(repo, identity)

	t.Run("no identity (pre-fix anonymous path) resolves NO masking and NO row filters", func(t *testing.T) {
		cfg, filters, err := svc.QueryPolicy(ctx, dsID)
		require.NoError(t, err)
		assert.Nil(t, cfg, "unauthenticated query must resolve no masking config — this is the leak the fix closes")
		assert.Empty(t, filters, "unauthenticated query must resolve no row filters — RLS is bypassed")
	})

	t.Run("creator identity injected (the fix) re-applies masking AND row-level security", func(t *testing.T) {
		creatorCtx := bimw.WithUserID(ctx, creator)
		cfg, filters, err := svc.QueryPolicy(creatorCtx, dsID)
		require.NoError(t, err)

		// PII masking: the email column resolves to masked (role default for an
		// empty/unknown role is the most restrictive — fail-closed, never raw).
		require.NotNil(t, cfg, "creator identity must resolve a PII masking config")
		assert.Equal(t, pii.AccessMasked, cfg.ColumnAccess["customers.email"],
			"PII column must be masked, not exposed raw, for the anonymous viewer")
		assert.Equal(t, pii.AccessMasked, cfg.ColumnAccess["public.customers.email"])

		// Row-level security: the creator's WHERE filter must be re-applied.
		require.Len(t, filters, 1, "creator's row-level-security filter must re-apply")
		assert.Equal(t, "region", filters[0].Field)
		assert.Equal(t, "eq", filters[0].Operator)
		assert.Equal(t, "EU", filters[0].Value)
	})
}
