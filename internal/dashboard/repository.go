package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Repository handles database operations for dashboards.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new dashboard repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new dashboard.
func (r *Repository) Create(ctx context.Context, d *Dashboard) error {
	query := `
		INSERT INTO dashboards (workspace_id, name, description, widgets)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id, created_at, updated_at
	`
	var wsID sql.NullString
	if d.WorkspaceID != nil && *d.WorkspaceID != "" {
		wsID = sql.NullString{String: *d.WorkspaceID, Valid: true}
	}

	err := r.db.QueryRowContext(ctx, query, wsID, d.Name, d.Description, d.Widgets).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create dashboard: %w", err)
	}
	return nil
}

// Get retrieves a dashboard by ID.
func (r *Repository) Get(ctx context.Context, id string) (*Dashboard, error) {
	query := `
		SELECT id, workspace_id, name, description, widgets, created_at, updated_at
		FROM dashboards
		WHERE id = $1
	`
	d := &Dashboard{}
	var wsID, desc sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(&d.ID, &wsID, &d.Name, &desc, &d.Widgets, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("dashboard not found: %w", err)
	} else if err != nil {
		return nil, fmt.Errorf("get dashboard: %w", err)
	}
	if wsID.Valid {
		d.WorkspaceID = new(wsID.String)
	}
	if desc.Valid {
		d.Description = new(desc.String)
	}
	return d, nil
}

// List returns dashboards. If workspaceID is not empty, it filters by it.
func (r *Repository) List(ctx context.Context, workspaceID string) ([]Dashboard, error) {
	var query string
	var args []any
	if workspaceID != "" {
		query = `
			SELECT id, workspace_id, name, description, widgets, created_at, updated_at
			FROM dashboards
			WHERE workspace_id = $1 OR workspace_id IS NULL
			ORDER BY created_at DESC
		`
		args = append(args, workspaceID)
	} else {
		query = `
			SELECT id, workspace_id, name, description, widgets, created_at, updated_at
			FROM dashboards
			ORDER BY created_at DESC
		`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dashboards: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			_ = err
		}
	}()

	var list []Dashboard
	for rows.Next() {
		var d Dashboard
		var wsID, desc sql.NullString
		if err := rows.Scan(&d.ID, &wsID, &d.Name, &desc, &d.Widgets, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dashboard row: %w", err)
		}
		if wsID.Valid {
			d.WorkspaceID = new(wsID.String)
		}
		if desc.Valid {
			d.Description = new(desc.String)
		}
		list = append(list, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if list == nil {
		list = []Dashboard{}
	}
	return list, nil
}

// Update updates name, description, and widgets configuration of a dashboard.
func (r *Repository) Update(ctx context.Context, d *Dashboard) error {
	query := `
		UPDATE dashboards
		SET name = $2, description = $3, widgets = $4::jsonb, updated_at = now()
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, d.ID, d.Name, d.Description, d.Widgets)
	if err != nil {
		return fmt.Errorf("update dashboard: %w", err)
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

// Delete removes a dashboard by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM dashboards WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete dashboard: %w", err)
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
