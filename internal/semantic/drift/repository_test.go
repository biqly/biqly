package drift

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- Simple SQL Mock Driver for Repository Tests ---

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

type mockRepositoryDriver struct {
	mu     sync.Mutex
	states map[string]*mockDBState
}

func (d *mockRepositoryDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[name]
	if !ok {
		return nil, fmt.Errorf("no mock db state registered for %s", name)
	}
	return &mockConn{db: state}, nil
}

var mDriver = &mockRepositoryDriver{states: make(map[string]*mockDBState)}

func init() {
	sql.Register("drift_mock", mDriver)
}

func setupMockDB(t *testing.T) (*sql.DB, *mockDBState) {
	name := fmt.Sprintf("db-%d-%s", time.Now().UnixNano(), t.Name())
	state := &mockDBState{}
	mDriver.mu.Lock()
	mDriver.states[name] = state
	mDriver.mu.Unlock()

	db, err := sql.Open("drift_mock", name)
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		mDriver.mu.Lock()
		delete(mDriver.states, name)
		mDriver.mu.Unlock()
	})
	return db, state
}

type mockConn struct {
	db *mockDBState
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{conn: c, query: query}, nil
}

func (c *mockConn) Close() error { return nil }
func (c *mockConn) Begin() (driver.Tx, error) {
	c.db.logCall("BEGIN", nil)
	return &mockTx{conn: c}, nil
}

func (c *mockConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, len(args))
	for i, arg := range args {
		vals[i] = arg.Value
	}
	c.db.logCall("QUERYContext: "+query, vals)
	return c.db.nextQueryRows(query, vals)
}

func (c *mockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
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

func (s *mockStmt) Close() error { return nil }
func (s *mockStmt) NumInput() int { return -1 }
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
func (r *mockRows) Close() error      { return nil }
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

// --- Repository Tests ---

func TestRepositoryInsertReport(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	report := &DriftReport{
		ModelID:      "model-1",
		DatasourceID: "ds-1",
		Severity:     SeverityCritical,
		Drifts: []DriftItem{
			{Type: DriftTypeColumnDropped, Field: "age", ColumnRef: "public.users.age"},
		},
		DetectedAt: time.Now(),
	}

	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO drift_reports",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"report-123"}},
		},
	}

	err := repo.InsertReport(ctx, report)
	assert.NoError(t, err)
	assert.Equal(t, "report-123", report.ID)
}

func TestRepositoryListUnresolved(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	driftsJSON, _ := json.Marshal([]DriftItem{
		{Type: DriftTypeColumnDropped, Field: "age", ColumnRef: "public.users.age"},
	})

	state.queries = []queryMock{
		{
			Pattern: "SELECT id::text, model_id::text, datasource_id::text, sync_event_id::text, severity, drifts, resolved, resolved_by, resolved_at, detected_at, created_at FROM drift_reports WHERE model_id =",
			Cols:    []string{"id", "model_id", "datasource_id", "sync_event_id", "severity", "drifts", "resolved", "resolved_by", "resolved_at", "detected_at", "created_at"},
			Rows: [][]driver.Value{
				{"report-123", "model-1", "ds-1", nil, SeverityCritical, driftsJSON, false, nil, nil, now, now},
			},
		},
	}

	list, err := repo.ListUnresolvedByModel(ctx, "model-1")
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "report-123", list[0].ID)
	assert.Equal(t, "model-1", list[0].ModelID)
	assert.Len(t, list[0].Drifts, 1)
	assert.Equal(t, DriftTypeColumnDropped, list[0].Drifts[0].Type)
}

func TestRepositoryResolve(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE drift_reports SET resolved = true", RowsAffected: 1},
	}

	err := repo.ResolveReport(ctx, "report-123", "user-admin")
	assert.NoError(t, err)
}
