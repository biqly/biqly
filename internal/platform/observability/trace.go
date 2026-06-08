package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns a named OTEL tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.GetTracerProvider().Tracer(name)
}

// SetupTracing initialises the global OTEL trace provider.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is set the provider exports spans via OTLP
// HTTP. Otherwise a no-op (noop) provider is installed, so instrumented code
// still compiles and runs without any trace back-end.
//
// The returned shutdown function flushes pending spans and must be deferred by
// the caller before process exit.
func SetupTracing(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return func(_ context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return func(_ context.Context) error { return nil }, err
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return func(_ context.Context) error { return nil }, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(newTraceSampler()),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
