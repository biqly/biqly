package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerAIProxyRoutes(r chi.Router, aiURL string) {
	proxy, ok := newAIProxy(aiURL)
	if !ok {
		return
	}
	r.Handle("/ai", proxy)
	r.Handle("/ai/*", proxy)
}

func newAIProxy(aiURL string) (http.Handler, bool) {
	return newUpstreamProxy(aiURL, "BI_AI_SERVICE_URL", "AI service")
}
