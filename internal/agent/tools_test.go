package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryReturnsPolicyDeniedError(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := baseRunContext()
	run.AllowedTools = nil

	_, err := registry.Execute(context.Background(), run, Proposal{
		Tool:      ToolCatalog,
		Arguments: marshalArgs(t, identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID}),
	})
	require.Error(t, err)
	var denied *PolicyDeniedError
	require.True(t, errors.As(err, &denied))
	assert.Equal(t, ReasonToolNotAllowlisted, denied.ReasonCode)
}

func TestRegistryReturnsToolNotRegisteredWhenAllowlistedButUnregistered(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := baseRunContext()
	run.AllowedTools = []ToolName{ToolSemantic}

	_, err := registry.Execute(context.Background(), run, Proposal{
		Tool:      ToolSemantic,
		Arguments: marshalArgs(t, identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID}),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrToolNotRegistered))
}

func TestRegistryDispatchesAllowedProposalToItsTool(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := baseRunContext()

	obs, err := registry.Execute(context.Background(), run, Proposal{
		Tool:      ToolCatalog,
		Arguments: marshalArgs(t, identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID}),
	})
	require.NoError(t, err)
	assert.Equal(t, ToolCatalog, obs.Tool)
	assert.Equal(t, 1, fake.calls)
}
