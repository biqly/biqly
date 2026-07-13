package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// datasourceForEntity resolves an entity id to its owning datasource id, the
// shared implementation behind the DatasourceFor* resolvers that feed the
// RequireResolvedDatasourceAccess IDOR middleware. Keeping one implementation
// avoids the copies drifting (e.g. one forgetting the ::uuid cast or the
// ErrNoRows→not-found mapping), which would weaken access control.
//
// table and label MUST be trusted constants supplied by the caller — never user
// input; id is always bound as a parameter.
func (r *Repository) datasourceForEntity(ctx context.Context, table, label, id string, notFound error) (string, error) {
	//nolint:gosec // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query -- table is a hardcoded constant, id is parameterized
	query := fmt.Sprintf(`SELECT datasource_id::text FROM %s WHERE id = $1::uuid`, table)
	var datasourceID string
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&datasourceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%s %s: %w", label, id, notFound)
		}
		return "", fmt.Errorf("datasource for %s: %w", label, err)
	}
	return datasourceID, nil
}
