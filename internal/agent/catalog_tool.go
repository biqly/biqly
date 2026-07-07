package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
)

// CatalogEntity is one resolved table/column the catalog service knows
// about for a datasource.
type CatalogEntity struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns,omitempty"`
}

// CatalogResolver is the subset of pkg/catalogclient.Client the catalog
// tool needs. Kept as a local interface so tests use a fake instead of a
// real HTTP client; pkg/catalogclient.Client already satisfies it via
// GetDatasource/ListTables/ListColumns — no client-side changes needed.
type CatalogResolver interface {
	ResolveEntities(ctx context.Context, datasourceID string, modelID string) ([]CatalogEntity, error)
}

// catalogResolveArgs is the strict shape of catalog.resolve arguments.
type catalogResolveArgs struct {
	identityArgs
	ModelID string `json:"model_id,omitempty"`
}

// CatalogTool implements the catalog.resolve tool.
type CatalogTool struct {
	resolver CatalogResolver
}

// NewCatalogTool builds a CatalogTool backed by resolver.
func NewCatalogTool(resolver CatalogResolver) *CatalogTool {
	return &CatalogTool{resolver: resolver}
}

// Name implements Tool.
func (*CatalogTool) Name() ToolName { return ToolCatalog }

// Execute implements Tool.
func (t *CatalogTool) Execute(ctx context.Context, run RunContext, arguments json.RawMessage) (Observation, error) {
	var args catalogResolveArgs
	if err := strictDecode(arguments, &args); err != nil {
		return Observation{}, fmt.Errorf("catalog.resolve: %w", err)
	}

	entities, err := callWithSingleRetry(ctx, func(ctx context.Context) ([]CatalogEntity, error) {
		return t.resolver.ResolveEntities(ctx, run.DatasourceID, args.ModelID)
	})
	if err != nil {
		return Observation{}, fmt.Errorf("catalog.resolve: %w", err)
	}

	payload, err := sonic.Marshal(entities)
	if err != nil {
		return Observation{}, fmt.Errorf("catalog.resolve: encode observation: %w", err)
	}
	return Observation{Tool: ToolCatalog, Payload: payload}, nil
}
