package metadata

import (
	"context"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// BusinessGlossaryInsert is input for creating a glossary term.
type BusinessGlossaryInsert struct {
	DatasourceID string
	ModelID      string
	Term         string
	Definition   string
	MapsToType   string
	MapsToName   string
	Aliases      []string
}

// BusinessGlossaryUpdate is input for updating a glossary term.
type BusinessGlossaryUpdate struct {
	Term       string
	Definition string
	MapsToType string
	MapsToName string
	Aliases    []string
	IsActive   *bool
}

// ListBusinessGlossary returns glossary terms for a datasource, optionally scoped to a model.
func (r *Repository) ListBusinessGlossary(ctx context.Context, datasourceID, modelID string) ([]BusinessGlossaryRow, error) {
	q := `SELECT id::text, datasource_id::text, COALESCE(model_id::text, ''), term, COALESCE(definition, ''),
		maps_to_type, maps_to_name, COALESCE(aliases, '{}'), is_active, created_at, updated_at
		FROM business_glossary_terms WHERE datasource_id = $1::uuid AND is_active = true`
	args := []any{datasourceID}
	if modelID != "" {
		q += ` AND (model_id IS NULL OR model_id = $2::uuid)`
		args = append(args, modelID)
	}
	q += ` ORDER BY term`

	return platformdb.QuerySliceErr(ctx, r.db, "list business glossary", q, args, scanBusinessGlossaryRow)
}

func scanBusinessGlossaryRow(s platformdb.Scanner) (BusinessGlossaryRow, error) {
	var e BusinessGlossaryRow
	var aliases pgarray.StringArray
	if err := s.Scan(&e.ID, &e.DatasourceID, &e.ModelID, &e.Term, &e.Definition,
		&e.MapsToType, &e.MapsToName, &aliases, &e.IsActive, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return e, fmt.Errorf("scan business glossary: %w", err)
	}
	e.Aliases = []string(aliases)
	return e, nil
}

// InsertBusinessGlossary inserts a row and returns the new id.
func (r *Repository) InsertBusinessGlossary(ctx context.Context, in BusinessGlossaryInsert) (string, error) {
	var modelID any
	if in.ModelID != "" {
		modelID = in.ModelID
	}
	var id string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO business_glossary_terms (datasource_id, model_id, term, definition, maps_to_type, maps_to_name, aliases)
		 VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, ''), $5, $6, $7)
		 RETURNING id::text`,
		in.DatasourceID, modelID, in.Term, in.Definition, in.MapsToType, in.MapsToName, pgarray.Strings(in.Aliases),
	).Scan(&id)
	return id, err
}

// UpdateBusinessGlossary updates a row by id.
func (r *Repository) UpdateBusinessGlossary(ctx context.Context, id string, in BusinessGlossaryUpdate) error {
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE business_glossary_terms
		 SET term = $1, definition = NULLIF($2, ''), maps_to_type = $3, maps_to_name = $4,
		     aliases = $5::text[], is_active = $6, updated_at = NOW()
		 WHERE id = $7::uuid`,
		in.Term, in.Definition, in.MapsToType, in.MapsToName, pgarray.Strings(in.Aliases), active, id,
	)
	return err
}

// DeleteBusinessGlossary deletes by id. Returns false if no row matched.
func (r *Repository) DeleteBusinessGlossary(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM business_glossary_terms WHERE id = $1::uuid`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
