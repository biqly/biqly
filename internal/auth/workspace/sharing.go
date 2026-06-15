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

var ErrShareNotFound = errors.New("share not found")

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
	db *sql.DB
	ws *Service
}

func NewSharingService(db *sql.DB, ws *Service) *SharingService {
	return &SharingService{db: db, ws: ws}
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

	sharedWithVal := "00000000-0000-0000-0000-000000000000"
	if sharedWith.Valid {
		sharedWithVal = sharedWith.String
	}
	workspaceVal := "00000000-0000-0000-0000-000000000000"
	if workspaceID.Valid {
		workspaceVal = workspaceID.String
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
	_ = sharedWithVal
	_ = workspaceVal
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
