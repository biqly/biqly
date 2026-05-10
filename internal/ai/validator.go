package ai

import (
	"encoding/json"
	"fmt"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// SchemaValidator validates AI-generated LogicalQuery against the JSON schema.
type SchemaValidator struct{}

// NewSchemaValidator creates a new AI output validator.
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

// Validate checks the raw AI response against the expected LogicalQuery schema.
//nolint:gocyclo // linear validation steps, each check is independent and clear
func (sv *SchemaValidator) Validate(rawJSON string, model *semantic.SemanticModel) (*query.LogicalQuery, error) {
	cleaned := TrimToJSONObject(rawJSON)
	if cleaned == "" {
		return nil, fmt.Errorf("empty AI response")
	}

	// Parse JSON
	var lq query.LogicalQuery
	if err := json.Unmarshal([]byte(cleaned), &lq); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w (raw: %s)", err, truncate(cleaned, 200))
	}

	// Validate schema constraints
	if len(lq.Select) == 0 {
		return nil, fmt.Errorf("missing required field: select")
	}

	// Build field maps
	dimSet := make(map[string]bool)
	for _, d := range model.Dimensions {
		dimSet[d.Name] = true
	}
	metricSet := make(map[string]bool)
	for _, m := range model.Metrics {
		metricSet[m.Name] = true
	}

	// Validate select items
	for i, item := range lq.Select {
		if item.Type != query.SelectTypeDimension && item.Type != query.SelectTypeMetric {
			return nil, fmt.Errorf("select[%d].type must be 'dimension' or 'metric', got '%s'", i, item.Type)
		}
		if item.Name == "" {
			return nil, fmt.Errorf("select[%d].name is required", i)
		}

		switch item.Type {
		case query.SelectTypeDimension:
			if !dimSet[item.Name] {
				return nil, fmt.Errorf("select[%d]: unknown dimension '%s'", i, item.Name)
			}
		case query.SelectTypeMetric:
			if !metricSet[item.Name] {
				return nil, fmt.Errorf("select[%d]: unknown metric '%s'", i, item.Name)
			}
		}
	}

	// Validate operators
	validOps := map[string]bool{
		query.OpEq: true, query.OpNeq: true, query.OpGt: true,
		query.OpGte: true, query.OpLt: true, query.OpLte: true,
		query.OpIn: true, query.OpNotIn: true, query.OpContains: true,
		query.OpStartsWith: true, query.OpEndsWith: true, query.OpBetween: true,
		query.OpIsNull: true, query.OpIsNotNull: true,
	}

	for i, f := range lq.Filters {
		if !validOps[f.Operator] {
			return nil, fmt.Errorf("filters[%d]: invalid operator '%s'", i, f.Operator)
		}
	}

	// Validate order direction
	for i, ob := range lq.OrderBy {
		if ob.Direction != "" && ob.Direction != query.OrderAsc && ob.Direction != query.OrderDesc {
			return nil, fmt.Errorf("order_by[%d]: direction must be 'asc' or 'desc', got '%s'", i, ob.Direction)
		}
	}

	// Validate limit
	if lq.Limit < 0 {
		return nil, fmt.Errorf("limit must be non-negative, got %d", lq.Limit)
	}
	if lq.Offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative, got %d", lq.Offset)
	}

	return &lq, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
