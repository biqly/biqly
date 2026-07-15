package dashboard

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dashboardGetCols matches the column order scanned by Repository.Get.
var dashboardGetCols = []string{
	"id", "workspace_id", "name", "description", "widgets", "created_at", "updated_at",
}

// newResolverConn builds a mock connection whose share lookup and dashboard Get
// both succeed, returning a dashboard carrying testWidgetsJSON.
func newResolverConn(now time.Time) *mockShareConn {
	return &mockShareConn{
		shareRows: &mockRows{
			cols: shareSelectCols,
			rows: [][]driver.Value{{"share-1", "dash-1", "ws-1", "hash", "user-1", now, nil, nil}},
		},
		dashboardRows: &mockRows{
			cols: dashboardGetCols,
			rows: [][]driver.Value{{"dash-1", "ws-1", "pub", "desc", []byte(testWidgetsJSON), now, now}},
		},
	}
}

func TestNewPublicResolver(t *testing.T) {
	assert.NotNil(t, NewPublicResolver(openMockShareDB(t, &mockShareConn{})))
}

func TestPublicResolverResolveDashboardMock(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	// Valid token → sanitized dashboard.
	res := NewPublicResolver(openMockShareDB(t, newResolverConn(now)))
	view, err := res.ResolveDashboard(ctx, "plain-token")
	require.NoError(t, err)
	assert.Equal(t, "dash-1", view.Dashboard.ID)
	assert.NotContains(t, string(view.Dashboard.Widgets), "logical_query")
	assert.NotContains(t, string(view.Dashboard.Widgets), "saved_query_id")

	// Bad token: share lookup returns no rows → ErrShareNotFound.
	conn := newResolverConn(now)
	conn.shareRows.rows = nil
	res = NewPublicResolver(openMockShareDB(t, conn))
	_, err = res.ResolveDashboard(ctx, "bogus")
	assert.ErrorIs(t, err, ErrShareNotFound)

	// Share found but dashboard missing → ErrShareNotFound.
	conn = newResolverConn(now)
	conn.dashboardRows.rows = nil
	res = NewPublicResolver(openMockShareDB(t, conn))
	_, err = res.ResolveDashboard(ctx, "plain-token")
	assert.ErrorIs(t, err, ErrShareNotFound)
}

func TestPublicResolverResolveWidgetQueryMock(t *testing.T) {
	now := time.Now()
	ctx := context.Background()

	res := NewPublicResolver(openMockShareDB(t, newResolverConn(now)))

	// Widget with a stored logical query.
	q, err := res.ResolveWidgetQuery(ctx, "plain-token", "w1")
	require.NoError(t, err)
	assert.Equal(t, "ws-1", q.WorkspaceID)
	assert.Equal(t, "user-1", q.CreatedBy)
	require.NotNil(t, q.LogicalQuery)
	assert.Equal(t, "ds-1", q.LogicalQuery.DatasourceID)

	// Text widget (no stored query) → ErrShareNotFound.
	_, err = res.ResolveWidgetQuery(ctx, "plain-token", "w2")
	assert.ErrorIs(t, err, ErrShareNotFound)

	// Unknown widget id → ErrShareNotFound.
	_, err = res.ResolveWidgetQuery(ctx, "plain-token", "missing")
	assert.ErrorIs(t, err, ErrShareNotFound)

	// Bad token propagates through ResolveWidgetQuery too.
	conn := newResolverConn(now)
	conn.shareRows.rows = nil
	res = NewPublicResolver(openMockShareDB(t, conn))
	_, err = res.ResolveWidgetQuery(ctx, "bogus", "w1")
	assert.ErrorIs(t, err, ErrShareNotFound)
}
