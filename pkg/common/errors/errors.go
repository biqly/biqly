package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/biqly/biqly/pkg/internalapi"
)

// ServiceError pairs an HTTP status with the canonical internalapi.Error
// envelope. It implements the standard error interface and can be unwrapped
// to recover the originating Go error for logging.
//
// Construct via New (handler side, building a typed error) or FromEnvelope
// (client side, parsing an HTTP response body).
type ServiceError struct {
	Status   int
	Code     string
	Message  string
	Detail   string
	cause    error
}

// Error returns the human-readable message, prefixed with the code when set
// so log lines stay diagnosable without a stack trace.
func (e *ServiceError) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.Code != "" && e.Message != "":
		return e.Code + ": " + e.Message
	case e.Message != "":
		return e.Message
	case e.Code != "":
		return e.Code
	default:
		return http.StatusText(e.Status)
	}
}

// Unwrap returns the originating Go error so callers can use errors.Is /
// errors.As against sentinels even after the ServiceError has been wrapped.
func (e *ServiceError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Envelope returns the JSON shape that should be written to an HTTP response
// body. The Error field is kept legacy-compatible: it always contains the
// human-readable message so older clients that only look at "error" still
// work; Code and Message let new clients branch on stable identifiers.
func (e *ServiceError) Envelope() internalapi.Error {
	if e == nil {
		return internalapi.Error{}
	}
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return internalapi.Error{
		Code:    e.Code,
		Error:   msg,
		Message: e.Detail,
	}
}

// New builds a ServiceError from an HTTP status, a stable code and a
// human-readable message. The optional cause is preserved for Unwrap and is
// not exposed over the wire.
func New(status int, code, message string, cause error) *ServiceError {
	return &ServiceError{
		Status:  status,
		Code:    code,
		Message: message,
		cause:   cause,
	}
}

// Wrap returns a ServiceError that wraps cause with a custom status + code.
// The cause's Error() is appended to message when non-empty.
func Wrap(status int, code, message string, cause error) *ServiceError {
	if cause == nil {
		return New(status, code, message, nil)
	}
	if message == "" {
		message = cause.Error()
	} else {
		message = fmt.Sprintf("%s: %v", message, cause)
	}
	return &ServiceError{
		Status:  status,
		Code:    code,
		Message: message,
		cause:   cause,
	}
}

// FromEnvelope rebuilds a ServiceError from a parsed wire envelope plus the
// HTTP status the client observed. It is the client-side inverse of
// Envelope; the round-trip preserves Code, Message and Status but cannot
// recover the original cause (that is server-side only).
func FromEnvelope(status int, env internalapi.Error) *ServiceError {
	msg := env.Error
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &ServiceError{
		Status:  status,
		Code:    env.Code,
		Message: msg,
		Detail:  env.Message,
	}
}

// HTTPStatusFor returns a reasonable HTTP status for a given canonical code
// when the server has not provided one explicitly. Unknown codes fall back
// to 500 so partial outages stay visible in monitoring.
func HTTPStatusFor(code string) int {
	switch code {
	case internalapi.CodeNotFound:
		return http.StatusNotFound
	case internalapi.CodeInvalidRequest:
		return http.StatusBadRequest
	case internalapi.CodeUnauthorized:
		return http.StatusUnauthorized
	case internalapi.CodePermissionError:
		return http.StatusForbidden
	case internalapi.CodeUpstream:
		return http.StatusBadGateway
	case internalapi.CodeReadOnlyError:
		return http.StatusBadRequest
	case internalapi.CodeCompileError, internalapi.CodeExecutionError:
		return http.StatusUnprocessableEntity
	case internalapi.CodeInternal, "":
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// As is a thin helper around errors.As that returns the matching
// *ServiceError, or nil when err does not embed one. It lets callers
// short-circuit logging chains with a single line:
//
//	if se := errors.As(err); se != nil { ... }
func As(err error) *ServiceError {
	if err == nil {
		return nil
	}
	var se *ServiceError
	if errors.As(err, &se) {
		return se
	}
	return nil
}
