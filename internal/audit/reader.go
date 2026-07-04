package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
)

// Reader loads persisted audit events for the query-audit detail API.
type Reader struct {
	db *sql.DB
}

// NewReader creates an audit event reader. Returns nil when db is nil.
func NewReader(db *sql.DB) *Reader {
	if db == nil {
		return nil
	}
	return &Reader{db: db}
}

const queryEventColumns = `id, COALESCE(user_id::text, ''), event_type,
	COALESCE(datasource_id::text, ''), COALESCE(model_id::text, ''), details, created_at`

// QueryExecutionEvent returns the most recent query execution audit event
// linked to the given query_history row, or nil when none exists.
func (r *Reader) QueryExecutionEvent(ctx context.Context, historyID string) (*Event, error) {
	if r == nil {
		return nil, nil //nolint:nilnil // optional result
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+queryEventColumns+`
		FROM audit_events
		WHERE event_type IN ($1, $2) AND details->>'history_id' = $3
		ORDER BY created_at DESC
		LIMIT 1
	`, string(EventQueryExecuted), string(EventQueryFailed), historyID)
	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // optional result
	}
	if err != nil {
		return nil, fmt.Errorf("load query audit event: %w", err)
	}
	return event, nil
}

// ListQueryExecutionEvents returns recent query execution audit events,
// newest first.
func (r *Reader) ListQueryExecutionEvents(ctx context.Context, limit int) ([]Event, error) {
	if r == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+queryEventColumns+`
		FROM audit_events
		WHERE event_type IN ($1, $2)
		ORDER BY created_at DESC
		LIMIT $3
	`, string(EventQueryExecuted), string(EventQueryFailed), limit)
	if err != nil {
		return nil, fmt.Errorf("list query audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	events := make([]Event, 0, min(limit, 500))
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan query audit event: %w", err)
		}
		events = append(events, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list query audit events: %w", err)
	}
	return events, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (*Event, error) {
	var event Event
	var eventType string
	var details sql.NullString
	if err := row.Scan(&event.ID, &event.UserID, &eventType,
		&event.DatasourceID, &event.ModelID, &details, &event.Timestamp); err != nil {
		return nil, err
	}
	event.EventType = EventType(eventType)
	if details.Valid && details.String != "" {
		if err := sonic.ConfigStd.Unmarshal([]byte(details.String), &event.Details); err != nil {
			return nil, fmt.Errorf("unmarshal details: %w", err)
		}
	}
	return &event, nil
}
