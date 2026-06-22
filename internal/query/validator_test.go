package query

import (
	"encoding/json"
	"testing"

	"github.com/biqly/biqly/internal/errmsg"
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

func TestValidateFromAndCTEs(t *testing.T) {
	t.Run("mutually exclusive from_subquery and from_cte", func(t *testing.T) {
		lq := &LogicalQuery{
			Select:       []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
			Limit:        10,
			FromSubquery: &SubqueryBody{Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}}},
			FromCTE:      "my_cte",
		}
		errs := validateFromAndCTEs(lq)
		if len(errs) == 0 {
			t.Fatal("expected error for mutually exclusive from")
		}
		if errs[0].Code != "MUTUALLY_EXCLUSIVE_FROM" {
			t.Errorf("code = %q, want MUTUALLY_EXCLUSIVE_FROM", errs[0].Code)
		}
	})

	t.Run("missing cte name", func(t *testing.T) {
		lq := &LogicalQuery{
			Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
			Limit:  10,
			CTEs:   []CTE{{Name: "", Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}}}},
		}
		errs := validateFromAndCTEs(lq)
		if len(errs) == 0 {
			t.Fatal("expected error for missing CTE name")
		}
		if errs[0].Code != "MISSING_CTE_NAME" {
			t.Errorf("code = %q, want MISSING_CTE_NAME", errs[0].Code)
		}
	})

	t.Run("from_cte references unknown cte", func(t *testing.T) {
		lq := &LogicalQuery{
			Select:  []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
			Limit:   10,
			FromCTE: "nonexistent",
			CTEs: []CTE{
				{Name: "my_cte", Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}}},
			},
		}
		errs := validateFromAndCTEs(lq)
		if len(errs) == 0 {
			t.Fatal("expected error for unknown CTE")
		}
		if errs[0].Code != "UNKNOWN_CTE" {
			t.Errorf("code = %q, want UNKNOWN_CTE", errs[0].Code)
		}
	})

	t.Run("valid from_cte passes", func(t *testing.T) {
		lq := &LogicalQuery{
			Select:  []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
			Limit:   10,
			FromCTE: "my_cte",
			CTEs: []CTE{
				{Name: "my_cte", Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}}},
			},
		}
		errs := validateFromAndCTEs(lq)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})
}

func TestValidateHavingClause_InvalidOperator(t *testing.T) {
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Having: []Filter{{Field: "revenue", Operator: OpContains, Value: "bad"}},
		Limit:  10,
	}
	errs := validateHavingClause(lq, newValidationLookups(validatorTestModel()))
	if len(errs) == 0 {
		t.Fatal("expected error for invalid having operator")
	}
	if errs[0].Code != errmsg.CodeInvalidOperator {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeInvalidOperator)
	}
}

func TestValidateGroupByClauses_TimeGrainOnNonDate(t *testing.T) {
	lq := &LogicalQuery{
		Select:  []SelectItem{{Type: SelectTypeDimension, Name: "country"}},
		GroupBy: []GroupBy{{Field: "country", TimeGrain: "month"}},
		Limit:   10,
	}
	lk := newValidationLookups(validatorTestModel())
	errs := validateGroupByClauses(lq, lk)
	if len(errs) == 0 {
		t.Fatal("expected error for time_grain on non-date dimension")
	}
	if errs[0].Code != errmsg.CodeTimeGrainOnNonDate {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeTimeGrainOnNonDate)
	}
}

func TestValidateGroupByClauses_InvalidTimeGrain(t *testing.T) {
	lq := &LogicalQuery{
		Select:  []SelectItem{{Type: SelectTypeDimension, Name: "order_date"}},
		GroupBy: []GroupBy{{Field: "order_date", TimeGrain: "invalid_grain"}},
		Limit:   10,
	}
	lk := newValidationLookups(validatorTestModel())
	errs := validateGroupByClauses(lq, lk)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid time_grain")
	}
	if errs[0].Code != errmsg.CodeInvalidTimeGrain {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeInvalidTimeGrain)
	}
}

func TestValidateLimitOffset_NegativeLimit(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Limit:  -1,
	}
	errs := v.validateLimitOffset(lq)
	if len(errs) == 0 {
		t.Fatal("expected error for negative limit")
	}
	if errs[0].Code != "NEGATIVE_LIMIT" {
		t.Errorf("code = %q, want NEGATIVE_LIMIT", errs[0].Code)
	}
}

func TestValidateLimitOffset_NegativeOffset(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Limit:  10,
		Offset: -1,
	}
	errs := v.validateLimitOffset(lq)
	if len(errs) == 0 {
		t.Fatal("expected error for negative offset")
	}
	if errs[0].Code != errmsg.CodeNegativeOffset {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeNegativeOffset)
	}
}

