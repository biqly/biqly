package query

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validatorTestModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", ColumnRef: "orders.created_at", Type: string(semantic.DimensionTypeDate)},
			{Name: "country", ColumnRef: "customers.country", Type: string(semantic.DimensionTypeText)},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Expression: "orders.total", Aggregation: string(semantic.AggSum)},
			{Name: "orders", Expression: "orders.id", Aggregation: string(semantic.AggCount)},
		},
	}
}

func TestValidatorValidateAllowlistBoundaries(t *testing.T) {
	model := validatorTestModel()
	validator := NewValidator(100)

	tests := []struct {
		name    string
		query   LogicalQuery
		wantErr string
	}{
		{
			name: "valid grouped metric query",
			query: LogicalQuery{
				Select: []SelectItem{
					{Type: SelectTypeDimension, Name: "country"},
					{Type: SelectTypeMetric, Name: "revenue"},
				},
				Filters: []Filter{{Field: "country", Operator: OpEq, Value: "TR"}},
				GroupBy: []GroupBy{{Field: "country"}},
				OrderBy: []OrderBy{{Field: "revenue", Direction: OrderDesc}},
				Limit:   25,
			},
		},
		{
			name: "unknown select dimension rejected",
			query: LogicalQuery{
				Select: []SelectItem{{Type: SelectTypeDimension, Name: "tenant_id"}},
				Limit:  10,
			},
			wantErr: "unknown dimension",
		},
		{
			name: "unknown filter field rejected",
			query: LogicalQuery{
				Select:  []SelectItem{{Type: SelectTypeMetric, Name: "orders"}},
				Filters: []Filter{{Field: "raw_sql", Operator: OpEq, Value: "1=1"}},
				Limit:   10,
			},
			wantErr: "unknown field",
		},
		{
			name: "unknown order by field rejected",
			query: LogicalQuery{
				Select:  []SelectItem{{Type: SelectTypeMetric, Name: "orders"}},
				OrderBy: []OrderBy{{Field: "created_at;drop table users", Direction: OrderAsc}},
				Limit:   10,
			},
			wantErr: "unknown field",
		},
		{
			name: "date grain injection rejected",
			query: LogicalQuery{
				Select:  []SelectItem{{Type: SelectTypeDimension, Name: "order_date"}},
				GroupBy: []GroupBy{{Field: "order_date", TimeGrain: "month); drop table users; --"}},
				Limit:   10,
			},
			wantErr: "invalid time_grain",
		},
		{
			name: "window partition allowlist enforced",
			query: LogicalQuery{
				Select: []SelectItem{{
					Type: SelectTypeWindow,
					Name: "bad_window",
					Window: &WindowSpec{
						Metric:      "revenue",
						PartitionBy: []string{"workspace_id"},
					},
				}},
				Limit: 10,
			},
			wantErr: "unknown dimension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(&tt.query, model)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
