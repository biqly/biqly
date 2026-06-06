package core

import (
	"errors"
	"net/http"

	"github.com/biqly/biqly/internal/query"
)

type ServiceError struct {
	Status  int
	Message string
	cause   error
}

func (e *ServiceError) Error() string {
	return e.Message
}

func (e *ServiceError) Unwrap() error {
	return e.cause
}

// ToServiceError maps known query-layer failures to a ServiceError while preserving
// the original error for logging via Unwrap. Returns nil when err is nil.
func ToServiceError(err error) *ServiceError {
	return MapQueryServiceError(err)
}

// ErrAsError returns nil for a nil *ServiceError; otherwise returns se as error.
// Use when passing *ServiceError to APIs that take error (avoids nil *ServiceError
// being a non-nil error interface).
func ErrAsError(se *ServiceError) error {
	if se == nil {
		return nil
	}
	return se
}

// LogCause returns the wrapped cause when err is a ServiceError, for structured logging.
func LogCause(err error) error {
	if se, ok := errors.AsType[*ServiceError](err); ok && se.cause != nil {
		return se.cause
	}
	return err
}

func MapQueryServiceError(err error) *ServiceError {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[*ServiceError](err); ok {
		return se
	}
	mapped := mapQueryServiceError(err)
	if mapped != nil && mapped.cause == nil {
		mapped.cause = err
	}
	return mapped
}

func mapQueryServiceError(err error) *ServiceError {
	switch {
	case errors.Is(err, ErrModelIDRequired):
		return &ServiceError{Status: http.StatusBadRequest, Message: MsgModelIDRequired}
	case errors.Is(err, ErrDatasourceIDRequired):
		return &ServiceError{Status: http.StatusBadRequest, Message: MsgDatasourceIDRequired}
	case errors.Is(err, ErrLoadSemanticModel), errors.Is(err, ErrLoadDatasource):
		return &ServiceError{Status: http.StatusNotFound, Message: "resource not found"}
	case errors.Is(err, ErrLoadDriver):
		return &ServiceError{Status: http.StatusBadRequest, Message: "unsupported datasource type"}
	case errors.Is(err, ErrConnection):
		return &ServiceError{Status: http.StatusInternalServerError, Message: "connection failed"}
	case errors.Is(err, ErrQueryExecution):
		return &ServiceError{Status: http.StatusInternalServerError, Message: "query failed"}
	}
	if valErrs, ok := errors.AsType[query.ValidationErrors](err); ok {
		return &ServiceError{Status: http.StatusBadRequest, Message: valErrs.Error()}
	}
	return &ServiceError{Status: http.StatusInternalServerError, Message: err.Error()}
}
