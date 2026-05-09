// Package audit provides query audit logging.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// EventType enumerates audit event types.
type EventType string

// Audit event types.
const (
	EventQueryExecuted  EventType = "query_executed"
	EventQueryCompiled  EventType = "query_compiled"
	EventQueryFailed    EventType = "query_failed"
	EventDatasourceSync EventType = "datasource_sync"
	EventPermissionDeny EventType = "permission_denied"
	EventAIGenerated    EventType = "ai_generated"
)

// Event represents an audit log entry.
type Event struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	EventType    EventType         `json:"event_type"`
	DatasourceID string            `json:"datasource_id"`
	ModelID      string            `json:"model_id"`
	Details      map[string]any    `json:"details,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// Logger writes audit events to structured logs.
type Logger struct {
	logger *slog.Logger
}

// NewLogger creates a new audit logger.
func NewLogger(logger *slog.Logger) *Logger {
	return &Logger{logger: logger}
}

// Log records an audit event.
func (l *Logger) Log(ctx context.Context, event Event) {
	attrs := []any{
		slog.String("event_type", string(event.EventType)),
		slog.String("user_id", event.UserID),
		slog.String("datasource_id", event.DatasourceID),
		slog.String("model_id", event.ModelID),
	}

	if len(event.Details) > 0 {
		attrs = append(attrs, slog.Any("details", event.Details))
	}

	switch event.EventType {
	case EventQueryFailed, EventPermissionDeny:
		l.logger.ErrorContext(ctx, "audit", attrs...)
	case EventQueryExecuted, EventQueryCompiled, EventDatasourceSync, EventAIGenerated:
		l.logger.InfoContext(ctx, "audit", attrs...)
	default:
		l.logger.InfoContext(ctx, "audit", attrs...)
	}
}

// Marshal serializes an event to JSON for storage.
func Marshal(event Event) ([]byte, error) {
	return json.Marshal(event)
}
