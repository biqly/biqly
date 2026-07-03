package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestSetupLogExportNoEndpointIsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	prev := slog.Default()
	defer slog.SetDefault(prev)

	shutdown, err := SetupLogExport(context.Background(), "api")
	if err != nil {
		t.Fatalf("SetupLogExport returned error: %v", err)
	}
	if slog.Default() != prev {
		t.Fatal("SetupLogExport must not replace the default logger without an endpoint")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown returned error: %v", err)
	}
}

func TestSetupLogExportDisabledByFlag(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("BI_OTEL_LOGS_ENABLED", "false")

	prev := slog.Default()
	defer slog.SetDefault(prev)

	if _, err := SetupLogExport(context.Background(), "api"); err != nil {
		t.Fatalf("SetupLogExport returned error: %v", err)
	}
	if slog.Default() != prev {
		t.Fatal("SetupLogExport must not replace the default logger when disabled")
	}
}

func TestSetupLogExportInstallsTeeHandler(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("BI_OTEL_LOGS_ENABLED", "")

	prev := slog.Default()
	defer slog.SetDefault(prev)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	shutdown, err := SetupLogExport(context.Background(), "api")
	if err != nil {
		t.Fatalf("SetupLogExport returned error: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	if _, ok := slog.Default().Handler().(teeHandler); !ok {
		t.Fatalf("default handler = %T, want teeHandler", slog.Default().Handler())
	}

	// The stdout leg must keep receiving records.
	slog.Info("tee smoke", "k", "v")
	if !strings.Contains(buf.String(), "tee smoke") {
		t.Fatalf("stdout handler did not receive the record; buffer: %q", buf.String())
	}
}

// recordingHandler captures records and honours a minimum level, standing in
// for the stdout leg of the tee.
type recordingHandler struct {
	min     slog.Level
	records *[]slog.Record
	attrs   []slog.Attr
}

func (h recordingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.min
}

func (h recordingHandler) Handle(_ context.Context, r slog.Record) error {
	*h.records = append(*h.records, r)
	return nil
}

func (h recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = append(h.attrs, attrs...)
	return h
}

func (h recordingHandler) WithGroup(string) slog.Handler { return h }

func TestTeeHandlerFansOutAndHonoursLevels(t *testing.T) {
	t.Parallel()

	var a, b []slog.Record
	tee := newTeeHandler(
		recordingHandler{min: slog.LevelInfo, records: &a},
		recordingHandler{min: slog.LevelError, records: &b},
	)
	logger := slog.New(tee)

	logger.Info("hello")
	logger.Error("boom")

	if len(a) != 2 {
		t.Fatalf("info-level handler got %d records, want 2", len(a))
	}
	if len(b) != 1 {
		t.Fatalf("error-level handler got %d records, want 1 (info must be filtered)", len(b))
	}
	if !tee.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("tee must be enabled when any leg is enabled")
	}
	if tee.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("tee must be disabled when every leg rejects the level")
	}
}
