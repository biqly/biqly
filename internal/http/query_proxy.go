package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerQueryProxyRoutes(r chi.Router, queryURL string) {
	proxy, ok := newQueryProxy(queryURL)
	if !ok {
		return
	}
	r.Handle("/query", proxy)
	r.Handle("/query/*", proxy)
}

func newQueryProxy(queryURL string) (http.Handler, bool) {
	return newUpstreamProxy(queryURL, "BI_QUERY_SERVICE_URL", "query service")
}
