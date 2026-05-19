package ai

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

func TestCosineSimilarity(t *testing.T) {
	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 2, 3}, []float32{1, 2, 3}, 1.0},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 0.0},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, -1.0},
		{"len mismatch", []float32{1, 0}, []float32{1, 0, 0}, 0.0},
		{"zero vector", []float32{0, 0}, []float32{1, 1}, 0.0},
		{"empty", nil, []float32{1, 1}, 0.0},
	}
	for _, c := range cases {
		got := CosineSimilarity(c.a, c.b)
		if math.Abs(got-c.want) > 1e-6 {
			t.Errorf("%s: CosineSimilarity = %v, want %v", c.name, got, c.want)
		}
	}
}

// fakeEmbedder returns a fixed embedding per known phrase; useful so router
// tests can pin which "question" is closest to which "table".
type fakeEmbedder struct {
	model    string
	vectors  map[string][]float32
	fallback []float32
}

func (f *fakeEmbedder) Model() string { return f.model }
func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if v, ok := f.vectors[t]; ok {
			out[i] = v
			continue
		}
		out[i] = f.fallback
	}
	return out, nil
}

type fakeEmbeddingReader struct {
	embeddings       []metadata.TableEmbedding
	columnEmbeddings []metadata.ColumnEmbedding
	err              error
}

func (r *fakeEmbeddingReader) ListTableEmbeddings(_ context.Context, _ string) ([]metadata.TableEmbedding, error) {
	return r.embeddings, r.err
}

func (r *fakeEmbeddingReader) ListColumnEmbeddings(_ context.Context, _ string) ([]metadata.ColumnEmbedding, error) {
	return r.columnEmbeddings, r.err
}

// TestTableRouter_EmbeddingBoostShiftsRanking verifies that a table whose
// embedding is closer to the question vector wins over a higher-keyword-score
// table, demonstrating the hybrid score actually moves the routing.
func TestTableRouter_EmbeddingBoostShiftsRanking(t *testing.T) {
	// Two tables: "sales" wins on keyword "sales", "events" has zero token
	// match. Question: "kullanıcı oturum açma" (no token overlap with either).
	// Without embeddings this would clarify; with embeddings biased toward
	// "events" we expect events to win.
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "sales", TableType: "BASE TABLE"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "events", TableType: "BASE TABLE"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "sales", ColumnName: "amount", DataType: "numeric"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "events", ColumnName: "user_id", DataType: "uuid"},
			{DatasourceID: "ds1", SchemaName: "public", TableName: "events", ColumnName: "kind", DataType: "text"},
		},
	}
	emb := &fakeEmbedder{
		model:    "fake",
		vectors:  map[string][]float32{"login activity": {0, 1, 0}},
		fallback: []float32{1, 0, 0},
	}
	embReader := &fakeEmbeddingReader{
		embeddings: []metadata.TableEmbedding{
			{SchemaName: "public", TableName: "sales", Model: "fake", Embedding: []float32{1, 0, 0}},
			{SchemaName: "public", TableName: "events", Model: "fake", Embedding: []float32{0, 1, 0}},
		},
	}
	router := NewTableRouterWithEmbeddings(reader, emb, embReader, 30.0)

	model, routing, err := router.Route(context.Background(), "ds1", "login activity", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("expected confident routing thanks to embedding boost, got clarification: %+v", routing)
	}
	if model.BaseTable != "events" {
		t.Errorf("expected base=events (embedding-favored), got %q; selected=%v", model.BaseTable, routing.SelectedTables)
	}
	if routing.RankingMethod != "hybrid" {
		t.Errorf("expected RankingMethod=hybrid, got %q", routing.RankingMethod)
	}
	eventsCandidate, ok := findRoutingCandidate(routing.Candidates, "public.events")
	if !ok {
		t.Fatalf("expected public.events candidate, got %+v", routing.Candidates)
	}
	if eventsCandidate.EmbeddingScore <= 0 {
		t.Errorf("public.events embedding score = %v, want > 0", eventsCandidate.EmbeddingScore)
	}
	if eventsCandidate.TotalScore <= eventsCandidate.KeywordScore {
		t.Errorf("public.events scores = %+v, want embedding to boost total above keyword", eventsCandidate)
	}
}

func TestTableRouter_EmbeddingBoostFallbackToKeyword(t *testing.T) {
	// Embedder errors out — router must fall back to keyword scoring without
	// returning an error.
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "sales", TableType: "BASE TABLE"},
		},
		columns: []metadata.Column{
			{DatasourceID: "ds1", SchemaName: "public", TableName: "sales", ColumnName: "amount", DataType: "numeric"},
		},
	}
	emb := &fakeEmbedder{model: "fake", fallback: nil}
	embReader := &fakeEmbeddingReader{err: errors.New("network down")}
	router := NewTableRouterWithEmbeddings(reader, emb, embReader, 30.0)

	model, routing, err := router.Route(context.Background(), "ds1", "show sales", nil, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v, want fallback success", err)
	}
	if routing.NeedsClarification {
		t.Fatalf("keyword fallback should have produced confident routing, got %+v", routing)
	}
	if model.BaseTable != "sales" {
		t.Errorf("expected base=sales via keyword fallback, got %q", model.BaseTable)
	}
	if routing.RankingMethod != "keyword" {
		t.Errorf("expected RankingMethod=keyword on embedder failure, got %q", routing.RankingMethod)
	}
}
