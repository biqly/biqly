package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrMemoryEntryNotFound is returned when a memory entry does not exist in
// the caller's scope.
var ErrMemoryEntryNotFound = errors.New("memory entry not found")

// MemoryEntryRow is one durable remembered fact, scoped to a workspace+user
// pair and injected into the AI prompt. Every entry is user-deletable (GDPR).
type MemoryEntryRow struct {
	ID          string
	WorkspaceID string
	UserID      string
	Content     string
	Source      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const memoryEntryColumns = `id, workspace_id, user_id, content, source, created_at, updated_at`

// InsertMemoryEntry stores a remembered fact and returns its generated id.
func (r *Repository) InsertMemoryEntry(ctx context.Context, workspaceID, userID, content, source string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO memory_entries (workspace_id, user_id, content, source)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, workspaceID, userID, content, source).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert memory entry: %w", err)
	}
	return id, nil
}

// ListMemoryEntries returns the caller's remembered facts, newest first.
func (r *Repository) ListMemoryEntries(ctx context.Context, workspaceID, userID string) ([]MemoryEntryRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+memoryEntryColumns+` FROM memory_entries
		WHERE workspace_id = $1 AND user_id = $2
		ORDER BY updated_at DESC
	`, workspaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("list memory entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []MemoryEntryRow
	for rows.Next() {
		var row MemoryEntryRow
		if err := rows.Scan(&row.ID, &row.WorkspaceID, &row.UserID, &row.Content, &row.Source, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan memory entry: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list memory entries rows: %w", err)
	}
	return out, nil
}

// UpdateMemoryEntry replaces the content of one of the caller's entries.
func (r *Repository) UpdateMemoryEntry(ctx context.Context, id, workspaceID, userID, content string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE memory_entries SET content = $4, updated_at = now()
		WHERE id = $1 AND workspace_id = $2 AND user_id = $3
	`, id, workspaceID, userID, content)
	if err != nil {
		return fmt.Errorf("update memory entry: %w", err)
	}
	return memoryEntryAffected(res, id)
}

// DeleteMemoryEntry removes one of the caller's entries.
func (r *Repository) DeleteMemoryEntry(ctx context.Context, id, workspaceID, userID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM memory_entries WHERE id = $1 AND workspace_id = $2 AND user_id = $3
	`, id, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("delete memory entry: %w", err)
	}
	return memoryEntryAffected(res, id)
}

func memoryEntryAffected(res sql.Result, id string) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("memory entry affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("memory entry %s: %w", id, ErrMemoryEntryNotFound)
	}
	return nil
}
