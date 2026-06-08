package observability

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SetAITokenAttributes records LLM token usage on a span. Total is derived as
// prompt+completion when total is zero but either count is positive.
func SetAITokenAttributes(span trace.Span, prompt, completion, total int) {
	if span == nil || !span.IsRecording() {
		return
	}
	var attrs []attribute.KeyValue
	if prompt > 0 {
		attrs = append(attrs, attribute.Int("ai.tokens.prompt", prompt))
	}
	if completion > 0 {
		attrs = append(attrs, attribute.Int("ai.tokens.completion", completion))
	}
	if total <= 0 && (prompt > 0 || completion > 0) {
		total = prompt + completion
	}
	if total > 0 {
		attrs = append(attrs, attribute.Int("ai.tokens.total", total))
	}
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
}

// DBSystemAttributes returns standard datasource driver attributes for spans.
func DBSystemAttributes(system string) []attribute.KeyValue {
	if system == "" {
		return nil
	}
	return []attribute.KeyValue{
		attribute.String("db.system", system),
		attribute.String("datasource.driver", system),
	}
}

// SetDBSystemAttributes attaches db.system and datasource.driver when system is set.
func SetDBSystemAttributes(span trace.Span, system string) {
	if span == nil || system == "" {
		return
	}
	span.SetAttributes(DBSystemAttributes(system)...)
}
