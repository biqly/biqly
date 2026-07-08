package middleware

import (
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/audit"
)

// ChannelHeader is the request header a trusted frontend/agent sets to
// declare its calling channel for audit attribution.
const ChannelHeader = "X-Biqly-Channel"

// ChannelTag records the calling channel (ui/api/mcp) in the request context
// so audit events can attribute the surface that issued the request.
// Unrecognized or missing header values default to "api".
func ChannelTag() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ch := audit.ChannelAPI
			switch strings.ToLower(r.Header.Get(ChannelHeader)) {
			case audit.ChannelUI:
				ch = audit.ChannelUI
			case audit.ChannelMCP:
				ch = audit.ChannelMCP
			case audit.ChannelAgent:
				ch = audit.ChannelAgent
			}
			next.ServeHTTP(w, r.WithContext(audit.WithChannel(r.Context(), ch)))
		})
	}
}

// ChannelStatic tags every request with a fixed channel, e.g. "internal" for
// service-to-service routes.
func ChannelStatic(channel string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(audit.WithChannel(r.Context(), channel)))
		})
	}
}
