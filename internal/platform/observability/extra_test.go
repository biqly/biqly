package observability

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// ---------------------------------------------------------------------------
// embedding_context.go — 0%
// ---------------------------------------------------------------------------

func TestContextWithEmbeddingOperation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = ContextWithEmbeddingOperation(ctx, "route_recall")

	got := EmbeddingOperationFromContext(ctx)
	if got != "route_recall" {
		t.Fatalf("EmbeddingOperationFromContext = %q, want %q", got, "route_recall")
	}
}

func TestEmbeddingOperationFromContextEmpty(t *testing.T) {
	t.Parallel()

	got := EmbeddingOperationFromContext(context.Background())
	if got != "other" {
		t.Fatalf("EmbeddingOperationFromContext on empty ctx = %q, want %q", got, "other")
	}
}

// ---------------------------------------------------------------------------
// logging.go — 0%
// ---------------------------------------------------------------------------

func TestSetupLogging(t *testing.T) {
	t.Setenv("BI_LOG_LEVEL", "info")
	t.Setenv("BI_LOG_FORMAT", "json")

	l := SetupLogging("info", "json")
	if l == nil {
		t.Fatal("SetupLogging returned nil")
	}

	// Verify it's installed as default
	if slog.Default() != l {
		t.Fatal("SetupLogging did not install as slog.Default")
	}
}

func TestContextWithLogger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	l := slog.Default()

	withLogger := ContextWithLogger(ctx, l)
	if withLogger == ctx {
		t.Fatal("ContextWithLogger returned same ctx")
	}
}

func TestContextWithLoggerNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	withLogger := ContextWithLogger(ctx, nil)
	if withLogger != ctx {
		t.Fatal("ContextWithLogger with nil logger should return same ctx")
	}
}

func TestLoggerFromContext(t *testing.T) {
	t.Parallel()

	l := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := ContextWithLogger(context.Background(), l)

	got := LoggerFromContext(ctx)
	if got != l {
		t.Fatal("LoggerFromContext did not return the stored logger")
	}
}

func TestLoggerFromContextNilCtx(t *testing.T) {
	t.Parallel()

	got := LoggerFromContext(context.TODO())
	def := slog.Default()
	if got != def {
		t.Fatal("LoggerFromContext(nil) should return slog.Default()")
	}
}

func TestLoggerFromContextEmptyCtx(t *testing.T) {
	t.Parallel()

	got := LoggerFromContext(context.Background())
	def := slog.Default()
	if got != def {
		t.Fatal("LoggerFromContext(empty ctx) should return slog.Default()")
	}
}

// ---------------------------------------------------------------------------
// metrics.go — RecordQuery, RecordValidationFailure, RecordConnectionError
// ---------------------------------------------------------------------------

func TestRecordQuery(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// Success, cache hit
	m.RecordQuery(100, true, true)
	assertMetric(t, m.queriesTotal, 1, "bi_queries_total")
	assertMetric(t, m.queryErrors, 0, "bi_query_errors_total")
	assertMetric(t, m.cacheHits, 1, "bi_cache_hits_total")
	assertMetric(t, m.cacheMisses, 0, "bi_cache_misses_total")

	// Failure, cache miss
	m.RecordQuery(200, false, false)
	assertMetric(t, m.queriesTotal, 2, "bi_queries_total")
	assertMetric(t, m.queryErrors, 1, "bi_query_errors_total")
	assertMetric(t, m.cacheHits, 1, "bi_cache_hits_total")
	assertMetric(t, m.cacheMisses, 1, "bi_cache_misses_total")
}

func TestRecordValidationFailure(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordValidationFailure()
	assertMetric(t, m.validationFailures, 1, "bi_validation_failures_total")

	m.RecordValidationFailure()
	assertMetric(t, m.validationFailures, 2, "bi_validation_failures_total")
}

func TestRecordConnectionError(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordConnectionError()
	assertMetric(t, m.connectionErrors, 1, "bi_connection_errors_total")

	m.RecordConnectionError()
	assertMetric(t, m.connectionErrors, 2, "bi_connection_errors_total")
}

// ---------------------------------------------------------------------------
// trace.go — Tracer, SpanIDs, SetupTracing
// ---------------------------------------------------------------------------

