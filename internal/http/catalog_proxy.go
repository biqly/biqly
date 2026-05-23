package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerCatalogProxyRoutes(r chi.Router, catalogURL string) {
	proxy, ok := newCatalogProxy(catalogURL)
	if !ok {
		return
	}
	r.Handle("/datasources", proxy)
	r.Handle("/datasources/*", proxy)
	r.Handle("/metadata/*", proxy)
	r.Handle("/semantic/*", proxy)
}

func newCatalogProxy(catalogURL string) (http.Handler, bool) {
	return newUpstreamProxy(catalogURL, "BI_CATALOG_SERVICE_URL", "catalog service")
}
