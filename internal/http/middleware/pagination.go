package middleware

import (
	"context"
	"net/http"
	"strconv"
)

type paginationContextKey struct{}

// PaginationConfig controls query parsing for list endpoints.
type PaginationConfig struct {
	DefaultPage     int
	DefaultPageSize int
	MaxPageSize     int
}

// PageParams is the normalized pagination window for the current request.
type PageParams struct {
	Page      int
	PageSize  int
	Limit     int
	Offset    int
	Requested bool
}

// Paginate parses page/page_size/limit query parameters and stores the result
// on the request context for downstream handlers.
func Paginate(config PaginationConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			params := paginationFromQuery(r, config)
			ctx := context.WithValue(r.Context(), paginationContextKey{}, params)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// PaginationFromContext returns normalized pagination values from ctx.
func PaginationFromContext(ctx context.Context) PageParams {
	params, ok := ctx.Value(paginationContextKey{}).(PageParams)
	if !ok {
		return PageParams{Page: 1}
	}
	return params
}

func paginationFromQuery(r *http.Request, config PaginationConfig) PageParams {
	defaultPage := config.DefaultPage
	if defaultPage <= 0 {
		defaultPage = 1
	}

	page := defaultPage
	pageValue := r.URL.Query().Get("page")
	pageSizeValue := r.URL.Query().Get("page_size")
	limitValue := r.URL.Query().Get("limit")
	requested := pageValue != "" || pageSizeValue != "" || limitValue != ""

	if n, ok := parsePositiveQueryInt(pageValue); ok {
		page = n
	}

	pageSize := config.DefaultPageSize
	if n, ok := parsePositiveQueryInt(pageSizeValue); ok {
		pageSize = n
	} else if n, ok := parsePositiveQueryInt(limitValue); ok {
		pageSize = n
	}
	if config.MaxPageSize > 0 {
		pageSize = min(pageSize, config.MaxPageSize)
	}

	offset := (page - 1) * pageSize
	return PageParams{
		Page:      page,
		PageSize:  pageSize,
		Limit:     pageSize,
		Offset:    offset,
		Requested: requested,
	}
}

func parsePositiveQueryInt(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
