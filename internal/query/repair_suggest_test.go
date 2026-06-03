package query

import (
	"errors"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/assert"
)

func TestSuggestAlternatives(t *testing.T) {
	candidates := []string{"gross_revenue", "net_revenue", "customer_name", "created_at"}

	tests := []struct {
		unknown  string
		expected []string
	}{
		{"revenue", []string{"net_revenue", "gross_revenue"}},
		{"cust_name", []string{"customer_name"}},
		{"created", []string{"created_at"}},
		{"completely_unrelated", []string{"created_at"}},
	}

	for _, tc := range tests {
		got := suggestAlternatives(tc.unknown, candidates)
		assert.Equal(t, tc.expected, got)
	}
}

func TestValidatorStructuredErrors(t *testing.T) {
	model := &semantic.SemanticModel{
		ID:           "test-model",
		DatasourceID: "test-ds",
		Name:         "orders",
		Dimensions: []semantic.Dimension{
			{Name: "customer_name", ColumnRef: "orders.customer_name", Type: "text"},
			{Name: "created_at", ColumnRef: "orders.created_at", Type: "timestamp"},
		},
		Metrics: []semantic.Metric{
			{Name: "gross_revenue", Expression: "orders.amount", Aggregation: "sum"},
		},
	}

	v := NewValidator(100)

	// Test 1: Unknown Dimension in select
	lq1 := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "customer_nam"},
		},
	}
	err1 := v.Validate(lq1, model)
	assert.NotNil(t, err1)
	var ve1 ValidationErrors
	assert.True(t, errors.As(err1, &ve1))
	assert.Len(t, ve1, 1)
	assert.Equal(t, "UNKNOWN_DIMENSION", ve1[0].Code)
	assert.Equal(t, "customer_nam", ve1[0].Value)
	assert.Equal(t, []string{"customer_name"}, ve1[0].AllowedAlternatives)

	// Test 2: Unknown Metric in select
	lq2 := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeMetric, Name: "revenue"},
		},
	}
	err2 := v.Validate(lq2, model)
	assert.NotNil(t, err2)
	var ve2 ValidationErrors
	assert.True(t, errors.As(err2, &ve2))
	assert.Len(t, ve2, 1)
	assert.Equal(t, "UNKNOWN_METRIC", ve2[0].Code)
	assert.Equal(t, "revenue", ve2[0].Value)
	assert.Equal(t, []string{"gross_revenue"}, ve2[0].AllowedAlternatives)
}
