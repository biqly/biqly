package semantic

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
)

// ExprNode is the sealed interface for semantic expression AST nodes.
type ExprNode interface {
	exprSealed()
}

const (
	exprTypeLiteral      = "literal"
	exprTypeColumnRef    = "column_ref"
	exprTypeMetricRef    = "metric_ref"
	exprTypeDimensionRef = "dimension_ref"
	exprTypeBinary       = "binary"
	exprTypeUnary        = "unary"
	exprTypeFunctionCall = "function_call"
	exprTypeCase         = "case"
)

// LiteralExpr represents a scalar literal value.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type LiteralExpr struct {
	Value any `json:"value"`
}

// ColumnRefExpr represents a physical column reference.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type ColumnRefExpr struct {
	Table  string `json:"table,omitempty"`
	Column string `json:"column"`
}

// MetricRefExpr references another semantic metric by name.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type MetricRefExpr struct {
	Name string `json:"name"`
}

// DimensionRefExpr references another semantic dimension by name.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type DimensionRefExpr struct {
	Name string `json:"name"`
}

// BinaryExpr combines two expressions with a binary operator.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type BinaryExpr struct {
	Op    BinaryOp `json:"op"`
	Left  ExprNode `json:"left"`
	Right ExprNode `json:"right"`
}

// UnaryExpr applies a unary operator to an expression.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type UnaryExpr struct {
	Op   UnaryOp  `json:"op"`
	Expr ExprNode `json:"expr"`
}

// FunctionCallExpr calls a whitelisted SQL function.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type FunctionCallExpr struct {
	Name string     `json:"name"`
	Args []ExprNode `json:"args,omitempty"`
}

// CaseExpr represents a SQL CASE expression.
//
//nolint:recvcheck // UnmarshalJSON uses pointer receiver; marshal/sealed use value receiver
type CaseExpr struct {
	Conditions []CaseWhen `json:"conditions,omitempty"`
	ElseExpr   ExprNode   `json:"else,omitempty"`
}

// CaseWhen represents one WHEN/THEN branch in a CASE expression.
type CaseWhen struct {
	When ExprNode `json:"when"`
	Then ExprNode `json:"then"`
}

// BinaryOp is a dialect-neutral binary operator.
type BinaryOp string

const (
	OpAdd      BinaryOp = "add"
	OpSubtract BinaryOp = "subtract"
	OpMultiply BinaryOp = "multiply"
	OpDivide   BinaryOp = "divide"
	OpModulo   BinaryOp = "modulo"
	OpConcat   BinaryOp = "concat"
	OpEq       BinaryOp = "eq"
	OpNeq      BinaryOp = "neq"
	OpLt       BinaryOp = "lt"
	OpLte      BinaryOp = "lte"
	OpGt       BinaryOp = "gt"
	OpGte      BinaryOp = "gte"
	OpAnd      BinaryOp = "and"
	OpOr       BinaryOp = "or"
)

// UnaryOp is a dialect-neutral unary operator.
type UnaryOp string

const (
	OpNot    UnaryOp = "not"
	OpNegate UnaryOp = "negate"
)

// AllowedFunctions maps approved SQL functions to arity. -1 means variadic.
var AllowedFunctions = map[string]int{
	"COALESCE":   -1,
	"CONCAT":     -1,
	"UPPER":      1,
	"LOWER":      1,
	"ROUND":      2,
	"LENGTH":     1,
	"TRIM":       1,
	"ABS":        1,
	"CEIL":       1,
	"FLOOR":      1,
	"CAST":       2,
	"EXTRACT":    2,
	"DATE_TRUNC": 2,
	"NULLIF":     2,
	"IFNULL":     2,
	"ISNULL":     1,
	"SUBSTRING":  3,
	"REPLACE":    3,
	"LEFT":       2,
	"RIGHT":      2,
}

