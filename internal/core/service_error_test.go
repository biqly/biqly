package core_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/query"
)

func TestMapQueryServiceErrorSentinels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
		msg    string
	}{
		{"model id", core.ErrModelIDRequired, http.StatusBadRequest, "model_id is required"},
		{"datasource id", core.ErrDatasourceIDRequired, http.StatusBadRequest, "datasource_id is required"},
		{"model load", core.ErrLoadSemanticModel, http.StatusNotFound, "resource not found"},
		{"datasource load", core.ErrLoadDatasource, http.StatusNotFound, "resource not found"},
		{"driver load", core.ErrLoadDriver, http.StatusBadRequest, "unsupported datasource type"},
		{"connection", core.ErrConnection, http.StatusInternalServerError, "connection failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			se := core.MapQueryServiceError(tc.err)
			if se.Status != tc.status {
				t.Fatalf("status = %d, want %d", se.Status, tc.status)
			}
			if se.Message != tc.msg {
				t.Fatalf("message = %q, want %q", se.Message, tc.msg)
			}
		})
	}
}

func TestMapQueryServiceErrorValidationErrors(t *testing.T) {
	t.Parallel()
	err := query.ValidationErrors{
		{Field: "select", Message: "unknown dimension: country"},
	}
	se := core.MapQueryServiceError(err)
	if se.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", se.Status)
	}
	if se.Message == "" || se.Message == "query failed" {
		t.Fatalf("message = %q, want validation detail", se.Message)
	}
}

func TestMapQueryServiceErrorWrappedValidation(t *testing.T) {
	t.Parallel()
	inner := query.ValidationErrors{{Field: "filters", Message: "unknown field: x"}}
	wrapped := errors.Join(errors.New("compile"), inner)
	se := core.MapQueryServiceError(wrapped)
	if se.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", se.Status)
	}
}

func TestToServiceErrorPreservesCause(t *testing.T) {
	t.Parallel()
	inner := core.ErrLoadSemanticModel
	se := core.ToServiceError(inner)
	if se == nil {
		t.Fatal("expected ServiceError")
	}
	if !errors.Is(se, core.ErrLoadSemanticModel) {
		t.Fatal("expected sentinel via unwrap chain")
	}
	if !errors.Is(core.LogCause(se), inner) {
		t.Fatal("LogCause should return original")
	}
}

func TestErrAsErrorNilServiceError(t *testing.T) {
	t.Parallel()
	if err := core.ErrAsError(nil); err != nil {
		t.Fatalf("ErrAsError(nil) = %v, want nil", err)
	}
}

func TestMapQueryServiceErrorNil(t *testing.T) {
	t.Parallel()
	if se := core.MapQueryServiceError(nil); se != nil {
		t.Fatalf("expected nil, got %+v", se)
	}
}

func TestMapQueryServiceErrorWrappedSentinel(t *testing.T) {
	t.Parallel()
	wrapped := errors.Join(core.ErrLoadSemanticModel, errors.New("pg: no rows"))
	se := core.MapQueryServiceError(wrapped)
	if se.Status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", se.Status)
	}
}
