package env

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// This file is the single source of truth for reading typed configuration from
// environment variables. Every service's config loader delegates here so the
// parsing/invalid-value policy cannot drift between packages.
//
// Policy: an unset (empty) variable returns the default. An invalid value for a
// type that has an unambiguous parse (Int/Float/Duration) logs a warning and
// returns the default. The "constrained" variants (PositiveInt / NonNegativeInt
// / PositiveDuration) silently fall back to the default when the value is
// invalid OR out of range, matching their pre-consolidation behavior.

// String returns the value of key, or def when unset/empty.
func String(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Int returns key parsed as an int, warning and using def on an invalid value.
func Int(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("ignoring invalid int env var; using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return n
}

// PositiveInt returns key parsed as an int when it is > 0, else def.
func PositiveInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// NonNegativeInt returns key parsed as an int when it is >= 0, else def.
func NonNegativeInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

// Float returns key parsed as a float64, warning and using def on an invalid value.
func Float(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		slog.Warn("ignoring invalid float env var; using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return f
}

// Bool returns key parsed as a bool. Accepted true values: 1/true/yes/on;
// false values: 0/false/no/off (case-insensitive). Anything else returns def.
func Bool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// Duration returns key parsed as a time.Duration, warning and using def on an
// invalid value.
func Duration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("ignoring invalid duration env var; using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return d
}

// PositiveDuration returns key parsed as a time.Duration when it is > 0, else def.
func PositiveDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

// CSV reads key and parses it as a comma-separated list (see SplitCSV).
func CSV(key string) []string {
	return SplitCSV(os.Getenv(key))
}

// SplitCSV parses an already-read comma-separated string into trimmed,
// non-empty values. Returns nil for an empty input so callers can distinguish
// "unset" from an explicit empty list.
func SplitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// CSVDefault is CSV but returns def when key is unset/empty.
func CSVDefault(key string, def []string) []string {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return def
	}
	return CSV(key)
}
