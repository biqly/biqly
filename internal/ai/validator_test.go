package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestSchemaValidator_ValidJSON(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Expression: "orders.total", Aggregation: "sum"},
		},
	}

	v := NewSchemaValidator()

	input := `{"select": [{"type": "dimension", "name": "country"}, {"type": "metric", "name": "revenue"}], "limit": 100}`
	lq, err := v.Validate(input, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lq.Select) != 2 {
		t.Errorf("expected 2 select items, got %d", len(lq.Select))
	}
}

func TestSchemaValidator_MarkdownStripping(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{Name: "id", ColumnRef: "t.id", Type: "text"}},
	}

	v := NewSchemaValidator()
	input := "```json\n{\"select\": [{\"type\": \"dimension\", \"name\": \"id\"}], \"limit\": 10}\n```"
	lq, err := v.Validate(input, model)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lq.Select) != 1 {
		t.Errorf("expected 1 select item, got %d", len(lq.Select))
	}
}

func TestSchemaValidator_RejectsInvalidType(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{Name: "id", ColumnRef: "t.id", Type: "text"}},
	}

	v := NewSchemaValidator()
	input := `{"select": [{"type": "invalid_type", "name": "id"}], "limit": 10}`
	_, err := v.Validate(input, model)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestSchemaValidator_RejectsUnknownField(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{Name: "id", ColumnRef: "t.id", Type: "text"}},
		Metrics:    []semantic.Metric{{Name: "cnt", Expression: "t.id", Aggregation: "count"}},
	}

	v := NewSchemaValidator()
	input := `{"select": [{"type": "dimension", "name": "nonexistent"}], "limit": 10}`
	_, err := v.Validate(input, model)
	if err == nil {
		t.Fatal("expected error for unknown dimension")
	}
}

func TestSchemaValidator_RejectsEmptySelect(t *testing.T) {
	model := &semantic.SemanticModel{}

	v := NewSchemaValidator()
	input := `{"select": [], "limit": 10}`
	_, err := v.Validate(input, model)
	if err == nil {
		t.Fatal("expected error for empty select")
	}
}

func TestSchemaValidator_RejectsInvalidOperator(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{Name: "name", ColumnRef: "t.name", Type: "text"}},
	}

	v := NewSchemaValidator()
	input := `{"select": [{"type": "dimension", "name": "name"}], "filters": [{"field": "name", "operator": "invalid_op"}], "limit": 10}`
	_, err := v.Validate(input, model)
	if err == nil {
		t.Fatal("expected error for invalid operator")
	}
}

func TestSchemaValidator_RejectsInvalidOrderDirection(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{Name: "name", ColumnRef: "t.name", Type: "text"}},
	}

	v := NewSchemaValidator()
	input := `{"select": [{"type": "dimension", "name": "name"}], "order_by": [{"field": "name", "direction": "random"}], "limit": 10}`
	_, err := v.Validate(input, model)
	if err == nil {
		t.Fatal("expected error for invalid direction")
	}
}

func TestSchemaValidator_RejectsNegativeLimit(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{Name: "name", ColumnRef: "t.name", Type: "text"}},
	}

	v := NewSchemaValidator()
	input := `{"select": [{"type": "dimension", "name": "name"}], "limit": -1}`
	_, err := v.Validate(input, model)
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestSchemaValidator_RejectsInvalidJSON(t *testing.T) {
	model := &semantic.SemanticModel{}

	v := NewSchemaValidator()
	_, err := v.Validate("{not valid json", model)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
