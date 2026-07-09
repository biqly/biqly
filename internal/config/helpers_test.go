package config

import (
	"slices"
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

func TestHTTPWriteTimeoutCoversWebAgentAndAI(t *testing.T) {
	c := &Config{}
	c.Query.MaxRuntimeSeconds = 60
	c.WebAgent.Timeout = 120 * time.Second
	c.AI.Connection.HTTPTimeoutSeconds = 12

	// max(60+15, 120+15, 12+30) = 135s — web agent budget wins over query.
	if got := c.HTTPWriteTimeout(); got != 135*time.Second {
		t.Fatalf("HTTPWriteTimeout() = %v, want 135s", got)
	}

	c.AI.Connection.HTTPTimeoutSeconds = 300
	// max(75, 135, 300+30) = 330s — AI provider HTTP budget wins.
	if got := c.HTTPWriteTimeout(); got != 330*time.Second {
		t.Fatalf("HTTPWriteTimeout() with long AI HTTP = %v, want 330s", got)
	}

	c.WebAgent.Timeout = 0
	c.AI.Connection.HTTPTimeoutSeconds = 0
	c.Query.MaxRuntimeSeconds = 60
	// RequestTimeout falls back to 300+30; still covers query-only path.
	if got := c.HTTPWriteTimeout(); got != 330*time.Second {
		t.Fatalf("HTTPWriteTimeout() AI default floor = %v, want 330s", got)
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

// TestProductionAuthEnabledFailClosed guards the invariant that auth cannot be
// disabled in production-like runtimes (BI_ENV=production/prod or Kubernetes).
// Re-run manually at least monthly alongside cookie Secure checks in staging/prod.
func TestProductionAuthEnabledFailClosed(t *testing.T) {
	base := func(t *testing.T) {
		t.Helper()
		t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
		t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
		t.Setenv("BI_AUTH_ENABLED", "false")
	}

	t.Run("production env", func(t *testing.T) {
		base(t)
		t.Setenv("BI_ENV", "production")
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_AUTH_ENABLED=false in production")
		}
	})

	t.Run("kubernetes", func(t *testing.T) {
		base(t)
		t.Setenv("BI_ENV", "development")
		t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_AUTH_ENABLED=false when running in Kubernetes")
		}
	})

	t.Run("production enabled auth succeeds", func(t *testing.T) {
		t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
		t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
		t.Setenv("BI_ENV", "production")
		t.Setenv("BI_AUTH_ENABLED", "true")
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		if _, err := Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
	})
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

func TestLoadAgentConfigDefaults(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
	t.Setenv("BI_ENV", "development")
	t.Setenv("BI_AUTH_ENABLED", "false")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Agent.Enabled {
		t.Error("Agent.Enabled should default to false")
	}
	if cfg.Agent.Mode != AgentModeShadow {
		t.Errorf("Agent.Mode = %q, want %q", cfg.Agent.Mode, AgentModeShadow)
	}
	if cfg.Agent.MaxSteps != 6 {
		t.Errorf("Agent.MaxSteps = %d, want 6", cfg.Agent.MaxSteps)
	}
	if cfg.Agent.MaxClarificationRounds != 2 {
		t.Errorf("Agent.MaxClarificationRounds = %d, want 2", cfg.Agent.MaxClarificationRounds)
	}
	if cfg.Agent.Timeout != 45*time.Second {
		t.Errorf("Agent.Timeout = %v, want 45s", cfg.Agent.Timeout)
	}
	if cfg.Agent.MaxRows != 1000 {
		t.Errorf("Agent.MaxRows = %d, want 1000", cfg.Agent.MaxRows)
	}
	if !cfg.Agent.LegacyFallbackEnabled {
		t.Error("Agent.LegacyFallbackEnabled should default to true")
	}
}

func TestLoadAgentConfigValidation(t *testing.T) {
	base := func(t *testing.T) {
		t.Helper()
		t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
		t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
		t.Setenv("BI_ENV", "development")
		t.Setenv("BI_AUTH_ENABLED", "false")
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
	}

	t.Run("rejects unknown mode", func(t *testing.T) {
		base(t)
		t.Setenv("BI_AGENT_MODE", "eager")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject an unknown BI_AGENT_MODE")
		}
	})
	t.Run("rejects out-of-range max_steps", func(t *testing.T) {
		base(t)
		t.Setenv("BI_AGENT_MAX_STEPS", "7")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_AGENT_MAX_STEPS above 6")
		}
	})
	t.Run("rejects out-of-range clarification rounds", func(t *testing.T) {
		base(t)
		t.Setenv("BI_AGENT_MAX_CLARIFICATION_ROUNDS", "3")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_AGENT_MAX_CLARIFICATION_ROUNDS above 2")
		}
	})
	t.Run("rejects out-of-range timeout", func(t *testing.T) {
		base(t)
		t.Setenv("BI_AGENT_TIMEOUT", "46s")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_AGENT_TIMEOUT above 45s")
		}
	})
	t.Run("rejects out-of-range max_rows", func(t *testing.T) {
		base(t)
		t.Setenv("BI_AGENT_MAX_ROWS", "1001")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_AGENT_MAX_ROWS above 1000")
		}
	})
	t.Run("accepts active mode and in-range values", func(t *testing.T) {
		base(t)
		t.Setenv("BI_AGENT_MODE", "active")
		t.Setenv("BI_AGENT_MAX_STEPS", "1")
		t.Setenv("BI_AGENT_MAX_CLARIFICATION_ROUNDS", "0")
		t.Setenv("BI_AGENT_TIMEOUT", "1s")
		t.Setenv("BI_AGENT_MAX_ROWS", "1")
		if _, err := Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func TestLoadWebAgentConfigDefaultsAndAllowlistReuse(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
	t.Setenv("BI_ENV", "development")
	t.Setenv("BI_AUTH_ENABLED", "false")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("BI_AGENT_WORKSPACE_ALLOWLIST", "ws-1, ws-2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.WebAgent.Enabled {
		t.Error("WebAgent.Enabled should default to false")
	}
	if cfg.WebAgent.MaxSteps != 6 {
		t.Errorf("WebAgent.MaxSteps = %d, want 6", cfg.WebAgent.MaxSteps)
	}
	if cfg.WebAgent.MaxClarificationRounds != 2 {
		t.Errorf("WebAgent.MaxClarificationRounds = %d, want 2", cfg.WebAgent.MaxClarificationRounds)
	}
	if cfg.WebAgent.Timeout != 120*time.Second {
		t.Errorf("WebAgent.Timeout = %v, want 120s", cfg.WebAgent.Timeout)
	}
	if got, want := cfg.WebAgent.WorkspaceAllowlist, []string{"ws-1", "ws-2"}; !slices.Equal(got, want) {
		t.Errorf("WebAgent.WorkspaceAllowlist = %v, want %v", got, want)
	}

	t.Setenv("BI_WEB_AGENT_WORKSPACE_ALLOWLIST", "web-only")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with web allowlist override error = %v", err)
	}
	if got, want := cfg.WebAgent.WorkspaceAllowlist, []string{"web-only"}; !slices.Equal(got, want) {
		t.Errorf("WebAgent.WorkspaceAllowlist override = %v, want %v", got, want)
	}
}

func TestLoadWebAgentConfigValidation(t *testing.T) {
	base := func(t *testing.T) {
		t.Helper()
		t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
		t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
		t.Setenv("BI_ENV", "development")
		t.Setenv("BI_AUTH_ENABLED", "false")
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
	}

	t.Run("rejects out-of-range max_steps", func(t *testing.T) {
		base(t)
		t.Setenv("BI_WEB_AGENT_MAX_STEPS", "7")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_WEB_AGENT_MAX_STEPS above 6")
		}
	})
	t.Run("rejects out-of-range clarification rounds", func(t *testing.T) {
		base(t)
		t.Setenv("BI_WEB_AGENT_MAX_CLARIFICATION_ROUNDS", "3")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_WEB_AGENT_MAX_CLARIFICATION_ROUNDS above 2")
		}
	})
	t.Run("rejects out-of-range timeout", func(t *testing.T) {
		base(t)
		t.Setenv("BI_WEB_AGENT_TIMEOUT", "121s")
		if _, err := Load(); err == nil {
			t.Fatal("Load should reject BI_WEB_AGENT_TIMEOUT above 120s")
		}
	})
	t.Run("accepts in-range values", func(t *testing.T) {
		base(t)
		t.Setenv("BI_WEB_AGENT_ENABLED", "true")
		t.Setenv("BI_WEB_AGENT_MAX_STEPS", "1")
		t.Setenv("BI_WEB_AGENT_MAX_CLARIFICATION_ROUNDS", "0")
		t.Setenv("BI_WEB_AGENT_TIMEOUT", "1s")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if !cfg.WebAgent.Enabled {
			t.Fatal("expected WebAgent.Enabled from env")
		}
	})
}

func TestLoadAIJobsConsumerEnabled(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", "a-real-32-byte-key-value-1234567")
	t.Setenv("BI_METADATA_DB_DSN", "postgres://example/db?sslmode=disable")
	t.Setenv("BI_ENV", "development")
	t.Setenv("BI_AUTH_ENABLED", "false")
	t.Setenv("BI_AI_JOBS_CONSUMER_ENABLED", "false")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Jobs.Enabled {
		t.Fatal("BI_AI_JOBS_CONSUMER_ENABLED must not disable job APIs")
	}
	if cfg.Jobs.ConsumerEnabled {
		t.Fatal("expected AI job consumer disabled")
	}
}
