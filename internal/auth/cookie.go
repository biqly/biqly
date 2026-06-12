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
	if CookieSecure(r, listenPort) {
		setSecureResponseCookie(w, cookie)
		return
	}
	writePlainHTTPDevCookie(w, cookie)
}

// setSecureResponseCookie copies src into a new cookie with Secure and HttpOnly so
// static analysis (CodeQL, Semgrep) sees both flags at the SetCookie site.
func setSecureResponseCookie(w http.ResponseWriter, src *http.Cookie) {
	c := *src
	http.SetCookie(w, &http.Cookie{
		Name:       c.Name,
		Value:      c.Value,
		Path:       c.Path,
		Domain:     c.Domain,
		Expires:    c.Expires,
		RawExpires: c.RawExpires,
		MaxAge:     c.MaxAge,
		Secure:     true,
		HttpOnly:   true,
		SameSite:   c.SameSite,
		Raw:        c.Raw,
		Unparsed:   c.Unparsed,
	})
}

// writePlainHTTPDevCookie sets a cookie without Secure for plain-HTTP local auth dev only.
//
//nolint:gosec // G124: intentional plain-HTTP local dev exception on port 8889 only.
func writePlainHTTPDevCookie(w http.ResponseWriter, src *http.Cookie) {
	c := *src
	// codeql[go/cookie-secure-not-set]: Plain HTTP on local auth dev port 8889 requires Secure=false so browsers accept the cookie.
	http.SetCookie(w, &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:       c.Name,
		Value:      c.Value,
		Path:       c.Path,
		Domain:     c.Domain,
		Expires:    c.Expires,
		RawExpires: c.RawExpires,
		MaxAge:     c.MaxAge,
		Secure:     false,
		HttpOnly:   true,
		SameSite:   c.SameSite,
		Raw:        c.Raw,
		Unparsed:   c.Unparsed,
	})
}
