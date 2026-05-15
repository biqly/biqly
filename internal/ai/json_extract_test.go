package ai

import (
	"encoding/json"
	"testing"
)

func TestExtractJSONObject_BracesInsideString(t *testing.T) {
	raw := `Here you go:
{"table_description": "Use } carefully", "columns": [{"name": "x", "description": "y"}]}`
	obj, ok := ExtractJSONObject(raw)
	if !ok {
		t.Fatal("expected object extracted")
	}
	var payload struct {
		TableDescription string `json:"table_description"`
		Columns          []any  `json:"columns"`
	}
	if err := json.Unmarshal([]byte(obj), &payload); err != nil {
		t.Fatalf("unmarshal: %v\nobj=%q", err, obj)
	}
	if payload.TableDescription != "Use } carefully" {
		t.Errorf("table_description = %q", payload.TableDescription)
	}
}

func TestTrimToJSONObject_Preamble(t *testing.T) {
	raw := "Sure.\n\n```json\n{\"select\": [{\"type\": \"dimension\", \"name\": \"id\"}], \"limit\": 5}\n```"
	got := TrimToJSONObject(raw)
	want := `{"select": [{"type": "dimension", "name": "id"}], "limit": 5}`
	if got != want {
		t.Errorf("TrimToJSONObject() = %q, want %q", got, want)
	}
}

func TestTrimToJSONObject_ReasoningPreamble(t *testing.T) {
	raw := "## Reasoning\n1. Intent: count orders\n2. Metric: row_count\n\n" +
		`{"select":[{"type":"metric","name":"row_count"}],"limit":100}`
	got := TrimToJSONObject(raw)
	want := `{"select":[{"type":"metric","name":"row_count"}],"limit":100}`
	if got != want {
		t.Errorf("TrimToJSONObject() = %q, want %q", got, want)
	}
}

func TestCleanAIResponseForJSON_BOM(t *testing.T) {
	raw := "\ufeff{\"a\": 1}"
	got := CleanAIResponseForJSON(raw)
	if got != `{"a": 1}` {
		t.Errorf("got %q", got)
	}
}