func (LiteralExpr) exprSealed()      {}
func (ColumnRefExpr) exprSealed()    {}
func (MetricRefExpr) exprSealed()    {}
func (DimensionRefExpr) exprSealed() {}
func (BinaryExpr) exprSealed()       {}
func (UnaryExpr) exprSealed()        {}
func (FunctionCallExpr) exprSealed() {}
func (CaseExpr) exprSealed()         {}

func (e LiteralExpr) MarshalJSON() ([]byte, error) {
	type literalExpr LiteralExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		literalExpr
	}{Type: exprTypeLiteral, literalExpr: literalExpr(e)})
}

func (e *LiteralExpr) UnmarshalJSON(data []byte) error {
	type literalExpr LiteralExpr
	var raw struct {
		Type string `json:"type"`
		literalExpr
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeLiteral); err != nil {
		return err
	}
	*e = LiteralExpr(raw.literalExpr)
	return nil
}

func (e ColumnRefExpr) MarshalJSON() ([]byte, error) {
	type columnRefExpr ColumnRefExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		columnRefExpr
	}{Type: exprTypeColumnRef, columnRefExpr: columnRefExpr(e)})
}

func (e *ColumnRefExpr) UnmarshalJSON(data []byte) error {
	type columnRefExpr ColumnRefExpr
	var raw struct {
		Type string `json:"type"`
		columnRefExpr
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeColumnRef); err != nil {
		return err
	}
	*e = ColumnRefExpr(raw.columnRefExpr)
	return nil
}

func (e MetricRefExpr) MarshalJSON() ([]byte, error) {
	type metricRefExpr MetricRefExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		metricRefExpr
	}{Type: exprTypeMetricRef, metricRefExpr: metricRefExpr(e)})
}

func (e *MetricRefExpr) UnmarshalJSON(data []byte) error {
	type metricRefExpr MetricRefExpr
	var raw struct {
		Type string `json:"type"`
		metricRefExpr
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeMetricRef); err != nil {
		return err
	}
	*e = MetricRefExpr(raw.metricRefExpr)
	return nil
}

func (e DimensionRefExpr) MarshalJSON() ([]byte, error) {
	type dimensionRefExpr DimensionRefExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		dimensionRefExpr
	}{Type: exprTypeDimensionRef, dimensionRefExpr: dimensionRefExpr(e)})
}

func (e *DimensionRefExpr) UnmarshalJSON(data []byte) error {
	type dimensionRefExpr DimensionRefExpr
	var raw struct {
		Type string `json:"type"`
		dimensionRefExpr
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeDimensionRef); err != nil {
		return err
	}
	*e = DimensionRefExpr(raw.dimensionRefExpr)
	return nil
}

func (e BinaryExpr) MarshalJSON() ([]byte, error) {
	type binaryExpr BinaryExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		binaryExpr
	}{Type: exprTypeBinary, binaryExpr: binaryExpr(e)})
}

func (e *BinaryExpr) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type  string          `json:"type"`
		Op    BinaryOp        `json:"op"`
		Left  json.RawMessage `json:"left"`
		Right json.RawMessage `json:"right"`
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeBinary); err != nil {
		return err
	}
	left, err := UnmarshalExprNode(raw.Left)
	if err != nil {
		return fmt.Errorf("unmarshal binary left: %w", err)
	}
	right, err := UnmarshalExprNode(raw.Right)
	if err != nil {
		return fmt.Errorf("unmarshal binary right: %w", err)
	}
	*e = BinaryExpr{Op: raw.Op, Left: left, Right: right}
	return nil
}

func (e UnaryExpr) MarshalJSON() ([]byte, error) {
	type unaryExpr UnaryExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		unaryExpr
	}{Type: exprTypeUnary, unaryExpr: unaryExpr(e)})
}

func (e *UnaryExpr) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string          `json:"type"`
		Op   UnaryOp         `json:"op"`
		Expr json.RawMessage `json:"expr"`
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeUnary); err != nil {
		return err
	}
	expr, err := UnmarshalExprNode(raw.Expr)
	if err != nil {
		return fmt.Errorf("unmarshal unary expr: %w", err)
	}
	*e = UnaryExpr{Op: raw.Op, Expr: expr}
	return nil
}

