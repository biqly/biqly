package dashboard

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRows is a minimal driver.Rows backed by an in-memory result set.
type mockRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockRows) Columns() []string { return r.cols }
func (*mockRows) Close() error        { return nil }

func (r *mockRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

// mockResult implements driver.Result for ExecContext (UPDATE/DELETE).
type mockResult struct{ affected int64 }

func (mockResult) LastInsertId() (int64, error)   { return 0, nil }
func (m mockResult) RowsAffected() (int64, error) { return m.affected, nil }

// mockDashboardConn routes queries by substring to canned rows / results and
// supports error injection for the failure paths.
type mockDashboardConn struct {
	insertRows *mockRows
	getRows    *mockRows
	listRows   *mockRows
	execRows   int64
	queryErr   error
	execErr    error
}

func (*mockDashboardConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}
func (*mockDashboardConn) Close() error              { return nil }
func (*mockDashboardConn) Begin() (driver.Tx, error) { return nil, errors.New("no tx") }

func (c *mockDashboardConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	q := strings.ToLower(query)
	switch {
	case strings.Contains(q, "insert into dashboards"):
		c.insertRows.pos = 0
		return c.insertRows, nil
	case strings.Contains(q, "where id ="):
		c.getRows.pos = 0
		return c.getRows, nil
	default:
		c.listRows.pos = 0
		return c.listRows, nil
	}
}

func (c *mockDashboardConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.execErr != nil {
		return nil, c.execErr
	}
	return mockResult{affected: c.execRows}, nil
}

var (
	activeDashboardConn *mockDashboardConn
	dashboardConnMu     sync.Mutex
)

type dashboardMockBridge struct{}

func (dashboardMockBridge) Open(string) (driver.Conn, error) {
	dashboardConnMu.Lock()
	defer dashboardConnMu.Unlock()
	return activeDashboardConn, nil
}

func init() { sql.Register("dashboard_mock_bridge", dashboardMockBridge{}) }

func openMockDB(t *testing.T, conn *mockDashboardConn) *sql.DB {
	t.Helper()
	dashboardConnMu.Lock()
	activeDashboardConn = conn
	dashboardConnMu.Unlock()
	db, err := sql.Open("dashboard_mock_bridge", "mock")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository(openMockDB(t, &mockDashboardConn{}))
	assert.NotNil(t, repo)
}

func TestRepositoryCreate(t *testing.T) {
	now := time.Now()
	conn := &mockDashboardConn{
		insertRows: &mockRows{
			cols: []string{"id", "created_at", "updated_at"},
			rows: [][]driver.Value{{"dash-1", now, now}},
		},
	}
	repo := NewRepository(openMockDB(t, conn))
	ws := "ws-1"
	d := &Dashboard{WorkspaceID: &ws, Name: "Sales", Widgets: json.RawMessage(`[]`)}
	require.NoError(t, repo.Create(context.Background(), d))
	assert.Equal(t, "dash-1", d.ID)

	// Error path.
	conn.queryErr = errors.New("boom")
	assert.Error(t, repo.Create(context.Background(), d))
}

func TestRepositoryGet(t *testing.T) {
	now := time.Now()
	conn := &mockDashboardConn{
		getRows: &mockRows{
			cols: []string{"id", "workspace_id", "name", "description", "widgets", "created_at", "updated_at"},
			rows: [][]driver.Value{{"dash-1", "ws-1", "Sales", "desc", []byte(`[]`), now, now}},
		},
	}
	repo := NewRepository(openMockDB(t, conn))
	d, err := repo.Get(context.Background(), "dash-1")
	require.NoError(t, err)
	assert.Equal(t, "Sales", d.Name)
	require.NotNil(t, d.WorkspaceID)
	assert.Equal(t, "ws-1", *d.WorkspaceID)
	require.NotNil(t, d.Description)
	assert.Equal(t, "desc", *d.Description)

	// Not found.
	conn.getRows.rows = nil
	_, err = repo.Get(context.Background(), "missing")
	assert.Error(t, err)
}

func TestRepositoryList(t *testing.T) {
	now := time.Now()
	cols := []string{"id", "workspace_id", "name", "description", "widgets", "created_at", "updated_at"}
	conn := &mockDashboardConn{
		listRows: &mockRows{
			cols: cols,
			rows: [][]driver.Value{
				{"d1", "ws-1", "A", "da", []byte(`[]`), now, now},
				{"d2", nil, "B", nil, []byte(`[]`), now, now}, // null workspace/description branch
			},
		},
	}
	repo := NewRepository(openMockDB(t, conn))

	all, err := repo.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Nil(t, all[1].WorkspaceID)

	// Filtered branch (workspaceID != "").
	conn.listRows.rows = [][]driver.Value{{"d1", "ws-1", "A", "da", []byte(`[]`), now, now}}
	filtered, err := repo.List(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)

	// Error path.
	conn.queryErr = errors.New("boom")
	_, err = repo.List(context.Background(), "")
	assert.Error(t, err)
}

func TestRepositoryUpdate(t *testing.T) {
	conn := &mockDashboardConn{execRows: 1}
	repo := NewRepository(openMockDB(t, conn))
	d := &Dashboard{ID: "dash-1", Name: "New", Widgets: json.RawMessage(`[]`)}
	require.NoError(t, repo.Update(context.Background(), d))

	// No rows affected → ErrNoRows.
	conn.execRows = 0
	assert.ErrorIs(t, repo.Update(context.Background(), d), sql.ErrNoRows)

	// Exec error.
	conn.execErr = errors.New("boom")
	assert.Error(t, repo.Update(context.Background(), d))
}

func TestRepositoryDelete(t *testing.T) {
	conn := &mockDashboardConn{execRows: 1}
	repo := NewRepository(openMockDB(t, conn))
	require.NoError(t, repo.Delete(context.Background(), "dash-1"))

	conn.execRows = 0
	assert.ErrorIs(t, repo.Delete(context.Background(), "dash-1"), sql.ErrNoRows)

	conn.execErr = errors.New("boom")
	assert.Error(t, repo.Delete(context.Background(), "dash-1"))
}
