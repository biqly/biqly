package ai

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/ai/lingua"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
)

// fakeEmbedder returns a fixed fallback vector for every input; sufficient for
// exercising the embed-metadata persistence path.
type fakeEmbedder struct {
	model    string
	fallback []float32
}

func (f *fakeEmbedder) Model() string { return f.model }
func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = f.fallback
	}
	return out, nil
}

type fakeMetadataWriter struct {
	tables           []metadata.Table
	columns          []metadata.Column
	tableEmbeddings  map[string][]float32
	columnEmbeddings map[string][]float32
}

func (w *fakeMetadataWriter) ListTables(context.Context, string, string) ([]metadata.Table, error) {
	return w.tables, nil
}

func (w *fakeMetadataWriter) ListColumns(context.Context, string, string, string) ([]metadata.Column, error) {
	return w.columns, nil
}

func (w *fakeMetadataWriter) UpsertTableEmbedding(_ context.Context, tableID, model string, embedding []float32) error {
	if w.tableEmbeddings == nil {
		w.tableEmbeddings = make(map[string][]float32)
	}
	w.tableEmbeddings[tableID+"|"+model] = embedding
	return nil
}

func (w *fakeMetadataWriter) UpsertColumnEmbedding(_ context.Context, columnID, model string, embedding []float32) error {
	if w.columnEmbeddings == nil {
		w.columnEmbeddings = make(map[string][]float32)
	}
	w.columnEmbeddings[columnID+"|"+model] = embedding
	return nil
}

func (*fakeMetadataWriter) ApplyTableTranslations(context.Context, []metadata.Table, i18n.Locale) error {
	return nil
}

func (*fakeMetadataWriter) ApplyColumnTranslations(context.Context, []metadata.Column, i18n.Locale) error {
	return nil
}

func TestEmbedMetadataService_EmbedsTablesAndColumns(t *testing.T) {
	writer := &fakeMetadataWriter{
		tables: []metadata.Table{
			{ID: "table-1", DatasourceID: "ds1", SchemaName: "sales", TableName: "orders"},
		},
		columns: []metadata.Column{
			{ID: "column-1", DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", ColumnName: "orderdate", DataType: "timestamp"},
			{ID: "column-2", DatasourceID: "ds1", SchemaName: "sales", TableName: "orders", ColumnName: "totaldue", DataType: "numeric"},
		},
	}
	embedder := &fakeEmbedder{model: "fake", fallback: []float32{1, 0}}
	service := NewEmbedMetadataService(embedder, writer)

	results, err := service.EmbedAllForDatasource(context.Background(), "ds1")
	if err != nil {
		t.Fatalf("EmbedAllForDatasource() error = %v, want nil", err)
	}
	if len(results) != 6 {
		t.Fatalf("results len = %d, want 6 (en+tr): %+v", len(results), results)
	}
	enModel := lingua.EmbeddingModelForLocale("fake", i18n.LocaleEN)
	trModel := lingua.EmbeddingModelForLocale("fake", i18n.LocaleTR)
	if len(writer.tableEmbeddings["table-1|"+enModel]) == 0 || len(writer.tableEmbeddings["table-1|"+trModel]) == 0 {
		t.Fatalf("table embeddings per locale were not stored: %+v", writer.tableEmbeddings)
	}
	if len(writer.columnEmbeddings["column-1|"+enModel]) == 0 || len(writer.columnEmbeddings["column-1|"+trModel]) == 0 {
		t.Fatalf("column embeddings per locale were not stored: %+v", writer.columnEmbeddings)
	}
}
