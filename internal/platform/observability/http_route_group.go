package observability

import "strings"

// HTTPRouteGroup maps a chi route pattern to a bounded route_group label (max ~20 values).
func HTTPRouteGroup(routePattern string) string {
	p := strings.TrimSpace(routePattern)
	if p == "" {
		return "other"
	}
	switch {
	case p == "/health" || p == "/ready":
		return "/health"
	case p == "/metrics":
		return "/metrics"
	case strings.HasPrefix(p, "/internal"):
		return "/internal"
	case strings.HasPrefix(p, "/api/ai/query/preview") || strings.HasPrefix(p, "/api/ai/preview"):
		return "/api/ai/preview"
	case strings.HasPrefix(p, "/api/ai/query"):
		return "/api/ai/query"
	case strings.HasPrefix(p, "/api/admin"):
		return "/api/admin"
	case strings.HasPrefix(p, "/api/auth"):
		return "/api/auth"
	case strings.HasPrefix(p, "/api/query"):
		return "/api/query"
	case strings.HasPrefix(p, "/api/datasources"),
		strings.HasPrefix(p, "/api/metadata"),
		strings.HasPrefix(p, "/api/semantic"),
		strings.HasPrefix(p, "/api/catalog"):
		return "/api/catalog"
	case strings.HasPrefix(p, "/api/ai"):
		return "/api/ai/other"
	case strings.HasPrefix(p, "/api"):
		return "/api/other"
	default:
		return BoundLabel(p, httpRouteGroups, "other")
	}
}
