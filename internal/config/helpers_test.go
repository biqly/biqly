package config

import (
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	if got := getEnv("BI_TEST_UNSET_KEY", "fallback"); got != "fallback" {
		t.Errorf("getEnv unset = %q, want fallback", got)
	}
	t.Setenv("BI_TEST_SET_KEY", "value")
	if got := getEnv("BI_TEST_SET_KEY", "fallback"); got != "value" {
		t.Errorf("getEnv set = %q, want value", got)
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   ", nil},
		{"single", "a", []string{"a"}},
		{"trimmed and skips empties", " a , b ,, c ", []string{"a", "b", "c"}},
		{"all empty segments", " , , ", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCSV(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("splitCSV(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	if got := getEnvAsInt("BI_TEST_INT_UNSET", 7); got != 7 {
		t.Errorf("unset = %d, want 7", got)
	}
	t.Setenv("BI_TEST_INT", "42")
	if got := getEnvAsInt("BI_TEST_INT", 7); got != 42 {
		t.Errorf("valid = %d, want 42", got)
	}
	t.Setenv("BI_TEST_INT_BAD", "notanint")
	if got := getEnvAsInt("BI_TEST_INT_BAD", 7); got != 7 {
		t.Errorf("invalid should fall back to default, got %d", got)
	}
}

func TestGetEnvAsFloat(t *testing.T) {
	if got := getEnvAsFloat("BI_TEST_FLOAT_UNSET", 1.5); got != 1.5 {
		t.Errorf("unset = %v, want 1.5", got)
	}
	t.Setenv("BI_TEST_FLOAT", "3.25")
	if got := getEnvAsFloat("BI_TEST_FLOAT", 1.5); got != 3.25 {
		t.Errorf("valid = %v, want 3.25", got)
	}
	t.Setenv("BI_TEST_FLOAT_BAD", "x")
	if got := getEnvAsFloat("BI_TEST_FLOAT_BAD", 1.5); got != 1.5 {
		t.Errorf("invalid should fall back, got %v", got)
	}
}

func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		val  string
		def  bool
		want bool
	}{
		{"1", false, true},
		{"true", false, true},
		{"YES", false, true},
		{"on", false, true},
		{"0", true, false},
		{"false", true, false},
		{"no", true, false},
		{"off", true, false},
		{"garbage", true, true},
		{"garbage", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			t.Setenv("BI_TEST_BOOL", tt.val)
			if got := getEnvAsBool("BI_TEST_BOOL", tt.def); got != tt.want {
				t.Errorf("getEnvAsBool(%q, %v) = %v, want %v", tt.val, tt.def, got, tt.want)
			}
		})
	}
	if got := getEnvAsBool("BI_TEST_BOOL_UNSET", true); !got {
		t.Error("unset should return default true")
	}
}

func TestGetEnvAsDuration(t *testing.T) {
	if got := getEnvAsDuration("BI_TEST_DUR_UNSET", 2*time.Second); got != 2*time.Second {
		t.Errorf("unset = %v, want 2s", got)
	}
	t.Setenv("BI_TEST_DUR", "1m30s")
	if got := getEnvAsDuration("BI_TEST_DUR", 2*time.Second); got != 90*time.Second {
		t.Errorf("valid = %v, want 90s", got)
	}
	t.Setenv("BI_TEST_DUR_BAD", "nope")
	if got := getEnvAsDuration("BI_TEST_DUR_BAD", 2*time.Second); got != 2*time.Second {
		t.Errorf("invalid should fall back, got %v", got)
	}
}

func TestValidateFloatRange(t *testing.T) {
	if err := validateFloatRange("KEY", 0.5, 0, 1); err != nil {
		t.Errorf("in-range should pass, got %v", err)
	}
	if err := validateFloatRange("KEY", 5, 1, 10); err != nil {
		t.Errorf("in-range with non-zero min should pass, got %v", err)
	}
	if err := validateFloatRange("KEY", 0.5, 1, 10); err == nil {
		t.Error("below non-zero min should error")
	}
	if err := validateFloatRange("KEY", -0.1, 0, 1); err == nil {
		t.Error("below min should error")
	}
	if err := validateFloatRange("KEY", 1.1, 0, 1); err == nil {
		t.Error("above max should error")
	}
}

func TestConfigAccessors(t *testing.T) {
	c := &Config{}
	c.HTTP.Host = "127.0.0.1"
	c.HTTP.Port = 9000
	c.Query.TimeoutSeconds = 30
	c.Query.MaxRuntimeSeconds = 60

	if got := c.HTTPAddr(); got != "127.0.0.1:9000" {
		t.Errorf("HTTPAddr() = %q", got)
	}
	if got := c.QueryTimeout(); got != 30*time.Second {
		t.Errorf("QueryTimeout() = %v", got)
	}
	if got := c.MaxQueryRuntime(); got != 60*time.Second {
		t.Errorf("MaxQueryRuntime() = %v", got)
	}
}

func TestLoadDefaultEncryptionKeyRejected(t *testing.T) {
	// Unset key resolves to the insecure default, which Load must reject.
	t.Setenv("BI_ENCRYPTION_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load should reject the default encryption key")
	}
}

func TestLoadFloatRangeValidation(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_PII_DETECTION_THRESHOLD", "2.0") // out of [0,1]
	if _, err := Load(); err == nil {
		t.Fatal("Load should reject out-of-range BI_PII_DETECTION_THRESHOLD")
	}
}

func TestLoadSuccess(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
	t.Setenv("BI_HTTP_PORT", "1234")
	t.Setenv("BI_ENV", "production")
	t.Setenv("BI_AUTH_ENABLED", "true")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Port != 1234 {
		t.Errorf("HTTP.Port = %d, want 1234", cfg.HTTP.Port)
	}
	if !cfg.HTTP.HSTSEnabled {
		t.Error("HSTSEnabled should default to true when BI_ENV=production")
	}
	if cfg.Metadata.DSN != "postgres://example/db?sslmode=disable" {
		t.Errorf("Metadata.DSN = %q", cfg.Metadata.DSN)
	}
}

func TestLoadProductionRequiresAuthEnabled(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
	t.Setenv("BI_ENV", "production")
	t.Setenv("BI_AUTH_ENABLED", "false")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load should reject BI_AUTH_ENABLED=false in production")
	}
}

func TestLoadKubernetesRequiresAuthEnabled(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
	t.Setenv("BI_ENV", "development")
	t.Setenv("BI_AUTH_ENABLED", "false")
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")

	if _, err := Load(); err == nil {
		t.Fatal("Load should reject BI_AUTH_ENABLED=false when running in Kubernetes")
	}
}

func TestLoadDevelopmentAllowsAuthDisabled(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
	t.Setenv("BI_ENV", "development")
	t.Setenv("BI_AUTH_ENABLED", "false")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Auth.Enabled {
		t.Fatal("expected auth disabled in development when BI_AUTH_ENABLED=false")
	}
}
