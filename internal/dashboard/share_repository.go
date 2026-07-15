package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ShareRepository handles database operations for dashboard public shares.
type ShareRepository struct {
	db *sql.DB
}

// NewShareRepository creates a new dashboard public-share repository.
func NewShareRepository(db *sql.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

// Rotate revokes any active share for the dashboard and inserts the new one
// atomically, keeping the one-active-share-per-dashboard invariant.
func (r *ShareRepository) Rotate(ctx context.Context, s *PublicShare) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rotate share: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE dashboard_public_shares SET revoked_at = now()
		WHERE dashboard_id = $1 AND revoked_at IS NULL
	`, s.DashboardID); err != nil {
		return fmt.Errorf("revoke previous share: %w", err)
	}

	var createdBy sql.NullString
	if s.CreatedBy != "" {
		createdBy = sql.NullString{String: s.CreatedBy, Valid: true}
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO dashboard_public_shares (dashboard_id, workspace_id, token_hash, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, s.DashboardID, s.WorkspaceID, s.TokenHash, createdBy, s.ExpiresAt).Scan(&s.ID, &s.CreatedAt); err != nil {
		return fmt.Errorf("insert share: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rotate share: %w", err)
	}
	return nil
}

// GetActive returns the live share for a dashboard, scoped to the workspace.
func (r *ShareRepository) GetActive(ctx context.Context, dashboardID, workspaceID string) (*PublicShare, error) {
	s := &PublicShare{}
	var createdBy sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, dashboard_id, workspace_id, token_hash, created_by, created_at, revoked_at, expires_at
		FROM dashboard_public_shares
		WHERE dashboard_id = $1 AND workspace_id = $2 AND revoked_at IS NULL
	`, dashboardID, workspaceID).Scan(&s.ID, &s.DashboardID, &s.WorkspaceID, &s.TokenHash, &createdBy, &s.CreatedAt, &s.RevokedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("get active share: %w", err)
	}
	if createdBy.Valid {
		s.CreatedBy = createdBy.String
	}
	return s, nil
}

// Revoke soft-deletes the active share for a dashboard within the workspace.
func (r *ShareRepository) Revoke(ctx context.Context, dashboardID, workspaceID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE dashboard_public_shares SET revoked_at = now()
		WHERE dashboard_id = $1 AND workspace_id = $2 AND revoked_at IS NULL
	`, dashboardID, workspaceID)
	if err != nil {
		return fmt.Errorf("revoke share: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// FindActiveByTokenHash is the anonymous lookup path: live, unexpired shares only.
func (r *ShareRepository) FindActiveByTokenHash(ctx context.Context, tokenHash string) (*PublicShare, error) {
	s := &PublicShare{}
	var createdBy sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, dashboard_id, workspace_id, token_hash, created_by, created_at, revoked_at, expires_at
		FROM dashboard_public_shares
		WHERE token_hash = $1 AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
	`, tokenHash).Scan(&s.ID, &s.DashboardID, &s.WorkspaceID, &s.TokenHash, &createdBy, &s.CreatedAt, &s.RevokedAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("find share by token: %w", err)
	}
	if createdBy.Valid {
		s.CreatedBy = createdBy.String
	}
	return s, nil
}
