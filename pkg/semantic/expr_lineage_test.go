package semantic

import (
	"reflect"
	"testing"
)

func TestExprDependencies(t *testing.T) {
	tests := []struct {
		name     string
		expr     ExprNode
		wantCols []ColumnRefExpr
		wantMets []string
		wantDims []string
	}{
		{
			name:     "empty",
			expr:     nil,
			wantCols: nil,
			wantMets: nil,
			wantDims: nil,
		},
		{
			name:     "literal only",
			expr:     &LiteralExpr{Value: 42},
			wantCols: nil,
			wantMets: nil,
			wantDims: nil,
		},
		{
			name: "flat references",
			expr: &BinaryExpr{
				Op:    OpAdd,
				Left:  &ColumnRefExpr{Table: "t1", Column: "c1"},
				Right: &MetricRefExpr{Name: "m1"},
			},
			wantCols: []ColumnRefExpr{{Table: "t1", Column: "c1"}},
			wantMets: []string{"m1"},
			wantDims: nil,
		},
		{
			name: "nested with deduplication",
			expr: &BinaryExpr{
				Op: OpMultiply,
				Left: &FunctionCallExpr{
					Name: "COALESCE",
					Args: []ExprNode{
						&ColumnRefExpr{Table: "t1", Column: "c1"},
						&DimensionRefExpr{Name: "d1"},
					},
				},
				Right: &BinaryExpr{
					Op:    OpSubtract,
					Left:  &MetricRefExpr{Name: "m1"},
					Right: &ColumnRefExpr{Table: "t1", Column: "c1"}, // duplicate column ref
				},
			},
			wantCols: []ColumnRefExpr{{Table: "t1", Column: "c1"}},
			wantMets: []string{"m1"},
			wantDims: []string{"d1"},
		},
		{
			name: "case expression dependencies",
			expr: &CaseExpr{
				Conditions: []CaseWhen{
					{
						When: &BinaryExpr{
							Op:    OpGt,
							Left:  &ColumnRefExpr{Table: "t1", Column: "c2"},
							Right: &LiteralExpr{Value: 0},
						},
						Then: &DimensionRefExpr{Name: "d2"},
					},
				},
				ElseExpr: &MetricRefExpr{Name: "m2"},
			},
			wantCols: []ColumnRefExpr{{Table: "t1", Column: "c2"}},
			wantMets: []string{"m2"},
			wantDims: []string{"d2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCols, gotMets, gotDims := ExprDependencies(tt.expr)
			if !reflect.DeepEqual(gotCols, tt.wantCols) {
				t.Errorf("ExprDependencies() gotCols = %v, want %v", gotCols, tt.wantCols)
			}
			if !reflect.DeepEqual(gotMets, tt.wantMets) {
				t.Errorf("ExprDependencies() gotMets = %v, want %v", gotMets, tt.wantMets)
			}
			if !reflect.DeepEqual(gotDims, tt.wantDims) {
				t.Errorf("ExprDependencies() gotDims = %v, want %v", gotDims, tt.wantDims)
			}
		})
	}
}
