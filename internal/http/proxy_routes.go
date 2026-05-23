package http

import "github.com/go-chi/chi/v5"

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

// catalogProxyPaths are the routes proxied to the Catalog service when
// BI_CATALOG_SERVICE_URL is set.
var catalogProxyPaths = []string{
	"/datasources",
	"/datasources/*",
	"/metadata/*",
	"/semantic/*",
}

// queryProxyPaths are the routes proxied to the Query service when
// BI_QUERY_SERVICE_URL is set.
var queryProxyPaths = []string{
	"/query",
	"/query/*",
}

// aiProxyPaths are the routes proxied to the AI service when
// BI_AI_SERVICE_URL is set.
var aiProxyPaths = []string{
	"/ai",
	"/ai/*",
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

func registerAIProxyRoutes(r chi.Router, aiURL string) {
	registerUpstreamProxy(r, upstreamProxySpec{
		targetURL:    aiURL,
		envVarName:   "BI_AI_SERVICE_URL",
		serviceLabel: "AI service",
		paths:        aiProxyPaths,
	})
}

