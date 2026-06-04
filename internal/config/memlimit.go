package config

import (
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
		if v, err := strconv.ParseInt(envMem, 10, 64); err == nil {
			debug.SetMemoryLimit(v)
		}
	} else if envGoMem := os.Getenv("GOMEMLIMIT"); envGoMem != "" {
		// If GOMEMLIMIT is already set by runtime environment, Go 1.19+ handles it natively
	} else {
		// Fallback to cgroup-aware memory limit
		if limit := readCgroupMemoryLimit(); limit > 0 {
			debug.SetMemoryLimit(int64(float64(limit) * 0.9))
		}
	}
}

func configureGCPercent() {
	if envGogc := os.Getenv("BI_GOGC"); envGogc != "" {
		if v, err := strconv.Atoi(envGogc); err == nil {
			debug.SetGCPercent(v)
		}
	} else if envGoGC := os.Getenv("GOGC"); envGoGC != "" {
		// If GOGC is already set, Go runtime handles it
	} else {
		debug.SetGCPercent(100)
	}
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
	// Try cgroup v1
	if limitBytes, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		limitStr := strings.TrimSpace(string(limitBytes))
		if limitStr != "" {
			if v, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
				if v > 0 && v < 9223372036854771712 {
					return v
				}
			}
		}
	}
	return 0
}
