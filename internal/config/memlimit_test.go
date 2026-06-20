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

func TestParseCgroupV2MemoryLimit(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "numeric limit", raw: "104857600\n", want: 104857600},
		{name: "max sentinel is unlimited", raw: "max\n", want: 0},
		{name: "empty is ignored", raw: "\n", want: 0},
		{name: "invalid is ignored", raw: "not-a-number", want: 0},
		{name: "negative is ignored", raw: "-1", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCgroupV2MemoryLimit(tt.raw); got != tt.want {
				t.Fatalf("parseCgroupV2MemoryLimit(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseCgroupV1MemoryLimit(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{name: "numeric limit", raw: "268435456\n", want: 268435456},
		{name: "unlimited sentinel is ignored", raw: "9223372036854771712", want: 0},
		{name: "larger than unlimited sentinel is ignored", raw: "9223372036854775807", want: 0},
		{name: "zero is ignored", raw: "0", want: 0},
		{name: "invalid is ignored", raw: "not-a-number", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCgroupV1MemoryLimit(tt.raw); got != tt.want {
				t.Fatalf("parseCgroupV1MemoryLimit(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
