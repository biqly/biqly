package metadata

import (
	"context"
	"fmt"

	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// SyncSnapshotKeys are the natural keys present in the CURRENT introspection
// of a datasource. Metadata sync upserts everything it sees; PruneStaleMetadata
// then deletes catalog rows that are no longer in this snapshot (tables dropped
// at the source, removed columns, dropped FK constraints) so the catalog never
// serves ghost objects — e.g. AI describe sampling a relation that 42P01s.
type SyncSnapshotKeys struct {
	// SchemaNames as-is; Tables as "schema.table"; Columns as
	// "schema.table.column"; RelationConstraints as constraint names (the
	// relations upsert key is (datasource_id, constraint_name)).
	SchemaNames         []string
	TableKeys           []string
	ColumnKeys          []string
	RelationConstraints []string
}

// PruneResult reports how many stale rows each catalog table lost.
type PruneResult struct {
	Relations int
	Columns   int
	Tables    int
	Schemas   int
}

// Total returns the total number of pruned rows.
func (p PruneResult) Total() int {
	return p.Relations + p.Columns + p.Tables + p.Schemas
}

// PruneStaleMetadata deletes catalog rows for objects missing from the current
// introspection snapshot. Deleting a table cascades its columns; deleting a
// schema cascades its tables — the explicit column/table passes additionally
// catch dropped columns on surviving tables and dropped tables in surviving
// schemas.
func (r *Repository) PruneStaleMetadata(ctx context.Context, datasourceID string, keep SyncSnapshotKeys) (PruneResult, error) {
	var result PruneResult
	steps := []struct {
		name  string
		query string
		keys  []string
		count *int
	}{
		{
			name:  "relations",
			query: `DELETE FROM relations WHERE datasource_id = $1::uuid AND NOT (constraint_name = ANY($2))`,
			keys:  keep.RelationConstraints,
			count: &result.Relations,
		},
		{
			name:  "columns",
			query: `DELETE FROM columns WHERE datasource_id = $1::uuid AND NOT (schema_name || '.' || table_name || '.' || column_name = ANY($2))`,
			keys:  keep.ColumnKeys,
			count: &result.Columns,
		},
		{
			name:  "tables",
			query: `DELETE FROM tables WHERE datasource_id = $1::uuid AND NOT (schema_name || '.' || table_name = ANY($2))`,
			keys:  keep.TableKeys,
			count: &result.Tables,
		},
		{
			name:  "schemas",
			query: `DELETE FROM schemas WHERE datasource_id = $1::uuid AND NOT (schema_name = ANY($2))`,
			keys:  keep.SchemaNames,
			count: &result.Schemas,
		},
	}
	for _, step := range steps {
		keys := step.keys
		if keys == nil {
			keys = []string{}
		}
		res, err := r.db.ExecContext(ctx, step.query, datasourceID, pgarray.Strings(keys))
		if err != nil {
			return result, fmt.Errorf("prune stale %s: %w", step.name, err)
		}
		if affected, err := res.RowsAffected(); err == nil {
			*step.count = int(affected)
		}
	}
	return result, nil
}
