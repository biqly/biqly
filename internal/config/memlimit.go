package config

import (
	"log/slog"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	// Automatically configure GOMAXPROCS for container CPU quotas.
	_ "go.uber.org/automaxprocs"
)

func init() {
	configureMemoryLimit()
	configureGCPercent()
}

func configureMemoryLimit() {
	if envMem := os.Getenv("BI_GOMEMLIMIT"); envMem != "" {
		v, err := strconv.ParseInt(envMem, 10, 64)
		if err == nil {
			debug.SetMemoryLimit(v)
			return
		}
		slog.Warn("ignoring invalid memory limit env var; using default",
			"key", "BI_GOMEMLIMIT",
			"value", envMem,
			"error", err,
		)
		return
	}
	if envGoMem := os.Getenv("GOMEMLIMIT"); envGoMem != "" {
		// If GOMEMLIMIT is already set by runtime environment, Go 1.19+ handles it natively
		return
	}
	// Fallback to cgroup-aware memory limit
	if limit := readCgroupMemoryLimit(); limit > 0 {
		debug.SetMemoryLimit(int64(float64(limit) * 0.9))
	}
}

func configureGCPercent() {
	if envGogc := os.Getenv("BI_GOGC"); envGogc != "" {
		v, err := strconv.Atoi(envGogc)
		if err == nil {
			debug.SetGCPercent(v)
			return
		}
		slog.Warn("ignoring invalid GC percent env var; using default",
			"key", "BI_GOGC",
			"value", envGogc,
			"error", err,
		)
		return
	}
	if envGoGC := os.Getenv("GOGC"); envGoGC != "" {
		// If GOGC is already set, Go runtime handles it
		return
	}
	debug.SetGCPercent(100)
}

func readCgroupMemoryLimit() int64 {
	// Try cgroup v2
	if limitBytes, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		limitStr := strings.TrimSpace(string(limitBytes))
		if limitStr != "max" && limitStr != "" {
			if v, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
				return v
			}
		}
	}
	return readCgroupV1MemoryLimit()
}

func readCgroupV1MemoryLimit() int64 {
	limitBytes, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return 0
	}
	limitStr := strings.TrimSpace(string(limitBytes))
	if limitStr == "" {
		return 0
	}
	v, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || v <= 0 || v >= 9223372036854771712 {
		return 0
	}
	return v
}