func (e FunctionCallExpr) MarshalJSON() ([]byte, error) {
	type functionCallExpr FunctionCallExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		functionCallExpr
	}{Type: exprTypeFunctionCall, functionCallExpr: functionCallExpr(e)})
}

func (e *FunctionCallExpr) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type string            `json:"type"`
		Name string            `json:"name"`
		Args []json.RawMessage `json:"args"`
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeFunctionCall); err != nil {
		return err
	}
	args := make([]ExprNode, 0, len(raw.Args))
	for i, rawArg := range raw.Args {
		arg, err := UnmarshalExprNode(rawArg)
		if err != nil {
			return fmt.Errorf("unmarshal function arg %d: %w", i, err)
		}
		args = append(args, arg)
	}
	*e = FunctionCallExpr{Name: raw.Name, Args: args}
	return nil
}

func (e CaseExpr) MarshalJSON() ([]byte, error) {
	type caseExpr CaseExpr
	return sonic.ConfigStd.Marshal(struct {
		Type string `json:"type"`
		caseExpr
	}{Type: exprTypeCase, caseExpr: caseExpr(e)})
}

func (e *CaseExpr) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type       string          `json:"type"`
		Conditions []caseWhenJSON  `json:"conditions"`
		ElseExpr   json.RawMessage `json:"else"`
	}
	if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := requireExprType(raw.Type, exprTypeCase); err != nil {
		return err
	}
	conditions := make([]CaseWhen, 0, len(raw.Conditions))
	for i, rawCondition := range raw.Conditions {
		condition, err := rawCondition.toCaseWhen()
		if err != nil {
			return fmt.Errorf("unmarshal case condition %d: %w", i, err)
		}
		conditions = append(conditions, condition)
	}
	var elseExpr ExprNode
	if len(raw.ElseExpr) > 0 && string(raw.ElseExpr) != "null" {
		var err error
		elseExpr, err = UnmarshalExprNode(raw.ElseExpr)
		if err != nil {
			return fmt.Errorf("unmarshal case else: %w", err)
		}
	}
	*e = CaseExpr{Conditions: conditions, ElseExpr: elseExpr}
	return nil
}

type caseWhenJSON struct {
	When json.RawMessage `json:"when"`
	Then json.RawMessage `json:"then"`
}

func (c caseWhenJSON) toCaseWhen() (CaseWhen, error) {
	when, err := UnmarshalExprNode(c.When)
	if err != nil {
		return CaseWhen{}, fmt.Errorf("unmarshal when: %w", err)
	}
	then, err := UnmarshalExprNode(c.Then)
	if err != nil {
		return CaseWhen{}, fmt.Errorf("unmarshal then: %w", err)
	}
	return CaseWhen{When: when, Then: then}, nil
}

func requireExprType(got, want string) error {
	if got != want {
		return fmt.Errorf("expression node type %q, want %q", got, want)
	}
	return nil
}

// UnmarshalExprNode decodes a JSON expression node using its type discriminator.
func UnmarshalExprNode(data []byte) (ExprNode, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, errors.New("expression node is empty")
	}
	var header struct {
		Type string `json:"type"`
	}
	if err := sonic.ConfigStd.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	switch header.Type {
	case exprTypeLiteral:
		var expr LiteralExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	case exprTypeColumnRef:
		var expr ColumnRefExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	case exprTypeMetricRef:
		var expr MetricRefExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	case exprTypeDimensionRef:
		var expr DimensionRefExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	case exprTypeBinary:
		var expr BinaryExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	case exprTypeUnary:
		var expr UnaryExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	case exprTypeFunctionCall:
		var expr FunctionCallExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	case exprTypeCase:
		var expr CaseExpr
		if err := sonic.ConfigStd.Unmarshal(data, &expr); err != nil {
			return nil, err
		}
		return expr, nil
	default:
		return nil, fmt.Errorf("unknown expression node type %q", header.Type)
	}
}
