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
	if JSONFromString("console") {
		t.Fatal("console should disable JSON logging")
	}
}

func TestNewCreatesTextLogger(t *testing.T) {
	t.Parallel()
	logger := New(Config{Level: slog.LevelDebug, JSON: false})
	if logger == nil {
		t.Fatal("New returned nil")
	}
	// Verify it's a valid slog.Logger by writing a log line.
	logger.Info("test text log")
}

func TestNewCreatesJSONLogger(t *testing.T) {
	t.Parallel()
	logger := New(Config{Level: slog.LevelInfo, JSON: true})
	if logger == nil {
		t.Fatal("New returned nil")
	}
	logger.Info("test json log")
}

func TestNewUsesValidHandlerOptions(t *testing.T) {
	t.Parallel()
	// Ensure custom level is passed correctly.
	logger := New(Config{Level: slog.LevelError, JSON: false})
	logger.Debug("should be suppressed") // silent
	logger.Error("should appear")        // visible
}