func TestValidateLimitOffset_ExceedsMaxRows(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Limit:  1000,
	}
	errs := v.validateLimitOffset(lq)
	if len(errs) == 0 {
		t.Fatal("expected error for limit > maxRows")
	}
	if errs[0].Code != errmsg.CodeRowLimitExceeded {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeRowLimitExceeded)
	}
}

func TestValidateWindowSelect_MissingSpec(t *testing.T) {
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "bad_window", Window: nil},
		},
		Limit: 10,
	}
	lk := newValidationLookups(validatorTestModel())
	errs := validateWindowSelect(lq.Select[0], validatorTestModel(), lk.dimMap, lk.metricRegistry, lk.dimensionNames, lk.metricNames, lk.allFieldNames)
	if len(errs) == 0 {
		t.Fatal("expected error for missing window spec")
	}
	if errs[0].Code != "MISSING_WINDOW_SPEC" {
		t.Errorf("code = %q, want MISSING_WINDOW_SPEC", errs[0].Code)
	}
}

func TestValidateWindowSelect_MissingMetric(t *testing.T) {
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "bad_window",
				Window: &WindowSpec{Metric: "nonexistent", Aggregation: "sum"}},
		},
		Limit: 10,
	}
	lk := newValidationLookups(validatorTestModel())
	errs := validateWindowSelect(lq.Select[0], validatorTestModel(), lk.dimMap, lk.metricRegistry, lk.dimensionNames, lk.metricNames, lk.allFieldNames)
	if len(errs) == 0 {
		t.Fatal("expected error for missing metric")
	}
	if errs[0].Code != errmsg.CodeUnknownMetric {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeUnknownMetric)
	}
}

func TestValidateWindowSelect_InvalidPartitionBy(t *testing.T) {
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "w",
				Window: &WindowSpec{
					Metric:      "revenue",
					Aggregation: "sum",
					PartitionBy: []string{"nonexistent_dim"},
				}},
		},
		Limit: 10,
	}
	lk := newValidationLookups(validatorTestModel())
	errs := validateWindowSelect(lq.Select[0], validatorTestModel(), lk.dimMap, lk.metricRegistry, lk.dimensionNames, lk.metricNames, lk.allFieldNames)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown partition by")
	}
	if errs[0].Code != errmsg.CodeUnknownDimension {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeUnknownDimension)
	}
}

func TestValidateWindowSelect_InvalidAggregation(t *testing.T) {
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "w",
				Window: &WindowSpec{
					Metric:      "",
					Aggregation: "invalid_agg",
				}},
		},
		Limit: 10,
	}
	lk := newValidationLookups(validatorTestModel())
	errs := validateWindowSelect(lq.Select[0], validatorTestModel(), lk.dimMap, lk.metricRegistry, lk.dimensionNames, lk.metricNames, lk.allFieldNames)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid aggregation")
	}
	if errs[0].Code != "INVALID_WINDOW_AGGREGATION" {
		t.Errorf("code = %q, want INVALID_WINDOW_AGGREGATION", errs[0].Code)
	}
}

func TestValidateCaseSelect_MissingBranches(t *testing.T) {
	item := SelectItem{Type: SelectTypeCase, Name: "case_col", Case: nil}
	errs := validateCaseSelect(item, nil, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for missing case branches")
	}
	if errs[0].Code != "MISSING_CASE_BRANCHES" {
		t.Errorf("code = %q, want MISSING_CASE_BRANCHES", errs[0].Code)
	}

	item2 := SelectItem{Type: SelectTypeCase, Name: "case_col", Case: &CaseExpr{Branches: nil}}
	errs = validateCaseSelect(item2, nil, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for empty case branches")
	}
}

func TestValidateCaseSelect_MissingNameAndAlias(t *testing.T) {
	literalVal := any("NA")
	item := SelectItem{
		Type: SelectTypeCase, Name: "", Alias: "",
		Case: &CaseExpr{
			Branches: []CaseBranch{
				{
					When: []Filter{{Field: "country", Operator: OpEq, Value: "US"}},
					Then: CaseThen{Type: CaseThenTypeLiteral, Literal: &literalVal},
				},
			},
		},
	}
	errs := validateCaseSelect(item, map[string]bool{"country": true}, []string{"country"})
	if len(errs) == 0 {
		t.Fatal("expected errors")
	}
	hasMissingName := false
	for _, e := range errs {
		if e.Code == "MISSING_CASE_NAME" {
			hasMissingName = true
			break
		}
	}
	if !hasMissingName {
		t.Fatalf("expected MISSING_CASE_NAME error, got codes: %v", errs)
	}
}

