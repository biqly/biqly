package audit

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/testutil"
)

func TestReader_ListQueryExecutionEventsPage_NilReader(t *testing.T) {
	var r *Reader
	events, total, err := r.ListQueryExecutionEventsPage(context.Background(), QueryEventPage{})
	require.NoError(t, err)
	require.Nil(t, events)
	require.Equal(t, 0, total)
}

func TestReader_ListQueryExecutionEventsPage_ScopedEmptyDatasourceIDs(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	r := NewReader(db)
	events, total, err := r.ListQueryExecutionEventsPage(context.Background(), QueryEventPage{Scoped: true})
	require.NoError(t, err)
	require.Empty(t, events)
	require.Equal(t, 0, total)
}

func TestReader_ListQueryExecutionEventsPage_PaginationSearchScope(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	r := NewReader(db)

	datasourceID := uuid.NewString()
	otherDatasourceID := uuid.NewString()
	base := time.Now().UTC().Add(-time.Hour)

	insert := func(eventType EventType, detail string, offset time.Duration) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO audit_events (event_type, datasource_id, details, created_at)
			VALUES ($1, $2::uuid, $3::jsonb, $4)
		`, string(eventType), datasourceID, `{"note":"`+detail+`"}`, base.Add(offset))
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_events WHERE datasource_id = $1::uuid`, datasourceID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_events WHERE datasource_id = $1::uuid`, otherDatasourceID)
	})

	insert(EventQueryExecuted, "alpha", 0)
	insert(EventQueryExecuted, "bravo", time.Second)
	insert(EventQueryFailed, "charlie-marker", 2*time.Second)
	// A row scoped to a different datasource must never leak into this
	// datasource's page.
	_, err := db.ExecContext(ctx, `
		INSERT INTO audit_events (event_type, datasource_id, details, created_at)
		VALUES ($1, $2::uuid, $3::jsonb, $4)
	`, string(EventQueryExecuted), otherDatasourceID, `{"note":"other"}`, base.Add(3*time.Second))
	require.NoError(t, err)

	events, total, err := r.ListQueryExecutionEventsPage(ctx, QueryEventPage{
		Scoped:        true,
		DatasourceIDs: []string{datasourceID},
		Limit:         2,
		Offset:        0,
	})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, events, 2)
	// Newest first.
	require.Contains(t, events[0].Details["note"], "charlie-marker")

	page2, total2, err := r.ListQueryExecutionEventsPage(ctx, QueryEventPage{
		Scoped:        true,
		DatasourceIDs: []string{datasourceID},
		Limit:         2,
		Offset:        2,
	})
	require.NoError(t, err)
	require.Equal(t, 3, total2)
	require.Len(t, page2, 1)

	searched, searchedTotal, err := r.ListQueryExecutionEventsPage(ctx, QueryEventPage{
		Scoped:        true,
		DatasourceIDs: []string{datasourceID},
		Search:        "charlie-marker",
	})
	require.NoError(t, err)
	require.Equal(t, 1, searchedTotal)
	require.Len(t, searched, 1)
	require.Equal(t, EventQueryFailed, searched[0].EventType)
}
