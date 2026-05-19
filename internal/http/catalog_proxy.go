package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/biqly/biqly/pkg/internalapi"
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
	catalogURL = strings.TrimSpace(catalogURL)
	if catalogURL == "" {
		return nil, false
	}
	target, err := url.Parse(catalogURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		slog.Error("invalid BI_CATALOG_SERVICE_URL; catalog proxy disabled", "url", catalogURL, "error", err)
		return nil, false
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			originalHost := pr.In.Host
			pr.SetURL(target)
			pr.Out.Host = target.Host
			pr.SetXForwarded()
			pr.Out.Header.Set("X-Forwarded-Host", originalHost)
			pr.Out.Header.Set("X-Forwarded-Proto", target.Scheme)
			if id := requestid.FromContext(pr.In.Context()); id != "" {
				pr.Out.Header.Set("X-Request-ID", id)
			}
			if traceparent := tracecontext.TraceparentFromContext(pr.In.Context()); traceparent != "" {
				pr.Out.Header.Set("traceparent", traceparent)
			}
		},
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.ErrorContext(r.Context(), "catalog proxy failed", "error", err, "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(internalapi.Error{
			Code:  internalapi.CodeUpstream,
			Error: "catalog service unavailable",
		})
	}
	return proxy, true
}
