// Package logger provides structured logging utilities.
package logger

import (
	"io"
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

// NewWithFile creates a logger that writes to both stdout and a file.
func NewWithFile(cfg Config, filepath string) (*slog.Logger, io.Closer, error) {
	// 0600: logs may contain DSN fragments, query SQL with literal values,
	// and other operational data that should never be world-readable.
	//nolint:gosec // G304: filepath is provided by application config, not user input
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, err
	}

	opts := &slog.HandlerOptions{Level: cfg.Level}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(io.MultiWriter(os.Stdout, f), opts)
	} else {
		handler = slog.NewTextHandler(io.MultiWriter(os.Stdout, f), opts)
	}

	return slog.New(handler), f, nil
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
