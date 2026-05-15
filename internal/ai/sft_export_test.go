package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

func TestCanonicalTrainingLogicalQueryStripsRuntimeIDs(t *testing.T) {
	raw := []byte(`{"datasource_id":"ds-1","model_id":"orders","select":[{"type":"metric","name":"row_count"}],"limit":0}`)
	got, err := canonicalTrainingLogicalQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "ds-1") || strings.Contains(got, "model_id") {
		t.Errorf("expected runtime ids stripped, got %s", got)
	}
	if !strings.Contains(got, `"limit":100`) {
		t.Errorf("expected default limit 100, got %s", got)
	}
}

func TestSplitBucketStable(t *testing.T) {
	a := SplitBucket("kaç sipariş var", 0.8, 0.1)
	b := SplitBucket("kaç sipariş var", 0.8, 0.1)
	if a != b {
		t.Fatalf("split not stable: %q vs %q", a, b)
	}
	if a != "train" && a != "validation" && a != "hard_eval" {
		t.Fatalf("unexpected split %q", a)
	}
}

func TestRenderGemma4SFTText(t *testing.T) {
	text := renderGemma4SFTText("sys", "user prompt", `{"select":[]}`)
	if !strings.Contains(text, "<|turn>user") {
		t.Errorf("missing user turn: %s", text)
	}
	if !strings.Contains(text, "<|turn>model") {
		t.Errorf("missing model turn: %s", text)
	}
	if strings.Contains(text, "<bos>") {
		t.Errorf("should not include bos in text field: %s", text)
	}
}

func TestValidateTrainingLogicalQuery(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "m",
		Metrics: []semantic.Metric{
			{Name: "row_count", Aggregation: "count", Expression: "*"},
		},
	}
	raw := []byte(`{"select":[{"type":"metric","name":"row_count"}]}`)
	if err := validateTrainingLogicalQuery(raw, model, query.NewValidator(1000)); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"select":[{"type":"metric","name":"missing"}]}`)
	if err := validateTrainingLogicalQuery(bad, model, query.NewValidator(1000)); err == nil {
		t.Fatal("expected validation error for unknown metric")
	}
}

func TestSFTRecordJSONShape(t *testing.T) {
	rec := SFTRecord{
		Messages: []SFTMessage{
			{Role: "system", Content: sftSystemMessage},
			{Role: "user", Content: "q"},
			{Role: "assistant", Content: `{"select":[{"type":"metric","name":"row_count"}],"limit":100}`},
		},
		Text: "x",
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	msgs, ok := decoded["messages"].([]any)
	if !ok || len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %v", decoded["messages"])
	}
}
