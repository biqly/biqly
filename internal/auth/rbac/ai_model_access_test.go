package rbac

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	registerAIModelAccessDriver sync.Once
	aiModelAccessMu             sync.Mutex
	aiModelAccessGrantCounts    int

	errAIModelAccessPrepareUnsupported      = errors.New("prepare is not supported")
	errAIModelAccessTransactionsUnsupported = errors.New("transactions are not supported")
	errAIModelAccessUnexpectedQuery         = errors.New("unexpected query")
)

func TestCanUseModelReusesKnownRestrictedFlag(t *testing.T) {
	registerAIModelAccessDriver.Do(func() {
		sql.Register("ai_model_access_check", aiModelAccessDriver{})
	})
	resetAIModelAccessGrantCounts()

	dbPool, err := sql.Open("ai_model_access_check", "")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, dbPool.Close())
	}()

	service := NewAIModelAccessService(dbPool, nil)
	ok, err := service.CanUseModel(context.Background(), "user-1", "model-2")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, currentAIModelAccessGrantCounts())
}

func TestHasModelID(t *testing.T) {
	require.False(t, hasModelID(nil, "model-1"))
	require.False(t, hasModelID(&UserAIAccess{ModelIDs: []string{"model-2"}}, "model-1"))
	require.True(t, hasModelID(&UserAIAccess{ModelIDs: []string{"model-1", "model-2"}}, "model-2"))
}

func resetAIModelAccessGrantCounts() {
	aiModelAccessMu.Lock()
	defer aiModelAccessMu.Unlock()
	aiModelAccessGrantCounts = 0
}

func currentAIModelAccessGrantCounts() int {
	aiModelAccessMu.Lock()
	defer aiModelAccessMu.Unlock()
	return aiModelAccessGrantCounts
}

type aiModelAccessDriver struct{}

func (aiModelAccessDriver) Open(string) (driver.Conn, error) {
	return aiModelAccessConn{}, nil
}

type aiModelAccessConn struct{}

func (aiModelAccessConn) Prepare(string) (driver.Stmt, error) {
	return nil, errAIModelAccessPrepareUnsupported
}

func (aiModelAccessConn) Close() error {
	return nil
}

func (aiModelAccessConn) Begin() (driver.Tx, error) {
	return nil, errAIModelAccessTransactionsUnsupported
}

func (aiModelAccessConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	switch {
	case strings.Contains(normalized, "select ( (select count(*) from ai_provider_workspace_grants)"):
		aiModelAccessMu.Lock()
		aiModelAccessGrantCounts++
		aiModelAccessMu.Unlock()
		return &aiModelAccessRows{columns: []string{"count"}, values: [][]driver.Value{{int64(1)}}}, nil
	case strings.Contains(normalized, "select distinct model_id::text"):
		return &aiModelAccessRows{columns: []string{"model_id"}, values: [][]driver.Value{{"model-1"}, {"model-2"}}}, nil
	case strings.Contains(normalized, "select distinct provider_id::text"):
		return &aiModelAccessRows{columns: []string{"provider_id"}}, nil
	case strings.Contains(normalized, "from user_ai_model_preferences"):
		return &aiModelAccessRows{columns: []string{"user_id", "purpose", "model_id", "updated_at"}}, nil
	default:
		return nil, errAIModelAccessUnexpectedQuery
	}
}

type aiModelAccessRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *aiModelAccessRows) Columns() []string {
	return r.columns
}

func (*aiModelAccessRows) Close() error {
	return nil
}

func (r *aiModelAccessRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
