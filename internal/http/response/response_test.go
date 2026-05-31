package response

import (
	"context"
	"encoding/json"
	"errors"
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

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusBadRequest, "bad input")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != "bad input" {
		t.Fatalf("error = %q, want %q", resp["error"], "bad input")
	}
}

func TestWriteInternalErrorSendsPublicMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteInternalError(context.Background(), rec, http.StatusInternalServerError,
		"something went wrong", errors.New("secret internal detail"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !contains(body, "something went wrong") {
		t.Fatalf("body = %q, want public message", body)
	}
	if contains(body, "secret internal detail") {
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
