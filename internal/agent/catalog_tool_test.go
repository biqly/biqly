package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCatalogResolver struct {
	calls        int
	gotCtx       []context.Context
	gotDS        string
	gotModel     string
	failuresLeft int
	failWith     error
	result       []CatalogEntity
}

func (f *fakeCatalogResolver) ResolveEntities(ctx context.Context, datasourceID, modelID string) ([]CatalogEntity, error) {
	f.calls++
	f.gotCtx = append(f.gotCtx, ctx)
	f.gotDS = datasourceID
	f.gotModel = modelID
	if f.failuresLeft > 0 {
		f.failuresLeft--
		return nil, f.failWith
	}
	return f.result, nil
}

func TestCatalogToolResolvesEntities(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders", Columns: []string{"id"}}}}
	tool := NewCatalogTool(fake)
	run := baseRunContext()

	args, err := sonic.Marshal(catalogResolveArgs{
		identityArgs: identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID},
		ModelID:      "model-1",
	})
	require.NoError(t, err)

	obs, err := tool.Execute(context.Background(), run, args)
	require.NoError(t, err)
	assert.Equal(t, ToolCatalog, obs.Tool)
	assert.Equal(t, "ds-1", fake.gotDS)
	assert.Equal(t, "model-1", fake.gotModel)

	var entities []CatalogEntity
	require.NoError(t, sonic.Unmarshal(obs.Payload, &entities))
	assert.Equal(t, []CatalogEntity{{Table: "orders", Columns: []string{"id"}}}, entities)
}

func TestCatalogToolRejectsUnknownFields(t *testing.T) {
	tool := NewCatalogTool(&fakeCatalogResolver{})
	_, err := tool.Execute(context.Background(), baseRunContext(), []byte(`{"tenant_id":"t","user_id":"u","datasource_id":"d","bogus":1}`))
	assert.Error(t, err)
}

func TestCatalogToolRejectsRawCredentialArgument(t *testing.T) {
	// api_key is not a field of catalogResolveArgs; strict decode rejects it
	// the same way it rejects any unrecognized field — a planner cannot
	// smuggle credentials or an unrestricted destination through arguments.
	tool := NewCatalogTool(&fakeCatalogResolver{})
	_, err := tool.Execute(context.Background(), baseRunContext(),
		[]byte(`{"tenant_id":"t","user_id":"u","datasource_id":"d","api_key":"sk-secret"}`))
	assert.Error(t, err)
}

func TestCatalogToolPropagatesContextDeadline(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{}}
	tool := NewCatalogTool(fake)
	run := baseRunContext()
	args, err := sonic.Marshal(catalogResolveArgs{identityArgs: identityArgs{
		TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID,
	}})
	require.NoError(t, err)

	deadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	_, err = tool.Execute(ctx, run, args)
	require.NoError(t, err)
	require.Len(t, fake.gotCtx, 1)
	gotDeadline, ok := fake.gotCtx[0].Deadline()
	require.True(t, ok, "deadline must propagate to the upstream call")
	assert.Equal(t, deadline, gotDeadline)
}

func TestCatalogToolRetriesTransientErrorAtMostOnce(t *testing.T) {
	fake := &fakeCatalogResolver{failuresLeft: 1, failWith: &TransientError{Err: errors.New("upstream flaked")}, result: []CatalogEntity{{Table: "orders"}}}
	tool := NewCatalogTool(fake)
	run := baseRunContext()
	args, err := sonic.Marshal(catalogResolveArgs{identityArgs: identityArgs{
		TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID,
	}})
	require.NoError(t, err)

	obs, err := tool.Execute(context.Background(), run, args)
	require.NoError(t, err)
	assert.Equal(t, 2, fake.calls, "one retry after a transient failure")
	var entities []CatalogEntity
	require.NoError(t, sonic.Unmarshal(obs.Payload, &entities))
	assert.Equal(t, []CatalogEntity{{Table: "orders"}}, entities)
}

func TestCatalogToolDoesNotRetryTwice(t *testing.T) {
	fake := &fakeCatalogResolver{failuresLeft: 2, failWith: &TransientError{Err: errors.New("still down")}}
	tool := NewCatalogTool(fake)
	run := baseRunContext()
	args, err := sonic.Marshal(catalogResolveArgs{identityArgs: identityArgs{
		TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID,
	}})
	require.NoError(t, err)

	_, err = tool.Execute(context.Background(), run, args)
	assert.Error(t, err)
	assert.Equal(t, 2, fake.calls, "must not retry a second time")
}

func TestCatalogToolDoesNotRetryNonTransientError(t *testing.T) {
	fake := &fakeCatalogResolver{failuresLeft: 1, failWith: errors.New("permanent")}
	tool := NewCatalogTool(fake)
	run := baseRunContext()
	args, err := sonic.Marshal(catalogResolveArgs{identityArgs: identityArgs{
		TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID,
	}})
	require.NoError(t, err)

	_, err = tool.Execute(context.Background(), run, args)
	assert.Error(t, err)
	assert.Equal(t, 1, fake.calls, "non-transient errors are never retried")
}
