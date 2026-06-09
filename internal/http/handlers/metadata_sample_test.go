package handlers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"
)

// fakeSampleDriver is a minimal in-memory driver that replays a fixed grid,
// letting us exercise querySampleRows' generic scan and []byte->string
// normalization without a real database.
type fakeSampleDriver struct {
	cols []string
	rows [][]driver.Value
}

func (d *fakeSampleDriver) Open(string) (driver.Conn, error) { return &fakeSampleConn{drv: d}, nil }

type fakeSampleConn struct{ drv *fakeSampleDriver }

func (*fakeSampleConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*fakeSampleConn) Close() error                        { return nil }
func (*fakeSampleConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *fakeSampleConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &fakeSampleRows{cols: c.drv.cols, rows: c.drv.rows}, nil
}

type fakeSampleRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *fakeSampleRows) Columns() []string { return r.cols }
func (*fakeSampleRows) Close() error        { return nil }

func (r *fakeSampleRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.pos])
	r.pos++
	return nil
}

func TestQuerySampleRows(t *testing.T) {
	sql.Register("fake-sample", &fakeSampleDriver{
		cols: []string{"id", "name"},
		rows: [][]driver.Value{
			{int64(1), []byte("alice")},
			{int64(2), nil},
		},
	})
	db, err := sql.Open("fake-sample", "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	defer func() { _ = db.Close() }()

	got, err := querySampleRows(context.Background(), db, "SELECT *")
	if err != nil {
		t.Fatalf("querySampleRows: %v", err)
	}

	wantCols := []string{"id", "name"}
	if len(got.Columns) != len(wantCols) {
		t.Fatalf("columns = %d, want %d", len(got.Columns), len(wantCols))
	}
	for i, c := range got.Columns {
		if c.Name != wantCols[i] {
			t.Errorf("column[%d] = %q, want %q", i, c.Name, wantCols[i])
		}
	}

	if len(got.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(got.Rows))
	}
	// []byte text cells must be normalized to string (not left as []byte,
	// which would JSON-encode as base64).
	if s, ok := got.Rows[0][1].(string); !ok || s != "alice" {
		t.Errorf("row[0][1] = %#v, want string \"alice\"", got.Rows[0][1])
	}
	if got.Rows[1][1] != nil {
		t.Errorf("row[1][1] = %#v, want nil", got.Rows[1][1])
	}
}
