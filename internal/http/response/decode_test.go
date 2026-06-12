package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeTestRequest struct {
	Name string `json:"name"`
}

func TestDecodeJSONRejectsInvalidBody(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	got, ok := DecodeJSON[decodeTestRequest](rec, req)

	if ok {
		t.Fatalf("DecodeJSON(invalid body) ok = true, want false with value %v", got)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("DecodeJSON(invalid body) status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if body := rec.Body.String(); !strings.Contains(body, "invalid request body") {
		t.Errorf("DecodeJSON(invalid body) response = %q, want invalid request body", body)
	}
}

func TestDecodeJSONRejectsTooLargeBody(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(strings.Repeat("x", MaxJSONRequestBytes+1)))
	rec := httptest.NewRecorder()

	got, ok := DecodeJSON[decodeTestRequest](rec, req)

	if ok {
		t.Fatalf("DecodeJSON(too large body) ok = true, want false with value %v", got)
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("DecodeJSON(too large body) status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if body := rec.Body.String(); !strings.Contains(body, "request body too large") {
		t.Errorf("DecodeJSON(too large body) response = %q, want request body too large", body)
	}
}

func TestDecodeJSONAllowEmptyAcceptsEmptyBody(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
	rec := httptest.NewRecorder()

	got, ok := DecodeJSONAllowEmpty[decodeTestRequest](rec, req)

	if !ok {
		t.Fatalf("DecodeJSONAllowEmpty(empty body) ok = false, want true")
	}
	if got == nil {
		t.Fatal("DecodeJSONAllowEmpty(empty body) = nil, want zero-value request")
	}
	if got.Name != "" {
		t.Errorf("DecodeJSONAllowEmpty(empty body).Name = %q, want empty", got.Name)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("DecodeJSONAllowEmpty(empty body) status = %d, want untouched %d", rec.Code, http.StatusOK)
	}
}
