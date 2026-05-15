// Package middleware contains chi-compatible HTTP middlewares specific to biqly.
package middleware

import (
	"net/http"

	"github.com/biqly/biqly/internal/i18n"
)

// LocaleQueryParam is the optional URL query override (e.g. ?lang=tr).
const LocaleQueryParam = "lang"

// Locale resolves the request locale from (in order) the `lang` query param,
// the `X-Locale` header, or `Accept-Language`, and stores it on the request
// context. Downstream handlers can call i18n.FromContext(ctx) to read it.
func Locale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loc := resolveLocale(r)
		ctx := i18n.WithLocale(r.Context(), loc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func resolveLocale(r *http.Request) i18n.Locale {
	if v := r.URL.Query().Get(LocaleQueryParam); v != "" {
		return i18n.ParseLocale(v)
	}
	if v := r.Header.Get("X-Locale"); v != "" {
		return i18n.ParseLocale(v)
	}
	return i18n.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
}
