package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net/http"
)

const csrfHeaderName = "X-CSRF-Token"

func CSRF(listenPort int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Method == http.MethodTrace {
				cookie, err := r.Cookie("csrf_token")
				if err != nil || cookie.Value == "" {
					token := setCSRFCookie(w, r, listenPort)
					w.Header().Set(csrfHeaderName, token)
				} else {
					w.Header().Set(csrfHeaderName, cookie.Value)
				}
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				http.Error(w, "Required CSRF cookie is missing", http.StatusForbidden)
				return
			}

			headerToken := r.Header.Get(csrfHeaderName)
			if headerToken == "" {
				http.Error(w, "Required CSRF token is missing", http.StatusForbidden)
				return
			}

			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(headerToken)) != 1 {
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setCSRFCookie(w http.ResponseWriter, r *http.Request, listenPort int) string {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Printf("csrf: failed to generate random token: %v", err)
		return ""
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
	//nolint:gosec // G124: Secure is set dynamically via CookieSecure.
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   CookieSecure(r, listenPort),
		SameSite: http.SameSiteStrictMode,
	})
	return token
}
