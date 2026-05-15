package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminKeyFromRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header http.Header
		want   string
	}{
		{
			name: "x-admin-key",
			header: http.Header{"X-Admin-Key": []string{"secret"}},
			want: "secret",
		},
		{
			name: "bearer",
			header: http.Header{"Authorization": []string{"Bearer secret"}},
			want: "secret",
		},
		{
			name:   "missing",
			header: http.Header{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodPost, "/api/ai/eval/run", nil)
			r.Header = tt.header
			if got := adminKeyFromRequest(r); got != tt.want {
				t.Fatalf("adminKeyFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}
