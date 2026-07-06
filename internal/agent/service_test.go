package agent

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
)

// --- minimal mock SQL driver, mirroring internal/metadata's test pattern,
// so GetAgentRun can be exercised without a live Postgres. Routes by a
// substring of the query text: agent_steps queries always return an empty
// result; everything else returns the configured agent_runs row. Each Query
// call gets a fresh cursor so pos never leaks across calls. ---

type serviceMockRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *serviceMockRows) Columns() []string { return r.cols }
func (*serviceMockRows) Close() error        { return nil }
func (r *serviceMockRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

type serviceMockConn struct{ runRows *serviceMockRows }

func (c *serviceMockConn) Prepare(query string) (driver.Stmt, error) {
	return &serviceMockStmt{conn: c, query: query}, nil
}
func (*serviceMockConn) Close() error              { return nil }
func (*serviceMockConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

type serviceMockStmt struct {
	conn  *serviceMockConn
	query string
}

func (*serviceMockStmt) Close() error  { return nil }
func (*serviceMockStmt) NumInput() int { return -1 }
func (*serviceMockStmt) Exec([]driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}
func (s *serviceMockStmt) Query([]driver.Value) (driver.Rows, error) {
	if strings.Contains(s.query, "agent_steps") || s.conn.runRows == nil {
		return &serviceMockRows{}, nil
	}
	// A fresh cursor over the same backing slice: pos always starts at 0.
	return &serviceMockRows{cols: s.conn.runRows.cols, rows: s.conn.runRows.rows}, nil
}

type serviceMockDriver struct{ runRows *serviceMockRows }

func (d *serviceMockDriver) Open(string) (driver.Conn, error) {
	return &serviceMockConn{runRows: d.runRows}, nil
}

func newTestRepo(t *testing.T, runRows *serviceMockRows) *metadata.Repository {
	t.Helper()
	name := "agent-service-test-" + t.Name()
	sql.Register(name, &serviceMockDriver{runRows: runRows})
	db, err := sql.Open(name, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return metadata.NewRepository(db)
}

func testDeps(t *testing.T, rows *serviceMockRows) *AgentDependencies {
	cfg := &config.Config{}
	cfg.Security.InternalAPIToken = "test-internal-token"
	return &AgentDependencies{
		Config:   cfg,
		MetaRepo: newTestRepo(t, rows),
		Ready:    &atomic.Bool{},
		Runs:     NewRunRegistry(),
	}
}

func TestServiceHealthzAlwaysOK(t *testing.T) {
	srv := NewServer(testDeps(t, nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", http.NoBody))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServiceReadyzBeforeSubscription(t *testing.T) {
	srv := NewServer(testDeps(t, nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestServiceReadyzAfterSubscription(t *testing.T) {
	deps := testDeps(t, nil)
	deps.Ready.Store(true)
	srv := NewServer(deps)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServiceReadyzNilFlagDefaultsUnready(t *testing.T) {
	deps := testDeps(t, nil)
	deps.Ready = nil
	srv := NewServer(deps)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestServiceMetricsEndpointServesPrometheusFormat(t *testing.T) {
	srv := NewServer(testDeps(t, nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
}

func TestServiceRunsEndpointRequiresInternalToken(t *testing.T) {
	srv := NewServer(testDeps(t, nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/internal/agent/runs/run-1", http.NoBody))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestServiceRunsEndpointAcceptsValidInternalToken(t *testing.T) {
	rows := &serviceMockRows{
		cols: []string{
			"id", "conversation_id", "datasource_id", "model_id", "user_id",
			"question", "question_hash", "mode", "status", "confidence", "answer",
			"created_at", "updated_at",
		},
		rows: [][]driver.Value{
			{"run-1", "", "ds-1", "", "user-1", "q", "h", "interactive", "completed", 0.9, "42",
				time.Now(), time.Now()},
		},
	}
	deps := testDeps(t, rows)
	srv := NewServer(deps)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/internal/agent/runs/run-1", http.NoBody)
	req.Header.Set("X-Internal-Token", "test-internal-token")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"run-1"`)
}

func TestServiceCancelEndpointRequiresInternalToken(t *testing.T) {
	srv := NewServer(testDeps(t, nil))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/agent/runs/run-1/cancel", http.NoBody))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestServiceCancelEndpointCancelsTrackedRun(t *testing.T) {
	deps := testDeps(t, nil)
	var canceled bool
	_, cancel := context.WithCancel(context.Background())
	deps.Runs.Register("run-1", func() { canceled = true; cancel() })
	srv := NewServer(deps)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/agent/runs/run-1/cancel", http.NoBody)
	req.Header.Set("X-Internal-Token", "test-internal-token")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.True(t, canceled)
}

func TestServiceCancelEndpointReportsNotFoundForUntrackedRun(t *testing.T) {
	deps := testDeps(t, nil)
	srv := NewServer(deps)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/agent/runs/unknown-run/cancel", http.NoBody)
	req.Header.Set("X-Internal-Token", "test-internal-token")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRunRegistryUnregisterStopsFutureCancel(t *testing.T) {
	registry := NewRunRegistry()
	called := false
	registry.Register("run-1", func() { called = true })
	registry.Unregister("run-1")

	assert.False(t, registry.Cancel("run-1"))
	assert.False(t, called)
}

func TestRunRegistryConcurrentRegisterCancelUnregister(_ *testing.T) {
	registry := NewRunRegistry()
	done := make(chan struct{})
	go func() {
		for i := range 100 {
			id := "run-x"
			registry.Register(id, func() {})
			registry.Cancel(id)
			registry.Unregister(id)
			_ = i
		}
		close(done)
	}()
	for i := range 100 {
		id := "run-y"
		registry.Register(id, func() {})
		registry.Cancel(id)
		registry.Unregister(id)
		_ = i
	}
	<-done
}
