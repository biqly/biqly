package metadata

import (
	"context"
	"encoding/json"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/lib/pq"
)

// ListSecurityPolicies returns all security policy records.
func (r *Repository) ListSecurityPolicies(ctx context.Context) ([]SecurityPolicy, error) {
	return platformdb.QuerySliceErr(ctx, r.db, "list security policies", `
		SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, created_at, updated_at
		FROM permissions
		ORDER BY user_id, datasource_id
	`, nil, scanSecurityPolicy)
}

// GetSecurityPolicy retrieves a security policy by ID.
func (r *Repository) GetSecurityPolicy(ctx context.Context, id string) (*SecurityPolicy, error) {
	query := `
		SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, created_at, updated_at
		FROM permissions
		WHERE id = $1::uuid
	`
	row := r.db.QueryRowContext(ctx, query, id)
	policy, err := scanSecurityPolicy(row)
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// GetSecurityPolicyByKeys retrieves a security policy by user_id and datasource_id.
func (r *Repository) GetSecurityPolicyByKeys(ctx context.Context, userID string, datasourceID string) (*SecurityPolicy, error) {
	query := `
		SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, created_at, updated_at
		FROM permissions
		WHERE user_id = $1 AND datasource_id = $2::uuid
	`
	row := r.db.QueryRowContext(ctx, query, userID, datasourceID)
	policy, err := scanSecurityPolicy(row)
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// DeleteSecurityPolicy deletes a security policy record by ID.
func (r *Repository) DeleteSecurityPolicy(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM permissions WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete security policy: %w", err)
	}
	return nil
}

// DeleteSecurityPolicyByKeys deletes a security policy record by user_id and datasource_id.
func (r *Repository) DeleteSecurityPolicyByKeys(ctx context.Context, userID string, datasourceID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM permissions WHERE user_id = $1 AND datasource_id = $2::uuid`, userID, datasourceID)
	if err != nil {
		return fmt.Errorf("delete security policy by keys: %w", err)
	}
	return nil
}

// UpsertSecurityPolicy inserts or updates a security policy.
func (r *Repository) UpsertSecurityPolicy(ctx context.Context, policy *SecurityPolicy) error {
	rowFiltersJSON, err := json.Marshal(policy.RowFilters)
	if err != nil {
		return fmt.Errorf("marshal row filters: %w", err)
	}

	query := `
		INSERT INTO permissions (
			id, user_id, datasource_id, allowed_models, denied_fields, row_filters, created_at, updated_at
		)
		VALUES ($1::uuid, $2, $3::uuid, $4, $5, $6::jsonb, now(), now())
		ON CONFLICT (user_id, datasource_id) DO UPDATE SET
			allowed_models = EXCLUDED.allowed_models,
			denied_fields = EXCLUDED.denied_fields,
			row_filters = EXCLUDED.row_filters,
			updated_at = now()
	`
	_, err = r.db.ExecContext(ctx, query,
		policy.ID,
		policy.UserID,
		policy.DatasourceID,
		pq.StringArray(policy.AllowedModels),
		pq.StringArray(policy.DeniedFields),
		rowFiltersJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert security policy: %w", err)
	}
	return nil
}

func scanSecurityPolicy(s platformdb.Scanner) (SecurityPolicy, error) {
	var (
		policy     SecurityPolicy
		allowed    pq.StringArray
		denied     pq.StringArray
		rowFilters []byte
	)
	if err := s.Scan(
		&policy.ID,
		&policy.UserID,
		&policy.DatasourceID,
		&allowed,
		&denied,
		&rowFilters,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return policy, fmt.Errorf("scan security policy: %w", err)
	}
	policy.AllowedModels = []string(allowed)
	policy.DeniedFields = []string(denied)
	if len(rowFilters) > 0 {
		if err := json.Unmarshal(rowFilters, &policy.RowFilters); err != nil {
			return policy, fmt.Errorf("row filters: %w", err)
		}
	}
	return policy, nil
}
