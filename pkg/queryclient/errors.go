package queryclient

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/biqly/biqly/pkg/internalapi"
)

// Sentinel errors. Use errors.Is to branch on them; the underlying APIError
// preserves the original HTTP status and code.
var (
	ErrNotFound       = errors.New("queryclient: not found")
	ErrInvalidRequest = errors.New("queryclient: invalid request")
	ErrUnauthorized   = errors.New("queryclient: unauthorized")
	ErrCompile        = errors.New("queryclient: compile error")
	ErrExecution      = errors.New("queryclient: execution error")
	ErrPermission     = errors.New("queryclient: permission denied")
	ErrReadOnly       = errors.New("queryclient: read-only violation")
	ErrUpstream       = errors.New("queryclient: upstream error")
)

// APIError is the structured error returned for every non-2xx response.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	sentinel   error
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("queryclient: %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("queryclient: %d: %s", e.StatusCode, e.Message)
}

// Is reports whether the wrapped sentinel matches the target. errors.Is
// walks this transparently.
func (e *APIError) Is(target error) bool { return errors.Is(e.sentinel, target) }

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

// sentinelForStatus picks a sentinel. Query-specific Code values map first
// (compile/execution/permission/read-only) because the same HTTP status
// (typically 400 or 422) is reused for several failure modes.
func sentinelForStatus(status int, code string) error {
	switch code {
	case internalapi.CodeNotFound:
		return ErrNotFound
	case internalapi.CodeInvalidRequest:
		return ErrInvalidRequest
	case internalapi.CodeCompileError:
		return ErrCompile
	case internalapi.CodeExecutionError:
		return ErrExecution
	case internalapi.CodePermissionError:
		return ErrPermission
	case internalapi.CodeReadOnlyError:
		return ErrReadOnly
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
