package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
)

func TestLocaleFromQueryParam(t *testing.T) {
	var got i18n.Locale
	handler := Locale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?lang=tr", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got != i18n.LocaleTR {
		t.Fatalf("expected LocaleTR, got %q", got)
	}
}

func TestLocaleFromHeader(t *testing.T) {
	var got i18n.Locale
	handler := Locale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-Locale", "tr")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got != i18n.LocaleTR {
		t.Fatalf("expected LocaleTR, got %q", got)
	}
}

func TestLocale_QueryParamTakesPriorityOverHeader(t *testing.T) {
	var got i18n.Locale
	handler := Locale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?lang=en", nil)
	req.Header.Set("X-Locale", "tr")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got != i18n.LocaleEN {
		t.Fatalf("query param should override header: expected LocaleEN, got %q", got)
	}
}

func TestLocaleFromAcceptLanguage(t *testing.T) {
	var got i18n.Locale
	handler := Locale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "tr-TR,en;q=0.9")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got != i18n.LocaleTR {
		t.Fatalf("expected LocaleTR from Accept-Language, got %q", got)
	}
}

func TestLocaleFallsBackToDefault(t *testing.T) {
	var got i18n.Locale
	handler := Locale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// No locale info -> falls back to default (en)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got != i18n.DefaultLocale {
		t.Fatalf("expected default locale %q, got %q", i18n.DefaultLocale, got)
	}
}

func TestLocale_AcceptLanguageWithUnsupportedLocale(t *testing.T) {
	var got i18n.Locale
	handler := Locale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "de-DE,fr;q=0.8")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Should fall back to default when Accept-Language only has unsupported locales
	if got != i18n.DefaultLocale {
		t.Fatalf("expected default locale for unsupported Accept-Language, got %q", got)
	}
}

func TestResolveLocale_QueryParam(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?lang=tr", nil)
	loc := resolveLocale(req)
	if loc != i18n.LocaleTR {
		t.Fatalf("expected LocaleTR, got %q", loc)
	}
}

func TestResolveLocale_XLocaleHeader(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-Locale", "en")
	loc := resolveLocale(req)
	if loc != i18n.LocaleEN {
		t.Fatalf("expected LocaleEN, got %q", loc)
	}
}

func TestResolveLocale_AcceptLanguage(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "tr-TR,en;q=0.9")
	loc := resolveLocale(req)
	if loc != i18n.LocaleTR {
		t.Fatalf("expected LocaleTR, got %q", loc)
	}
}

func TestResolveLocale_Empty(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	loc := resolveLocale(req)
	if loc != i18n.DefaultLocale {
		t.Fatalf("expected default locale %q, got %q", i18n.DefaultLocale, loc)
	}
}

func TestResolveLocale_QueryParamOverridesHeader(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?lang=en", nil)
	req.Header.Set("X-Locale", "tr")
	loc := resolveLocale(req)
	if loc != i18n.LocaleEN {
		t.Fatalf("query param should override X-Locale: expected LocaleEN, got %q", loc)
	}
}
