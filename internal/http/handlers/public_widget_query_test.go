package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/dashboard"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	pkgquery "github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubQueryRunner is a hand-written internalQueryRunner double: it records the
// LogicalQuery it was asked to run (so tests can prove the STORED query — not
// visitor input — reached the engine) and returns a canned result.
type stubQueryRunner struct {
	calls  int
	gotLQ  *pkgquery.LogicalQuery
	gotCtx context.Context
	result *core.RunResult
	svcErr *core.ServiceError
}

func (*stubQueryRunner) Compile(context.Context, *pkgquery.LogicalQuery) (*core.CompileResult, *core.ServiceError) {
	return nil, nil
}

func (*stubQueryRunner) Run(context.Context, *pkgquery.LogicalQuery) (*core.RunResult, *core.ServiceError) {
	return nil, nil
}

func (*stubQueryRunner) CompileWithModel(context.Context, *pkgquery.LogicalQuery, *semantic.SemanticModel) (*core.CompileResult, *core.ServiceError) {
	return nil, nil
}

func (s *stubQueryRunner) RunWithModel(ctx context.Context, lq *pkgquery.LogicalQuery, _ *semantic.SemanticModel) (*core.RunResult, *core.ServiceError) {
	s.calls++
	s.gotLQ = lq
	s.gotCtx = ctx
	return s.result, s.svcErr
}

func (*stubQueryRunner) DryRunWithModel(context.Context, *pkgquery.LogicalQuery, *semantic.SemanticModel) (*core.CompileResult, *core.ServiceError) {
	return nil, nil
}

// widgetQueryFixture is a seeded shared dashboard: widget "w1" carries a stored
// logical query (distinctive datasource/model ids), "w2" is a text widget.
type widgetQueryFixture struct {
	resolver *dashboard.PublicResolver
	token    string
}

func newWidgetQueryFixture(t *testing.T) widgetQueryFixture {
	t.Helper()
	ctx := context.Background()
	db := testutil.OpenMetadataDB(t)
	createDashboardShareTestTables(ctx, t, db)

	dashRepo := dashboard.NewRepository(db)
	shareRepo := dashboard.NewShareRepository(db)

	ws := "66666666-6666-6666-6666-666666666666"
	widgets := []byte(`[
		{"id":"w1","type":"chart","logical_query":{"datasource_id":"stored-ds-777","model_id":"stored-model-888","select":[]}},
		{"id":"w2","type":"text","title":"note"}
	]`)
	d := &dashboard.Dashboard{WorkspaceID: &ws, Name: "wq dashboard", Widgets: widgets}
	require.NoError(t, dashRepo.Create(ctx, d))

	token, err := dashboard.GenerateShareToken()
	require.NoError(t, err)
	require.NoError(t, shareRepo.Rotate(ctx, &dashboard.PublicShare{
		DashboardID: d.ID,
		WorkspaceID: ws,
		TokenHash:   dashboard.HashShareToken(token),
		CreatedBy:   testUserID,
	}))

	return widgetQueryFixture{resolver: dashboard.NewPublicResolver(db), token: token}
}

func cannedWidgetResult() *core.RunResult {
	return &core.RunResult{Result: &pkgquery.Result{
		Columns: []pkgquery.ResultColumn{{Name: "sentinel_col", Type: "int"}},
		Rows:    [][]any{{42}},
	}}
}

