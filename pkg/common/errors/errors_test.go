package errors_test

import (
	"errors"
	"net/http"
	"testing"

	commonerrors "github.com/biqly/biqly/pkg/common/errors"
	"github.com/biqly/biqly/pkg/internalapi"
)

func TestServiceError_ErrorFallsBackToStatusText(t *testing.T) {
	t.Parallel()
	se := commonerrors.New(http.StatusNotFound, "", "", nil)
	if got := se.Error(); got != "Not Found" {
		t.Fatalf("Error() = %q, want %q", got, "Not Found")
	}
}

func TestServiceError_ErrorReturnsMessageOnly(t *testing.T) {
	t.Parallel()
	se := commonerrors.New(http.StatusInternalServerError, "", "something broke", nil)
	if got := se.Error(); got != "something broke" {
		t.Fatalf("Error() = %q, want %q", got, "something broke")
	}
}

func TestServiceError_ErrorReturnsCodeOnly(t *testing.T) {
	t.Parallel()
	se := commonerrors.New(http.StatusInternalServerError, internalapi.CodeInternal, "", nil)
	if got := se.Error(); got != internalapi.CodeInternal {
		t.Fatalf("Error() = %q, want %q", got, internalapi.CodeInternal)
	}
}

func TestServiceError_ErrorReturnsCodeAndMessage(t *testing.T) {
	t.Parallel()
	se := commonerrors.New(http.StatusBadRequest, internalapi.CodeInvalidRequest, "bad input", nil)
	if got := se.Error(); got != "invalid_request: bad input" {
		t.Fatalf("Error() = %q, want %q", got, "invalid_request: bad input")
	}
}

func TestServiceError_EnvelopeRoundTrip(t *testing.T) {
	t.Parallel()
	cause := errors.New("upstream timeout")
	se := commonerrors.Wrap(http.StatusBadGateway, internalapi.CodeUpstream, "catalog unreachable", cause)
	env := se.Envelope()

	got := commonerrors.FromEnvelope(http.StatusBadGateway, env)
	if got.Status != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", got.Status, http.StatusBadGateway)
	}
	if got.Code != internalapi.CodeUpstream {
		t.Fatalf("code = %q, want %q", got.Code, internalapi.CodeUpstream)
	}
	if got.Message == "" {
		t.Fatal("message empty after round-trip")
	}
}

func TestServiceError_EnvelopeWithEmptyMessage(t *testing.T) {
	t.Parallel()
	se := commonerrors.New(http.StatusNotFound, internalapi.CodeNotFound, "", nil)
	env := se.Envelope()
	if env.Error != "Not Found" {
		t.Fatalf("Envelope().Error = %q, want %q", env.Error, "Not Found")
	}
}

func TestServiceError_UnwrapPreservesCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("inner")
	se := commonerrors.Wrap(http.StatusInternalServerError, internalapi.CodeInternal, "wrapper", cause)
	if !errors.Is(se, cause) {
		t.Fatalf("errors.Is should reach the cause")
	}
}

func TestServiceError_UnwrapNilReceiver(t *testing.T) {
	t.Parallel()
	var se *commonerrors.ServiceError
	if got := se.Unwrap(); got != nil {
		t.Fatalf("Unwrap() = %v, want nil", got)
	}
}

func TestServiceError_EnvelopeNilReceiver(t *testing.T) {
	t.Parallel()
	var se *commonerrors.ServiceError
	env := se.Envelope()
	if env != (internalapi.Error{}) {
		t.Fatalf("Envelope() = %+v, want empty", env)
	}
}

func TestServiceError_ErrorNilReceiver(t *testing.T) {
	t.Parallel()
	var se *commonerrors.ServiceError
	if got := se.Error(); got != "" {
		t.Fatalf("Error() = %q, want empty", got)
	}
}

