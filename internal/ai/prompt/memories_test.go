package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteMemories(t *testing.T) {
	var b Builder
	t.Run("empty renders nothing", func(t *testing.T) {
		var buf bytes.Buffer
		b.writeMemories(&buf, nil)
		if buf.Len() != 0 {
			t.Fatalf("expected empty output, got %q", buf.String())
		}
	})
	t.Run("renders section with facts", func(t *testing.T) {
		var buf bytes.Buffer
		b.writeMemories(&buf, []string{"revenue means net_amount", "fiscal year starts in April"})
		out := buf.String()
		if !strings.Contains(out, "## Remembered Facts") {
			t.Fatalf("missing section header: %q", out)
		}
		if !strings.Contains(out, "- revenue means net_amount\n") || !strings.Contains(out, "- fiscal year starts in April\n") {
			t.Fatalf("missing facts: %q", out)
		}
	})
	t.Run("caps entries", func(t *testing.T) {
		facts := make([]string, maxPromptMemories+5)
		for i := range facts {
			facts[i] = "fact"
		}
		var buf bytes.Buffer
		b.writeMemories(&buf, facts)
		if got := strings.Count(buf.String(), "- fact\n"); got != maxPromptMemories {
			t.Fatalf("expected %d facts, got %d", maxPromptMemories, got)
		}
	})
}
