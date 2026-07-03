package observability

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SetupLogExport mirrors SetupTracing for logs: when OTEL_EXPORTER_OTLP_ENDPOINT
// is set (and BI_OTEL_LOGS_ENABLED is not "false"), it builds an OTLP/HTTP log
// exporter and re-installs the process-default slog logger as a fan-out of the
// current handler (stdout JSON/text) and the OTel bridge, so every slog call
// keeps writing to stdout AND ships to the collector. Records logged with the
// *Context variants carry trace/span IDs for trace↔log correlation.
//
// Call it after SetupLogging (it wraps whatever handler is installed as the
// default at call time). The returned shutdown flushes buffered records and
// must be deferred by the caller before process exit.
func SetupLogExport(ctx context.Context, component string) (shutdown func(context.Context) error, err error) {
	noop := func(_ context.Context) error { return nil }

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return noop, nil
	}
	if v := strings.TrimSpace(os.Getenv("BI_OTEL_LOGS_ENABLED")); strings.EqualFold(v, "false") {
		return noop, nil
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
		return noop, err
	}

	var exporterOpts []otlploghttp.Option
	if headers := otlpHeadersFromEnv(); len(headers) > 0 {
		exporterOpts = append(exporterOpts, otlploghttp.WithHeaders(headers))
	}
	exporter, err := otlploghttp.New(ctx, exporterOpts...)
	if err != nil {
		return noop, err
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	bridge := otelslog.NewHandler(ServiceName(defaultServiceName), otelslog.WithLoggerProvider(provider))
	slog.SetDefault(slog.New(newTeeHandler(slog.Default().Handler(), bridge)))

	return provider.Shutdown, nil
}

// teeHandler fans every record out to all wrapped handlers. The stdout handler
// keeps enforcing its own level; the OTel bridge accepts everything and the
// backend filters, so shipping is never stricter than the local log level.
type teeHandler struct {
	handlers []slog.Handler
}

func newTeeHandler(handlers ...slog.Handler) teeHandler {
	return teeHandler{handlers: handlers}
}

func (t teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range t.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t teeHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range t.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (t teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return teeHandler{handlers: next}
}

func (t teeHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		next[i] = h.WithGroup(name)
	}
	return teeHandler{handlers: next}
}
