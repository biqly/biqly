package internalapi

// Error is the canonical error envelope returned by every /internal/*
// endpoint when the HTTP status is >= 400. Code is a stable machine-readable
// slug; Message is a human-readable description that callers may log.
type Error struct {
	Code    string `json:"code,omitempty"`
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Common error code slugs. Servers SHOULD populate Code so clients can branch
// on stable identifiers instead of HTTP status alone.
const (
	CodeNotFound        = "not_found"
	CodeInvalidRequest  = "invalid_request"
	CodeUnauthorized    = "unauthorized"
	CodeInternal        = "internal_error"
	CodeUpstream        = "upstream_unavailable"
	CodeCompileError    = "compile_error"
	CodeExecutionError  = "execution_error"
	CodePermissionError = "permission_error"
	CodeReadOnlyError   = "read_only_violation"
)

// HealthResponse is the body of /internal/health on every service.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service,omitempty"`
	Version string `json:"version,omitempty"`
}
