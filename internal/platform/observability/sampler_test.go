package observability

import (
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestParseTraceSampleRatio(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  string
		want float64
	}{
		{"", defaultTraceSampleRatio},
		{"0.1", 0.1},
		{"1", 1},
		{"0", 0},
		{"bad", defaultTraceSampleRatio},
		{"1.5", defaultTraceSampleRatio},
	}
	for _, tc := range tests {
		if got := parseTraceSampleRatio(tc.raw); got != tc.want {
			t.Fatalf("parseTraceSampleRatio(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestNewTraceSamplerDefaultsToParentBasedRatio(t *testing.T) {
	t.Setenv(envTracesSampler, "")
	t.Setenv(envTracesSamplerArg, "")

	if newTraceSampler() == nil {
		t.Fatal("expected non-nil sampler")
	}
}

func TestNewTraceSamplerAlwaysOff(t *testing.T) {
	t.Setenv(envTracesSampler, "always_off")
	t.Setenv(envTracesSamplerArg, "")

	s := newTraceSampler()
	decision := s.ShouldSample(sdktrace.SamplingParameters{})
	if decision.Decision != sdktrace.Drop {
		t.Fatalf("expected Drop, got %v", decision.Decision)
	}
}
