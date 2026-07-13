package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// ErrKnowledgeFileNotFound is returned when a knowledge file id does not exist.
var ErrKnowledgeFileNotFound = errors.New("knowledge file not found")

// KnowledgeFileRow is one markdown document of the datasource's knowledge base.
// Files live in virtual folders (glossary/, instructions/, metrics/,
// sql-pairs/); publishing extracts structured records into the existing prompt
// stores, linked back via knowledge_file_id.
type KnowledgeFileRow struct {
	ID           string
	DatasourceID string
	Path         string
	Folder       string
	Title        string
	ContentMD    string
	Frontmatter  []byte
	Status       string
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// KnowledgeFileInsert is the input for creating a knowledge file.
type KnowledgeFileInsert struct {
	DatasourceID string
	Path         string
	Title        string
	ContentMD    string
	Frontmatter  []byte
	Status       string
	CreatedBy    string
}

// KnowledgeFileUpdate replaces the editable fields of a knowledge file.
type KnowledgeFileUpdate struct {
	Path        string
	Title       string
	ContentMD   string
	Frontmatter []byte
	Status      string
}

// KnowledgeFolder derives the virtual folder from a file path
// ("instructions/rounding.md" → "instructions"; a bare "README.md" → "").
func KnowledgeFolder(path string) string {
	idx := strings.Index(path, "/")
	if idx <= 0 {
		return ""
	}
	return path[:idx]
}

const knowledgeFileColumns = `id::text, datasource_id::text, path, folder, title, content_md, COALESCE(frontmatter, 'null'::jsonb)::text, status, created_by, created_at, updated_at`

// InsertKnowledgeFile stores a new knowledge file and returns its generated id.
func (r *Repository) InsertKnowledgeFile(ctx context.Context, in KnowledgeFileInsert) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ai_knowledge_files (datasource_id, path, folder, title, content_md, frontmatter, status, created_by)
		VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb, $7, $8)
		RETURNING id::text
	`, in.DatasourceID, in.Path, KnowledgeFolder(in.Path), in.Title, in.ContentMD,
		platformdb.NullIfEmpty(string(in.Frontmatter)), in.Status, in.CreatedBy).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert knowledge file: %w", err)
	}
	return id, nil
}

// ListKnowledgeFiles returns a datasource's files ordered by folder then path.
func (r *Repository) ListKnowledgeFiles(ctx context.Context, datasourceID string) ([]KnowledgeFileRow, error) {
	q := `SELECT ` + knowledgeFileColumns + ` FROM ai_knowledge_files
		WHERE datasource_id = $1::uuid
		ORDER BY folder, path`
	return platformdb.QuerySliceErr(ctx, r.db, "list knowledge files", q,
		[]any{datasourceID}, scanKnowledgeFileRow)
}

// ListPublishedKnowledgeFiles returns only published files — the agent-facing
// view (list_knowledge_files / read_knowledge_file tools).
func (r *Repository) ListPublishedKnowledgeFiles(ctx context.Context, datasourceID string) ([]KnowledgeFileRow, error) {
	q := `SELECT ` + knowledgeFileColumns + ` FROM ai_knowledge_files
		WHERE datasource_id = $1::uuid AND status = 'published'
		ORDER BY folder, path`
	return platformdb.QuerySliceErr(ctx, r.db, "list published knowledge files", q,
		[]any{datasourceID}, scanKnowledgeFileRow)
}

// GetKnowledgeFile returns a single knowledge file by id.
func (r *Repository) GetKnowledgeFile(ctx context.Context, id string) (KnowledgeFileRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+knowledgeFileColumns+` FROM ai_knowledge_files WHERE id = $1::uuid`, id)
	file, err := scanKnowledgeFileRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return KnowledgeFileRow{}, fmt.Errorf("knowledge file %s: %w", id, ErrKnowledgeFileNotFound)
		}
		return KnowledgeFileRow{}, err
	}
	return file, nil
}

// GetKnowledgeFileByPath returns a published file by its path — the agent
// read_knowledge_file lookup.
func (r *Repository) GetKnowledgeFileByPath(ctx context.Context, datasourceID, path string) (KnowledgeFileRow, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+knowledgeFileColumns+` FROM ai_knowledge_files
		 WHERE datasource_id = $1::uuid AND path = $2 AND status = 'published'`,
		datasourceID, path)
	file, err := scanKnowledgeFileRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return KnowledgeFileRow{}, fmt.Errorf("knowledge file %s: %w", path, ErrKnowledgeFileNotFound)
		}
		return KnowledgeFileRow{}, err
	}
	return file, nil
}

// UpdateKnowledgeFile replaces the editable fields of a knowledge file.
func (r *Repository) UpdateKnowledgeFile(ctx context.Context, id string, in KnowledgeFileUpdate) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_knowledge_files
		SET path = $2, folder = $3, title = $4, content_md = $5, frontmatter = $6::jsonb, status = $7, updated_at = now()
		WHERE id = $1::uuid
	`, id, in.Path, KnowledgeFolder(in.Path), in.Title, in.ContentMD,
		platformdb.NullIfEmpty(string(in.Frontmatter)), in.Status)
	if err != nil {
		return fmt.Errorf("update knowledge file: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update knowledge file affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("knowledge file %s: %w", id, ErrKnowledgeFileNotFound)
	}
	return nil
}

// DeleteKnowledgeFile removes a file; extraction rows keep living but lose the
// link (knowledge_file_id → NULL via FK) — callers deactivate them explicitly.
func (r *Repository) DeleteKnowledgeFile(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM ai_knowledge_files WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete knowledge file: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete knowledge file affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("knowledge file %s: %w", id, ErrKnowledgeFileNotFound)
	}
	return nil
}

// DatasourceForKnowledgeFile resolves a file id to its owning datasource for
// access-control middleware.
func (r *Repository) DatasourceForKnowledgeFile(ctx context.Context, id string) (string, error) {
	return r.datasourceForEntity(ctx, "ai_knowledge_files", "knowledge file", id, ErrKnowledgeFileNotFound)
}

// DeactivateExtractionsForKnowledgeFile soft-disables every structured record
// that was extracted from the file — called before deleting or re-publishing.
func (r *Repository) DeactivateExtractionsForKnowledgeFile(ctx context.Context, id string) error {
	stmts := []string{
		`UPDATE ai_instructions SET is_active = false, updated_at = now() WHERE knowledge_file_id = $1::uuid`,
		`UPDATE business_glossary_terms SET is_active = false, updated_at = now() WHERE knowledge_file_id = $1::uuid`,
		`UPDATE ai_saved_queries SET is_active = false, updated_at = now() WHERE knowledge_file_id = $1::uuid`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt, id); err != nil {
			return fmt.Errorf("deactivate knowledge extractions: %w", err)
		}
	}
	return nil
}

func scanKnowledgeFileRow(s platformdb.Scanner) (KnowledgeFileRow, error) {
	var row KnowledgeFileRow
	var frontmatter string
	if err := s.Scan(
		&row.ID, &row.DatasourceID, &row.Path, &row.Folder, &row.Title, &row.ContentMD,
		&frontmatter, &row.Status, &row.CreatedBy, &row.CreatedAt, &row.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return row, err
		}
		return row, fmt.Errorf("scan knowledge file: %w", err)
	}
	if frontmatter != "" && frontmatter != "null" {
		row.Frontmatter = []byte(frontmatter)
	}
	return row, nil
}
