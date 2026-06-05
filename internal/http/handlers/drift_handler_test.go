package handlers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/semantic/drift"
	"github.com/stretchr/testify/require"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

// --- Simple SQL Mock Driver for Handler Tests ---

type mockCall struct {
	Op   string
	Args []driver.Value
}

type queryMock struct {
	Pattern string
	Cols    []string
	Rows    [][]driver.Value
	Err     error
}

type execMock struct {
	Pattern      string
	LastInsertID int64
	RowsAffected int64
	Err          error
}

type mockDBState struct {
	mu      sync.Mutex
	calls   []mockCall
	queries []queryMock
	execs   []execMock
}

func (s *mockDBState) logCall(op string, args []driver.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, mockCall{Op: strings.Join(strings.Fields(op), " "), Args: args})
}

func (s *mockDBState) nextQueryRows(query string, _ []driver.Value) (driver.Rows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	qNorm := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for _, qm := range s.queries {
		if strings.Contains(qNorm, strings.ToLower(qm.Pattern)) {
			if qm.Err != nil {
				return nil, qm.Err
			}
			return &mockRows{cols: qm.Cols, rows: qm.Rows, pos: 0}, nil
		}
	}
	return nil, fmt.Errorf("no mock query matched: %s", query)
}

func (s *mockDBState) nextExecResult(query string, _ []driver.Value) (driver.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	qNorm := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for _, em := range s.execs {
		if strings.Contains(qNorm, strings.ToLower(em.Pattern)) {
			if em.Err != nil {
				return nil, em.Err
			}
			return driver.RowsAffected(em.RowsAffected), nil
		}
	}
	return driver.RowsAffected(1), nil
}

type mockHandlerDriver struct {
	mu     sync.Mutex
	states map[string]*mockDBState
}

func (d *mockHandlerDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[name]
	if !ok {
		return nil, fmt.Errorf("no mock db state registered for %s", name)
	}
	return &mockConn{db: state}, nil
}

var hDriver = &mockHandlerDriver{states: make(map[string]*mockDBState)}

func init() {
	sql.Register("drift_handler_mock", hDriver)
}

func setupMockDB(t *testing.T) (*sql.DB, *mockDBState) {
	name := fmt.Sprintf("db-%d-%s", time.Now().UnixNano(), t.Name())
	state := &mockDBState{}
	hDriver.mu.Lock()
	hDriver.states[name] = state
	hDriver.mu.Unlock()

	db, err := sql.Open("drift_handler_mock", name)
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		hDriver.mu.Lock()
		delete(hDriver.states, name)
		hDriver.mu.Unlock()
	})
	return db, state
}

type mockConn struct {
	db *mockDBState
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{conn: c, query: query}, nil
}

func (*mockConn) Close() error { return nil }
func (c *mockConn) Begin() (driver.Tx, error) {
	c.db.logCall("BEGIN", nil)
	return &mockTx{conn: c}, nil
}

func (c *mockConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, len(args))
	for i, arg := range args {
		vals[i] = arg.Value
	}
	c.db.logCall("QUERYContext: "+query, vals)
	return c.db.nextQueryRows(query, vals)
}

func (c *mockConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]driver.Value, len(args))
	for i, arg := range args {
		vals[i] = arg.Value
	}
	c.db.logCall("EXECContext: "+query, vals)
	return c.db.nextExecResult(query, vals)
}

type mockStmt struct {
	conn  *mockConn
	query string
}

func (*mockStmt) Close() error { return nil }
func (*mockStmt) NumInput() int { return -1 }
func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.conn.db.logCall("EXEC: "+s.query, args)
	return s.conn.db.nextExecResult(s.query, args)
}
func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.conn.db.logCall("QUERY: "+s.query, args)
	return s.conn.db.nextQueryRows(s.query, args)
}

type mockTx struct{ conn *mockConn }

func (tx *mockTx) Commit() error {
	tx.conn.db.logCall("COMMIT", nil)
	return nil
}
func (tx *mockTx) Rollback() error {
	tx.conn.db.logCall("ROLLBACK", nil)
	return nil
}

type mockRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockRows) Columns() []string { return r.cols }
func (*mockRows) Close() error      { return nil }
func (r *mockRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.pos]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	r.pos++
	return nil
}

// --- Handler Tests ---

func TestDriftHandler(t *testing.T) {
	db, state := setupMockDB(t)

	driftRepo := drift.NewRepository(db)
	auditLogger := audit.NewLogger(slog.Default())

	catalogDeps := &app.CatalogDeps{
		DriftRepo:   driftRepo,
		AuditLogger: auditLogger,
	}

	handler := NewDriftHandler(catalogDeps)
	router := chi.NewRouter()
	router.Get("/api/v1/models/{id}/drift", handler.ListForModel)
	router.Get("/api/v1/datasources/{id}/drift", handler.ListForDatasource)
	router.Post("/api/v1/drift/{id}/resolve", handler.Resolve)

	now := time.Now()
	driftsJSON, err := json.Marshal([]drift.DriftItem{
		{Type: drift.DriftTypeColumnDropped, Field: "age", ColumnRef: "public.users.age", Description: "Dropped"},
	})
	require.NoError(t, err)

	t.Run("ListForModel", func(t *testing.T) {
		state.queries = []queryMock{
			{
				Pattern: "SELECT id::text, model_id::text",
				Cols:    []string{"id", "model_id", "datasource_id", "sync_event_id", "severity", "drifts", "resolved", "resolved_by", "resolved_at", "detected_at", "created_at"},
				Rows: [][]driver.Value{
					{"report-123", "model-1", "ds-1", nil, drift.SeverityCritical, driftsJSON, false, nil, nil, now, now},
				},
			},
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/models/model-1/drift", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp []driftReportResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		assert.NoError(t, err)
		assert.Len(t, resp, 1)
		assert.Equal(t, "report-123", resp[0].ID)
		assert.Equal(t, "model-1", resp[0].ModelID)
		assert.Len(t, resp[0].Drifts, 1)
		assert.Equal(t, "column_dropped", resp[0].Drifts[0].Type)
	})

	t.Run("ListForDatasource", func(t *testing.T) {
		state.queries = []queryMock{
			{
				Pattern: "SELECT id::text, model_id::text",
				Cols:    []string{"id", "model_id", "datasource_id", "sync_event_id", "severity", "drifts", "resolved", "resolved_by", "resolved_at", "detected_at", "created_at"},
				Rows: [][]driver.Value{
					{"report-123", "model-1", "ds-1", nil, drift.SeverityCritical, driftsJSON, false, nil, nil, now, now},
				},
			},
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/datasources/ds-1/drift", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp []driftReportResponse
		err := json.NewDecoder(rec.Body).Decode(&resp)
		assert.NoError(t, err)
		assert.Len(t, resp, 1)
		assert.Equal(t, "ds-1", resp[0].DatasourceID)
	})

	t.Run("Resolve", func(t *testing.T) {
		state.execs = []execMock{
			{Pattern: "UPDATE drift_reports SET resolved = true", RowsAffected: 1},
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/drift/report-123/resolve", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		assert.NoError(t, err)
		success, ok := resp["success"].(bool)
		assert.True(t, ok)
		assert.True(t, success)
	})
}
