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

func TestServiceError_UnwrapPreservesCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("inner")
	se := commonerrors.Wrap(http.StatusInternalServerError, internalapi.CodeInternal, "wrapper", cause)
	if !errors.Is(se, cause) {
		t.Fatalf("errors.Is should reach the cause")
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
