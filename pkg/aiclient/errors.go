package aiclient

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/biqly/biqly/pkg/internalapi"
)

// Sentinel errors. Callers should use errors.Is to branch on them; the
// underlying APIError or ClarificationError preserves detail for logging.
var (
	// ErrNotFound is returned when the server responds 404.
	ErrNotFound = errors.New("aiclient: not found")
	// ErrInvalidRequest is returned when the server responds 400.
	ErrInvalidRequest = errors.New("aiclient: invalid request")
	// ErrUnauthorized is returned when the server responds 401/403.
	ErrUnauthorized = errors.New("aiclient: unauthorized")
	// ErrUpstream is returned for transient 5xx responses.
	ErrUpstream = errors.New("aiclient: upstream error")
	// ErrNeedsClarification is returned when the server responds 2xx but the
	// body has needs_clarification set (table routing or validator could not
	// proceed without user input).
	ErrNeedsClarification = errors.New("aiclient: needs clarification")
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
		return fmt.Sprintf("aiclient: %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("aiclient: %d: %s", e.StatusCode, e.Message)
}

// Is reports whether the wrapped sentinel matches the target.
func (e *APIError) Is(target error) bool {
	return errors.Is(e.sentinel, target)
}

// ClarificationError is returned for successful HTTP responses where the AI
// or table router needs more input before producing a LogicalQuery.
type ClarificationError struct {
	Response *QueryResponse
	sentinel error
}

func (e *ClarificationError) Error() string {
	if e.Response != nil && e.Response.Clarification != nil && e.Response.Clarification.ClarificationQuestion != "" {
		return "aiclient: needs clarification: " + e.Response.Clarification.ClarificationQuestion
	}
	return "aiclient: needs clarification"
}

// Is reports whether the wrapped sentinel matches the target.
func (e *ClarificationError) Is(target error) bool {
	return errors.Is(e.sentinel, target)
}

func newAPIErrorFromResponse(status int, body internalapi.Error) *APIError {
	msg := body.Error
	if msg == "" && body.Message != "" {
		msg = body.Message
	}
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

func newClarificationError(resp *QueryResponse) *ClarificationError {
	return &ClarificationError{
		Response: resp,
		sentinel: ErrNeedsClarification,
	}
}

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
