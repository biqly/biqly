package workspace

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/pkg/internalapi"
)

// HTTPResourceResolver resolves a shareable resource to its governing datasource
// by calling the catalog service's GET /internal/resource-datasource endpoint.
// The shareable resources (semantic models, AI query history, …) live in the
// catalog service's database, not the auth service's, so the sharing ownership
// guard cannot resolve them locally and must ask the owning service.
type HTTPResourceResolver struct {
	baseURL       string
	internalToken string
	client        *http.Client
}

// NewHTTPResourceResolver builds a resolver against baseURL (the catalog/api
// service), authenticating with the shared internal API token.
func NewHTTPResourceResolver(baseURL, internalToken string) *HTTPResourceResolver {
	return &HTTPResourceResolver{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		client:        &http.Client{Timeout: 5 * time.Second},
	}
}

// ResolveDatasource returns the datasource id governing (resourceType,
// resourceID). It maps the endpoint's 404/400 to ErrResourceNotFound /
// ErrResourceTypeUnsupported so Share fails closed appropriately.
func (r *HTTPResourceResolver) ResolveDatasource(ctx context.Context, resourceType, resourceID string) (string, error) {
	endpoint := fmt.Sprintf("%s/internal/resource-datasource?resource_type=%s&resource_id=%s",
		r.baseURL, url.QueryEscape(resourceType), url.QueryEscape(resourceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build resolve request: %w", err)
	}
	req.Header.Set("X-Internal-Token", r.internalToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call resource resolver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var out internalapi.ResourceDatasourceResponse
		if err := sonic.ConfigStd.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", fmt.Errorf("decode resolve response: %w", err)
		}
		if strings.TrimSpace(out.DatasourceID) == "" {
			return "", fmt.Errorf("resolver returned empty datasource for %s %s", resourceType, resourceID)
		}
		return out.DatasourceID, nil
	case http.StatusNotFound:
		return "", ErrResourceNotFound
	case http.StatusBadRequest:
		return "", ErrResourceTypeUnsupported
	default:
		return "", fmt.Errorf("resource resolver returned status %d", resp.StatusCode)
	}
}