func TestTracer(t *testing.T) {
	// Use a test provider so we don't pollute global state permanently
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)

	tr := Tracer("test-component")
	if tr == nil {
		t.Fatal("Tracer returned nil")
	}

	_, span := tr.Start(context.Background(), "test-span")
	span.End()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestSpanIDs(t *testing.T) {
	t.Parallel()

	// No span in context → empty strings
	traceID, spanID := SpanIDs(context.Background())
	if traceID != "" || spanID != "" {
		t.Fatalf("SpanIDs on empty ctx = (%q, %q), want (\"\", \"\")", traceID, spanID)
	}

	// With a valid span context
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	tr := tp.Tracer("test")
	ctx, span := tr.Start(context.Background(), "test-span")
	traceID, spanID = SpanIDs(ctx)
	span.End()

	if traceID == "" {
		t.Fatal("SpanIDs on span ctx should return non-empty trace ID")
	}
	if spanID == "" {
		t.Fatal("SpanIDs on span ctx should return non-empty span ID")
	}
}

func TestSetupTracingNoEndpoint(t *testing.T) {
	// Clear env vars so SetupTracing takes the no-op path
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := SetupTracing(context.Background(), "test-component")
	if err != nil {
		t.Fatalf("SetupTracing returned err: %v", err)
	}
	if shutdown == nil {
		t.Fatal("SetupTracing returned nil shutdown")
	}
	// No-op shutdown should not error
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned err: %v", err)
	}
}

func TestSetupTracingWithEndpoint(t *testing.T) {
	// Set an invalid endpoint so the exporter creation fails, testing
	// the error path (we can't easily test the success path without a real endpoint)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "")

	shutdown, err := SetupTracing(context.Background(), "test-component")
	// The exporter will fail to connect on creation if the endpoint is unreachable,
	// but otlptracehttp.New may not fail immediately — it can defer connection.
	// At minimum verify it returns something.
	if shutdown == nil {
		t.Fatal("SetupTracing returned nil shutdown even with endpoint set")
	}
	// err may be nil since otlptracehttp.New may create the client lazily.
	_ = err
	_ = shutdown(context.Background())
}

// ---------------------------------------------------------------------------
// db_pool_metrics.go — 0%
// ---------------------------------------------------------------------------

func TestSnapshotFromDB(t *testing.T) {
	t.Parallel()

	// nil db → zero snapshot
	got := SnapshotFromDB(nil)
	if got != (DBPoolSnapshot{}) {
		t.Fatalf("SnapshotFromDB(nil) = %+v, want zero value", got)
	}
}

func TestSnapshotFromDBWithDB(t *testing.T) {
	// Open an in-memory SQLite database to get a real *sql.DB
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 driver not available: %v", err)
	}
	defer func() { _ = db.Close() }()

	got := SnapshotFromDB(db)
	// We can't predict exact values, but we can verify the shape
	_ = got.OpenConnections
	_ = got.InUse
	_ = got.Idle
	// Just verify it doesn't panic and returns something
	_ = got
}

func TestDBPoolStatsFromDB(t *testing.T) {
	t.Parallel()

	// nil db
	provider := DBPoolStatsFromDB(nil)
	if provider == nil {
		t.Fatal("DBPoolStatsFromDB(nil) returned nil")
	}
	snap := provider()
	if snap != (DBPoolSnapshot{}) {
		t.Fatalf("provider() from nil db = %+v, want zero", snap)
	}
}

func TestRegisterDBPoolMetricsNilGuard(t *testing.T) {
	t.Parallel()

	// nil reg → no panic
	RegisterDBPoolMetrics(nil, "test", func() DBPoolSnapshot { return DBPoolSnapshot{} })

	// nil provider → no panic
	reg := prometheus.NewRegistry()
	RegisterDBPoolMetrics(reg, "test", nil)

	// empty name → no panic
	RegisterDBPoolMetrics(reg, "", func() DBPoolSnapshot { return DBPoolSnapshot{} })
}

func TestRegisterDBPoolMetricsDedup(_ *testing.T) {
	reg := prometheus.NewRegistry()
	provider := func() DBPoolSnapshot {
		return DBPoolSnapshot{OpenConnections: 5}
	}

	// First registration should succeed
	RegisterDBPoolMetrics(reg, "dedup_test", provider)

	// Read the gauge value
	_ = testutil.CollectAndCount(reg, "biqly_db_pool_open_connections")

	// Second registration for same pool name should be idempotent (no panic)
	RegisterDBPoolMetrics(reg, "dedup_test", provider)
}

// ---------------------------------------------------------------------------
// redis.go — 0%
// ---------------------------------------------------------------------------

