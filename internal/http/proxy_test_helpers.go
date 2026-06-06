package http

import (
	"bytes"
	"context"
	"github.com/bytedance/sonic"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/pkg/internalapi"
)

type proxyRouteCase struct {
	method string
	path   string
}

func routerWithCountingUpstream(t *testing.T, applyServiceURL func(*config.ServicesConfig, string)) (stdhttp.Handler, *int) {
	t.Helper()
	var calls int
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		calls++
		w.WriteHeader(stdhttp.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	services := config.ServicesConfig{}
	applyServiceURL(&services, upstream.URL)
	handler := Router(&app.Dependencies{
		Config: &config.Config{Services: services},
	})
	return handler, &calls
}

func assertProxiedRoutes(t *testing.T, handler stdhttp.Handler, cases []proxyRouteCase) {
	t.Helper()
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, bytes.NewBufferString(`{}`))
		handler.ServeHTTP(rec, req)

		if rec.Code != stdhttp.StatusOK {
			t.Fatalf("%s %s status: got %d, want 200; body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		var body map[string]bool
		if err := sonic.ConfigStd.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode %s %s: %v", tc.method, tc.path, err)
		}
		if !body["proxied"] {
			t.Fatalf("%s %s expected proxied response, got %+v", tc.method, tc.path, body)
		}
	}
}

func assertRouteStaysLocal(
	t *testing.T,
	handler stdhttp.Handler,
	method, path, body string,
	wantStatus int,
	upstreamCalls *int,
	localRouteLabel string,
) {
	t.Helper()
	rec := httptest.NewRecorder()
	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}
	req := httptest.NewRequestWithContext(context.Background(), method, path, reqBody)
	handler.ServeHTTP(rec, req)

	if rec.Code != wantStatus {
		t.Fatalf("status: got %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	if upstreamCalls != nil && *upstreamCalls != 0 {
		t.Fatalf("%s route should stay local, upstream calls=%d", localRouteLabel, *upstreamCalls)
	}
}

func assertProxyErrorEnvelope(t *testing.T, handler stdhttp.Handler, method, path, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}
	req := httptest.NewRequestWithContext(context.Background(), method, path, reqBody)
	handler.ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusBadGateway {
		t.Fatalf("status: got %d, want 502; body=%s", rec.Code, rec.Body.String())
	}
	var env internalapi.Error
	if err := sonic.ConfigStd.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != internalapi.CodeUpstream {
		t.Fatalf("code: got %q, want %q", env.Code, internalapi.CodeUpstream)
	}
}
