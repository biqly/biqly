package handlers

import "net/http"

// handleCSRF is a lightweight, public endpoint whose only purpose is to let the
// SPA obtain a CSRF token before issuing its first unsafe (POST/PUT/DELETE)
// request. The CSRF middleware sets the X-CSRF-Token response header and the
// csrf_token cookie on every safe request before this handler runs, so the body
// is intentionally empty and 204 No Content is returned. It does not require
// authentication, which is why the SPA uses it instead of the auth-protected
// /me endpoint (which returns 401 when no session exists yet).
func (*AuthHandler) handleCSRF(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
