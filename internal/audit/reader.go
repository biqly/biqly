package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/platform/db/pgarray"
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

// queryEventWhere filters by event type plus two optional, always-bound
// conditions (datasource scoping, free-text search) that short-circuit via a
// sentinel value when unused. Fixed placeholder positions keep the SQL text
// a compile-time constant, so no query string is ever built at runtime.
const queryEventWhere = `WHERE event_type = ANY($1)
	AND ($2 = false OR datasource_id::text = ANY($3))
	AND ($4 = '' OR details::text ILIKE '%' || $4 || '%'
		OR COALESCE(datasource_id::text, '') ILIKE '%' || $4 || '%'
		OR COALESCE(user_id::text, '') ILIKE '%' || $4 || '%')`

const countQueryEventsSQL = `SELECT COUNT(*) FROM audit_events ` + queryEventWhere

const listQueryEventsSQL = `SELECT ` + queryEventColumns + `
	FROM audit_events
	` + queryEventWhere + `
	ORDER BY created_at DESC
	LIMIT $5 OFFSET $6`

// QueryEventPage is the filter/window for ListQueryExecutionEventsPage.
type QueryEventPage struct {
	Limit  int
	Offset int
	// Search matches case-insensitively against the event details JSON (user
	// email, channel, status, error...) and the datasource/user ids.
	Search string
	// Scoped restricts results to DatasourceIDs (workspace scoping). Scoped
	// with an empty list matches nothing.
	Scoped        bool
	DatasourceIDs []string
}

// ListQueryExecutionEventsPage returns one page of query execution audit
// events (newest first) plus the total row count for the same filter, so the
// admin UI can paginate server-side instead of loading the whole log.
func (r *Reader) ListQueryExecutionEventsPage(ctx context.Context, page QueryEventPage) ([]Event, int, error) {
	if r == nil {
		return nil, 0, nil
	}
	if page.Scoped && len(page.DatasourceIDs) == 0 {
		return []Event{}, 0, nil
	}
	const (
		maxListLimit     = 500
		defaultListLimit = 100
	)
	if page.Limit <= 0 || page.Limit > maxListLimit {
		page.Limit = defaultListLimit
	}
	page.Offset = max(page.Offset, 0)

	eventTypes := pgarray.Strings([]string{string(EventQueryExecuted), string(EventQueryFailed)})
	search := strings.TrimSpace(page.Search)
	args := []any{eventTypes, page.Scoped, pgarray.Strings(page.DatasourceIDs), search}

	var total int
	if err := r.db.QueryRowContext(ctx, countQueryEventsSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count query audit events: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, listQueryEventsSQL, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list query audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	// Capacity is a constant so the allocation never depends on user input
	// (CodeQL go/uncontrolled-allocation-size).
	events := make([]Event, 0, defaultListLimit)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan query audit event: %w", err)
		}
		events = append(events, *event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list query audit events: %w", err)
	}
	return events, total, nil
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
