package http

import (
	"context"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestMain installs a W3C propagator and a recording trace provider so the
// otelhttp server handler and proxy transport behave as they do in production
// (SetupTracing wires the same propagator). Without a recording provider the
// server span would not continue the inbound trace, breaking span-context
// assertions in proxy and audit tests.
func TestMain(m *testing.M) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	code := m.Run()
	_ = tp.Shutdown(context.Background())
	os.Exit(code)
}
