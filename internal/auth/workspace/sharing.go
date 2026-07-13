package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SharePermission string

var (
	ErrShareNotFound = errors.New("share not found")
	// ErrResourceNotFound is returned by a ResourceOwnershipResolver when the
	// resource being shared does not exist.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrResourceTypeUnsupported is returned by a ResourceOwnershipResolver for
	// a resource_type whose ownership cannot be verified. Share fails closed on
	// it rather than persisting an unverifiable grant.
	ErrResourceTypeUnsupported = errors.New("resource type not supported for sharing")
	// ErrResourceAccessDenied is returned when the caller lacks access to the
	// resource they are trying to share.
	ErrResourceAccessDenied = errors.New("caller does not have access to the resource")
)

// ResourceOwnershipResolver resolves a shareable resource to the datasource that
// governs access to it. The shareable resources (semantic models, AI query
// history, …) live in another service's database, so this is implemented as an
// HTTP call out to that service and stubbed in tests. Implementations must
// return ErrResourceNotFound / ErrResourceTypeUnsupported for the respective
// cases so Share can map them to the right status.
type ResourceOwnershipResolver interface {
	ResolveDatasource(ctx context.Context, resourceType, resourceID string) (string, error)
}

// DatasourceAccessChecker verifies a user's access to a datasource at a given
// level ("read"/"write"). The auth service's rbac.DatasourceAccessService
// satisfies this; a returned error means access is denied or could not be
// verified.
type DatasourceAccessChecker interface {
	CheckAccess(ctx context.Context, userID, datasourceID, level string) error
}

// datasourceLevelForPermission maps a share permission to the datasource access
// level the caller must already hold to be allowed to grant it: read is enough
// to share a view/execute grant, write is required to share an edit grant.
func datasourceLevelForPermission(permission string) string {
	if strings.EqualFold(permission, "edit") {
		return "write"
	}
	return "read"
}

type ResourceShare struct {
	ID           string          `json:"id"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	OwnerID      string          `json:"owner_id"`
	SharedWith   *string         `json:"shared_with,omitempty"`
	WorkspaceID  *string         `json:"workspace_id,omitempty"`
	Permission   SharePermission `json:"permission"`
	CreatedAt    time.Time       `json:"created_at"`
}

type ShareRequest struct {
	ResourceType string  `json:"resource_type"`
	ResourceID   string  `json:"resource_id"`
	SharedWith   *string `json:"shared_with,omitempty"`
	WorkspaceID  *string `json:"workspace_id,omitempty"`
	Permission   string  `json:"permission"`
}

type SharingService struct {
	db       *sql.DB
	ws       *Service
	resolver ResourceOwnershipResolver
	access   DatasourceAccessChecker
}

// NewSharingService builds the sharing service. resolver+access enforce the
// ownership guard on Share: the caller may only share a resource they can
// access. Both may be nil (e.g. local dev without the resource-owning service
// reachable), in which case the ownership guard is skipped — production wiring
// must supply them so Share fails closed for resources the caller cannot access.
func NewSharingService(db *sql.DB, ws *Service, resolver ResourceOwnershipResolver, access DatasourceAccessChecker) *SharingService {
	return &SharingService{db: db, ws: ws, resolver: resolver, access: access}
}

func (s *SharingService) Share(ctx context.Context, callerID string, req ShareRequest) (*ResourceShare, error) {
	if req.ResourceType == "" || req.ResourceID == "" {
		return nil, errors.New("resource_type and resource_id are required")
	}
	if req.SharedWith == nil && req.WorkspaceID == nil {
		return nil, errors.New("either shared_with or workspace_id must be provided")
	}
	if !isValidPermission(req.Permission) {
		return nil, fmt.Errorf("invalid permission: %s", req.Permission)
	}

	// Ownership guard: the caller may only create a share for a resource they can
	// access. The resource lives in another service, so resolve it to its
	// governing datasource and verify the caller holds access at the level this
	// share grants. Fails closed on unresolvable/unsupported resources. Skipped
	// only when no resolver is wired (local dev) — see NewSharingService.
	if s.resolver != nil && s.access != nil {
		datasourceID, err := s.resolver.ResolveDatasource(ctx, req.ResourceType, req.ResourceID)
		if err != nil {
			return nil, fmt.Errorf("resolve resource for sharing: %w", err)
		}
		if err := s.access.CheckAccess(ctx, callerID, datasourceID, datasourceLevelForPermission(req.Permission)); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrResourceAccessDenied, err)
		}
	}

	// IDOR guard: caller must be a member of the target workspace
	if req.WorkspaceID != nil {
		isMember, err := s.ws.IsMember(ctx, *req.WorkspaceID, callerID)
		if err != nil {
			return nil, fmt.Errorf("verify workspace membership: %w", err)
		}
		if !isMember {
			return nil, errors.New("caller is not a member of the target workspace")
		}
	}

	var sharedWith, workspaceID sql.NullString
	if req.SharedWith != nil {
		sharedWith = sql.NullString{String: *req.SharedWith, Valid: true}
	}
	if req.WorkspaceID != nil {
		workspaceID = sql.NullString{String: *req.WorkspaceID, Valid: true}
	}

	query := `
		INSERT INTO resource_shares (resource_type, resource_id, owner_id, shared_with, workspace_id, permission)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (resource_type, resource_id, COALESCE(shared_with, $7::uuid), COALESCE(workspace_id, $7::uuid))
		DO UPDATE SET permission = EXCLUDED.permission
		RETURNING id, resource_type, resource_id, owner_id, shared_with, workspace_id, permission, created_at
	`

	var share ResourceShare
	var swNull, wsNull sql.NullString
	err := s.db.QueryRowContext(ctx, query,
		req.ResourceType, req.ResourceID, callerID, sharedWith, workspaceID, req.Permission,
		"00000000-0000-0000-0000-000000000000",
	).Scan(&share.ID, &share.ResourceType, &share.ResourceID, &share.OwnerID, &swNull, &wsNull, &share.Permission, &share.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("share resource: %w", err)
	}
	if swNull.Valid {
		share.SharedWith = new(swNull.String)
	}
	if wsNull.Valid {
		share.WorkspaceID = new(wsNull.String)
	}
	return &share, nil
}

func (s *SharingService) Revoke(ctx context.Context, shareID, callerID string) error {
	var ownerID string
	err := s.db.QueryRowContext(ctx, `SELECT owner_id FROM resource_shares WHERE id = $1`, shareID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrShareNotFound
	}
	if err != nil {
		return err
	}
	if ownerID != callerID {
		return errors.New("only owner can revoke share")
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM resource_shares WHERE id = $1`, shareID)
	return err
}

