package ai

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLogLLMCompletionIncludesAPIFlag(t *testing.T) {
	var buf strings.Builder
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	logLLMCompletion(context.Background(), "openai", "m", 50, GenerationResult{
		Content: "ok",
		Usage:   &TokenUsage{Prompt: 50, Completion: 10, Total: 60},
	})
	out := buf.String()
	if !strings.Contains(out, "tokens_from_api=true") {
		t.Fatalf("expected tokens_from_api=true in log: %s", out)
	}
	if !strings.Contains(out, "total_tokens=60") {
		t.Fatalf("expected total_tokens in log: %s", out)
	}
}
