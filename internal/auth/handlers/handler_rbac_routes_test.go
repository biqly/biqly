package handlers

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestRegisterAuthRoutes_NoConflict ensures the permission-gated admin and
// workspace route groups register without a chi path/method conflict (the
// /users/{id}/roles GET and POST live in separate permission groups).
func TestRegisterAuthRoutes_NoConflict(t *testing.T) {
	h := &RBACHandler{}
	r := chi.NewRouter()
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("route registration panicked: %v", rec)
		}
	}()
	h.RegisterAuthRoutes(r, func(next http.Handler) http.Handler { return next })
}
