package dashboard

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWidgetsJSON = `[
  {"id":"w1","type":"chart","title":"Sales","w":6,"h":"medium","chart_type":"bar",
   "config":{"xAxisColumn":"month","yAxisColumns":["total"]},
   "logical_query":{"datasource_id":"ds-1","model_id":"m-1","select":[{"kind":"dimension","field":"month"}],"limit":100},
   "saved_query_id":"sq-1"},
  {"id":"w2","type":"text","title":"Note","w":6,"h":"small","content":"hello"}
]`

func TestSanitizeWidgets(t *testing.T) {
	out, err := SanitizeWidgets(json.RawMessage(testWidgetsJSON))
	require.NoError(t, err)
	var widgets []map[string]any
	require.NoError(t, sonic.Unmarshal(out, &widgets))
	require.Len(t, widgets, 2)
	for _, w := range widgets {
		assert.NotContains(t, w, "logical_query")
		assert.NotContains(t, w, "saved_query_id")
	}
	assert.Equal(t, "Sales", widgets[0]["title"])
	assert.Equal(t, "hello", widgets[1]["content"])
}

func TestPublicResolver(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	createShareTestTables(ctx, t, db)

	ws := "11111111-1111-1111-1111-111111111111"
	d := &Dashboard{WorkspaceID: new(ws), Name: "pub", Widgets: json.RawMessage(testWidgetsJSON)}
	require.NoError(t, NewRepository(db).Create(ctx, d))

	tok, err := GenerateShareToken()
	require.NoError(t, err)
	require.NoError(t, NewShareRepository(db).Rotate(ctx, &PublicShare{
		DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok),
	}))

	res := NewPublicResolver(db)

	t.Run("ResolveDashboard returns sanitized widgets", func(t *testing.T) {
		view, err := res.ResolveDashboard(ctx, tok)
		require.NoError(t, err)
		assert.Equal(t, d.ID, view.Dashboard.ID)
		assert.NotContains(t, string(view.Dashboard.Widgets), "logical_query")
		assert.NotContains(t, string(view.Dashboard.Widgets), "saved_query_id")
	})

	t.Run("bad token is ErrShareNotFound", func(t *testing.T) {
		_, err := res.ResolveDashboard(ctx, "bogus")
		assert.ErrorIs(t, err, ErrShareNotFound)
	})

	t.Run("ResolveWidgetQuery returns the stored logical query", func(t *testing.T) {
		q, err := res.ResolveWidgetQuery(ctx, tok, "w1")
		require.NoError(t, err)
		assert.Equal(t, ws, q.WorkspaceID)
		assert.Equal(t, "ds-1", q.LogicalQuery.DatasourceID)
	})

	t.Run("text widget and unknown widget are ErrShareNotFound", func(t *testing.T) {
		_, err := res.ResolveWidgetQuery(ctx, tok, "w2")
		assert.ErrorIs(t, err, ErrShareNotFound)
		_, err = res.ResolveWidgetQuery(ctx, tok, "missing")
		assert.ErrorIs(t, err, ErrShareNotFound)
	})
}
