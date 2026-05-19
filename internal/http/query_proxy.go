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

func registerQueryProxyRoutes(r chi.Router, queryURL string) {
	proxy, ok := newQueryProxy(queryURL)
	if !ok {
		return
	}

	r.Handle("/query", proxy)
	r.Handle("/query/*", proxy)
}

func newQueryProxy(queryURL string) (http.Handler, bool) {
	queryURL = strings.TrimSpace(queryURL)
	if queryURL == "" {
		return nil, false
	}
	target, err := url.Parse(queryURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		slog.Error("invalid BI_QUERY_SERVICE_URL; query proxy disabled", "url", queryURL, "error", err)
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
		slog.ErrorContext(r.Context(), "query proxy failed", "error", err, "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(internalapi.Error{
			Code:  internalapi.CodeUpstream,
			Error: "query service unavailable",
		})
	}
	return proxy, true
}
