package query

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

func TestHistoryModelID_NilModel(t *testing.T) {
	if id := HistoryModelID(nil); id != nil {
		t.Fatalf("expected nil, got %v", *id)
	}
}

func TestHistoryModelID_InvalidUUID(t *testing.T) {
	m := &semantic.SemanticModel{ID: "not-a-uuid"}
	if id := HistoryModelID(m); id != nil {
		t.Fatalf("expected nil for non-UUID, got %v", *id)
	}
}

func TestHistoryModelID_ValidUUID(t *testing.T) {
	uid := uuid.New().String()
	m := &semantic.SemanticModel{ID: uid}
	id := HistoryModelID(m)
	if id == nil {
		t.Fatal("expected non-nil for valid UUID")
	}
	if *id != uid {
		t.Fatalf("got %q, want %q", *id, uid)
	}
}

func TestMarshalSQLArgs_Nil(t *testing.T) {
	s, err := MarshalSQLArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil for nil args, got %v", *s)
	}
}

func TestMarshalSQLArgs_Empty(t *testing.T) {
	s, err := MarshalSQLArgs([]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil string for empty slice")
	}
	if *s != "[]" {
		t.Fatalf("got %q, want \"[]\"", *s)
	}
}

func TestMarshalSQLArgs_Values(t *testing.T) {
	s, err := MarshalSQLArgs([]any{"hello", int64(42), 3.14, true, nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil string")
	}
	if *s == "" {
		t.Fatal("expected non-empty JSON")
	}
}

func TestBuildQueryHistoryEntry_Success(t *testing.T) {
	lq := &LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "m1",
		Select:       []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Limit:        100,
	}
	model := &semantic.SemanticModel{
		ID:      uuid.New().String(),
		Name:    "test",
		Version: 3,
	}
	cq := &CompiledQuery{
		SQL:  "SELECT SUM(revenue) FROM orders",
		Args: []any{"arg1"},
	}
	result := &Result{
		Stats: Stats{RowCount: 10, DurationMs: 42},
	}

	entry, err := BuildQueryHistoryEntry(lq, model, cq, result, "success", nil)
	if err != nil {
		t.Fatalf("BuildQueryHistoryEntry error: %v", err)
	}

	if entry.DatasourceID != "ds1" {
		t.Errorf("DatasourceID = %q, want ds1", entry.DatasourceID)
	}
	if entry.Fingerprint == "" {
		t.Error("Fingerprint should not be empty")
	}
	if entry.Status != "success" {
		t.Errorf("Status = %q, want success", entry.Status)
	}
	if entry.CompiledSQL == nil {
		t.Fatal("CompiledSQL should not be nil")
	}
	if *entry.CompiledSQL != cq.SQL {
		t.Errorf("CompiledSQL = %q, want %q", *entry.CompiledSQL, cq.SQL)
	}
	if entry.SQLArgs == nil {
		t.Fatal("SQLArgs should not be nil")
	}
	if entry.RowCount == nil || *entry.RowCount != 10 {
		t.Errorf("RowCount = %v, want 10", entry.RowCount)
	}
	if entry.DurationMs == nil || *entry.DurationMs != 42 {
		t.Errorf("DurationMs = %v, want 42", entry.DurationMs)
	}
}

func TestBuildQueryHistoryEntry_WithError(t *testing.T) {
	lq := &LogicalQuery{
		DatasourceID: "ds1",
		Select:       []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Limit:        100,
	}
	model := &semantic.SemanticModel{
		ID:      uuid.New().String(),
		Version: 1,
	}

	entry, err := BuildQueryHistoryEntry(lq, model, nil, nil, "error", nil)
	if err != nil {
		t.Fatalf("BuildQueryHistoryEntry error: %v", err)
	}
	if entry.Status != "error" {
		t.Errorf("Status = %q, want error", entry.Status)
	}
	if entry.CompiledSQL != nil {
		t.Error("CompiledSQL should be nil when cq is nil")
	}
	if entry.SQLArgs != nil {
		t.Error("SQLArgs should be nil when cq is nil")
	}
	if entry.RowCount != nil {
		t.Error("RowCount should be nil when result is nil")
	}
}

func TestBuildQueryHistoryEntry_WithQueryError(t *testing.T) {
	lq := &LogicalQuery{
		DatasourceID: "ds1",
		Select:       []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Limit:        100,
	}
	model := &semantic.SemanticModel{
		ID:      uuid.New().String(),
		Version: 1,
	}
	execErr := queryError("division by zero")

	entry, err := BuildQueryHistoryEntry(lq, model, nil, nil, "error", execErr)
	if err != nil {
		t.Fatalf("BuildQueryHistoryEntry error: %v", err)
	}
	if entry.ErrorMessage == nil || *entry.ErrorMessage != "division by zero" {
		t.Errorf("ErrorMessage = %v, want division by zero", entry.ErrorMessage)
	}
}

type queryError string

func (e queryError) Error() string { return string(e) }
