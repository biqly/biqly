package observability

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSetAITokenAttributesDerivesTotal(t *testing.T) {
	t.Parallel()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	tr := tp.Tracer("test")
	_, span := tr.Start(t.Context(), "llm")
	SetAITokenAttributes(span, 10, 5, 0)
	span.End()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	attrs := spans[0].Attributes()
	assertAttrInt(t, attrs, "ai.tokens.prompt", 10)
	assertAttrInt(t, attrs, "ai.tokens.completion", 5)
	assertAttrInt(t, attrs, "ai.tokens.total", 15)
}

func TestDBSystemAttributes(t *testing.T) {
	t.Parallel()
	attrs := DBSystemAttributes("postgres")
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}
	if attrs[0].Key != "db.system" || attrs[0].Value.AsString() != "postgres" {
		t.Fatalf("unexpected db.system: %+v", attrs[0])
	}
	if attrs[1].Key != "datasource.driver" {
		t.Fatalf("unexpected second attr: %+v", attrs[1])
	}
}

func assertAttrInt(t *testing.T, attrs []attribute.KeyValue, key string, want int64) {
	t.Helper()
	for _, a := range attrs {
		if string(a.Key) == key {
			if a.Value.Type() != attribute.INT64 || a.Value.AsInt64() != want {
				t.Fatalf("%s = %v, want %d", key, a.Value, want)
			}
			return
		}
	}
	t.Fatalf("missing attribute %s in %+v", key, attrs)
}
