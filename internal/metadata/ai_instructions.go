package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// ErrInstructionNotFound is returned when an instruction id does not exist.
var ErrInstructionNotFound = errors.New("instruction not found")

// InstructionRow is a free-form business rule ("instruction") injected into the
// text-to-SQL prompt as a "## Business Rules" block.
type InstructionRow struct {
	ID           string
	DatasourceID string
	ModelID      string
	Title        string
	BodyMD       string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// InstructionInsert is the input for creating an instruction.
type InstructionInsert struct {
	DatasourceID string
	ModelID      string
	Title        string
	BodyMD       string
}

// InstructionUpdate replaces the editable fields of an instruction.
type InstructionUpdate struct {
	Title    string
	BodyMD   string
	IsActive bool
}

const instructionColumns = `id::text, datasource_id::text, COALESCE(model_id::text, ''), title, body_md, is_active, created_at, updated_at`

// InsertInstruction stores a new instruction and returns its generated id.
func (r *Repository) InsertInstruction(ctx context.Context, in InstructionInsert) (string, error) {
	var modelID any
	if in.ModelID != "" {
		modelID = in.ModelID
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ai_instructions (datasource_id, model_id, title, body_md)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		RETURNING id::text
	`, in.DatasourceID, modelID, in.Title, in.BodyMD).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert instruction: %w", err)
	}
	return id, nil
}

// ListInstructions returns instructions for a datasource, newest-updated first.
// An empty datasourceID lists across all datasources.
func (r *Repository) ListInstructions(ctx context.Context, datasourceID string) ([]InstructionRow, error) {
	q := `SELECT ` + instructionColumns + ` FROM ai_instructions
		WHERE ($1 = '' OR datasource_id::text = $1)
		ORDER BY updated_at DESC`
	return platformdb.QuerySliceErr(ctx, r.db, "list instructions", q,
		[]any{datasourceID}, scanInstructionRow)
}

// ListActiveInstructions returns active instructions for a datasource for prompt
// injection, newest-updated first.
func (r *Repository) ListActiveInstructions(ctx context.Context, datasourceID string) ([]InstructionRow, error) {
	q := `SELECT ` + instructionColumns + ` FROM ai_instructions
		WHERE datasource_id = $1::uuid AND is_active = true
		ORDER BY updated_at DESC`
	return platformdb.QuerySliceErr(ctx, r.db, "list active instructions", q,
		[]any{datasourceID}, scanInstructionRow)
}

// GetInstruction returns a single instruction by id.
func (r *Repository) GetInstruction(ctx context.Context, id string) (InstructionRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+instructionColumns+` FROM ai_instructions WHERE id = $1::uuid`, id)
	inst, err := scanInstructionRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InstructionRow{}, fmt.Errorf("instruction %s: %w", id, ErrInstructionNotFound)
		}
		return InstructionRow{}, err
	}
	return inst, nil
}

// UpdateInstruction replaces the editable fields of an instruction.
func (r *Repository) UpdateInstruction(ctx context.Context, id string, in InstructionUpdate) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_instructions
		SET title = $2, body_md = $3, is_active = $4, updated_at = now()
		WHERE id = $1::uuid
	`, id, in.Title, in.BodyMD, in.IsActive)
	if err != nil {
		return fmt.Errorf("update instruction: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update instruction affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("instruction %s: %w", id, ErrInstructionNotFound)
	}
	return nil
}

// DeleteInstruction removes an instruction.
func (r *Repository) DeleteInstruction(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM ai_instructions WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete instruction: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete instruction affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("instruction %s: %w", id, ErrInstructionNotFound)
	}
	return nil
}

func scanInstructionRow(s platformdb.Scanner) (InstructionRow, error) {
	var row InstructionRow
	if err := s.Scan(
		&row.ID, &row.DatasourceID, &row.ModelID, &row.Title, &row.BodyMD,
		&row.IsActive, &row.CreatedAt, &row.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return row, err
		}
		return row, fmt.Errorf("scan instruction: %w", err)
	}
	return row, nil
}
