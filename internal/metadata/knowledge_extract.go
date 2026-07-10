package metadata

import (
	"context"
	"fmt"

	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// Knowledge publish-extraction upserts: each keeps exactly one structured
// record per source knowledge file (keyed by knowledge_file_id) so
// re-publishing an edited file updates in place instead of duplicating.

// UpsertInstructionFromKnowledge stores/refreshes the instruction extracted
// from an instructions/*.md file.
func (r *Repository) UpsertInstructionFromKnowledge(ctx context.Context, fileID, datasourceID, title, bodyMD string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_instructions
		SET title = $2, body_md = $3, is_active = true, updated_at = now()
		WHERE knowledge_file_id = $1::uuid
	`, fileID, title, bodyMD)
	if err != nil {
		return fmt.Errorf("update instruction from knowledge: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		return nil
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO ai_instructions (datasource_id, title, body_md, knowledge_file_id)
		VALUES ($1::uuid, $2, $3, $4::uuid)
	`, datasourceID, title, bodyMD, fileID)
	if err != nil {
		return fmt.Errorf("insert instruction from knowledge: %w", err)
	}
	return nil
}

// KnowledgeGlossaryUpsert is the glossary payload parsed from a
// glossary/*.md file's frontmatter.
type KnowledgeGlossaryUpsert struct {
	Term       string
	Definition string
	MapsToType string
	MapsToName string
	Aliases    []string
}

// UpsertGlossaryFromKnowledge stores/refreshes the glossary term extracted
// from a glossary/*.md file. Terms are unique per datasource, so an existing
// hand-created term with the same name is adopted by the file.
func (r *Repository) UpsertGlossaryFromKnowledge(ctx context.Context, fileID, datasourceID string, in KnowledgeGlossaryUpsert) error {
	aliases := in.Aliases
	if aliases == nil {
		aliases = []string{}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO business_glossary_terms (datasource_id, term, definition, maps_to_type, maps_to_name, aliases, knowledge_file_id)
		VALUES ($1::uuid, $2, NULLIF($3, ''), $4, $5, $6, $7::uuid)
		ON CONFLICT (datasource_id, term) DO UPDATE SET
			definition = EXCLUDED.definition,
			maps_to_type = EXCLUDED.maps_to_type,
			maps_to_name = EXCLUDED.maps_to_name,
			aliases = EXCLUDED.aliases,
			knowledge_file_id = EXCLUDED.knowledge_file_id,
			is_active = true,
			updated_at = now()
	`, datasourceID, in.Term, in.Definition, in.MapsToType, in.MapsToName, pgarray.Strings(aliases), fileID)
	if err != nil {
		return fmt.Errorf("upsert glossary from knowledge: %w", err)
	}
	return nil
}

// LinkInstructionKnowledgeFile attaches an existing instruction row to its
// backfilled knowledge file.
func (r *Repository) LinkInstructionKnowledgeFile(ctx context.Context, instructionID, fileID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ai_instructions SET knowledge_file_id = $2::uuid WHERE id = $1::uuid`, instructionID, fileID)
	if err != nil {
		return fmt.Errorf("link instruction knowledge file: %w", err)
	}
	return nil
}

// LinkGlossaryKnowledgeFile attaches an existing glossary term to its
// backfilled knowledge file.
func (r *Repository) LinkGlossaryKnowledgeFile(ctx context.Context, termID, fileID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE business_glossary_terms SET knowledge_file_id = $2::uuid WHERE id = $1::uuid`, termID, fileID)
	if err != nil {
		return fmt.Errorf("link glossary knowledge file: %w", err)
	}
	return nil
}

// LinkSavedQueryKnowledgeFile attaches an existing saved-query example to its
// backfilled knowledge file.
func (r *Repository) LinkSavedQueryKnowledgeFile(ctx context.Context, savedQueryID, fileID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ai_saved_queries SET knowledge_file_id = $2::uuid WHERE id = $1::uuid`, savedQueryID, fileID)
	if err != nil {
		return fmt.Errorf("link saved query knowledge file: %w", err)
	}
	return nil
}

// KnowledgeExampleUpsert is the question/SQL pair parsed from a
// sql-pairs/*.md file.
type KnowledgeExampleUpsert struct {
	Name         string
	Description  string
	Question     string
	QuestionHash string
	SQLQuery     string
	LogicalQuery []byte
	CreatedBy    string
}

// UpsertSavedQueryExampleFromKnowledge stores/refreshes the grounding example
// extracted from a sql-pairs/*.md file (source 'example', not runnable).
func (r *Repository) UpsertSavedQueryExampleFromKnowledge(ctx context.Context, fileID, datasourceID string, in KnowledgeExampleUpsert) error {
	var logicalQuery any
	if len(in.LogicalQuery) > 0 {
		logicalQuery = in.LogicalQuery
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_saved_queries
		SET name = $2, description = $3, question = $4, question_hash = $5,
		    sql_query = $6, logical_query = $7::jsonb, is_active = true, updated_at = now()
		WHERE knowledge_file_id = $1::uuid
	`, fileID, in.Name, in.Description, in.Question, in.QuestionHash, in.SQLQuery, logicalQuery)
	if err != nil {
		return fmt.Errorf("update saved query from knowledge: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		return nil
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO ai_saved_queries (
			datasource_id, name, description, question, question_hash, sql_query,
			logical_query, parameters, tags, source, runnable, created_by, knowledge_file_id
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::jsonb, '[]'::jsonb, '{}', 'example', false, $8, $9::uuid)
	`, datasourceID, in.Name, in.Description, in.Question, in.QuestionHash, in.SQLQuery,
		logicalQuery, in.CreatedBy, fileID)
	if err != nil {
		return fmt.Errorf("insert saved query from knowledge: %w", err)
	}
	return nil
}
