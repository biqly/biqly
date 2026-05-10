package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/metadata"
)

// MetadataWriter is the subset of metadata.Repository the EmbedMetadataService
// relies on for storing computed embeddings.
type MetadataWriter interface {
	ListTables(ctx context.Context, datasourceID, schemaName string) ([]metadata.Table, error)
	ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error)
	UpsertTableEmbedding(ctx context.Context, tableID, modelName string, embedding []float32) error
}

// EmbedMetadataService computes a per-table embedding from the table's name,
// description, and column-name list, then persists it for use by the
// vector-aware table router.
type EmbedMetadataService struct {
	embedder Embedder
	writer   MetadataWriter
}

// NewEmbedMetadataService wires the embedder and metadata repository.
func NewEmbedMetadataService(embedder Embedder, writer MetadataWriter) *EmbedMetadataService {
	return &EmbedMetadataService{embedder: embedder, writer: writer}
}

// EmbedTableResult is one entry in the response — schema/table identifier
// plus the outcome (embedded or skipped+reason).
type EmbedTableResult struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// EmbedAllForDatasource computes embeddings for every table in the datasource
// and writes them to the metadata DB. Returns one result per table.
func (s *EmbedMetadataService) EmbedAllForDatasource(ctx context.Context, datasourceID string) ([]EmbedTableResult, error) {
	tables, err := s.writer.ListTables(ctx, datasourceID, "")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	cols, err := s.writer.ListColumns(ctx, datasourceID, "", "")
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	colsByTable := make(map[string][]metadata.Column, len(tables))
	for _, c := range cols {
		k := c.SchemaName + "." + c.TableName
		colsByTable[k] = append(colsByTable[k], c)
	}

	if len(tables) == 0 {
		return nil, nil
	}

	texts := make([]string, 0, len(tables))
	indices := make([]int, 0, len(tables))
	for i, t := range tables {
		key := t.SchemaName + "." + t.TableName
		txt := buildTableEmbeddingText(t, colsByTable[key])
		if strings.TrimSpace(txt) == "" {
			continue
		}
		texts = append(texts, txt)
		indices = append(indices, i)
	}

	vectors, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	model := s.embedder.Model()
	results := make([]EmbedTableResult, 0, len(tables))
	for i, t := range tables {
		results = append(results, EmbedTableResult{Schema: t.SchemaName, Table: t.TableName})
		// Find the matching vector by index lookup.
		pos := -1
		for j, ix := range indices {
			if ix == i {
				pos = j
				break
			}
		}
		if pos < 0 || pos >= len(vectors) || len(vectors[pos]) == 0 {
			results[len(results)-1].Skipped = true
			results[len(results)-1].Reason = "no embedding produced"
			continue
		}
		if err := s.writer.UpsertTableEmbedding(ctx, t.ID, model, vectors[pos]); err != nil {
			results[len(results)-1].Skipped = true
			results[len(results)-1].Reason = "store failed: " + err.Error()
			continue
		}
	}
	return results, nil
}

// buildTableEmbeddingText assembles the natural-language summary we feed to the
// embedder for a single table: schema-qualified name, optional description,
// and a compact column list (names + types). Kept short on purpose — table
// retrieval doesn't need full data — so embedding cost stays bounded even
// for catalogs with hundreds of columns per table.
func buildTableEmbeddingText(t metadata.Table, cols []metadata.Column) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Table: %s.%s", t.SchemaName, t.TableName)
	if t.Description != nil && strings.TrimSpace(*t.Description) != "" {
		fmt.Fprintf(&sb, "\nDescription: %s", strings.TrimSpace(*t.Description))
	}
	if len(cols) > 0 {
		sb.WriteString("\nColumns: ")
		const maxCols = 60 // cap for very wide tables
		for i, c := range cols {
			if i >= maxCols {
				fmt.Fprintf(&sb, ", … (+%d more)", len(cols)-maxCols)
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s (%s)", c.ColumnName, c.DataType)
		}
	}
	return sb.String()
}
