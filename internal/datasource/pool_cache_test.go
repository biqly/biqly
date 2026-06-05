package datasource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
)

// fakeConn / fakeConnector implement database/sql/driver just enough for
// sql.OpenDB() to hand back a *sql.DB pool. No real network or filesystem.
type fakeConn struct{}

func (fakeConn) Prepare(_ string) (driver.Stmt, error)   { return nil, io.EOF }
func (fakeConn) Close() error                            { return nil }
func (fakeConn) Begin() (driver.Tx, error)               { return nil, io.EOF }

type fakeConnector struct{}

func (fakeConnector) Connect(_ context.Context) (driver.Conn, error) { return fakeConn{}, nil }
func (fakeConnector) Driver() driver.Driver                          { return nil }

type stubDriver struct {
	opens atomic.Int32
}

func (*stubDriver) Type() string { return "stub" }
func (*stubDriver) Dialect() dialect.Dialect { return dialect.PostgresDialect{} }
func (*stubDriver) Ping(_ context.Context, _ string) error {
	return nil
}
func (*stubDriver) Introspect(_ context.Context, _ *sql.DB) (*IntrospectionResult, error) {
	return &IntrospectionResult{}, nil
}
func (s *stubDriver) Open(_ context.Context, _ string) (*sql.DB, error) {
	s.opens.Add(1)
	return sql.OpenDB(fakeConnector{}), nil
}

func TestPoolCache_GetCachesByID(t *testing.T) {
	cache := NewPoolCache()
	defer func() { _ = cache.Close() }()
	d := &stubDriver{}

	a, err := cache.Get(context.Background(), d, "ds-1", "dsn-1")
	if err != nil {
		t.Fatalf("first open failed: %v", err)
	}
	b, err := cache.Get(context.Background(), d, "ds-1", "dsn-1")
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	if a != b {
		t.Fatal("expected cached *sql.DB to be reused for same key")
	}
	if got := d.opens.Load(); got != 1 {
		t.Fatalf("expected 1 driver.Open call, got %d", got)
	}
}

func TestPoolCache_DifferentDSNOpensFreshPool(t *testing.T) {
	cache := NewPoolCache()
	defer func() { _ = cache.Close() }()
	d := &stubDriver{}

	if _, err := cache.Get(context.Background(), d, "ds-1", "dsn-a"); err != nil {
		t.Fatalf("open A: %v", err)
	}
	if _, err := cache.Get(context.Background(), d, "ds-1", "dsn-b"); err != nil {
		t.Fatalf("open B: %v", err)
	}
	if got := d.opens.Load(); got != 2 {
		t.Fatalf("expected 2 driver.Open calls when DSN changes, got %d", got)
	}
}

func TestPoolCache_InvalidateClosesPool(t *testing.T) {
	cache := NewPoolCache()
	defer func() { _ = cache.Close() }()
	d := &stubDriver{}

	_, err := cache.Get(context.Background(), d, "ds-1", "dsn-1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cache.Invalidate("ds-1")

	if _, err := cache.Get(context.Background(), d, "ds-1", "dsn-1"); err != nil {
		t.Fatalf("re-open: %v", err)
	}
	if got := d.opens.Load(); got != 2 {
		t.Fatalf("expected 2 opens after invalidate, got %d", got)
	}
}

func TestPoolCache_NilSafe(t *testing.T) {
	var c *PoolCache
	if err := c.Close(); err != nil {
		t.Fatalf("nil cache Close should be no-op, got %v", err)
	}
	c.Invalidate("anything")
}

func TestPoolCache_NilDriverErrors(t *testing.T) {
	c := NewPoolCache()
	defer func() { _ = c.Close() }()
	if _, err := c.Get(context.Background(), nil, "ds", "dsn"); err == nil {
		t.Fatal("expected error for nil driver")
	}
}

func TestPoolCache_CloseTwiceSafe(t *testing.T) {
	c := NewPoolCache()
	d := &stubDriver{}
	if _, err := c.Get(context.Background(), d, "ds", "dsn"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// keep errors import for IDE; harmless.
var _ = errors.New
