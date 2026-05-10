package ai

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

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

func (w *fakeMetadataWriter) UpsertTableEmbedding(_ context.Context, tableID, _ string, embedding []float32) error {
	if w.tableEmbeddings == nil {
		w.tableEmbeddings = make(map[string][]float32)
	}
	w.tableEmbeddings[tableID] = embedding
	return nil
}

func (w *fakeMetadataWriter) UpsertColumnEmbedding(_ context.Context, columnID, _ string, embedding []float32) error {
	if w.columnEmbeddings == nil {
		w.columnEmbeddings = make(map[string][]float32)
	}
	w.columnEmbeddings[columnID] = embedding
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
	embedder := &fakeEmbedder{model: "fake", default_: []float32{1, 0}}
	service := NewEmbedMetadataService(embedder, writer)

	results, err := service.EmbedAllForDatasource(context.Background(), "ds1")
	if err != nil {
		t.Fatalf("EmbedAllForDatasource() error = %v, want nil", err)
	}
	if len(results) != 3 {
		t.Fatalf("results len = %d, want 3: %+v", len(results), results)
	}
	if len(writer.tableEmbeddings["table-1"]) == 0 {
		t.Fatalf("table embedding was not stored")
	}
	if len(writer.columnEmbeddings["column-1"]) == 0 || len(writer.columnEmbeddings["column-2"]) == 0 {
		t.Fatalf("column embeddings were not stored: %+v", writer.columnEmbeddings)
	}
}
