package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaginateStoresDefaults(t *testing.T) {
	got := capturePagination(t, "/api/history", PaginationConfig{
		DefaultPage:     1,
		DefaultPageSize: 10,
		MaxPageSize:     100,
	})

	want := PageParams{Page: 1, PageSize: 10, Limit: 10, Offset: 0, Requested: false}
	if got != want {
		t.Errorf("Paginate(defaults) = %+v, want %+v", got, want)
	}
}

func TestPaginateUsesPageSizeAliasBeforeLimit(t *testing.T) {
	got := capturePagination(t, "/api/history?page=3&page_size=20&limit=30", PaginationConfig{
		DefaultPage:     1,
		DefaultPageSize: 10,
		MaxPageSize:     100,
	})

	want := PageParams{Page: 3, PageSize: 20, Limit: 20, Offset: 40, Requested: true}
	if got != want {
		t.Errorf("Paginate(page_size alias) = %+v, want %+v", got, want)
	}
}

func TestPaginateFallsBackToLimitAlias(t *testing.T) {
	got := capturePagination(t, "/api/history?page=2&limit=25", PaginationConfig{
		DefaultPage:     1,
		DefaultPageSize: 10,
		MaxPageSize:     100,
	})

	want := PageParams{Page: 2, PageSize: 25, Limit: 25, Offset: 25, Requested: true}
	if got != want {
		t.Errorf("Paginate(limit alias) = %+v, want %+v", got, want)
	}
}

func TestPaginateIgnoresInvalidValuesAndClampsPageSize(t *testing.T) {
	got := capturePagination(t, "/api/history?page=-4&page_size=500", PaginationConfig{
		DefaultPage:     1,
		DefaultPageSize: 10,
		MaxPageSize:     100,
	})

	want := PageParams{Page: 1, PageSize: 100, Limit: 100, Offset: 0, Requested: true}
	if got != want {
		t.Errorf("Paginate(invalid and clamped) = %+v, want %+v", got, want)
	}
}

func TestPaginationFromContextReturnsSafeDefault(t *testing.T) {
	got := PaginationFromContext(context.Background())

	want := PageParams{Page: 1, PageSize: 0, Limit: 0, Offset: 0, Requested: false}
	if got != want {
		t.Errorf("PaginationFromContext(empty context) = %+v, want %+v", got, want)
	}
}

func capturePagination(t *testing.T, target string, config PaginationConfig) PageParams {
	t.Helper()

	var got PageParams
	handler := Paginate(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = PaginationFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Paginate(%q) status = %d, want %d", target, w.Code, http.StatusOK)
	}
	return got
}

func TestParsePositiveIntQueryParam(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		key     string
		wantVal int
		wantOk  bool
	}{
		{"valid positive integer", "/?limit=10", "limit", 10, true},
		{"zero value is not positive", "/?limit=0", "limit", 0, false},
		{"negative value is not positive", "/?limit=-5", "limit", 0, false},
		{"invalid integer string", "/?limit=abc", "limit", 0, false},
		{"missing query parameter", "/?", "limit", 0, false},
		{"empty query parameter", "/?limit=", "limit", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), "GET", tt.url, http.NoBody)
			val, ok := ParsePositiveIntQueryParam(req, tt.key)
			if val != tt.wantVal || ok != tt.wantOk {
				t.Errorf("ParsePositiveIntQueryParam() = (%d, %t), want (%d, %t)", val, ok, tt.wantVal, tt.wantOk)
			}
		})
	}
}
