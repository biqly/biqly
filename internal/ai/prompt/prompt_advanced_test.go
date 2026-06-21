package prompt

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/query"
)

func TestRepairStrategyEN(t *testing.T) {
	t.Parallel()
	if got := RepairStrategy(i18n.DefaultLocale, 1); got == "" {
		t.Fatal("expected non-empty repair strategy")
	}
	if got := RepairStrategy(i18n.DefaultLocale, 2); got == "" {
		t.Fatal("expected non-empty repair strategy for attempt 2")
	}
	if got := RepairStrategy(i18n.DefaultLocale, 3); got == "" {
		t.Fatal("expected non-empty repair strategy for attempt 3")
	}
}

func TestRepairStrategyTR(t *testing.T) {
	t.Parallel()
	if got := RepairStrategy(i18n.LocaleTR, 1); got == "" {
		t.Fatal("expected non-empty TR repair strategy")
	}
	if got := RepairStrategy(i18n.LocaleTR, 2); got == "" {
		t.Fatal("expected non-empty TR repair strategy")
	}
	if got := RepairStrategy(i18n.LocaleTR, 99); got == "" {
		t.Fatal("expected non-empty TR default repair strategy")
	}
}

func TestBuildRepairPromptWithRetryFallback(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildRepairPrompt(ctx, i18n.DefaultLocale,
		"original prompt",
		`{"select":[{"type":"metric","name":"revenue"}]}`,
		query.ValidationErrors{
			{Field: "revenue", Code: "UNKNOWN_METRIC", Message: "unknown metric"},
		},
		1,
	)
	if got == "" {
		t.Fatal("expected non-empty repair prompt")
	}
}

func TestBuildRepairPromptWithAlternatives(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildRepairPrompt(ctx, i18n.DefaultLocale,
		"original",
		"bad response",
		query.ValidationErrors{
			{
				Field:               "revenue",
				Code:                "UNKNOWN_METRIC",
				Message:             "unknown metric",
				AllowedAlternatives: []string{"row_count", "total_revenue"},
			},
		},
		2,
	)
	if got == "" {
		t.Fatal("expected non-empty repair prompt")
	}
}

func TestBuildRepairPromptWithMultipleErrors(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildRepairPrompt(ctx, i18n.DefaultLocale,
		"base",
		"bad",
		query.ValidationErrors{
			{Field: "revenue", Code: "UNKNOWN_METRIC", Message: "unknown metric"},
			{Field: "country", Code: "UNKNOWN_DIMENSION", Message: "unknown dimension"},
		},
		3,
	)
	if got == "" {
		t.Fatal("expected non-empty repair prompt")
	}
}

func TestBuildAmbiguityAnalysis(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	model := &semantic.SemanticModel{
		Name:    "orders",
		Metrics: []semantic.Metric{{Name: "revenue", Aggregation: "sum", Expression: "amount"}},
	}
	got := pb.BuildAmbiguityAnalysis(ctx, i18n.DefaultLocale, "how much revenue?", model, nil)
	if got == "" {
		t.Fatal("expected non-empty ambiguity analysis prompt")
	}
}

func TestBuildAmbiguityAnalysisNilModel(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildAmbiguityAnalysis(ctx, i18n.DefaultLocale, "question?", nil, nil)
	if got == "" {
		t.Fatal("expected non-empty ambiguity analysis prompt")
	}
}

func TestBuildAmbiguityAnalysisWithGlossary(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	model := &semantic.SemanticModel{
		Name: "sales",
		Dimensions: []semantic.Dimension{
			{Name: "country", Type: "text", ColumnRef: "countries.name"},
		},
	}
	got := pb.BuildAmbiguityAnalysis(ctx, i18n.DefaultLocale, "sales by country",
		model, []GlossaryEntry{{Term: "ülke", Definition: "country"}})
	if got == "" {
		t.Fatal("expected non-empty ambiguity analysis with glossary")
	}
}

func TestBuildClarificationBasic(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	model := &semantic.SemanticModel{Name: "orders"}
	got := pb.BuildClarification(ctx, i18n.DefaultLocale, "show me data", model, "ambiguous column reference")
	if got == "" {
		t.Fatal("expected non-empty clarification prompt")
	}
}

func TestBuildClarificationNilModel(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildClarification(ctx, i18n.DefaultLocale, "question?", nil, "")
	if got == "" {
		t.Fatal("expected non-empty clarification prompt for nil model")
	}
}

func TestBuildRetryBasic(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildRetry(ctx, i18n.DefaultLocale,
		"original prompt",
		`{"select":[{"type":"metric","name":"revenue"}]}`,
		"validation error: unknown metric")
	if got == "" {
		t.Fatal("expected non-empty retry prompt")
	}
}

func TestWriteCompositeContextNil(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	var buf bytes.Buffer
	pb.writeCompositeContext(&buf, nil)
	if buf.String() != "" {
		t.Fatalf("expected empty for nil composite, got %q", buf.String())
	}
}

