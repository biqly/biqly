package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -- Null conversion helpers tests --

func TestNullHelpers(t *testing.T) {
	t.Run("NullUUIDPtr", func(t *testing.T) {
		assert.Nil(t, NullUUIDPtr(nil))
		empty := ""
		assert.Nil(t, NullUUIDPtr(&empty))
		invalid := "composite:abc"
		assert.Nil(t, NullUUIDPtr(&invalid))
		valid := "550e8400-e29b-41d4-a716-446655440000"
		assert.Equal(t, valid, NullUUIDPtr(&valid))
	})

	t.Run("NullIfEmptyPtr", func(t *testing.T) {
		assert.Nil(t, NullIfEmptyPtr(nil))
		empty := ""
		assert.Nil(t, NullIfEmptyPtr(&empty))
		spaces := "   "
		assert.Nil(t, NullIfEmptyPtr(&spaces))
		valid := "hello"
		assert.Equal(t, "hello", NullIfEmptyPtr(&valid))
	})

	t.Run("NullIfEmpty", func(t *testing.T) {
		assert.Nil(t, NullIfEmpty(""))
		assert.Nil(t, NullIfEmpty("   "))
		assert.Equal(t, "hello", NullIfEmpty("hello"))
	})

	t.Run("StringPtrFromNull", func(t *testing.T) {
		var nullStr sql.NullString
		assert.Nil(t, StringPtrFromNull(nullStr))

		nullStr = sql.NullString{String: "hello", Valid: true}
		res := StringPtrFromNull(nullStr)
		assert.NotNil(t, res)
		assert.Equal(t, "hello", *res)
	})

	t.Run("TimePtrFromNull", func(t *testing.T) {
		var nullTime sql.NullTime
		assert.Nil(t, TimePtrFromNull(nullTime))

		now := time.Now()
		nullTime = sql.NullTime{Time: now, Valid: true}
		res := TimePtrFromNull(nullTime)
		assert.NotNil(t, res)
		assert.True(t, now.Equal(*res))
	})

	t.Run("IntPtrFromNull", func(t *testing.T) {
		var nullInt sql.NullInt64
		assert.Nil(t, IntPtrFromNull(nullInt))

		nullInt = sql.NullInt64{Int64: 42, Valid: true}
		res := IntPtrFromNull(nullInt)
		assert.NotNil(t, res)
		assert.Equal(t, 42, *res)
	})

	t.Run("NullIfNilIntPtr", func(t *testing.T) {
		assert.Nil(t, NullIfNilIntPtr(nil))
		val := 100
		assert.Equal(t, 100, NullIfNilIntPtr(&val))
	})
}

// -- NullStringArray tests --

func TestNullStringArray(t *testing.T) {
	t.Run("ParseStringArray", func(t *testing.T) {
		assert.Equal(t, []string{}, ParseStringArray(""))
		assert.Equal(t, []string{"a", "b", "c"}, ParseStringArray(`{"a","b","c"}`))
		assert.Equal(t, []string{"foo", "bar"}, ParseStringArray(`{foo,bar}`))
	})

	t.Run("Scan NullStringArray", func(t *testing.T) {
		var target []string
		nsa := NullStringArray{S: &target}

		// nil source
		err := nsa.Scan(nil)
		assert.NoError(t, err)
		assert.Equal(t, []string{}, target)

		// string source
		err = nsa.Scan(`{"hello","world"}`)
		assert.NoError(t, err)
		assert.Equal(t, []string{"hello", "world"}, target)

		// []byte source
		err = nsa.Scan([]byte(`{"byte","slice"}`))
		assert.NoError(t, err)
		assert.Equal(t, []string{"byte", "slice"}, target)

		// unexpected type source
		err = nsa.Scan(42)
		assert.NoError(t, err)
		assert.Equal(t, []string{}, target)
	})
}

// -- General DB configurations tests --

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("test-dsn")
	assert.Equal(t, "test-dsn", cfg.DSN)
	assert.Equal(t, 25, cfg.MaxOpenConns)
	assert.Equal(t, 5, cfg.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, cfg.ConnMaxLifetime)
	assert.Equal(t, 10*time.Minute, cfg.ConnMaxIdleTime)
}

// -- Database / SQL Mocking Core --

type mockDBConn struct {
	mu         sync.Mutex
	onBegin    func() (driver.Tx, error)
	onQuery    func(query string, args []driver.NamedValue) (driver.Rows, error)
	onPrepare  func(query string) (driver.Stmt, error)
	closeError error
}

func (c *mockDBConn) Prepare(query string) (driver.Stmt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.onPrepare != nil {
		return c.onPrepare(query)
	}
	return nil, errors.New("prepare not implemented")
}

func (c *mockDBConn) Close() error {
	return c.closeError
}

func (c *mockDBConn) Begin() (driver.Tx, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.onBegin != nil {
		return c.onBegin()
	}
	return nil, errors.New("begin transaction not implemented")
}

func (c *mockDBConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.onQuery != nil {
		return c.onQuery(query, args)
	}
	return nil, errors.New("query not implemented")
}

type mockDBTx struct {
	commitCalled   bool
	rollbackCalled bool
	commitErr      error
	rollbackErr    error
}

func (t *mockDBTx) Commit() error {
	t.commitCalled = true
	return t.commitErr
}

