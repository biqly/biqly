package observability

import (
	"os"
	"strconv"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	defaultTraceSampleRatio = 0.25
	envTracesSampler        = "OTEL_TRACES_SAMPLER"
	envTracesSamplerArg     = "OTEL_TRACES_SAMPLER_ARG"
)

// newTraceSampler builds the process trace sampler from OTEL env vars.
// Defaults to parentbased_traceidratio at 25% for production load control.
func newTraceSampler() sdktrace.Sampler {
	name := os.Getenv(envTracesSampler)
	if name == "" {
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(defaultTraceSampleRatio))
	}
	ratio := parseTraceSampleRatio(os.Getenv(envTracesSamplerArg))
	switch name {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}

func parseTraceSampleRatio(raw string) float64 {
	if raw == "" {
		return defaultTraceSampleRatio
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > 1 {
		return defaultTraceSampleRatio
	}
	return v
}
