package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security/pii"
)

// PIIPolicyStore loads PII-annotated columns and per-user permission
// policies from metadata storage. *metadata.Repository satisfies it.
type PIIPolicyStore interface {
	ListPIIColumns(ctx context.Context, datasourceID string) ([]metadata.Column, error)
	GetSecurityPolicyByKeys(ctx context.Context, userID, datasourceID string) (*metadata.SecurityPolicy, error)
}

// IdentityResolver returns the calling user's ID and roles from the request
// context. Wired by composition roots to the HTTP auth middleware accessors.
type IdentityResolver func(ctx context.Context) (userID string, roles []string)

// PIIPolicyService resolves the PII masking config the compiler applies for
// the calling user.
type PIIPolicyService struct {
	store    PIIPolicyStore
	identity IdentityResolver
	audit    *audit.Logger
}

// NewPIIPolicyService creates a PIIPolicyService.
func NewPIIPolicyService(store PIIPolicyStore, identity IdentityResolver) *PIIPolicyService {
	return &PIIPolicyService{store: store, identity: identity}
}

// WithAudit enables pii.masking_applied audit events on query compilation.
func (s *PIIPolicyService) WithAudit(logger *audit.Logger) *PIIPolicyService {
	s.audit = logger
	return s
}

// MaskingConfig builds the per-user PII masking config for a datasource.
// Returns nil (no masking) when the caller is unauthenticated (auth disabled
// or internal call) or when the datasource has no PII-annotated columns.
// Storage errors are returned so the query fails closed rather than running
// unmasked.
func (s *PIIPolicyService) MaskingConfig(ctx context.Context, datasourceID string) (*query.PIIMaskingConfig, error) {
	if s == nil || s.store == nil || s.identity == nil || datasourceID == "" {
		return nil, nil //nolint:nilnil // optional result
	}
	userID, roles := s.identity(ctx)
	if userID == "" {
		return nil, nil //nolint:nilnil // optional result
	}

	cols, err := s.store.ListPIIColumns(ctx, datasourceID)
	if err != nil {
		return nil, fmt.Errorf("load pii columns: %w", err)
	}
	if len(cols) == 0 {
		return nil, nil //nolint:nilnil // optional result
	}

	overrides := map[string]string{}
	policy, err := s.store.GetSecurityPolicyByKeys(ctx, userID, datasourceID)
	switch {
	case err == nil && policy != nil:
		for col, entry := range policy.PIIPolicy {
			overrides[col] = entry.Access
		}
	case errors.Is(err, sql.ErrNoRows):
		// No explicit policy: role defaults apply.
	case err != nil:
		return nil, fmt.Errorf("load permission policy: %w", err)
	}

	access, types := pii.BuildColumnAccessMaps(pii.PrimaryRole(roles), cols, overrides)
	strategies := pii.BuildColumnMaskingStrategyMaps(cols)
	info := make(map[string]query.PIIColumnInfo, len(access))
	for ref, level := range access {
		info[ref] = query.PIIColumnInfo{
			Access:   level,
			PIIType:  types[ref],
			Strategy: strategies[ref],
		}
	}

	if s.audit != nil {
		masked := make([]string, 0, len(cols))
		for i := range cols {
			col := &cols[i]
			qualified := col.SchemaName + "." + col.TableName + "." + col.ColumnName
			if level := access[qualified]; level == pii.AccessMasked || level == pii.AccessHidden {
				masked = append(masked, qualified+":"+level)
			}
		}
		if len(masked) > 0 {
			s.audit.Log(ctx, audit.Event{
				UserID:       userID,
				EventType:    audit.EventPIIMaskingApplied,
				DatasourceID: datasourceID,
				Details:      map[string]any{"columns": masked},
			})
		}
	}

	return &query.PIIMaskingConfig{
		ColumnInfo:       info,
		ColumnAccess:     access,
		ColumnTypes:      types,
		ColumnStrategies: strategies,
	}, nil
}
