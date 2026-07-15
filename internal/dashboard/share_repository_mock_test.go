package dashboard

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockShareTx is a no-op driver.Tx; Commit can be forced to fail for the
// commit-error branch of Rotate.
type mockShareTx struct{ commitErr error }

func (t mockShareTx) Commit() error { return t.commitErr }
func (mockShareTx) Rollback() error { return nil }

// mockShareConn routes queries by substring so a single *sql.DB can serve both
// the dashboard_public_shares table (share repo) and the dashboards table (the
// public resolver's Get) without a real database.
type mockShareConn struct {
	shareRows     *mockRows // SELECT ... FROM dashboard_public_shares
	insertRows    *mockRows // INSERT INTO dashboard_public_shares ... RETURNING
	dashboardRows *mockRows // SELECT ... FROM dashboards (resolver Get)
	execRows      int64
	beginErr      error
	execErr       error
	queryErr      error
	commitErr     error
}

func (*mockShareConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not implemented")
}
func (*mockShareConn) Close() error { return nil }

func (c *mockShareConn) Begin() (driver.Tx, error) {
	if c.beginErr != nil {
		return nil, c.beginErr
	}
	return mockShareTx{commitErr: c.commitErr}, nil
}

func (c *mockShareConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	q := strings.ToLower(query)
	switch {
	case strings.Contains(q, "insert into dashboard_public_shares"):
		c.insertRows.pos = 0
		return c.insertRows, nil
	case strings.Contains(q, "dashboard_public_shares"):
		c.shareRows.pos = 0
		return c.shareRows, nil
	default: // SELECT ... FROM dashboards
		c.dashboardRows.pos = 0
		return c.dashboardRows, nil
	}
}

func (c *mockShareConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.execErr != nil {
		return nil, c.execErr
	}
	return mockResult{affected: c.execRows}, nil
}

var (
	activeShareConn *mockShareConn
	shareConnMu     sync.Mutex
)

type shareMockBridge struct{}

func (shareMockBridge) Open(string) (driver.Conn, error) {
	shareConnMu.Lock()
	defer shareConnMu.Unlock()
	return activeShareConn, nil
}

func init() { sql.Register("share_mock_bridge", shareMockBridge{}) }

func openMockShareDB(t *testing.T, conn *mockShareConn) *sql.DB {
	t.Helper()
	shareConnMu.Lock()
	activeShareConn = conn
	shareConnMu.Unlock()
	db, err := sql.Open("share_mock_bridge", "mock")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// shareSelectCols matches the column order scanned by GetActive /
// FindActiveByTokenHash.
var shareSelectCols = []string{
	"id", "dashboard_id", "workspace_id", "token_hash",
	"created_by", "created_at", "revoked_at", "expires_at",
}

func TestNewShareRepository(t *testing.T) {
	repo := NewShareRepository(openMockShareDB(t, &mockShareConn{}))
	assert.NotNil(t, repo)
}

func TestShareRepositoryRotate(t *testing.T) {
	now := time.Now()
	newConn := func() *mockShareConn {
		return &mockShareConn{
			execRows: 1,
			insertRows: &mockRows{
				cols: []string{"id", "created_at"},
				rows: [][]driver.Value{{"share-1", now}},
			},
		}
	}
	share := func() *PublicShare {
		return &PublicShare{DashboardID: "dash-1", WorkspaceID: "ws-1", TokenHash: "hash", CreatedBy: "user-1"}
	}

	// Happy path: revoke previous + insert new + commit.
	s := share()
	require.NoError(t, NewShareRepository(openMockShareDB(t, newConn())).Rotate(context.Background(), s))
	assert.Equal(t, "share-1", s.ID)

	// Happy path with empty CreatedBy (exercises the NULL createdBy branch).
	s2 := share()
	s2.CreatedBy = ""
	require.NoError(t, NewShareRepository(openMockShareDB(t, newConn())).Rotate(context.Background(), s2))

	// Begin error.
	c := newConn()
	c.beginErr = errors.New("begin boom")
	assert.Error(t, NewShareRepository(openMockShareDB(t, c)).Rotate(context.Background(), share()))

	// Revoke exec error.
	c = newConn()
	c.execErr = errors.New("revoke boom")
	assert.Error(t, NewShareRepository(openMockShareDB(t, c)).Rotate(context.Background(), share()))

	// Insert (query) error.
	c = newConn()
	c.queryErr = errors.New("insert boom")
	assert.Error(t, NewShareRepository(openMockShareDB(t, c)).Rotate(context.Background(), share()))

	// Commit error.
	c = newConn()
	c.commitErr = errors.New("commit boom")
	assert.Error(t, NewShareRepository(openMockShareDB(t, c)).Rotate(context.Background(), share()))
}

func TestShareRepositoryGetActive(t *testing.T) {
	now := time.Now()
	conn := &mockShareConn{
		shareRows: &mockRows{
			cols: shareSelectCols,
			rows: [][]driver.Value{{"share-1", "dash-1", "ws-1", "hash", "user-1", now, nil, nil}},
		},
	}
	repo := NewShareRepository(openMockShareDB(t, conn))

	got, err := repo.GetActive(context.Background(), "dash-1", "ws-1")
	require.NoError(t, err)
	assert.Equal(t, "share-1", got.ID)
	assert.Equal(t, "user-1", got.CreatedBy)

	// No rows → sql.ErrNoRows (returned verbatim).
	conn.shareRows.rows = nil
	_, err = repo.GetActive(context.Background(), "dash-1", "ws-1")
	assert.ErrorIs(t, err, sql.ErrNoRows)

	// Query error → wrapped error.
	conn.queryErr = errors.New("boom")
	_, err = repo.GetActive(context.Background(), "dash-1", "ws-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, sql.ErrNoRows)
}

func TestShareRepositoryRevoke(t *testing.T) {
	conn := &mockShareConn{execRows: 1}
	repo := NewShareRepository(openMockShareDB(t, conn))
	require.NoError(t, repo.Revoke(context.Background(), "dash-1", "ws-1"))

	// No rows affected → sql.ErrNoRows.
	conn.execRows = 0
	assert.ErrorIs(t, repo.Revoke(context.Background(), "dash-1", "ws-1"), sql.ErrNoRows)

	// Exec error.
	conn.execErr = errors.New("boom")
	assert.Error(t, repo.Revoke(context.Background(), "dash-1", "ws-1"))
}

func TestShareRepositoryFindActiveByTokenHash(t *testing.T) {
	now := time.Now()
	expires := now.Add(time.Hour)
	conn := &mockShareConn{
		shareRows: &mockRows{
			cols: shareSelectCols,
			rows: [][]driver.Value{{"share-1", "dash-1", "ws-1", "hash", "user-1", now, nil, expires}},
		},
	}
	repo := NewShareRepository(openMockShareDB(t, conn))

	got, err := repo.FindActiveByTokenHash(context.Background(), "hash")
	require.NoError(t, err)
	assert.Equal(t, "dash-1", got.DashboardID)
	require.NotNil(t, got.ExpiresAt)

	// No rows → sql.ErrNoRows.
	conn.shareRows.rows = nil
	_, err = repo.FindActiveByTokenHash(context.Background(), "missing")
	assert.ErrorIs(t, err, sql.ErrNoRows)

	// Query error → wrapped error.
	conn.queryErr = errors.New("boom")
	_, err = repo.FindActiveByTokenHash(context.Background(), "hash")
	require.Error(t, err)
	assert.NotErrorIs(t, err, sql.ErrNoRows)
}
