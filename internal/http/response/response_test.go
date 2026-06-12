package response

import (
	"context"
	"errors"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONNormalizesNilSlice(t *testing.T) {
	rec := httptest.NewRecorder()
	var data []string // nil slice
	WriteJSON(rec, http.StatusOK, data)

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "[]\n" {
		t.Fatalf("body = %q, want %q", body, "[]\n")
	}
}

func TestWriteError5xxSanitized(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusInternalServerError, "secret server details")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var resp map[string]string
	if err := sonic.ConfigStd.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != "internal server error" {
		t.Fatalf("error = %q, want %q", resp["error"], "internal server error")
	}
}

func TestWriteError4xxRaw(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "bad input format")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	if err := sonic.ConfigStd.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != "bad input format" {
		t.Fatalf("error = %q, want %q", resp["error"], "bad input format")
	}
}

func TestWriteInternalErrorSanitizes5xx(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteInternalError(context.Background(), rec, http.StatusInternalServerError,
		"something went wrong", errors.New("secret internal detail"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !contains(body, "internal server error") {
		t.Fatalf("body = %q, want internal server error", body)
	}
	if contains(body, "secret internal detail") {
		t.Fatalf("body leaked internal detail: %q", body)
	}
	if contains(body, "something went wrong") {
		t.Fatalf("body leaked publicMsg which should be sanitized for 5xx: %q", body)
	}
}

func TestWriteInternalErrorAllows4xx(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteInternalError(context.Background(), rec, http.StatusBadRequest,
		"invalid field value", errors.New("validation failed"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	body := rec.Body.String()
	if !contains(body, "invalid field value") {
		t.Fatalf("body = %q, want raw public message for 4xx", body)
	}
	if contains(body, "validation failed") {
		t.Fatalf("body leaked internal detail: %q", body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestStatusRecorder(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	sr := NewStatusRecorder(rec)

	if got := sr.Status(); got != http.StatusOK {
		t.Fatalf("initial status = %d, want %d", got, http.StatusOK)
	}

	sr.WriteHeader(http.StatusCreated)
	if got := sr.Status(); got != http.StatusCreated {
		t.Fatalf("after WriteHeader status = %d, want %d", got, http.StatusCreated)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("underlying recorder Code = %d, want %d", rec.Code, http.StatusCreated)
	}

	_, _ = sr.Write([]byte("hello"))
	if body := rec.Body.String(); body != "hello" {
		t.Fatalf("underlying recorder body = %q, want %q", body, "hello")
	}
}

func TestWriteOK(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	WriteOK(rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	var resp map[string]string
	if err := sonic.ConfigStd.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %q, want %q", resp["status"], "ok")
	}
}