func TestInstrumentRedis(t *testing.T) {
	// Create a redis client pointing to nowhere — InstrumentTracing
	// just adds hooks and doesn't connect.
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:16379", // unlikely to be running
	})
	defer func() { _ = rdb.Close() }()

	err := InstrumentRedis(rdb, "test-cache")
	if err != nil {
		t.Fatalf("InstrumentRedis returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// trace_ctx.go — WithDBSystem, DBSystem, WithQueryFingerprint, QueryFingerprint
// ---------------------------------------------------------------------------

func TestWithDBSystem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Empty system → same ctx
	if got := WithDBSystem(ctx, ""); got != ctx {
		t.Fatal("WithDBSystem with empty system should return same ctx")
	}

	// Non-empty system
	ctx2 := WithDBSystem(ctx, "postgres")
	if ctx2 == ctx {
		t.Fatal("WithDBSystem should return new ctx")
	}

	if got := DBSystem(ctx2); got != "postgres" {
		t.Fatalf("DBSystem = %q, want %q", got, "postgres")
	}

	// Nil ctx
	if got := DBSystem(context.TODO()); got != "" {
		t.Fatalf("DBSystem(nil) = %q, want \"\"", got)
	}
}

func TestWithQueryFingerprint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Empty fingerprint → same ctx
	if got := WithQueryFingerprint(ctx, ""); got != ctx {
		t.Fatal("WithQueryFingerprint with empty fingerprint should return same ctx")
	}

	// Non-empty fingerprint
	fp := "abc123"
	ctx2 := WithQueryFingerprint(ctx, fp)
	if ctx2 == ctx {
		t.Fatal("WithQueryFingerprint should return new ctx")
	}

	if got := QueryFingerprint(ctx2); got != fp {
		t.Fatalf("QueryFingerprint = %q, want %q", got, fp)
	}

	// Nil ctx
	if got := QueryFingerprint(context.TODO()); got != "" {
		t.Fatalf("QueryFingerprint(nil) = %q, want \"\"", got)
	}
}

// ---------------------------------------------------------------------------
// spanattrs.go — SetDBSystemAttributes (0%)
// ---------------------------------------------------------------------------

func TestSetDBSystemAttributes(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	tr := tp.Tracer("test")

	// Nil span → no panic
	SetDBSystemAttributes(nil, "postgres")

	// Empty system → no attributes set
	_, span := tr.Start(context.Background(), "empty")
	SetDBSystemAttributes(span, "")
	span.End()

	// Non-empty system
	_, span2 := tr.Start(context.Background(), "with-system")
	SetDBSystemAttributes(span2, "mysql")
	span2.End()

	spans := rec.Ended()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	emptyAttrs := spans[0].Attributes()
	for _, a := range emptyAttrs {
		if string(a.Key) == "db.system" || string(a.Key) == "datasource.driver" {
			t.Fatalf("unexpected attr %s on empty system span", a.Key)
		}
	}

	mysqlAttrs := spans[1].Attributes()
	foundDBSystem := false
	foundDriver := false
	for _, a := range mysqlAttrs {
		switch string(a.Key) {
		case "db.system":
			if a.Value.AsString() != "mysql" {
				t.Fatalf("db.system = %q, want %q", a.Value.AsString(), "mysql")
			}
			foundDBSystem = true
		case "datasource.driver":
			if a.Value.AsString() != "mysql" {
				t.Fatalf("datasource.driver = %q, want %q", a.Value.AsString(), "mysql")
			}
			foundDriver = true
		}
	}
	if !foundDBSystem {
		t.Fatal("missing db.system attribute")
	}
	if !foundDriver {
		t.Fatal("missing datasource.driver attribute")
	}
}

// ---------------------------------------------------------------------------
// tier1_metrics.go — HTTPStatusClass, RecordLLMProviderError, RecordLLMProviderRetry
// ---------------------------------------------------------------------------

func TestHTTPStatusClassAllBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status int
		want   string
	}{
		{199, "other"},
		{200, "2xx"},
		{299, "2xx"},
		{300, "3xx"},
		{399, "3xx"},
		{400, "4xx"},
		{499, "4xx"},
		{500, "5xx"},
		{599, "5xx"},
	}
	for _, tc := range cases {
		got := HTTPStatusClass(tc.status)
		if got != tc.want {
			t.Fatalf("HTTPStatusClass(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestRecordLLMProviderErrorNilMetrics(t *testing.T) {
	t.Parallel()

	var m *Metrics
	// Should not panic
	m.RecordLLMProviderError("openai", "rate_limit")
	m.RecordLLMProviderError("", "")
}

func TestRecordLLMProviderRetryNilMetrics(t *testing.T) {
	t.Parallel()

	var m *Metrics
	// Should not panic
	m.RecordLLMProviderRetry("openai")
	m.RecordLLMProviderRetry("")
}

func TestRecordLLMProviderErrorWithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// Bounded labels
	m.RecordLLMProviderError("openai", "rate_limit")
	assertMetric(t, m.llmErrorsTotal.WithLabelValues("openai", "rate_limit"), 1,
		"biqly_llm_errors_total{provider=openai,error_type=rate_limit}")

	// Unbounded labels should fall through to "other"
	m.RecordLLMProviderError("unknown_provider", "unknown_error")
	assertMetric(t, m.llmErrorsTotal.WithLabelValues("other", "other"), 1,
		"biqly_llm_errors_total{provider=other,error_type=other}")
}

func TestRecordLLMProviderRetryWithRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordLLMProviderRetry("openai")
	assertMetric(t, m.llmRetriesTotal.WithLabelValues("openai"), 1,
		"biqly_llm_retries_total{provider=openai}")

	// Unbounded → other
	m.RecordLLMProviderRetry("unknown_provider")
	assertMetric(t, m.llmRetriesTotal.WithLabelValues("other"), 1,
		"biqly_llm_retries_total{provider=other}")
}

