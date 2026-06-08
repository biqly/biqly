package observability

import "testing"

func TestServiceName(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "")
	if got := ServiceName(""); got != "biqly" {
		t.Fatalf("ServiceName(\"\") = %q, want biqly", got)
	}
	if got := ServiceName("fallback"); got != "fallback" {
		t.Fatalf("ServiceName(fallback) = %q, want fallback", got)
	}
	t.Setenv("OTEL_SERVICE_NAME", "biqly")
	if got := ServiceName("other"); got != "biqly" {
		t.Fatalf("ServiceName with env = %q, want biqly", got)
	}
}
