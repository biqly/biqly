package logger

import (
	"io"
	"log/slog"
	"os"
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
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
