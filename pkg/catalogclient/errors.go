package catalogclient

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/biqly/biqly/pkg/internalapi"
)

// Sentinel errors. Callers should use errors.Is to branch on them; the
// underlying APIError preserves the original HTTP status and code so deeper
// inspection (logging, metrics) is still possible.
var (
	// ErrNotFound is returned when the server responds 404.
	ErrNotFound = errors.New("catalogclient: not found")
	// ErrInvalidRequest is returned when the server responds 400 (typically
	// because the caller supplied empty or malformed parameters).
	ErrInvalidRequest = errors.New("catalogclient: invalid request")
	// ErrUnauthorized is returned when the server responds 401/403.
	ErrUnauthorized = errors.New("catalogclient: unauthorized")
	// ErrUpstream is returned for transient 5xx responses; safe-ish to retry.
	ErrUpstream = errors.New("catalogclient: upstream error")
)

// APIError is the structured error returned for every non-2xx response. It
// wraps a sentinel via errors.Is so callers can pattern-match on HTTP class
// without parsing the server's Code string.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	sentinel   error
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("catalogclient: %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("catalogclient: %d: %s", e.StatusCode, e.Message)
}

// Is reports whether the wrapped sentinel matches the target. errors.Is
// walks this transparently.
func (e *APIError) Is(target error) bool {
	return errors.Is(e.sentinel, target)
}

// newAPIErrorFromResponse builds an APIError from the parsed internalapi.Error
// envelope, attaching the right sentinel so callers can branch with errors.Is.
// When the server omitted Code (or sent garbage), the sentinel is chosen from
// the HTTP status class.
func newAPIErrorFromResponse(status int, body internalapi.Error) *APIError {
	msg := body.Error
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &APIError{
		StatusCode: status,
		Code:       body.Code,
		Message:    msg,
		sentinel:   sentinelForStatus(status, body.Code),
	}
}

// sentinelForStatus picks a sentinel for errors.Is checks. Code wins over
// status when both are present (e.g. a 400 with CodeNotFound would still map
// to ErrNotFound — unusual but lets the server be explicit if it ever needs
// to).
func sentinelForStatus(status int, code string) error {
	switch code {
	case internalapi.CodeNotFound:
		return ErrNotFound
	case internalapi.CodeInvalidRequest:
		return ErrInvalidRequest
	}
	switch {
	case status == http.StatusNotFound:
		return ErrNotFound
	case status == http.StatusBadRequest:
		return ErrInvalidRequest
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return ErrUnauthorized
	case status >= 500:
		return ErrUpstream
	}
	return nil
}