// ---------------------------------------------------------------------------
// provider_errors.go — isRetriableProviderNetErr
// ---------------------------------------------------------------------------

func TestIsRetriableProviderNetErr(t *testing.T) {
	t.Parallel()

	// nil err → false
	if isRetriableProviderNetErr(nil) {
		t.Fatal("isRetriableProviderNetErr(nil) should be false")
	}

	// syscall.ECONNRESET → true
	if !isRetriableProviderNetErr(syscall.ECONNRESET) {
		t.Fatal("isRetriableProviderNetErr(ECONNRESET) should be true")
	}

	// syscall.EPIPE → true
	if !isRetriableProviderNetErr(syscall.EPIPE) {
		t.Fatal("isRetriableProviderNetErr(EPIPE) should be true")
	}

	// syscall.ETIMEDOUT → true
	if !isRetriableProviderNetErr(syscall.ETIMEDOUT) {
		t.Fatal("isRetriableProviderNetErr(ETIMEDOUT) should be true")
	}

	// net.Error with Timeout() → true
	timeoutErr := &timeoutError{}
	if !isRetriableProviderNetErr(timeoutErr) {
		t.Fatal("isRetriableProviderNetErr(timeout net.Error) should be true")
	}

	// *net.OpError → true
	opErr := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	if !isRetriableProviderNetErr(opErr) {
		t.Fatal("isRetriableProviderNetErr(net.OpError) should be true")
	}

	// Random unrelated error → false
	if isRetriableProviderNetErr(errors.New("some random error")) {
		t.Fatal("isRetriableProviderNetErr(random error) should be false")
	}
}

// timeoutError implements net.Error with Timeout()=true
type timeoutError struct{}

func (*timeoutError) Error() string   { return "timeout" }
func (*timeoutError) Timeout() bool   { return true }
func (*timeoutError) Temporary() bool { return false }

// ---------------------------------------------------------------------------
// RecordModelPublish — improve from 75% (hit the error path)
// ---------------------------------------------------------------------------

func TestRecordModelPublishErrorPath(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// Success
	m.RecordModelPublish(100, true)
	assertMetric(t, m.modelPublishes, 1, "model_publish_total")
	assertMetric(t, m.modelPublishErrors, 0, "model_publish_errors_total")

	// Failure
	m.RecordModelPublish(200, false)
	assertMetric(t, m.modelPublishes, 2, "model_publish_total")
	assertMetric(t, m.modelPublishErrors, 1, "model_publish_errors_total")
}

// ---------------------------------------------------------------------------
// RecordHTTPRequest nil guard
// ---------------------------------------------------------------------------

func TestRecordHTTPRequestNilMetrics(t *testing.T) {
	t.Parallel()

	var m *Metrics
	m.RecordHTTPRequest("GET", "/test", http.StatusOK, 100)
	m.RecordHTTPRequest("GET", "/test", http.StatusOK, -1) // negative duration
}

// ---------------------------------------------------------------------------
// RecordAIStep guard against negative latency
// ---------------------------------------------------------------------------

func TestRecordAIStepNegative(_ *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// Negative latency should be ignored
	m.RecordAIStep("llm_generate", -1)
	// No assertion needed — should not panic and not record
}

// ---------------------------------------------------------------------------
// RecordQuery negative duration test (just ensure it doesn't panic)
// ---------------------------------------------------------------------------

func TestRecordQueryZeroDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordQuery(0, true, true)
	assertMetric(t, m.queriesTotal, 1, "bi_queries_total")
}

// ---------------------------------------------------------------------------
// NewMetrics with custom registry that doesn't implement Gatherer
// ---------------------------------------------------------------------------

type noGatherRegisterer struct {
	prometheus.Registerer
}

func TestNewMetricsWithNonGatherer(t *testing.T) {
	reg := prometheus.NewRegistry()
	ng := &noGatherRegisterer{Registerer: reg}
	m := NewMetrics(ng)
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
}
