package catalogclient

import (
	"context"
	"net/url"

	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/biqly/biqly/pkg/metadata"
	"github.com/biqly/biqly/pkg/semantic"
)

// Health calls /internal/health. Cheap — does not touch the database — so
// it is safe to use from a liveness/readiness probe.
func (c *Client) Health(ctx context.Context) (*internalapi.HealthResponse, error) {
	var out internalapi.HealthResponse
	if err := c.get(ctx, "/health", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDatasource returns the full datasource record (encrypted DSN included).
// Callers MUST share BI_ENCRYPTION_KEY with the Catalog service to decrypt
// the DSN locally; the cluster never carries plaintext credentials.
func (c *Client) GetDatasource(ctx context.Context, id string) (*metadata.Datasource, error) {
	var out metadata.Datasource
	if err := c.get(ctx, "/datasources/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListDatasources returns every datasource known to the catalog. Encrypted
// DSNs are included (see GetDatasource for the reasoning).
func (c *Client) ListDatasources(ctx context.Context) ([]metadata.Datasource, error) {
	var out []metadata.Datasource
	if err := c.get(ctx, "/datasources", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetModel returns the published semantic model with dimensions, metrics and
// joins inlined. The endpoint always returns the published version; drafts
// are not exposed over /internal/*.
func (c *Client) GetModel(ctx context.Context, id string) (*semantic.SemanticModel, error) {
	var out semantic.SemanticModel
	if err := c.get(ctx, "/models/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListModels returns every semantic model header for a datasource (no
// dimensions/metrics/joins). Use GetModel for the full payload.
func (c *Client) ListModels(ctx context.Context, datasourceID string) ([]semantic.SemanticModel, error) {
	var out []semantic.SemanticModel
	if err := c.get(ctx, "/models", url.Values{"datasource_id": {datasourceID}}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTables returns introspected tables for a datasource, optionally scoped
// to a single schema. An empty schemaName returns every schema's tables.
func (c *Client) ListTables(ctx context.Context, datasourceID, schemaName string) ([]metadata.Table, error) {
	q := url.Values{"datasource_id": {datasourceID}}
	if schemaName != "" {
		q.Set("schema_name", schemaName)
	}
	var out []metadata.Table
	if err := c.get(ctx, "/datasources/"+url.PathEscape(datasourceID)+"/tables", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListColumns returns columns for a datasource. Both schemaName and tableName
// are optional but callers SHOULD pass at least one to keep responses small.
func (c *Client) ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error) {
	q := url.Values{"datasource_id": {datasourceID}}
	if schemaName != "" {
		q.Set("schema_name", schemaName)
	}
	if tableName != "" {
		q.Set("table_name", tableName)
	}
	var out []metadata.Column
	if err := c.get(ctx, "/datasources/"+url.PathEscape(datasourceID)+"/columns", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListRelations returns foreign-key relations for a datasource.
func (c *Client) ListRelations(ctx context.Context, datasourceID string) ([]metadata.Relation, error) {
	q := url.Values{"datasource_id": {datasourceID}}
	var out []metadata.Relation
	if err := c.get(ctx, "/datasources/"+url.PathEscape(datasourceID)+"/relations", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListFewShot returns curated few-shot examples. modelID is optional; when
// empty every example for the datasource is returned.
func (c *Client) ListFewShot(ctx context.Context, datasourceID, modelID string) ([]metadata.FewShotCuratedRow, error) {
	q := url.Values{"datasource_id": {datasourceID}}
	if modelID != "" {
		q.Set("model_id", modelID)
	}
	var out []metadata.FewShotCuratedRow
	if err := c.get(ctx, "/few-shot", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListGlossary returns business glossary entries. modelID is optional; when
// empty every entry for the datasource is returned.
func (c *Client) ListGlossary(ctx context.Context, datasourceID, modelID string) ([]metadata.BusinessGlossaryRow, error) {
	q := url.Values{"datasource_id": {datasourceID}}
	if modelID != "" {
		q.Set("model_id", modelID)
	}
	var out []metadata.BusinessGlossaryRow
	if err := c.get(ctx, "/glossary", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
