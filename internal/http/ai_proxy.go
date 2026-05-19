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

func registerAIProxyRoutes(r chi.Router, aiURL string) {
	proxy, ok := newAIProxy(aiURL)
	if !ok {
		return
	}

	r.Handle("/ai", proxy)
	r.Handle("/ai/*", proxy)
}

func newAIProxy(aiURL string) (http.Handler, bool) {
	aiURL = strings.TrimSpace(aiURL)
	if aiURL == "" {
		return nil, false
	}
	target, err := url.Parse(aiURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		slog.Error("invalid BI_AI_SERVICE_URL; AI proxy disabled", "url", aiURL, "error", err)
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
		slog.ErrorContext(r.Context(), "AI proxy failed", "error", err, "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(internalapi.Error{
			Code:  internalapi.CodeUpstream,
			Error: "AI service unavailable",
		})
	}
	return proxy, true
}
