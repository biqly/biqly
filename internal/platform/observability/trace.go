package observability

import (
	"context"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns a named OTEL tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.GetTracerProvider().Tracer(name)
}

// SpanIDs returns the active trace and span IDs from ctx for log/audit
// correlation. Both are empty when ctx carries no valid span context (e.g.
// tracing disabled or a request that never entered an instrumented handler).
func SpanIDs(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

const defaultServiceName = "biqly"

// ServiceName returns OTEL_SERVICE_NAME when set, otherwise fallback (or biqly).
func ServiceName(fallback string) string {
	if name := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); name != "" {
		return name
	}
	if name := strings.TrimSpace(fallback); name != "" {
		return name
	}
	return defaultServiceName
}

// SetupTracing initialises the global OTEL trace provider.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is set the provider exports spans via OTLP
// HTTP. Optional OTEL_EXPORTER_OTLP_HEADERS supplies auth (e.g.
// Authorization=Bearer <token>). Otherwise a no-op (noop) provider is
// installed, so instrumented code still compiles and runs without any trace
// back-end.
//
// component labels the running binary (api, worker, auth) on each span resource
// while service.name comes from OTEL_SERVICE_NAME (default biqly).
//
// The returned shutdown function flushes pending spans and must be deferred by
// the caller before process exit.
func SetupTracing(ctx context.Context, component string) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return func(_ context.Context) error { return nil }, nil
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceName(ServiceName(defaultServiceName)),
	}
	if c := strings.TrimSpace(component); c != "" {
		attrs = append(attrs, attribute.String("service.component", c))
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return func(_ context.Context) error { return nil }, err
	}

	var exporterOpts []otlptracehttp.Option
	if headers := otlpHeadersFromEnv(); len(headers) > 0 {
		exporterOpts = append(exporterOpts, otlptracehttp.WithHeaders(headers))
	}
	exporter, err := otlptracehttp.New(ctx, exporterOpts...)
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
