package logger

import (
	"log/slog"
	"testing"
)

func TestLevelFromString(t *testing.T) {
	t.Parallel()
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"INFO":    slog.LevelInfo,
		"warning": slog.LevelWarn,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
		"":        slog.LevelInfo,
		"nope":    slog.LevelInfo,
	}
	for input, want := range cases {
		if got := LevelFromString(input); got != want {
			t.Fatalf("LevelFromString(%q): got %v, want %v", input, got, want)
		}
	}
}

func TestJSONFromString(t *testing.T) {
	t.Parallel()
	if JSONFromString("text") {
		t.Fatal("text should disable JSON logging")
	}
	if !JSONFromString("json") {
		t.Fatal("json should enable JSON logging")
	}
	if !JSONFromString("") {
		t.Fatal("empty format should default to JSON logging")
	}
}
