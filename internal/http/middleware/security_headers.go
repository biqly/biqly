package middleware

import (
	"net/http"
	"strconv"
)

// SecurityHeadersConfig controls which headers SecurityHeaders sets.
// All fields default to safe values when zero.
type SecurityHeadersConfig struct {
	// HSTSEnabled enables Strict-Transport-Security. Only enable when the
	// service is served exclusively over HTTPS.
	HSTSEnabled bool
	// HSTSMaxAgeSeconds defaults to 63072000 (2 years) when zero.
	HSTSMaxAgeSeconds int
	// HSTSIncludeSubdomains adds the includeSubDomains directive.
	HSTSIncludeSubdomains bool
	// HSTSPreload adds the preload directive (requires submission to the
	// preload list).
	HSTSPreload bool
	// ContentSecurityPolicy is the full CSP header value. Empty disables the
	// header.
	ContentSecurityPolicy string
	// FrameOptions defaults to "DENY" when empty. Set to "SAMEORIGIN" for
	// embedding the page in same-origin iframes.
	FrameOptions string
	// ReferrerPolicy defaults to "strict-origin-when-cross-origin" when empty.
	ReferrerPolicy string
	// PermissionsPolicy is the full Permissions-Policy header value. Empty
	// disables the header.
	PermissionsPolicy string
}

// SecurityHeaders attaches a baseline set of security headers to every
// response. Apply globally before route-specific middleware.
func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	maxAge := cfg.HSTSMaxAgeSeconds
	if maxAge <= 0 {
		maxAge = 63072000
	}
	frame := cfg.FrameOptions
	if frame == "" {
		frame = "DENY"
	}
	referrer := cfg.ReferrerPolicy
	if referrer == "" {
		referrer = "strict-origin-when-cross-origin"
	}

	hstsValue := ""
	if cfg.HSTSEnabled {
		hstsValue = "max-age=" + strconv.Itoa(maxAge)
		if cfg.HSTSIncludeSubdomains {
			hstsValue += "; includeSubDomains"
		}
		if cfg.HSTSPreload {
			hstsValue += "; preload"
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", frame)
			h.Set("Referrer-Policy", referrer)
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-site")
			if cfg.PermissionsPolicy != "" {
				h.Set("Permissions-Policy", cfg.PermissionsPolicy)
			}
			if cfg.ContentSecurityPolicy != "" {
				h.Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
			}
			if hstsValue != "" {
				h.Set("Strict-Transport-Security", hstsValue)
			}
			next.ServeHTTP(w, r)
		})
	}
}

