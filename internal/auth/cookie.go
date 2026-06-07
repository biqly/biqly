package auth

import "net/http"

const localHTTPDevPort = 8889

// CookieSecure reports whether Set-Cookie should include the Secure attribute.
// TLS-terminated requests are always secure; the auth dev server on :8889 over
// plain HTTP omits Secure so browsers accept cookies during local development.
func CookieSecure(r *http.Request, listenPort int) bool {
	if r.URL.Scheme == "https" || r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return listenPort != localHTTPDevPort
}
