package prompt

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

func TestPromptBuildIncludesFewShotExamples(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{
		ID:           "m",
		DatasourceID: "d",
		Name:         "public.orders",
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}
	examples := []FewShotExample{
		{Question: "kaç sipariş var", LogicalQuery: `{"select":[{"type":"metric","name":"row_count"}]}`},
		{Question: "    ", LogicalQuery: "junk"}, // should be skipped (empty question)
	}
	got := pb.Build(context.Background(), "ne kadar sipariş var", model, Config{
		Locale:   i18n.DefaultLocale,
		Dialect:  "postgres",
		Examples: examples,
	})

	if !strings.Contains(got, "Successful Past Queries") {
		t.Errorf("expected few-shot section header in prompt; got:\n%s", got)
	}
	if !strings.Contains(got, "kaç sipariş var") {
		t.Errorf("expected first example question to appear in prompt")
	}
	if strings.Count(got, `"row_count"`) < 2 {
		t.Errorf("expected the example logical_query to appear at least once alongside the static example")
	}
	if strings.Contains(got, "junk") {
		t.Errorf("blank-question example should have been skipped, but found 'junk' in prompt")
	}
}

func TestPromptBuildIncludesSoftDeleteRules(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x", BaseSchema: "public", BaseTable: "t"}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale: i18n.DefaultLocale,
	})
	if !strings.Contains(got, "Soft-delete") {
		t.Errorf("expected soft-delete rules in prompt, got excerpt:\n%s", truncatePrompt(got, 800))
	}
	if !strings.Contains(got, "is_not_null") {
		t.Errorf("expected is_not_null in soft-delete guidance")
	}
	if !strings.Contains(got, "delete_flag") {
		t.Errorf("expected delete_flag pattern in soft-delete guidance")
	}
	if !strings.Contains(got, "created_at_ts_month") {
		t.Errorf("expected prompt to mention created_at_ts_month stem example")
	}
}

func TestPromptBuildIncludesEnumValues(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{
		ID: "m", DatasourceID: "d", Name: "x", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{{
			Name:      "status",
			ColumnRef: "orders.status",
			Type:      "number",
			EnumValues: []semantic.EnumMapping{
				{RawValue: "1", Label: "pending", SortOrder: 0},
				{RawValue: "3", Label: "shipped", SortOrder: 1},
			},
		}},
	}
	got := pb.Build(context.Background(), "kaç sipariş kargolandı", model, Config{Locale: i18n.DefaultLocale})
	if !strings.Contains(got, "values: 1=pending, 3=shipped") {
		t.Errorf("expected enum values in dimension line, got excerpt:\n%s", truncatePrompt(got, 1200))
	}
}

func TestFormatEnumValuesEmpty(t *testing.T) {
	if got := formatEnumValues(nil); got != "" {
		t.Errorf("expected empty string for no enum values, got %q", got)
	}
}

func TestFormatEnumValuesTruncates(t *testing.T) {
	vals := make([]semantic.EnumMapping, maxEnumValuesPerLine+5)
	for i := range vals {
		vals[i] = semantic.EnumMapping{RawValue: "r", Label: "l"}
	}
	got := formatEnumValues(vals)
	if !strings.HasSuffix(got, ", …") {
		t.Errorf("expected truncation ellipsis, got %q", got)
	}
	if strings.Count(got, "=") != maxEnumValuesPerLine {
		t.Errorf("expected %d pairs, got %d in %q", maxEnumValuesPerLine, strings.Count(got, "="), got)
	}
}

func TestWithPooledBufferResetsAndIsConcurrentSafe(t *testing.T) {
	longText := strings.Repeat("x", 128)
	if got := withPooledBuffer(func(buf *bytes.Buffer) { buf.WriteString(longText) }); got != longText {
		t.Fatalf("withPooledBuffer long output = %q, want %q", got, longText)
	}
	if got := withPooledBuffer(func(buf *bytes.Buffer) { buf.WriteString("ok") }); got != "ok" {
		t.Fatalf("withPooledBuffer did not reset pooled buffer; got %q, want ok", got)
	}

	var wg sync.WaitGroup
	errs := make(chan string, 32)
	for i := range 32 {
		want := "prompt-" + strconv.Itoa(i) + strings.Repeat("!", i%7)
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := withPooledBuffer(func(buf *bytes.Buffer) { buf.WriteString(want) })
			if got != want {
				errs <- "got " + got + ", want " + want
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func truncatePrompt(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

func TestPromptBuildOmitsFewShotSectionWhenEmpty(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x"}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale: i18n.DefaultLocale,
	})
	if strings.Contains(got, "Successful Past Queries") {
		t.Errorf("expected no few-shot header when examples nil")
	}
}

func TestPromptBuildIncludesPriorTurns(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "public.orders"}
	turns := []ConversationTurn{
		{
			Question:      "ne kadar sipariş var",
			LogicalQuery:  `{"select":[{"type":"metric","name":"row_count"}]}`,
			Note:          "executed",
			ResultSummary: "May 20, 2026: 2,932 tweets",
		},
		{Question: "    "}, // blank — should be skipped
		{Question: "müşteri kırılımı yap"},
	}
	got := pb.Build(context.Background(), "şimdi son çeyrek için filtrele", model, Config{
		Locale:     i18n.DefaultLocale,
		PriorTurns: turns,
	})
	if !strings.Contains(got, "## Prior Turns in This Conversation") {
		t.Errorf("expected prior-turns header in prompt, got:\n%s", got)
	}
	if !strings.Contains(got, "ne kadar sipariş var") {
		t.Errorf("expected first prior turn question to appear")
	}
	if !strings.Contains(got, `"row_count"`) {
		t.Errorf("expected prior LogicalQuery JSON to appear")
	}
	if !strings.Contains(got, "Result: May 20, 2026: 2,932 tweets") {
		t.Errorf("expected prior result summary to appear")
	}
	if !strings.Contains(got, "müşteri kırılımı yap") {
		t.Errorf("expected later prior turn question to appear")
	}
	// Latest user question should still be present.
	if !strings.Contains(got, "şimdi son çeyrek için filtrele") {
		t.Errorf("expected the current user question to remain in prompt")
	}
}

func TestPromptBuildOmitsPriorTurnsHeaderWhenEmpty(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x"}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale: i18n.DefaultLocale,
	})
	if strings.Contains(got, "Prior Turns in This Conversation") {
		t.Errorf("expected no prior-turns header when turns nil")
	}
}

