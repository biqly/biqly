package metadata

import (
	"context"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/platform/db/pgarray"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	"github.com/bytedance/sonic"
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
	AIContext    *pkgmetadata.GlossaryAIContext
}

// BusinessGlossaryUpdate is input for updating a glossary term.
type BusinessGlossaryUpdate struct {
	Term       string
	Definition string
	MapsToType string
	MapsToName string
	Aliases    []string
	AIContext  *pkgmetadata.GlossaryAIContext
	IsActive   *bool
}

// ListBusinessGlossary returns glossary terms for a datasource, optionally scoped to a model.
func (r *Repository) ListBusinessGlossary(ctx context.Context, datasourceID, modelID string) ([]BusinessGlossaryRow, error) {
	q := `SELECT id::text, datasource_id::text, COALESCE(model_id::text, ''), term, COALESCE(definition, ''),
		maps_to_type, maps_to_name, COALESCE(aliases, '{}'), ai_context, is_active, created_at, updated_at
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
	var aiContextRaw []byte
	if err := s.Scan(&e.ID, &e.DatasourceID, &e.ModelID, &e.Term, &e.Definition,
		&e.MapsToType, &e.MapsToName, &aliases, &aiContextRaw, &e.IsActive, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return e, fmt.Errorf("scan business glossary: %w", err)
	}
	e.Aliases = []string(aliases)
	if len(aiContextRaw) > 0 {
		var ctx pkgmetadata.GlossaryAIContext
		if err := sonic.Unmarshal(aiContextRaw, &ctx); err != nil {
			return e, fmt.Errorf("unmarshal glossary ai_context: %w", err)
		}
		if !ctx.IsZero() {
			e.AIContext = &ctx
		}
	}
	return e, nil
}

// InsertBusinessGlossary inserts a row and returns the new id.
func (r *Repository) InsertBusinessGlossary(ctx context.Context, in BusinessGlossaryInsert) (string, error) {
	var modelID any
	if in.ModelID != "" {
		modelID = in.ModelID
	}
	aiContextJSON, err := marshalGlossaryAIContext(in.AIContext)
	if err != nil {
		return "", err
	}
	var id string
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO business_glossary_terms (datasource_id, model_id, term, definition, maps_to_type, maps_to_name, aliases, ai_context)
		 VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, ''), $5, $6, $7, $8::jsonb)
		 RETURNING id::text`,
		in.DatasourceID, modelID, in.Term, in.Definition, in.MapsToType, in.MapsToName, pgarray.Strings(in.Aliases), aiContextJSON,
	).Scan(&id)
	return id, err
}

// UpdateBusinessGlossary updates a row by id.
func (r *Repository) UpdateBusinessGlossary(ctx context.Context, id string, in BusinessGlossaryUpdate) error {
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	aiContextJSON, err := marshalGlossaryAIContext(in.AIContext)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE business_glossary_terms
		 SET term = $1, definition = NULLIF($2, ''), maps_to_type = $3, maps_to_name = $4,
		     aliases = $5::text[], ai_context = $6::jsonb, is_active = $7, updated_at = NOW()
		 WHERE id = $8::uuid`,
		in.Term, in.Definition, in.MapsToType, in.MapsToName, pgarray.Strings(in.Aliases), aiContextJSON, active, id,
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

func marshalGlossaryAIContext(ctx *pkgmetadata.GlossaryAIContext) (any, error) {
	if ctx == nil || ctx.IsZero() {
		return nil, nil //nolint:nilnil // nil value serializes as SQL NULL
	}
	raw, err := sonic.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("marshal glossary ai_context: %w", err)
	}
	return raw, nil
}
