package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

func CSRF(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.Method == http.MethodTrace {
				cookie, err := r.Cookie("csrf_token")
				if err != nil || cookie.Value == "" {
					setCSRFCookie(w, secure)
				}
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				http.Error(w, "Required CSRF cookie is missing", http.StatusForbidden)
				return
			}

			headerToken := r.Header.Get("X-CSRF-Token")
			if headerToken == "" {
				http.Error(w, "Required CSRF token is missing", http.StatusForbidden)
				return
			}

			if cookie.Value != headerToken {
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setCSRFCookie(w http.ResponseWriter, secure bool) {
	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	//nolint:gosec
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
