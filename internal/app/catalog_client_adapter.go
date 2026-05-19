package app

import (
	"context"
	"fmt"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
)

type queryCatalogAdapter struct {
	client *catalogclient.Client
}

func newQueryCatalogAdapter(client *catalogclient.Client) *queryCatalogAdapter {
	return &queryCatalogAdapter{client: client}
}

func (a *queryCatalogAdapter) GetPublishedFullModel(ctx context.Context, modelID string) (*semantic.SemanticModel, error) {
	model, err := a.client.GetModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("catalog get model: %w", err)
	}
	return model, nil
}

func (a *queryCatalogAdapter) GetDatasource(ctx context.Context, datasourceID string) (*metadata.Datasource, error) {
	ds, err := a.client.GetDatasource(ctx, datasourceID)
	if err != nil {
		return nil, fmt.Errorf("catalog get datasource: %w", err)
	}
	return ds, nil
}

func (a *queryCatalogAdapter) CreateQueryHistory(ctx context.Context, entry *query.HistoryEntry) error {
	if entry == nil {
		return nil
	}
	if _, err := a.client.CreateQueryHistory(ctx, *entry); err != nil {
		return fmt.Errorf("catalog create query history: %w", err)
	}
	return nil
}
