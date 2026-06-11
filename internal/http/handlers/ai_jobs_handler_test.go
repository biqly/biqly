package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var aiJobMockCols = []string{
	"id", "client_session_id", "kind", "status", "phase", "phase_message", "progress_pct",
	"datasource_id", "scope_schemas", "progress_json", "request_json", "result_json",
	"error_message", "created_at", "updated_at", "started_at", "finished_at", "user_id", "locale",
}

func aiJobMockRow(now time.Time, userID string) []driver.Value {
	return []driver.Value{
		"job-1", "sess-1", "run", "running", "generating", "", 40,
		"ds-1", "{public}", []byte(`{}`), []byte(`{}`), []byte(`{}`),
		"", now, now, now, nil, userID, "tr",
	}
}

func newAIJobsTestHandler(t *testing.T) (*AIJobsHandler, *mockDBState) {
	t.Helper()
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	svc := NewAIJobService(repo, nil, nil)
	return NewAIJobsHandler(svc, nil), state
}

func ctxWithIdentity(userID string, roles ...string) context.Context {
	ctx := context.WithValue(context.Background(), bimw.UserIDKey, userID)
	return context.WithValue(ctx, bimw.UserRolesKey, roles)
}

func serveCancel(ctx context.Context, h *AIJobsHandler, path string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Delete("/api/ai/jobs/{id}", h.Cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, path, http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAIJobsCancelOwnershipAllowsOwner(t *testing.T) {
	h, state := newAIJobsTestHandler(t)
	now := time.Now()
	state.queries = []queryMock{
		{Pattern: "FROM ai_jobs WHERE id = $1", Cols: aiJobMockCols, Rows: [][]driver.Value{aiJobMockRow(now, "user-1")}},
	}
	state.execs = []execMock{
		{Pattern: "UPDATE ai_jobs SET status =", RowsAffected: 1},
	}

	rec := serveCancel(ctxWithIdentity("user-1"), h, "/api/ai/jobs/job-1")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAIJobsCancelOwnershipAllowsLegacySessionOwner(t *testing.T) {
	h, state := newAIJobsTestHandler(t)
	now := time.Now()
	state.queries = []queryMock{
		{Pattern: "FROM ai_jobs WHERE id = $1", Cols: aiJobMockCols, Rows: [][]driver.Value{aiJobMockRow(now, "")}},
	}
	state.execs = []execMock{
		{Pattern: "UPDATE ai_jobs SET status =", RowsAffected: 1},
	}

	rec := serveCancel(ctxWithIdentity("user-1"), h, "/api/ai/jobs/job-1?client_session_id=sess-1")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAIJobsCancelOwnershipRejectsForeignUser(t *testing.T) {
	h, state := newAIJobsTestHandler(t)
	now := time.Now()
	state.queries = []queryMock{
		{Pattern: "FROM ai_jobs WHERE id = $1", Cols: aiJobMockCols, Rows: [][]driver.Value{aiJobMockRow(now, "user-1")}},
	}

	rec := serveCancel(ctxWithIdentity("intruder"), h, "/api/ai/jobs/job-1")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestAIJobsCancelAllowsAdminRoles(t *testing.T) {
	for _, role := range []string{bimw.RoleSuperAdmin, "admin"} {
		t.Run(role, func(t *testing.T) {
			h, state := newAIJobsTestHandler(t)
			now := time.Now()
			state.queries = []queryMock{
				{Pattern: "FROM ai_jobs WHERE id = $1", Cols: aiJobMockCols, Rows: [][]driver.Value{aiJobMockRow(now, "user-1")}},
			}
			state.execs = []execMock{
				{Pattern: "UPDATE ai_jobs SET status =", RowsAffected: 1},
			}

			rec := serveCancel(ctxWithIdentity("someone-else", role), h, "/api/ai/jobs/job-1")
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func serveCancelBatch(ctx context.Context, h *AIJobsHandler) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"ids":["job-1","job-2"],"client_session_id":"sess-1"}`)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/ai/jobs/cancel-batch", body)
	rec := httptest.NewRecorder()
	h.CancelBatch(rec, req)
	return rec
}

func execOps(state *mockDBState) []string {
	state.mu.Lock()
	defer state.mu.Unlock()
	ops := make([]string, 0, len(state.calls))
	for _, c := range state.calls {
		ops = append(ops, c.Op)
	}
	return ops
}

func TestAIJobsCancelBatchScopesToOwnerForNonAdmins(t *testing.T) {
	h, state := newAIJobsTestHandler(t)
	state.execs = []execMock{
		{Pattern: "UPDATE ai_jobs SET status =", RowsAffected: 1},
	}

	rec := serveCancelBatch(ctxWithIdentity("user-1"), h)
	require.Equal(t, http.StatusOK, rec.Code)

	ops := strings.Join(execOps(state), " | ")
	assert.Contains(t, ops, "user_id = $6", "non-admin batch cancel must carry the ownership predicate")
}

func TestAIJobsCancelBatchUnscopedForAdmins(t *testing.T) {
	h, state := newAIJobsTestHandler(t)
	state.execs = []execMock{
		{Pattern: "UPDATE ai_jobs SET status =", RowsAffected: 2},
	}

	rec := serveCancelBatch(ctxWithIdentity("admin-1", bimw.RoleSuperAdmin), h)
	require.Equal(t, http.StatusOK, rec.Code)

	ops := strings.Join(execOps(state), " | ")
	assert.NotContains(t, ops, "user_id = $6", "admin batch cancel must not be ownership-scoped")
}

func TestAIJobsAdminListReturnsJobs(t *testing.T) {
	h, state := newAIJobsTestHandler(t)
	now := time.Now()
	state.queries = []queryMock{
		{Pattern: "ORDER BY (status IN", Cols: aiJobMockCols, Rows: [][]driver.Value{aiJobMockRow(now, "user-1")}},
	}

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/ai/jobs/admin?status=running&kind=run&limit=10",
		http.NoBody,
	)
	rec := httptest.NewRecorder()
	h.AdminList(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"job-1"`)
}
