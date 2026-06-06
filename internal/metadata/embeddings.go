package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"strings"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

var errCorruptEmbedding = errors.New("corrupt embedding payload")

const embeddingLocaleSep = "@"

type localeStoredEmbedding struct {
	Model string    `json:"model"`
	V     []float32 `json:"v"`
}

type multiLocaleEmbeddingPayload struct {
	Locales map[string]localeStoredEmbedding `json:"locales"`
}

func localeKeyFromEmbeddingModel(modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if i := strings.LastIndex(modelName, embeddingLocaleSep); i >= 0 && i < len(modelName)-1 {
		return modelName[i+1:]
	}
	return "en"
}

func baseEmbeddingModel(modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if i := strings.LastIndex(modelName, embeddingLocaleSep); i > 0 {
		return modelName[:i]
	}
	return modelName
}

func mergeEmbeddingPayload(existing []byte, existingModel *string, modelName string, embedding []float32) (string, string, error) {
	store := multiLocaleEmbeddingPayload{Locales: make(map[string]localeStoredEmbedding)}
	if len(existing) > 0 { //nolint:nestif // supports legacy single-vector and multi-locale payloads
		if err := sonic.ConfigStd.Unmarshal(existing, &store); err == nil && len(store.Locales) > 0 {
			// multi-locale payload
		} else if vec, err := decodeEmbedding(existing); err == nil && len(vec) > 0 {
			legacyModel := strings.TrimSpace(modelName)
			if existingModel != nil && strings.TrimSpace(*existingModel) != "" {
				legacyModel = strings.TrimSpace(*existingModel)
			}
			store.Locales[localeKeyFromEmbeddingModel(legacyModel)] = localeStoredEmbedding{
				Model: legacyModel,
				V:     vec,
			}
		} else {
			return "", "", errCorruptEmbedding
		}
	}
	loc := localeKeyFromEmbeddingModel(modelName)
	store.Locales[loc] = localeStoredEmbedding{Model: strings.TrimSpace(modelName), V: embedding}
	b, err := sonic.ConfigStd.Marshal(store)
	if err != nil {
		return "", "", fmt.Errorf("encode multi-locale embedding: %w", err)
	}
	return string(b), baseEmbeddingModel(modelName), nil
}

func expandTableEmbeddings(schemaName, tableName string, modelN *string, rawJSON []byte) ([]TableEmbedding, error) {
	if len(rawJSON) == 0 {
		return nil, nil
	}
	var store multiLocaleEmbeddingPayload
	if err := sonic.ConfigStd.Unmarshal(rawJSON, &store); err == nil && len(store.Locales) > 0 {
		out := make([]TableEmbedding, 0, len(store.Locales))
		for _, le := range store.Locales {
			if len(le.V) == 0 {
				continue
			}
			out = append(out, TableEmbedding{
				SchemaName: schemaName,
				TableName:  tableName,
				Model:      le.Model,
				Embedding:  le.V,
			})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	vec, err := decodeEmbedding(rawJSON)
	if err != nil {
		return nil, errCorruptEmbedding
	}
	model := ""
	if modelN != nil {
		model = *modelN
	}
	return []TableEmbedding{{
		SchemaName: schemaName,
		TableName:  tableName,
		Model:      model,
		Embedding:  vec,
	}}, nil
}

func expandColumnEmbeddings(schemaName, tableName, columnName string, modelN *string, rawJSON []byte) ([]ColumnEmbedding, error) {
	if len(rawJSON) == 0 {
		return nil, nil
	}
	var store multiLocaleEmbeddingPayload
	if err := sonic.ConfigStd.Unmarshal(rawJSON, &store); err == nil && len(store.Locales) > 0 {
		out := make([]ColumnEmbedding, 0, len(store.Locales))
		for _, le := range store.Locales {
			if len(le.V) == 0 {
				continue
			}
			out = append(out, ColumnEmbedding{
				SchemaName: schemaName,
				TableName:  tableName,
				ColumnName: columnName,
				Model:      le.Model,
				Embedding:  le.V,
			})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	vec, err := decodeEmbedding(rawJSON)
	if err != nil {
		return nil, errCorruptEmbedding
	}
	model := ""
	if modelN != nil {
		model = *modelN
	}
	return []ColumnEmbedding{{
		SchemaName: schemaName,
		TableName:  tableName,
		ColumnName: columnName,
		Model:      model,
		Embedding:  vec,
	}}, nil
}

// upsertEntityEmbedding loads the current embedding payload for an entity,
// merges in the new locale-specific vector, and writes it back. Wrapped in a
// transaction with SELECT ... FOR UPDATE so concurrent calls targeting the
// same row don't lose locales via a lost-update race (two readers see the
// same baseline, each merges in its own locale, the second write overwrites
// the first).
func (r *Repository) upsertEntityEmbedding(ctx context.Context, table string, entityID, modelName string, embedding []float32) error {
	var selectQ, updateQ string
	switch table {
	case "tables":
		selectQ = `SELECT embedding, embedding_model FROM tables WHERE id = $1 FOR UPDATE`
		updateQ = `
			UPDATE tables
			SET embedding = $2::jsonb,
			    embedding_model = $3,
			    embedding_updated_at = now()
			WHERE id = $1`
	case "columns":
		selectQ = `SELECT embedding, embedding_model FROM columns WHERE id = $1 FOR UPDATE`
		updateQ = `
			UPDATE columns
			SET embedding = $2::jsonb,
			    embedding_model = $3,
			    embedding_updated_at = now()
			WHERE id = $1`
	default:
		return fmt.Errorf("upsert embedding: unknown table %q", table)
	}

	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		var existing []byte
		var existingModel sql.NullString
		if err := tx.QueryRowContext(ctx, selectQ, entityID).Scan(&existing, &existingModel); err != nil {
			return fmt.Errorf("load %s embedding: %w", table, err)
		}
		payload, displayModel, err := mergeEmbeddingPayload(existing, nullStringPtr(existingModel), modelName, embedding)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, updateQ, entityID, payload, displayModel); err != nil {
			return fmt.Errorf("upsert %s embedding: %w", table, err)
		}
		return nil
	})
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return new(ns.String)
}

func scanTableEmbeddingRow(s platformdb.Scanner) ([]TableEmbedding, error) {
	var (
		schemaName, tableName string
		modelN                *string
		rawJSON               []byte
	)
	if err := s.Scan(&schemaName, &tableName, &modelN, &rawJSON); err != nil {
		return nil, fmt.Errorf("scan table embedding: %w", err)
	}
	return expandTableEmbeddings(schemaName, tableName, modelN, rawJSON)
}

func scanColumnEmbeddingRow(s platformdb.Scanner) ([]ColumnEmbedding, error) {
	var (
		schemaName, tableName, columnName string
		modelN                            *string
		rawJSON                           []byte
	)
	if err := s.Scan(&schemaName, &tableName, &columnName, &modelN, &rawJSON); err != nil {
		return nil, fmt.Errorf("scan column embedding: %w", err)
	}
	return expandColumnEmbeddings(schemaName, tableName, columnName, modelN, rawJSON)
}

func listEmbeddingsExpanded[T any](ctx context.Context, db *sql.DB, op, query string, args []any, scan func(platformdb.Scanner) ([]T, error)) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer func() { _ = rows.Close() }()

	var out []T
	for rows.Next() {
		entries, err := scan(rows)
		if err != nil {
			if errors.Is(err, errCorruptEmbedding) {
				continue
			}
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		out = append(out, entries...)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return out, nil
}