func (t *mockDBTx) Rollback() error {
	t.rollbackCalled = true
	return t.rollbackErr
}

type mockDBRows struct {
	columns []string
	data    [][]driver.Value
	index   int
}

func (r *mockDBRows) Columns() []string {
	return r.columns
}

func (*mockDBRows) Close() error {
	return nil
}

func (r *mockDBRows) Next(dest []driver.Value) error {
	if r.index >= len(r.data) {
		return io.EOF
	}
	row := r.data[r.index]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	r.index++
	return nil
}

var (
	persistentMockConn = &mockDBConn{}
)

type dbMockDriver struct{}

func (dbMockDriver) Open(_ string) (driver.Conn, error) {
	return persistentMockConn, nil
}

func init() {
	sql.Register("db_mock_driver", dbMockDriver{})
}

func TestRunInTx(t *testing.T) {
	db, err := sql.Open("db_mock_driver", "mock-dsn-tx")
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	t.Run("successful commit", func(t *testing.T) {
		var txState mockDBTx

		persistentMockConn.mu.Lock()
		persistentMockConn.onBegin = func() (driver.Tx, error) {
			return &txState, nil
		}
		persistentMockConn.mu.Unlock()

		err := RunInTx(context.Background(), db, func(_ *sql.Tx) error {
			return nil
		})

		assert.NoError(t, err)
		assert.True(t, txState.commitCalled)
		assert.False(t, txState.rollbackCalled) // deferred rollback will be ignored/noop after commit
	})

	t.Run("callback error triggers rollback", func(t *testing.T) {
		var txState mockDBTx
		expectedErr := errors.New("custom db failure")

		persistentMockConn.mu.Lock()
		persistentMockConn.onBegin = func() (driver.Tx, error) {
			return &txState, nil
		}
		persistentMockConn.mu.Unlock()

		err := RunInTx(context.Background(), db, func(_ *sql.Tx) error {
			return expectedErr
		})

		assert.ErrorIs(t, err, expectedErr)
		assert.False(t, txState.commitCalled)
		assert.True(t, txState.rollbackCalled)
	})

	t.Run("begin transaction failure", func(t *testing.T) {
		beginErr := errors.New("begin fail")

		persistentMockConn.mu.Lock()
		persistentMockConn.onBegin = func() (driver.Tx, error) {
			return nil, beginErr
		}
		persistentMockConn.mu.Unlock()

		err := RunInTx(context.Background(), db, func(_ *sql.Tx) error {
			return nil
		})

		assert.ErrorContains(t, err, "begin tx:")
	})

	t.Run("commit failure", func(t *testing.T) {
		commitErr := errors.New("commit fail")
		txState := mockDBTx{commitErr: commitErr}

		persistentMockConn.mu.Lock()
		persistentMockConn.onBegin = func() (driver.Tx, error) {
			return &txState, nil
		}
		persistentMockConn.mu.Unlock()

		err := RunInTx(context.Background(), db, func(_ *sql.Tx) error {
			return nil
		})

		assert.ErrorContains(t, err, "commit tx:")
	})
}

func TestQuerySlice(t *testing.T) {
	db, err := sql.Open("db_mock_driver", "mock-dsn-slice")
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	}()

	t.Run("successful query scan", func(t *testing.T) {
		persistentMockConn.mu.Lock()
		persistentMockConn.onQuery = func(_ string, _ []driver.NamedValue) (driver.Rows, error) {
			return &mockDBRows{
				columns: []string{"id", "name"},
				data: [][]driver.Value{
					{int64(1), "Alice"},
					{int64(2), "Bob"},
				},
			}, nil
		}
		persistentMockConn.mu.Unlock()

		type user struct {
			id   int
			name string
		}

		res, err := QuerySlice(context.Background(), db, "SELECT id, name FROM users", nil, func(s Scanner) (user, error) {
			var u user
			err := s.Scan(&u.id, &u.name)
			return u, err
		})

		assert.NoError(t, err)
		require.Len(t, res, 2)
		assert.Equal(t, 1, res[0].id)
		assert.Equal(t, "Alice", res[0].name)
		assert.Equal(t, 2, res[1].id)
		assert.Equal(t, "Bob", res[1].name)
	})

	t.Run("unsuccessful query execution", func(t *testing.T) {
		queryErr := errors.New("table not found")

		persistentMockConn.mu.Lock()
		persistentMockConn.onQuery = func(_ string, _ []driver.NamedValue) (driver.Rows, error) {
			return nil, queryErr
		}
		persistentMockConn.mu.Unlock()

		_, err := QuerySlice(context.Background(), db, "SELECT * FROM absent", nil, func(_ Scanner) (int, error) {
			return 0, nil
		})

		assert.ErrorIs(t, err, queryErr)
	})

	t.Run("QuerySliceErr wraps error with operation", func(t *testing.T) {
		queryErr := errors.New("table not found")

		persistentMockConn.mu.Lock()
		persistentMockConn.onQuery = func(_ string, _ []driver.NamedValue) (driver.Rows, error) {
			return nil, queryErr
		}
		persistentMockConn.mu.Unlock()

		_, err := QuerySliceErr(context.Background(), db, "GetUsers", "SELECT * FROM absent", nil, func(_ Scanner) (int, error) {
			return 0, nil
		})

		assert.ErrorContains(t, err, "GetUsers:")
		assert.ErrorIs(t, err, queryErr)
	})
}