func TestValidateCaseSelect_MissingWhen(t *testing.T) {
	literalVal := any("NA")
	item := SelectItem{
		Type: SelectTypeCase, Name: "case_col",
		Case: &CaseExpr{
			Branches: []CaseBranch{
				{
					When: nil,
					Then: CaseThen{Type: CaseThenTypeLiteral, Literal: &literalVal},
				},
			},
		},
	}
	errs := validateCaseSelect(item, map[string]bool{"country": true}, []string{"country"})
	if len(errs) == 0 {
		t.Fatal("expected error for missing when")
	}
	if errs[0].Code != "MISSING_CASE_WHEN" {
		t.Errorf("code = %q, want MISSING_CASE_WHEN", errs[0].Code)
	}
}

func TestValidateCaseThen_Dimension(t *testing.T) {
	errs := validateCaseThen(CaseThen{Type: CaseThenTypeDimension, Dimension: "nonexistent"}, map[string]bool{"country": true}, []string{"country"})
	if len(errs) == 0 {
		t.Fatal("expected error for unknown dimension")
	}
	if errs[0].Code != errmsg.CodeUnknownDimension {
		t.Errorf("code = %q, want %q", errs[0].Code, errmsg.CodeUnknownDimension)
	}

	// Valid dimension
	errs = validateCaseThen(CaseThen{Type: CaseThenTypeDimension, Dimension: "country"}, map[string]bool{"country": true}, []string{"country"})
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid dimension, got %v", errs)
	}
}

func TestValidateCaseThen_Literal(t *testing.T) {
	errs := validateCaseThen(CaseThen{Type: CaseThenTypeLiteral, Literal: nil}, map[string]bool{"country": true}, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for missing literal")
	}
}

func TestValidateCaseThen_InvalidType(t *testing.T) {
	v := any("val")
	errs := validateCaseThen(CaseThen{Type: "invalid_type", Literal: &v}, map[string]bool{"country": true}, nil)
	if len(errs) == 0 {
		t.Fatal("expected error for invalid then type")
	}
	if errs[0].Code != "INVALID_CASE_THEN_TYPE" {
		t.Errorf("code = %q, want INVALID_CASE_THEN_TYPE", errs[0].Code)
	}
}

func TestValidateSubqueryFilter(t *testing.T) {
	t.Run("invalid operator", func(t *testing.T) {
		f := Filter{
			Field:    "country",
			Operator: OpEq,
			Value:    "US",
			Subquery: &SubqueryFilter{
				ResultField: "name",
				Body:        SubqueryBody{Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}}},
			},
		}
		errs := validateSubqueryFilter(f)
		if len(errs) == 0 {
			t.Fatal("expected error for invalid subquery operator")
		}
		if errs[0].Code != "INVALID_SUBQUERY_OPERATOR" {
			t.Errorf("code = %q, want INVALID_SUBQUERY_OPERATOR", errs[0].Code)
		}
	})

	t.Run("missing result field", func(t *testing.T) {
		f := Filter{
			Field:    "country",
			Operator: OpIn,
			Value:    "US",
			Subquery: &SubqueryFilter{
				ResultField: "",
				Body:        SubqueryBody{Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}}},
			},
		}
		errs := validateSubqueryFilter(f)
		if len(errs) == 0 {
			t.Fatal("expected error for missing result field")
		}
		if errs[0].Code != "MISSING_SUBQUERY_RESULT_FIELD" {
			t.Errorf("code = %q, want MISSING_SUBQUERY_RESULT_FIELD", errs[0].Code)
		}
	})

	t.Run("missing subquery select", func(t *testing.T) {
		f := Filter{
			Field:    "country",
			Operator: OpIn,
			Value:    "US",
			Subquery: &SubqueryFilter{
				ResultField: "name",
				Body:        SubqueryBody{Select: nil},
			},
		}
		errs := validateSubqueryFilter(f)
		if len(errs) == 0 {
			t.Fatal("expected error for missing subquery select")
		}
		if errs[len(errs)-1].Code != "MISSING_SUBQUERY_SELECT" {
			t.Errorf("expected MISSING_SUBQUERY_SELECT, got %v", errs)
		}
	})
}

func TestIsNumericFilterValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"int", 42, true},
		{"int8", int8(8), true},
		{"int16", int16(16), true},
		{"int32", int32(32), true},
		{"int64", int64(64), true},
		{"uint", uint(1), true},
		{"uint8", uint8(8), true},
		{"uint16", uint16(16), true},
		{"uint32", uint32(32), true},
		{"uint64", uint64(64), true},
		{"float32", float32(1.5), true},
		{"float64", float64(3.14), true},
		{"json.Number", json.Number("123"), true},
		{"string", "hello", false},
		{"bool", true, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNumericFilterValue(tt.value)
			if got != tt.want {
				t.Errorf("isNumericFilterValue(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestValidateDateFilterValueType_SkipsNonComparisons(t *testing.T) {
	dimensions := validatorTestModel().Dimensions
	f := Filter{Field: "order_date", Operator: OpIn, Value: "2024-01-01"}
	err := validateDateFilterValueType(f, dimensions)
	if err != nil {
		t.Errorf("expected nil for OpIn, got %v", err)
	}
}

func TestValidateDateFilterValueType_SkipsNonDateDimensions(t *testing.T) {
	dimensions := validatorTestModel().Dimensions
	f := Filter{Field: "country", Operator: OpEq, Value: "TR"}
	err := validateDateFilterValueType(f, dimensions)
	if err != nil {
		t.Errorf("expected nil for non-date dimension, got %v", err)
	}
}

func TestValidateFormulaSelect_Valid(t *testing.T) {
	v := NewValidator(100)
	for _, op := range []string{FormulaOpAdd, FormulaOpSubtract, FormulaOpDivide, FormulaOpPercentOf, FormulaOpPercentChange} {
		lq := &LogicalQuery{
			Select: []SelectItem{
				{
					Type: SelectTypeFormula,
					Name: "result",
					Formula: &FormulaSpec{
						Op:    op,
						Left:  MeasureRef{Metric: "orders", Filters: []Filter{{Field: "country", Operator: OpEq, Value: "TR"}}},
						Right: MeasureRef{Metric: "orders"},
					},
				},
			},
			Limit: 100,
		}
		require.NoErrorf(t, v.Validate(lq, validatorTestModel()), "op %q should validate", op)
	}
}

func TestValidateFormulaSelect_UnknownMetricAndBadFilter(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{
			{
				Type: SelectTypeFormula,
				Name: "result",
				Formula: &FormulaSpec{
					Op:    FormulaOpDivide,
					Left:  MeasureRef{Metric: "nope", Filters: []Filter{{Field: "ghost", Operator: OpEq, Value: 1}}},
					Right: MeasureRef{Metric: "orders"},
				},
			},
		},
		Limit: 100,
	}
	err := v.Validate(lq, validatorTestModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
	assert.Contains(t, err.Error(), "ghost")
}

func TestValidateFormulaSelect_InvalidOp(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{
			{
				Type: SelectTypeFormula,
				Name: "result",
				Formula: &FormulaSpec{
					Op:    "multiply",
					Left:  MeasureRef{Metric: "orders"},
					Right: MeasureRef{Metric: "orders"},
				},
			},
		},
		Limit: 100,
	}
	err := v.Validate(lq, validatorTestModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiply")
}

func TestValidateFormulaSelect_MissingSpec(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeFormula, Name: "result"}},
		Limit:  100,
	}
	require.Error(t, v.Validate(lq, validatorTestModel()))
}

func TestValidateMetricSelect_PerMeasureFilterValidated(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeMetric, Name: "orders", Alias: "tr_orders", Filters: []Filter{{Field: "ghost", Operator: OpEq, Value: 1}}},
		},
		Limit: 100,
	}
	err := v.Validate(lq, validatorTestModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestValidateWindowSelect_CountDistinctRejected(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "w", Window: &WindowSpec{
				Aggregation: "count_distinct", Metric: "orders",
				OrderBy: []OrderBy{{Field: "order_date", Direction: "asc"}},
			}},
		},
		Limit: 100,
	}
	err := v.Validate(lq, validatorTestModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count_distinct")
}

func TestValidateWindowSelect_RankingRequiresOrderBy(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "rn", Window: &WindowSpec{Aggregation: "row_number"}},
		},
		Limit: 100,
	}
	err := v.Validate(lq, validatorTestModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "order_by")
}

func TestValidateWindowSelect_LagRequiresValue(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "lg", Window: &WindowSpec{
				Aggregation: "lag",
				OrderBy:     []OrderBy{{Field: "order_date", Direction: "asc"}},
			}},
		},
		Limit: 100,
	}
	err := v.Validate(lq, validatorTestModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lag")
}

func TestValidateWindowSelect_LagValid(t *testing.T) {
	v := NewValidator(100)
	lq := &LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeWindow, Name: "lg", Window: &WindowSpec{
				Aggregation: "lag", Metric: "revenue", Offset: 1,
				OrderBy: []OrderBy{{Field: "order_date", Direction: "asc"}},
			}},
		},
		Limit: 100,
	}
	require.NoError(t, v.Validate(lq, validatorTestModel()))
}
