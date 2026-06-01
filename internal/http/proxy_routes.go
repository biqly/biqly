package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// upstreamProxySpec describes a reverse proxy mount: which upstream URL it
// forwards to, the env var that supplied it (for misconfiguration logging),
// the service label used in error responses, and the chi paths to bind.
type upstreamProxySpec struct {
	targetURL    string
	envVarName   string
	serviceLabel string
	paths        []string
}

// registerUpstreamProxy mounts a ReverseProxy on r for every path in spec.paths.
// When the upstream URL is empty or invalid, registration is skipped (see
// newUpstreamProxy) so the monolith can serve those routes directly.
func registerUpstreamProxy(r chi.Router, spec upstreamProxySpec) {
	proxy, ok := newUpstreamProxy(spec.targetURL, spec.envVarName, spec.serviceLabel)
	if !ok {
		return
	}
	for _, p := range spec.paths {
		r.Handle(p, proxy)
	}
}

var catalogProxyPaths = []string{
	"/datasources",
	"/datasources/*",
	"/metadata/*",
	"/semantic/*",
	"/permissions",
	"/permissions/*",
	"/dashboards",
	"/dashboards/*",
}

// queryProxyPaths are the routes proxied to the Query service when
// BI_QUERY_SERVICE_URL is set.
var queryProxyPaths = []string{
	"/query",
	"/query/*",
}

func registerCatalogProxyRoutes(r chi.Router, catalogURL string) {
	registerUpstreamProxy(r, upstreamProxySpec{
		targetURL:    catalogURL,
		envVarName:   "BI_CATALOG_SERVICE_URL",
		serviceLabel: "catalog service",
		paths:        catalogProxyPaths,
	})
}

func registerQueryProxyRoutes(r chi.Router, queryURL string) {
	registerUpstreamProxy(r, upstreamProxySpec{
		targetURL:    queryURL,
		envVarName:   "BI_QUERY_SERVICE_URL",
		serviceLabel: "query service",
		paths:        queryProxyPaths,
	})
}

// aiProxyDatasourceGuardedPaths are AI routes that accept a datasource_id and
// must be gated by RequireDatasourceAccess before being forwarded to the AI
// service. Other AI routes (history, examples, glossary, settings…) are
// proxied without the per-datasource check.
var aiProxyDatasourceGuardedPaths = []string{
	"/ai/query",
	"/ai/query/preview",
	"/ai/query/run",
	"/ai/metadata/describe",
	"/ai/metadata/embed",
	"/ai/jobs",
}

// registerAIProxyRoutesWithDatasourceGuard mounts the AI reverse proxy but
// wraps the datasource-consuming POST endpoints with dsAccess first. Remaining
// /ai/* paths fall through to the wildcard proxy without the extra check.
func registerAIProxyRoutesWithDatasourceGuard(r chi.Router, aiURL string, dsAccess func(http.Handler) http.Handler) {
	proxy, ok := newUpstreamProxy(aiURL, "BI_AI_SERVICE_URL", "AI service")
	if !ok {
		return
	}
	for _, p := range aiProxyDatasourceGuardedPaths {
		r.With(dsAccess).Method(http.MethodPost, p, proxy)
	}
	// Catch-all for the remaining /ai routes (history, examples, glossary,
	// settings, GET on /ai/jobs/*, etc.).
	r.Handle("/ai", proxy)
	r.Handle("/ai/*", proxy)
}

