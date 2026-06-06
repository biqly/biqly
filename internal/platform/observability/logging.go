package observability

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/platform/logger"
)

// SetupLogging builds a structured slog.Logger from BI_LOG_LEVEL / BI_LOG_FORMAT
// style strings, installs it as the process default, and returns it. Every
// binary should call this exactly once at startup so log level and JSON/text
// formatting are configured identically across the api, worker, auth, query,
// and catalog processes.
func SetupLogging(level, format string) *slog.Logger {
	l := logger.New(logger.Config{
		Level: logger.LevelFromString(level),
		JSON:  logger.JSONFromString(format),
	})
	slog.SetDefault(l)
	return l
}

type loggerCtxKey struct{}

// ContextWithLogger returns a context carrying a request-scoped logger.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFromContext retrieves the request-scoped logger from context, falling back
// to slog.Default() if none is found.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}
