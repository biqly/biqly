package routing

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/ai/lingua"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
)

func TestTableRouter_TurkishQuestionUsesLocaleEmbeddings(t *testing.T) {
	descEN := "social posts"
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "tweets", TableType: "BASE TABLE", Description: &descEN},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "users", TableType: "BASE TABLE"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "tweets", ColumnName: "id", DataType: "uuid"},
		},
	}
	emb := &fakeEmbedder{
		model:    "fake",
		vectors:  map[string][]float32{"dün kaç tweet": {0, 1, 0}},
		fallback: []float32{1, 0, 0},
	}
	trModel := lingua.EmbeddingModelForLocale("fake", i18n.LocaleTR)
	embReader := &fakeEmbeddingReader{
		embeddings: []metadata.TableEmbedding{
			{SchemaName: "public", TableName: "tweets", Model: trModel, Embedding: []float32{0, 1, 0}},
			{SchemaName: "public", TableName: "users", Model: trModel, Embedding: []float32{1, 0, 0}},
		},
	}
	router := NewTableRouterWithEmbeddings(reader, emb, embReader, 30.0)

	model, routing, err := router.Route(context.Background(), "ds1", "dün kaç tweet", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("expected routing, got clarification: %+v", routing)
	}
	if model.BaseTable != "tweets" {
		t.Errorf("base table = %q, want tweets; selected=%v", model.BaseTable, routing.SelectedTables)
	}
}
