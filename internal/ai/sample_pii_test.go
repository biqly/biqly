package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

func TestExcludePIIColumns(t *testing.T) {
	email := "email"
	cols := []metadata.Column{
		{ColumnName: "id"},
		{ColumnName: "email", PIIType: &email},
		{ColumnName: "country"},
	}
	got := ExcludePIIColumns(cols)
	if len(got) != 2 {
		t.Fatalf("got %d columns, want 2 (PII column dropped)", len(got))
	}
	for _, c := range got {
		if c.ColumnName == "email" {
			t.Fatal("PII-annotated column must be excluded from the sample")
		}
	}
}
