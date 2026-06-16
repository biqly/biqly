package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRuntimeTuningWarnsOnInvalidOverrides(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	t.Setenv("BI_GOMEMLIMIT", "not-bytes")
	t.Setenv("GOMEMLIMIT", "")
	t.Setenv("BI_GOGC", "not-percent")
	t.Setenv("GOGC", "")

	configureMemoryLimit()
	configureGCPercent()

	got := logs.String()
	for _, want := range []string{
		"ignoring invalid memory limit env var",
		"BI_GOMEMLIMIT",
		"ignoring invalid GC percent env var",
		"BI_GOGC",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("logs missing %q: %s", want, got)
		}
	}
}