func (s *SharingService) ListShared(ctx context.Context, userID, resourceType string) ([]ResourceShare, error) {
	var query string
	var args []any
	if resourceType != "" {
		query = `
			SELECT DISTINCT rs.id, rs.resource_type, rs.resource_id, rs.owner_id, rs.shared_with, rs.workspace_id, rs.permission, rs.created_at
			FROM resource_shares rs
			LEFT JOIN workspace_members wm ON rs.workspace_id = wm.workspace_id
			WHERE (rs.shared_with = $1 OR wm.user_id = $1) AND rs.resource_type = $2
			ORDER BY rs.created_at DESC
		`
		args = []any{userID, resourceType}
	} else {
		query = `
			SELECT DISTINCT rs.id, rs.resource_type, rs.resource_id, rs.owner_id, rs.shared_with, rs.workspace_id, rs.permission, rs.created_at
			FROM resource_shares rs
			LEFT JOIN workspace_members wm ON rs.workspace_id = wm.workspace_id
			WHERE rs.shared_with = $1 OR wm.user_id = $1
			ORDER BY rs.created_at DESC
		`
		args = []any{userID}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list shared: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []ResourceShare
	for rows.Next() {
		var share ResourceShare
		var swNull, wsNull sql.NullString
		if err := rows.Scan(&share.ID, &share.ResourceType, &share.ResourceID, &share.OwnerID, &swNull, &wsNull, &share.Permission, &share.CreatedAt); err != nil {
			return nil, err
		}
		if swNull.Valid {
			share.SharedWith = new(swNull.String)
		}
		if wsNull.Valid {
			share.WorkspaceID = new(wsNull.String)
		}
		list = append(list, share)
	}
	return list, rows.Err()
}

func (s *SharingService) ListOwned(ctx context.Context, ownerID, resourceType, resourceID string) ([]ResourceShare, error) {
	query := `
		SELECT id, resource_type, resource_id, owner_id, shared_with, workspace_id, permission, created_at
		FROM resource_shares
		WHERE owner_id = $1
	`
	args := []any{ownerID}
	if resourceType != "" {
		query += ` AND resource_type = $2`
		args = append(args, resourceType)
		if resourceID != "" {
			query += ` AND resource_id = $3`
			args = append(args, resourceID)
		}
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list owned: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []ResourceShare
	for rows.Next() {
		var share ResourceShare
		var swNull, wsNull sql.NullString
		if err := rows.Scan(&share.ID, &share.ResourceType, &share.ResourceID, &share.OwnerID, &swNull, &wsNull, &share.Permission, &share.CreatedAt); err != nil {
			return nil, err
		}
		if swNull.Valid {
			share.SharedWith = new(swNull.String)
		}
		if wsNull.Valid {
			share.WorkspaceID = new(wsNull.String)
		}
		list = append(list, share)
	}
	return list, rows.Err()
}

func isValidPermission(p string) bool {
	switch strings.ToLower(p) {
	case "view", "execute", "edit":
		return true
	}
	return false
}