func TestWrapWithNilCause(t *testing.T) {
	t.Parallel()
	se := commonerrors.Wrap(http.StatusBadRequest, internalapi.CodeInvalidRequest, "bad input", nil)
	if se == nil {
		t.Fatal("Wrap returned nil")
	}
	if se.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", se.Status, http.StatusBadRequest)
	}
	if se.Message != "bad input" {
		t.Fatalf("message = %q, want %q", se.Message, "bad input")
	}
}

func TestWrapWithNonNilCauseAndEmptyMessage(t *testing.T) {
	t.Parallel()
	cause := errors.New("underlying issue")
	se := commonerrors.Wrap(http.StatusServiceUnavailable, internalapi.CodeUpstream, "", cause)
	if se.Message != "underlying issue" {
		t.Fatalf("message = %q, want %q", se.Message, "underlying issue")
	}
	if !errors.Is(se, cause) {
		t.Fatal("errors.Is should reach the cause")
	}
}

func TestFromEnvelopeWithEmptyErrorField(t *testing.T) {
	t.Parallel()
	env := internalapi.Error{Code: internalapi.CodeNotFound, Error: "", Message: "missing detail"}
	se := commonerrors.FromEnvelope(http.StatusNotFound, env)
	if se.Message != "Not Found" {
		t.Fatalf("message = %q, want %q", se.Message, "Not Found")
	}
	if se.Code != internalapi.CodeNotFound {
		t.Fatalf("code = %q, want %q", se.Code, internalapi.CodeNotFound)
	}
	if se.Detail != "missing detail" {
		t.Fatalf("detail = %q, want %q", se.Detail, "missing detail")
	}
}

func TestHTTPStatusFor_PermissionError(t *testing.T) {
	t.Parallel()
	if got := commonerrors.HTTPStatusFor(internalapi.CodePermissionError); got != http.StatusForbidden {
		t.Errorf("HTTPStatusFor(%q) = %d, want %d", internalapi.CodePermissionError, got, http.StatusForbidden)
	}
}

func TestHTTPStatusFor_ReadOnlyError(t *testing.T) {
	t.Parallel()
	if got := commonerrors.HTTPStatusFor(internalapi.CodeReadOnlyError); got != http.StatusBadRequest {
		t.Errorf("HTTPStatusFor(%q) = %d, want %d", internalapi.CodeReadOnlyError, got, http.StatusBadRequest)
	}
}

func TestHTTPStatusFor_ExecutionError(t *testing.T) {
	t.Parallel()
	if got := commonerrors.HTTPStatusFor(internalapi.CodeExecutionError); got != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatusFor(%q) = %d, want %d", internalapi.CodeExecutionError, got, http.StatusUnprocessableEntity)
	}
}

func TestHTTPStatusFor(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		internalapi.CodeNotFound:       http.StatusNotFound,
		internalapi.CodeInvalidRequest: http.StatusBadRequest,
		internalapi.CodeUnauthorized:   http.StatusUnauthorized,
		internalapi.CodeUpstream:       http.StatusBadGateway,
		internalapi.CodeCompileError:   http.StatusUnprocessableEntity,
		"":                             http.StatusInternalServerError,
		"unknown-code":                 http.StatusInternalServerError,
	}
	for code, want := range cases {
		if got := commonerrors.HTTPStatusFor(code); got != want {
			t.Errorf("HTTPStatusFor(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestAs(t *testing.T) {
	t.Parallel()
	se := commonerrors.New(http.StatusNotFound, internalapi.CodeNotFound, "missing", nil)
	wrapped := errors.Join(errors.New("context"), se)
	if got := commonerrors.As(wrapped); got == nil || got.Code != internalapi.CodeNotFound {
		t.Fatalf("As should recover ServiceError, got %+v", got)
	}
	if got := commonerrors.As(nil); got != nil {
		t.Fatalf("As(nil) = %+v, want nil", got)
	}
	if got := commonerrors.As(errors.New("plain")); got != nil {
		t.Fatalf("As(plain) = %+v, want nil", got)
	}
}
