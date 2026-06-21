package query

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
	"time"
)

var errFakeExecNotSupported = errors.New("fake stmt exec not supported")

// fakeDriver implements database/sql/driver for testing execute/scanRows.
type fakeDriver struct {
	openFn func(name string) (driver.Conn, error)
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	return d.openFn(name)
}

type fakeConn struct {
	queryFn func(query string, args []driver.Value) (driver.Rows, error)
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeStmt{conn: c, query: query}, nil
}

var errFakeBeginNotSupported = errors.New("fake conn begin not supported")

func (*fakeConn) Close() error              { return nil }
func (*fakeConn) Begin() (driver.Tx, error) { return nil, errFakeBeginNotSupported }

type fakeStmt struct {
	conn  *fakeConn
	query string
}

func (*fakeStmt) Close() error  { return nil }
func (*fakeStmt) NumInput() int { return -1 }
func (*fakeStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return nil, errFakeExecNotSupported
}
func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.conn.queryFn(s.query, args)
}

type fakeRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (*fakeRows) Close() error        { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

func TestExecute_WithFakeDB(t *testing.T) {
	driverName := "test_execute_" + t.Name()
	sql.Register(driverName, &fakeDriver{
		openFn: func(_ string) (driver.Conn, error) {
			return &fakeConn{
				queryFn: func(_ string, _ []driver.Value) (driver.Rows, error) {
					return &fakeRows{
						cols: []string{"count"},
						rows: [][]driver.Value{
							{int64(42)},
						},
					}, nil
				},
			}, nil
		},
	})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	executor := NewExecutor(100, time.Second)
	cq := &CompiledQuery{SQL: "SELECT count(*) FROM orders", Args: nil}
	result, err := executor.Execute(context.Background(), db, cq)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if len(result.Columns) != 1 || result.Columns[0].Name != "count" {
		t.Errorf("unexpected columns: %+v", result.Columns)
	}
	if result.Stats.RowCount != 1 {
		t.Errorf("RowCount = %d, want 1", result.Stats.RowCount)
	}
	if result.Stats.Truncated {
		t.Error("expected Truncated = false")
	}
}

func TestExecute_EmptyTimeout(t *testing.T) {
	driverName := "test_execute_notimeout_" + t.Name()
	sql.Register(driverName, &fakeDriver{
		openFn: func(_ string) (driver.Conn, error) {
			return &fakeConn{
				queryFn: func(_ string, _ []driver.Value) (driver.Rows, error) {
					return &fakeRows{
						cols: []string{"val"},
						rows: [][]driver.Value{
							{int64(1)},
							{int64(2)},
						},
					}, nil
				},
			}, nil
		},
	})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Zero timeout = no timeout applied
	executor := NewExecutor(10, 0)
	cq := &CompiledQuery{SQL: "SELECT 1 UNION SELECT 2", Args: nil}
	result, err := executor.Execute(context.Background(), db, cq)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Stats.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", result.Stats.RowCount)
	}
}

func TestExecute_MaxRowsTruncates(t *testing.T) {
	driverName := "test_execute_truncate_" + t.Name()
	sql.Register(driverName, &fakeDriver{
		openFn: func(_ string) (driver.Conn, error) {
			return &fakeConn{
				queryFn: func(_ string, _ []driver.Value) (driver.Rows, error) {
					return &fakeRows{
						cols: []string{"val"},
						rows: [][]driver.Value{
							{int64(1)},
							{int64(2)},
							{int64(3)},
							{int64(4)},
							{int64(5)},
						},
					}, nil
				},
			}, nil
		},
	})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	executor := NewExecutor(3, time.Second)
	cq := &CompiledQuery{SQL: "SELECT generate_series(1,5)", Args: nil}
	result, err := executor.Execute(context.Background(), db, cq)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result.Stats.RowCount != 3 {
		t.Errorf("RowCount = %d, want 3", result.Stats.RowCount)
	}
	if !result.Stats.Truncated {
		t.Error("expected Truncated = true")
	}
}