func TestPromptBuildIncludesDialectGuide(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "public.orders"}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale:  i18n.DefaultLocale,
		Dialect: "mysql",
	})
	if !strings.Contains(got, "## Datasource SQL Dialect") {
		t.Errorf("expected dialect section in prompt")
	}
	if !strings.Contains(got, "**this datasource**") {
		t.Errorf("expected active dialect marker for mysql")
	}
	if !strings.Contains(got, "LOWER(`name`) LIKE LOWER(?)") {
		t.Errorf("expected mysql contains compilation hint")
	}
}

func TestPromptBuildIncludesPlanningSteps(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x", BaseSchema: "public", BaseTable: "t"}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale: i18n.DefaultLocale,
	})
	if !strings.Contains(got, "## Planning Steps") {
		t.Errorf("expected planning-steps section in prompt")
	}
	if !strings.Contains(got, "1. Parse the question") {
		t.Errorf("expected first planning step")
	}
	if !strings.Contains(got, "8. Build and verify JSON") {
		t.Errorf("expected final planning step")
	}
	if !strings.Contains(got, "## Reasoning") {
		t.Errorf("expected optional reasoning block instructions")
	}
}

func TestPromptBuildIncludesFailureExamples(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x"}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale:  i18n.DefaultLocale,
		Dialect: "postgres",
	})
	if !strings.Contains(got, "## Examples — Common Mistakes") {
		t.Errorf("expected failure-examples section in prompt")
	}
	if !strings.Contains(got, "Raw SQL instead of LogicalQuery") {
		t.Errorf("expected anti-pattern title for raw SQL")
	}
	if !strings.Contains(got, `"having"`) {
		t.Errorf("expected having vs filters anti-pattern")
	}
}

func TestNormalizeDialectName(t *testing.T) {
	tests := map[string]string{
		"postgresql": "postgres",
		"Postgres":   "postgres",
		"mssql":      "sqlserver",
		"clickhouse": "clickhouse",
		"":           "postgres",
		"unknown":    "postgres",
	}
	for in, want := range tests {
		if got := normalizeDialectName(in); got != want {
			t.Errorf("normalizeDialectName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPromptBuildFiltersFewShotByLocale(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "public.orders",
		Metrics: []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}}}
	examples := []FewShotExample{
		{Question: "tr_only", LogicalQuery: `{"select":[{"type":"metric","name":"row_count"}]}`, Locale: "tr"},
		{Question: "en_only", LogicalQuery: `{"select":[{"type":"metric","name":"row_count"}]}`, Locale: "en"},
		{Question: "global", LogicalQuery: `{"select":[{"type":"metric","name":"row_count"}]}`, Locale: ""},
	}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale:   "tr",
		Examples: examples,
	})
	if !strings.Contains(got, "tr_only") {
		t.Errorf("expected tr-tagged example to survive tr locale filter")
	}
	if strings.Contains(got, "en_only") {
		t.Errorf("expected en-tagged example to be filtered out under tr locale")
	}
	if !strings.Contains(got, "global") {
		t.Errorf("expected locale-empty example to remain eligible under any locale")
	}
}

func TestPromptBuildLoadsLocaleSpecificTemplate(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x", BaseSchema: "public", BaseTable: "t"}
	// Verify the loader produces non-empty output for tr and that the locale-specific
	// template is wired (presence of the stable Turkish rules header proves the
	// bundle was read for the chosen locale).
	got := pb.Build(context.Background(), "q", model, Config{
		Locale: "tr",
	})
	if !strings.Contains(got, "Bir Business Intelligence sorgu motorusun") {
		t.Errorf("expected tr-locale prompt to include the embedded rules header")
	}
}

func TestPromptBuildIncludesSampleData(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "public.orders"}
	samples := []TableSample{
		{
			Schema: "public",
			Table:  "orders",
			Rows: []map[string]any{
				{"id": 1, "status": "shipped"},
				{"id": 2, "status": "pending"},
			},
		},
		{Schema: "public", Table: "empty", Rows: nil}, // skipped
	}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale:  i18n.DefaultLocale,
		Samples: samples,
	})
	if !strings.Contains(got, "## Sample Data") {
		t.Errorf("expected '## Sample Data' header in prompt")
	}
	if !strings.Contains(got, "### public.orders") {
		t.Errorf("expected 'public.orders' subheader in prompt")
	}
	if !strings.Contains(got, `"shipped"`) {
		t.Errorf("expected sample row value to appear in prompt")
	}
	if strings.Contains(got, "### public.empty") {
		t.Errorf("empty table should be skipped, got header in prompt")
	}
}