func TestPublicWidgetQueryHandler_Run(t *testing.T) {
	ctx := context.Background()
	fx := newWidgetQueryFixture(t)
	resolver, token := fx.resolver, fx.token
	cannedResult := cannedWidgetResult()

	newRouter := func(runner *stubQueryRunner, sharingEnabled bool) *chi.Mux {
		h := NewPublicWidgetQueryHandler(resolver, runner, &stubSharingChecker{enabled: sharingEnabled}, nil, 0)
		r := chi.NewRouter()
		r.Post("/widget-query/{token}/{widgetID}", h.Run)
		return r
	}
	post := func(r *chi.Mux, tok, widgetID, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/widget-query/"+tok+"/"+widgetID, strings.NewReader(body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// Baseline 404 for a bogus token, captured so every "hidden" failure mode
	// can be asserted byte-identical (no leak of which check failed).
	invalidRec := post(newRouter(&stubQueryRunner{}, true), "not-a-real-token", "w1", "")

	t.Run("valid token+widget executes the STORED query and returns its result", func(t *testing.T) {
		runner := &stubQueryRunner{result: cannedResult}
		// Malicious body: a different datasource_id the server must ignore.
		rec := post(newRouter(runner, true), token, "w1", `{"logical_query":{"datasource_id":"attacker-ds-000","model_id":"attacker-model"}}`)

		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		require.Equal(t, 1, runner.calls, "runner must be invoked exactly once")
		require.NotNil(t, runner.gotLQ)
		// Proves the server used the STORED query, not the request body.
		assert.Equal(t, "stored-ds-777", runner.gotLQ.DatasourceID)
		assert.Equal(t, "stored-model-888", runner.gotLQ.ModelID)
		assert.NotEqual(t, "attacker-ds-000", runner.gotLQ.DatasourceID)
		// Response carries the runner's result payload.
		assert.Contains(t, rec.Body.String(), "sentinel_col")
	})

	t.Run("query runs under the share creator's identity so PII masking / RLS re-apply", func(t *testing.T) {
		runner := &stubQueryRunner{result: cannedResult}
		rec := post(newRouter(runner, true), token, "w1", "")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 1, runner.calls)
		require.NotNil(t, runner.gotCtx)
		// The anonymous request path has no auth middleware; the handler must
		// inject the share creator's user ID so PIIPolicyService resolves the
		// creator's masking config and row filters instead of running unscoped.
		assert.Equal(t, testUserID, bimw.UserID(runner.gotCtx),
			"RunWithModel must receive the share creator's identity, not an empty one")
	})

	t.Run("malicious body is fully ignored even for a valid empty-body request", func(t *testing.T) {
		runner := &stubQueryRunner{result: cannedResult}
		rec := post(newRouter(runner, true), token, "w1", `{"datasource_id":"evil","widgetID":"w2","token":"other"}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "stored-ds-777", runner.gotLQ.DatasourceID)
	})

	t.Run("unknown widget id returns 404 and does not run the query", func(t *testing.T) {
		runner := &stubQueryRunner{result: cannedResult}
		rec := post(newRouter(runner, true), token, "does-not-exist", "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, 0, runner.calls)
	})

	t.Run("text widget (no stored query) returns 404 and does not run the query", func(t *testing.T) {
		runner := &stubQueryRunner{result: cannedResult}
		rec := post(newRouter(runner, true), token, "w2", "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, 0, runner.calls)
	})

	t.Run("bad token returns 404", func(t *testing.T) {
		rec := post(newRouter(&stubQueryRunner{}, true), "totally-bogus", "w1", "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("kill-switch off returns the same 404 as an invalid token", func(t *testing.T) {
		runner := &stubQueryRunner{result: cannedResult}
		rec := post(newRouter(runner, false), token, "w1", "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, invalidRec.Code, rec.Code, "status must match the invalid-token case")
		assert.Equal(t, invalidRec.Body.String(), rec.Body.String(), "body must match the invalid-token case")
		assert.Equal(t, 0, runner.calls, "the query must not run when sharing is disabled")
	})

	t.Run("with nil cache the runner executes on every call", func(t *testing.T) {
		runner := &stubQueryRunner{result: cannedResult}
		r := newRouter(runner, true)
		post(r, token, "w1", "")
		post(r, token, "w1", "")
		assert.Equal(t, 2, runner.calls, "nil cache must not memoize between calls")
	})
}

// TestPublicWidgetQueryHandler_CacheHitSkipsRunner proves the short-TTL cache
// shields the customer datasource: a second request within the TTL is served
// from redis without re-invoking the query runner. Skips when no redis.
func TestPublicWidgetQueryHandler_CacheHitSkipsRunner(t *testing.T) {
	ctx := context.Background()

	dsn := os.Getenv("BI_REDIS_DSN")
	if dsn == "" {
		dsn = "redis://localhost:6379"
	}
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		t.Skipf("skipping cache test; bad redis DSN: %v", err)
	}
	rc := redis.NewClient(opt)
	t.Cleanup(func() { _ = rc.Close() })
	if pingErr := rc.Ping(ctx).Err(); pingErr != nil {
		t.Skipf("skipping cache test; redis unreachable: %v", pingErr)
	}

	fx := newWidgetQueryFixture(t)
	runner := &stubQueryRunner{result: cannedWidgetResult()}
	h := NewPublicWidgetQueryHandler(fx.resolver, runner, &stubSharingChecker{enabled: true}, rc, time.Minute)
	r := chi.NewRouter()
	r.Post("/widget-query/{token}/{widgetID}", h.Run)

	call := func() *httptest.ResponseRecorder {
		// fx.token is random per run, so the cache key is unique — no stale state.
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/widget-query/"+fx.token+"/w1", http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	rec1 := call()
	require.Equal(t, http.StatusOK, rec1.Code)
	require.Equal(t, 1, runner.calls, "first call must execute the query")

	rec2 := call()
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, 1, runner.calls, "second call within TTL must be served from cache, not the runner")
	assert.Contains(t, rec2.Body.String(), "sentinel_col")
}
