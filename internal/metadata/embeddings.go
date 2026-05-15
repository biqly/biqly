package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

var errCorruptEmbedding = errors.New("corrupt embedding payload")

func (r *Repository) upsertEntityEmbedding(ctx context.Context, table string, entityID, modelName string, embedding []float32) error {
	encoded, err := encodeEmbedding(embedding)
	if err != nil {
		return err
	}
	var query string
	switch table {
	case "tables":
		query = `
			UPDATE tables
			SET embedding = $2::jsonb,
			    embedding_model = $3,
			    embedding_updated_at = now()
			WHERE id = $1`
	case "columns":
		query = `
			UPDATE columns
			SET embedding = $2::jsonb,
			    embedding_model = $3,
			    embedding_updated_at = now()
			WHERE id = $1`
	default:
		return fmt.Errorf("upsert embedding: unknown table %q", table)
	}
	if _, err := r.db.ExecContext(ctx, query, entityID, encoded, modelName); err != nil {
		return fmt.Errorf("upsert %s embedding: %w", table, err)
	}
	return nil
}

func scanTableEmbedding(s platformdb.Scanner) (TableEmbedding, error) {
	var (
		te      TableEmbedding
		modelN  *string
		rawJSON []byte
	)
	if err := s.Scan(&te.SchemaName, &te.TableName, &modelN, &rawJSON); err != nil {
		return te, fmt.Errorf("scan table embedding: %w", err)
	}
	if modelN != nil {
		te.Model = *modelN
	}
	emb, err := decodeEmbedding(rawJSON)
	if err != nil {
		return te, errCorruptEmbedding
	}
	te.Embedding = emb
	return te, nil
}

func scanColumnEmbedding(s platformdb.Scanner) (ColumnEmbedding, error) {
	var (
		ce      ColumnEmbedding
		modelN  *string
		rawJSON []byte
	)
	if err := s.Scan(&ce.SchemaName, &ce.TableName, &ce.ColumnName, &modelN, &rawJSON); err != nil {
		return ce, fmt.Errorf("scan column embedding: %w", err)
	}
	if modelN != nil {
		ce.Model = *modelN
	}
	emb, err := decodeEmbedding(rawJSON)
	if err != nil {
		return ce, errCorruptEmbedding
	}
	ce.Embedding = emb
	return ce, nil
}

func listEmbeddingsSkippingCorrupt[T any](ctx context.Context, db *sql.DB, op, query string, args []any, scan func(platformdb.Scanner) (T, error)) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer func() { _ = rows.Close() }()

	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			if errors.Is(err, errCorruptEmbedding) {
				continue
			}
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return out, nil
}
