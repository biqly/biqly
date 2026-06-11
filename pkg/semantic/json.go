package semantic

import (
	"encoding/json"

	"github.com/bytedance/sonic"
)

// Dimension and Metric carry ExprNode interface fields (calculated_expr /
// expr). A plain json.Unmarshal cannot decode into an interface, so any
// payload containing those keys — catalog /internal/models responses, model
// snapshots, inline models on /internal/query requests — would fail without
// these custom unmarshalers. They shadow the interface field with raw JSON
// and decode it through UnmarshalExprNode's type discriminator.

// UnmarshalJSON decodes a Dimension, reconstructing CalculatedExpr from its
// type-discriminated JSON form.
func (d *Dimension) UnmarshalJSON(data []byte) error {
	type alias Dimension
	aux := struct {
		*alias
		CalculatedExpr json.RawMessage `json:"calculated_expr,omitempty"`
	}{alias: (*alias)(d)}
	if err := sonic.ConfigStd.Unmarshal(data, &aux); err != nil {
		return err
	}
	d.CalculatedExpr = exprNodeFromRaw(aux.CalculatedExpr)
	return nil
}

// UnmarshalJSON decodes a Metric, reconstructing Expr from its
// type-discriminated JSON form.
func (m *Metric) UnmarshalJSON(data []byte) error {
	type alias Metric
	aux := struct {
		*alias
		Expr json.RawMessage `json:"expr,omitempty"`
	}{alias: (*alias)(m)}
	if err := sonic.ConfigStd.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.Expr = exprNodeFromRaw(aux.Expr)
	return nil
}

// exprNodeFromRaw decodes an expression AST, dropping nodes that fail to
// parse: the Expression/CalculatedExpression strings remain the source of
// truth and downstream hydration re-parses them when the AST is absent.
func exprNodeFromRaw(raw json.RawMessage) ExprNode {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	expr, err := UnmarshalExprNode(raw)
	if err != nil {
		return nil
	}
	return expr
}
