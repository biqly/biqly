package http

import (
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/biqly/biqly/pkg/internalapi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const maxUpstreamProxyBodyBytes = 1 << 20 // 1 MiB

// newUpstreamProxy returns a ReverseProxy that forwards to targetURL, copying
// the request-id into outgoing requests and emitting a 502 JSON error envelope
// on connection failure. Trace context is propagated (and a client span
// emitted) by the otelhttp transport from the inbound request's active span.
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
// newUpstreamProxy reverse-proxies to targetURL. hostOverride, when set,
// becomes the outgoing Host header instead of the target's host — required
// when the target is an in-cluster gateway whose routes match on a public
// hostname.
func newUpstreamProxy(targetURL, hostOverride, envVarName, serviceLabel string) (http.Handler, bool) {
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
			if hostOverride != "" {
				pr.Out.Host = hostOverride
			} else {
				pr.Out.Host = target.Host
			}
			pr.SetXForwarded()
			pr.Out.Header.Set("X-Forwarded-Host", originalHost)
			pr.Out.Header.Set("X-Forwarded-Proto", target.Scheme)
			if id := requestid.FromContext(pr.In.Context()); id != "" {
				pr.Out.Header.Set("X-Request-ID", id)
			}
		},
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.ErrorContext(r.Context(), logTag, "error", err, "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			if err := sonic.ConfigStd.NewEncoder(w).Encode(internalapi.Error{
				Code:  internalapi.CodeUpstream,
				Error: bodyMessage,
			}); err != nil {
				return
			}
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxUpstreamProxyBodyBytes)
		}
		proxy.ServeHTTP(w, r)
	}), true
}
