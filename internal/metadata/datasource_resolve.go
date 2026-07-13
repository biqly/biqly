package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// datasourceForEntity runs a caller-supplied static query that selects a single
// datasource_id::text for an entity id, the shared implementation behind the
// DatasourceFor* resolvers that feed the RequireResolvedDatasourceAccess IDOR
// middleware. Keeping one implementation stops the copies drifting (e.g. one
// forgetting the ::uuid cast or the ErrNoRows→not-found mapping), which would
// weaken access control. query MUST be a static constant (never built from
// input); id is always bound as a parameter.
func (r *Repository) datasourceForEntity(ctx context.Context, query, label, id string, notFound error) (string, error) {
	var datasourceID string
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&datasourceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%s %s: %w", label, id, notFound)
		}
		return "", fmt.Errorf("datasource for %s: %w", label, err)
	}
	return datasourceID, nil
}
