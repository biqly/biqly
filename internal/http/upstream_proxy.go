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
)

// newUpstreamProxy returns a ReverseProxy that forwards to targetURL, copying
// request-id and traceparent into outgoing requests and emitting a 502 JSON
// error envelope on connection failure.
//
// envVarName is the name of the environment variable that supplied the URL,
// logged when the URL fails to parse so operators can find the misconfigured
// variable.
//
// serviceLabel is the noun rendered in the 502 body ("AI service",
// "query service", "catalog service") and in error logs.
//
// Returns ok=false (no proxy, no logs) when the URL is empty. Returns
// ok=false with an error log when the URL is non-empty but unparseable.
func newUpstreamProxy(targetURL, envVarName, serviceLabel string) (http.Handler, bool) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil, false
	}
	target, err := url.Parse(targetURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		slog.Error("invalid upstream URL; proxy disabled",
			"env", envVarName,
			"url", targetURL,
			"error", err,
		)
		return nil, false
	}

	logTag := serviceLabel + " proxy failed"
	bodyMessage := serviceLabel + " unavailable"
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
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.ErrorContext(r.Context(), logTag, "error", err, "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(internalapi.Error{
				Code:  internalapi.CodeUpstream,
				Error: bodyMessage,
			})
		},
	}
	return proxy, true
}
