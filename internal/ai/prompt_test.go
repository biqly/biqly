package ai

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestPromptBuildIncludesFewShotExamples(t *testing.T) {
	pb := &PromptBuilder{}
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
	got := pb.Build("ne kadar sipariş var", model, 0, examples, nil, nil)

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

func TestPromptBuildOmitsFewShotSectionWhenEmpty(t *testing.T) {
	pb := &PromptBuilder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x"}
	got := pb.Build("q", model, 0, nil, nil, nil)
	if strings.Contains(got, "Successful Past Queries") {
		t.Errorf("expected no few-shot header when examples nil")
	}
}

func TestPromptBuildIncludesPriorTurns(t *testing.T) {
	pb := &PromptBuilder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "public.orders"}
	turns := []ConversationTurn{
		{Question: "ne kadar sipariş var", LogicalQuery: `{"select":[{"type":"metric","name":"row_count"}]}`, Note: "executed"},
		{Question: "    "}, // blank — should be skipped
		{Question: "müşteri kırılımı yap"},
	}
	got := pb.Build("şimdi son çeyrek için filtrele", model, 0, nil, nil, turns)
	if !strings.Contains(got, "## Prior Turns in This Conversation") {
		t.Errorf("expected prior-turns header in prompt, got:\n%s", got)
	}
	if !strings.Contains(got, "ne kadar sipariş var") {
		t.Errorf("expected first prior turn question to appear")
	}
	if !strings.Contains(got, `"row_count"`) {
		t.Errorf("expected prior LogicalQuery JSON to appear")
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
	pb := &PromptBuilder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x"}
	got := pb.Build("q", model, 0, nil, nil, nil)
	if strings.Contains(got, "Prior Turns in This Conversation") {
		t.Errorf("expected no prior-turns header when turns nil")
	}
}

func TestPromptBuildIncludesSampleData(t *testing.T) {
	pb := &PromptBuilder{}
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
	got := pb.Build("q", model, 0, nil, samples, nil)
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
