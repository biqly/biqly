package observability

import "testing"

func TestParseOTLPHeaders(t *testing.T) {
	t.Parallel()
	got := parseOTLPHeaders("Authorization=Bearer abc123, X-Custom=foo")
	if got["Authorization"] != "Bearer abc123" {
		t.Fatalf("Authorization = %q", got["Authorization"])
	}
	if got["X-Custom"] != "foo" {
		t.Fatalf("X-Custom = %q", got["X-Custom"])
	}
}

func TestOtlpHeadersFromEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=Bearer token")
	got := otlpHeadersFromEnv()
	if got["Authorization"] != "Bearer token" {
		t.Fatalf("Authorization = %q", got["Authorization"])
	}
}
