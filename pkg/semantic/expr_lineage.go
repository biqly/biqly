package semantic

import "slices"

// ExprDependencies traverses the given ExprNode and collects all physical column references,
// referenced metric names, and referenced dimension names.
func ExprDependencies(node ExprNode) ([]ColumnRefExpr, []string, []string) {
	var cols []ColumnRefExpr
	var mets []string
	var dims []string

	var collect func(n ExprNode)
	collect = func(n ExprNode) {
		switch e := n.(type) {
		case nil:
			return
		case LiteralExpr:
			// Literals carry no dependencies
		case ColumnRefExpr:
			dup := false
			for _, c := range cols {
				if c.Table == e.Table && c.Column == e.Column {
					dup = true
					break
				}
			}
			if !dup {
				cols = append(cols, e)
			}
		case MetricRefExpr:
			dup := slices.Contains(mets, e.Name)
			if !dup {
				mets = append(mets, e.Name)
			}
		case DimensionRefExpr:
			dup := slices.Contains(dims, e.Name)
			if !dup {
				dims = append(dims, e.Name)
			}
		case BinaryExpr:
			collect(e.Left)
			collect(e.Right)
		case UnaryExpr:
			collect(e.Expr)
		case FunctionCallExpr:
			for _, arg := range e.Args {
				collect(arg)
			}
		case CaseExpr:
			for _, cond := range e.Conditions {
				collect(cond.When)
				collect(cond.Then)
			}
			collect(e.ElseExpr)
		}
	}

	collect(node)
	return cols, mets, dims
}
