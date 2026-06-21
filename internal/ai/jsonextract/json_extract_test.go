package jsonextract

import (
	"testing"

	"github.com/bytedance/sonic"
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
	if err := sonic.ConfigStd.Unmarshal([]byte(obj), &payload); err != nil {
		t.Fatalf("unmarshal: %v\nobj=%q", err, obj)
	}
	if payload.TableDescription != "Use } carefully" {
		t.Errorf("table_description = %q", payload.TableDescription)
	}
}

func TestExtractJSONObject_NoBraces(t *testing.T) {
	_, ok := ExtractJSONObject("just some text without braces")
	if ok {
		t.Error("expected false for input without braces")
	}
}

func TestExtractJSONObject_EmptyString(t *testing.T) {
	_, ok := ExtractJSONObject("")
	if ok {
		t.Error("expected false for empty string")
	}
}

func TestExtractJSONObject_UnclosedBrace(t *testing.T) {
	_, ok := ExtractJSONObject(`{"a": 1, "b": 2`)
	if ok {
		t.Error("expected false for unclosed object")
	}
}

func TestExtractJSONObject_EscapedQuotesInsideString(t *testing.T) {
	raw := `{"msg": "hello \"world\"", "value": 42}`
	obj, ok := ExtractJSONObject(raw)
	if !ok {
		t.Fatal("expected object extracted")
	}
	if obj != raw {
		t.Errorf("got %q, want %q", obj, raw)
	}
}

func TestExtractJSONObject_BackslashInString(t *testing.T) {
	raw := `{"path": "c:\\users\	est", "ok": true}`
	obj, ok := ExtractJSONObject(raw)
	if !ok {
		t.Fatal("expected object extracted")
	}
	if obj != raw {
		t.Errorf("got %q, want %q", obj, raw)
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

func TestTrimToJSONObject_NoJSONObject(t *testing.T) {
	got := TrimToJSONObject("Just some text without any JSON object")
	want := "Just some text without any JSON object"
	if got != want {
		t.Errorf("TrimToJSONObject() = %q, want %q", got, want)
	}
}

func TestTrimToJSONObject_Empty(t *testing.T) {
	got := TrimToJSONObject("")
	if got != "" {
		t.Errorf("TrimToJSONObject() = %q, want empty", got)
	}
}

func TestCleanAIResponseForJSON_BOM(t *testing.T) {
	raw := "\ufeff{\"a\": 1}"
	got := CleanAIResponseForJSON(raw)
	if got != `{"a": 1}` {
		t.Errorf("got %q", got)
	}
}

func TestTrimGenericCodeFenceContent_WithJSON(t *testing.T) {
	input := "json\n{\"key\": \"value\"}\n```"
	got := trimGenericCodeFenceContent(input)
	want := "{\"key\": \"value\"}"
	if got != want {
		t.Errorf("trimGenericCodeFenceContent() = %q, want %q", got, want)
	}
}

func TestTrimGenericCodeFenceContent_WithJSONUppercase(t *testing.T) {
	input := "JSON\n{\"key\": \"value\"}\n```"
	got := trimGenericCodeFenceContent(input)
	want := "{\"key\": \"value\"}"
	if got != want {
		t.Errorf("trimGenericCodeFenceContent() = %q, want %q", got, want)
	}
}

func TestTrimGenericCodeFenceContent_NonJSONLang(t *testing.T) {
	input := "python\nprint('hello')\n```"
	got := trimGenericCodeFenceContent(input)
	// Not JSON, so first line is NOT stripped; trimCodeFenceContent strips from ```
	want := "python\nprint('hello')"
	if got != want {
		t.Errorf("trimGenericCodeFenceContent() = %q, want %q", got, want)
	}
}

func TestTrimGenericCodeFenceContent_NoNewline(t *testing.T) {
	input := "json"
	got := trimGenericCodeFenceContent(input)
	// No newline, skips the if block, trimCodeFenceContent doesn't find ```, returns as-is
	want := "json"
	if got != want {
		t.Errorf("trimGenericCodeFenceContent() = %q, want %q", got, want)
	}
}

func TestTrimGenericCodeFenceContent_LongFirstLine(t *testing.T) {
	// First line >= 24 chars, so the if nl < 24 condition fails
	firstLine := "a23456789012345678901234" // 24 chars
	input := firstLine + "\nrest of content\n```"
	got := trimGenericCodeFenceContent(input)
	// Should keep the first line since nl >= 24
	want := firstLine + "\nrest of content"
	if got != want {
		t.Errorf("trimGenericCodeFenceContent() = %q, want %q", got, want)
	}
}

func TestTrimGenericCodeFenceContent_Empty(t *testing.T) {
	got := trimGenericCodeFenceContent("")
	if got != "" {
		t.Errorf("trimGenericCodeFenceContent() = %q, want empty", got)
	}
}

func TestTrimGenericCodeFenceContent_WithNoClosingFence(t *testing.T) {
	input := "json\n{\"key\": \"value\"}"
	got := trimGenericCodeFenceContent(input)
	want := "{\"key\": \"value\"}"
	if got != want {
		t.Errorf("trimGenericCodeFenceContent() = %q, want %q", got, want)
	}
}

func TestTrimGenericCodeFenceContent_JustCodeFence(t *testing.T) {
	input := "json\n\n```"
	got := trimGenericCodeFenceContent(input)
	// After stripping "json\n", we have "\n```", trimCodeFenceContent finds ``` and returns ""
	want := ""
	if got != want {
		t.Errorf("trimGenericCodeFenceContent() = %q, want %q", got, want)
	}
}
