package auth

import (
	"net/http"

	"github.com/biqly/biqly/internal/env"
)

const localHTTPDevPort = 8889

// CookieSecure reports whether Set-Cookie should include the Secure attribute.
// Production is always fail-closed (Secure=true). Non-production: TLS-terminated
// requests are secure; only the local auth dev server on :8889 over plain HTTP
// omits Secure so browsers accept cookies during local development.
func CookieSecure(r *http.Request, listenPort int) bool {
	if env.IsProduction() {
		return true
	}
	if r.URL.Scheme == "https" || r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return listenPort != localHTTPDevPort
}

// WriteResponseCookie sets cookie on w with Secure when the request is served over
// HTTPS (or production). Only the local auth dev port over plain HTTP omits Secure.
func WriteResponseCookie(w http.ResponseWriter, r *http.Request, listenPort int, cookie *http.Cookie) {
	writeResponseCookie(w, r, listenPort, cookie, true)
}

// WriteReadableResponseCookie sets a cookie that client-side code may read.
// Use only for double-submit style tokens, never for bearer/session secrets.
func WriteReadableResponseCookie(w http.ResponseWriter, r *http.Request, listenPort int, cookie *http.Cookie) {
	writeResponseCookie(w, r, listenPort, cookie, false)
}

func writeResponseCookie(w http.ResponseWriter, r *http.Request, listenPort int, cookie *http.Cookie, httpOnly bool) {
	if CookieSecure(r, listenPort) {
		setSecureResponseCookie(w, cookie, httpOnly)
		return
	}
	writePlainHTTPDevCookie(w, cookie, httpOnly)
}

// setSecureResponseCookie copies src into a new cookie with Secure and SameSite.
//
//nolint:gosec // G124: Secure, HttpOnly, and SameSite (default Lax) are enforced below.
func setSecureResponseCookie(w http.ResponseWriter, src *http.Cookie, httpOnly bool) {
	c := *src
	sameSite := c.SameSite
	if sameSite == 0 {
		sameSite = http.SameSiteLaxMode
	}
	http.SetCookie(w, &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-httponly.cookie-missing-httponly
		Name:       c.Name,
		Value:      c.Value,
		Path:       c.Path,
		Domain:     c.Domain,
		Expires:    c.Expires,
		RawExpires: c.RawExpires,
		MaxAge:     c.MaxAge,
		Secure:     true,
		HttpOnly:   httpOnly,
		SameSite:   sameSite,
		Raw:        c.Raw,
		Unparsed:   c.Unparsed,
	})
}

// writePlainHTTPDevCookie sets a cookie without Secure for plain-HTTP local auth dev only.
//
//nolint:gosec // G124: intentional plain-HTTP local dev exception on port 8889 only.
func writePlainHTTPDevCookie(w http.ResponseWriter, src *http.Cookie, httpOnly bool) {
	c := *src
	// Plain HTTP on local auth dev port 8889 requires Secure=false so browsers accept the cookie.
	// codeql[go/cookie-secure-not-set]
	// lgtm[go/cookie-secure-not-set]
	http.SetCookie(w, &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure, go.lang.security.audit.net.cookie-missing-httponly.cookie-missing-httponly
		Name:       c.Name,
		Value:      c.Value,
		Path:       c.Path,
		Domain:     c.Domain,
		Expires:    c.Expires,
		RawExpires: c.RawExpires,
		MaxAge:     c.MaxAge,
		// codeql[go/cookie-secure-not-set]
		// lgtm[go/cookie-secure-not-set]
		Secure:   false,
		HttpOnly: httpOnly,
		SameSite: c.SameSite,
		Raw:      c.Raw,
		Unparsed: c.Unparsed,
	})
}
