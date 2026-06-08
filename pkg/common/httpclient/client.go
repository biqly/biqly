// Package httpclient provides shared HTTP client defaults for service-to-service calls.
package httpclient

import (
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewServiceClient returns an HTTP client tuned for internal service traffic.
//
// The transport is wrapped with otelhttp so every outbound call emits a client
// span and injects the active trace context (W3C traceparent + baggage) via the
// global propagator. When no TracerProvider is configured the wrapper is inert,
// so callers pay nothing extra in traces-disabled deployments.
func NewServiceClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(NewServiceTransport()),
	}
}

// NewServiceTransport returns a transport with bounded connection setup and
// reusable keep-alive connections.
func NewServiceTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
