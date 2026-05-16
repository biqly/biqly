package metadata

import (
	"context"
	"fmt"
	"strings"
)

const (
	relationUpsertRowCols    = 10
	relationUpsertChunkRows  = 500
	relationUpsertValueQuery = `
		INSERT INTO relations (id, datasource_id, constraint_name, from_schema, from_table, from_column, to_schema, to_table, to_column, relationship_type)
		VALUES %s
		ON CONFLICT (datasource_id, constraint_name) DO UPDATE
		SET relationship_type = EXCLUDED.relationship_type`
)

func upsertRelationsBatch(ctx context.Context, q execContexter, datasourceID string, relations []Relation) error {
	for start := 0; start < len(relations); start += relationUpsertChunkRows {
		end := min(start+relationUpsertChunkRows, len(relations))
		if err := upsertRelationsChunk(ctx, q, datasourceID, relations[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func upsertRelationsChunk(ctx context.Context, q execContexter, datasourceID string, relations []Relation) error {
	if len(relations) == 0 {
		return nil
	}

	var values strings.Builder
	args := make([]any, 0, len(relations)*relationUpsertRowCols)
	placeholder := 1
	for i, rel := range relations {
		if i > 0 {
			values.WriteByte(',')
		}
		values.WriteByte('(')
		for col := range relationUpsertRowCols {
			if col > 0 {
				values.WriteByte(',')
			}
			fmt.Fprintf(&values, "$%d", placeholder)
			placeholder++
		}
		values.WriteByte(')')
		args = append(args,
			rel.ID, datasourceID, rel.ConstraintName,
			rel.FromSchema, rel.FromTable, rel.FromColumn,
			rel.ToSchema, rel.ToTable, rel.ToColumn, rel.RelationshipType,
		)
	}

	query := fmt.Sprintf(relationUpsertValueQuery, values.String())
	if _, err := q.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert relations batch: %w", err)
	}
	return nil
}