func TestWriteCompositeContextFull(t *testing.T) {
	t.Parallel()
	pb := &Builder{}
	var buf bytes.Buffer
	pb.writeCompositeContext(&buf, &CompositeContext{
		Name: "cross-sales",
		Components: []CompositeComponentHint{
			{Alias: "sales", Label: "Sales CRM"},
			{Alias: "support", Label: ""},
		},
		CrossModelJoins: []CompositeJoinHint{
			{FromModel: "sales", ToModel: "support", Relationship: "one-to-many"},
			{FromModel: "support", ToModel: "billing", Relationship: ""},
		},
		CanonicalDate:     "order_date",
		RenamedDimensions: []string{"sales_name", "support_name"},
	})
	result := buf.String()
	if result == "" {
		t.Fatal("expected content for composite context")
	}
	if !strings.Contains(result, "Composite Model") {
		t.Fatal("expected 'Composite Model' header")
	}
	if !strings.Contains(result, "order_date") {
		t.Fatal("expected canonical date in output")
	}
}

func TestPromptTemplateEmbedded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	loc := i18n.DefaultLocale

	for _, name := range KnownPromptTemplateNames() {
		tmpl := promptTemplate(ctx, loc, name)
		if tmpl == "" {
			t.Logf("template %q resolved empty — may be expected", name)
		}
	}
}

// emptyTemplateStore returns empty content to force fallback paths.
type emptyTemplateStore struct{}

func (emptyTemplateStore) Template(_ context.Context, _ i18n.Locale, _ string) string { return "" }
func (emptyTemplateStore) Snapshot(_ context.Context, _ i18n.Locale, _ string) TemplateSnapshot {
	return TemplateSnapshot{}
}
func (emptyTemplateStore) SnapshotForUser(_ context.Context, _ string, _ i18n.Locale, _ string) TemplateSnapshot {
	return TemplateSnapshot{}
}

func TestBuildClarificationFallback(t *testing.T) {
	// Save original store and restore afterward
	origStore := getActivePromptStore()
	defer SetPromptTemplateStore(origStore)

	SetPromptTemplateStore(emptyTemplateStore{})

	pb := &Builder{}
	ctx := context.Background()
	model := &semantic.SemanticModel{
		Name: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country"},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Aggregation: "sum", Expression: "amount"},
		},
	}
	got := pb.BuildClarification(ctx, i18n.DefaultLocale, "show me revenue by country", model, "ambiguous reference")
	if got == "" {
		t.Fatal("expected non-empty fallback clarification")
	}
	if !strings.Contains(got, "clarifying question") && !strings.Contains(got, "Business Intelligence assistant") {
		t.Fatalf("fallback clarification = %q, expected fallback content", got)
	}
}

func TestBuildClarificationFallbackNilModel(t *testing.T) {
	origStore := getActivePromptStore()
	defer SetPromptTemplateStore(origStore)
	SetPromptTemplateStore(emptyTemplateStore{})

	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildClarification(ctx, i18n.DefaultLocale, "question?", &semantic.SemanticModel{}, "")
	if got == "" {
		t.Fatal("expected non-empty fallback clarification for nil model")
	}
}

func TestBuildRetryFallback(t *testing.T) {
	origStore := getActivePromptStore()
	defer SetPromptTemplateStore(origStore)
	SetPromptTemplateStore(emptyTemplateStore{})

	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildRetry(ctx, i18n.DefaultLocale,
		"original prompt",
		`{"select":[{"type":"metric","name":"revenue"}]}`,
		"validation error")
	if got == "" {
		t.Fatal("expected non-empty fallback retry prompt")
	}
	if !strings.Contains(got, "Previous Attempt") {
		t.Fatalf("fallback retry = %q, expected 'Previous Attempt'", got)
	}
}

func TestBuildRepairPromptFallback(t *testing.T) {
	origStore := getActivePromptStore()
	defer SetPromptTemplateStore(origStore)
	SetPromptTemplateStore(emptyTemplateStore{})

	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildRepairPrompt(ctx, i18n.DefaultLocale,
		"base prompt",
		"bad response",
		query.ValidationErrors{
			{Field: "revenue", Code: "UNKNOWN_METRIC", Message: "unknown metric"},
		},
		1,
	)
	if got == "" {
		t.Fatal("expected non-empty fallback repair prompt")
	}
	if !strings.Contains(got, "Previous Attempt") {
		t.Fatalf("fallback repair prompt = %q, expected 'Previous Attempt'", got)
	}
}

func TestBuildRepairPromptFallbackNoCode(t *testing.T) {
	origStore := getActivePromptStore()
	defer SetPromptTemplateStore(origStore)
	SetPromptTemplateStore(emptyTemplateStore{})

	pb := &Builder{}
	ctx := context.Background()
	got := pb.BuildRepairPrompt(ctx, i18n.DefaultLocale,
		"base",
		"bad",
		query.ValidationErrors{
			{Field: "revenue", Message: "field not found"},
		},
		2,
	)
	if got == "" {
		t.Fatal("expected non-empty fallback repair prompt without code")
	}
}
