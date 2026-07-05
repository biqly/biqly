package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteInstructions(t *testing.T) {
	var b Builder
	t.Run("empty renders nothing", func(t *testing.T) {
		var buf bytes.Buffer
		b.writeInstructions(&buf, nil)
		if buf.Len() != 0 {
			t.Fatalf("expected empty output, got %q", buf.String())
		}
	})
	t.Run("renders section with title and body", func(t *testing.T) {
		var buf bytes.Buffer
		b.writeInstructions(&buf, []Instruction{
			{Title: "Fiscal year", Body: "starts in April"},
			{Body: "exclude test accounts"},
		})
		out := buf.String()
		if !strings.Contains(out, "## Business Rules") {
			t.Fatalf("missing section header: %q", out)
		}
		if !strings.Contains(out, "- **Fiscal year**: starts in April\n") {
			t.Fatalf("missing titled rule: %q", out)
		}
		if !strings.Contains(out, "- exclude test accounts\n") {
			t.Fatalf("missing untitled rule: %q", out)
		}
	})
	t.Run("skips fully empty entries", func(t *testing.T) {
		var buf bytes.Buffer
		b.writeInstructions(&buf, []Instruction{{Title: "  ", Body: "  "}})
		if buf.Len() != 0 {
			t.Fatalf("expected empty output, got %q", buf.String())
		}
	})
	t.Run("caps entries", func(t *testing.T) {
		rules := make([]Instruction, maxPromptInstructions+5)
		for i := range rules {
			rules[i] = Instruction{Body: "rule"}
		}
		var buf bytes.Buffer
		b.writeInstructions(&buf, rules)
		if got := strings.Count(buf.String(), "- rule\n"); got != maxPromptInstructions {
			t.Fatalf("expected %d rules, got %d", maxPromptInstructions, got)
		}
	})
}
