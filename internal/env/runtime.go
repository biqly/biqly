package env

import (
	"os"
	"strings"
)

func isProdName(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}

// IsProduction reports whether the process runs in a production-like environment.
// Used for fail-closed security defaults (JWT keys required, etc.).
//
// Signals: BI_ENV or APP_ENV set to production/prod, or running inside Kubernetes
// (KUBERNETES_SERVICE_HOST set).
func IsProduction() bool {
	if isProdName(os.Getenv("BI_ENV")) || isProdName(os.Getenv("APP_ENV")) {
		return true
	}
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

// HSTSEnabledDefault is the default for Strict-Transport-Security when
// BI_HSTS_ENABLED is unset. True when BI_ENV is production; also true when any
// httpsOrigins entry uses https:// (auth WebAuthn / local TLS setups).
func HSTSEnabledDefault(httpsOrigins ...string) bool {
	if isProdName(os.Getenv("BI_ENV")) {
		return true
	}
	for _, o := range httpsOrigins {
		if strings.HasPrefix(strings.TrimSpace(o), "https://") {
			return true
		}
	}
	return false
}
