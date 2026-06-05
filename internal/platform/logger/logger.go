// Package logger provides structured logging utilities.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Config holds logger configuration.
type Config struct {
	Level slog.Level
	JSON  bool
}

// New creates a configured slog.Logger.
func New(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: cfg.Level,
	}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// LevelFromString parses BI_LOG_LEVEL-style values.
func LevelFromString(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// JSONFromString reports whether logs should be emitted as JSON.
func JSONFromString(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "text", "console":
		return false
	default:
		return true
	}
}
