package query

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/security/pii"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

var dialectFunctions = map[string]map[string]string{
	"COALESCE": {
		"clickhouse": "coalesce",
	},
	"CONCAT": {
		"clickhouse": "concat",
	},
	"ROUND": {
		"clickhouse": "round",
	},
	"NULLIF": {
		"clickhouse": "nullif",
	},
	"SUBSTRING": {
		"clickhouse": "substring",
		"oracle":     "SUBSTR",
	},
	"REPLACE": {
		"clickhouse": "replace",
	},
	"DATE_TRUNC": {
		"postgres":   "DATE_TRUNC",
		"mysql":      "DATE_FORMAT",
		"sqlserver":  "DATETRUNC",
		"clickhouse": "toStartOfInterval",
	},
}

var exprReadOnlyChecker = security.NewReadOnlyChecker()

// ExprCompileOptions configures semantic expression compilation.
type ExprCompileOptions struct {
	AllowAggregates bool
}

// ExprCompileOptsForMetric returns compile options for a metric expression AST.
// Custom metrics may embed aggregate calls (SUM, COUNT, etc.) in the expression.
func ExprCompileOptsForMetric(metric *pkgsemantic.Metric) ExprCompileOptions {
	if metric != nil && strings.EqualFold(strings.TrimSpace(metric.Aggregation), "custom") {
		return ExprCompileOptions{AllowAggregates: true}
	}
	return ExprCompileOptions{}
}

// CompileExpr emits safe SQL from a canonical semantic expression AST.
func CompileExpr(expr pkgsemantic.ExprNode, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opts ...ExprCompileOptions) (string, error) {
	var opt ExprCompileOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	d = normalizeExprDialect(d)
	sql, err := compileExpr(expr, d, resolver, args, piiConfig, opt)
	if err != nil {
		return "", err
	}
	if sql != "" {
		if err := exprReadOnlyChecker.Check("SELECT " + sql); err != nil {
			slog.Error("expression compiled to unsafe SQL, aborting compilation", "error", err)
			return "", err
		}
	}
	return sql, nil
}

// normalizeExprDialect delegates to dialect.Normalize (kept as a local alias so
// existing call sites and tests are unchanged).
func normalizeExprDialect(d dialect.Dialect) dialect.Dialect {
	return dialect.Normalize(d)
}

func compileExpr(expr pkgsemantic.ExprNode, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	switch e := expr.(type) {
	case nil:
		return "", nil
	case *pkgsemantic.LiteralExpr:
		if args != nil {
			*args = append(*args, e.Value)
			return d.Placeholder(len(*args)), nil
		}
		return literalSQL(e.Value, d), nil
	case *pkgsemantic.ColumnRefExpr:
		colPath := e.Column
		if e.Table != "" {
			colPath = e.Table + "." + e.Column
		}
		var physicalPath string
		if resolver != nil {
			physicalPath = resolver.PhysicalColumnRef(colPath)
		} else {
			physicalPath = colPath
		}
		if piiConfig != nil {
			if masked, ok := maskColumnRef(colPath, physicalPath, d, piiConfig); ok {
				return masked, nil
			}
		}
		return d.QuoteIdent(physicalPath), nil
	case *pkgsemantic.MetricRefExpr:
		return d.QuoteIdent(e.Name), nil
	case *pkgsemantic.DimensionRefExpr:
		return d.QuoteIdent(e.Name), nil
	case *pkgsemantic.BinaryExpr:
		return binaryExprSQL(e, d, resolver, args, piiConfig, opt)
	case *pkgsemantic.UnaryExpr:
		return unaryExprSQL(e, d, resolver, args, piiConfig, opt)
	case *pkgsemantic.FunctionCallExpr:
		return functionCallSQL(e, d, resolver, args, piiConfig, opt)
	case *pkgsemantic.CaseExpr:
		return caseExprSQL(e, d, resolver, args, piiConfig, opt)
	default:
		return "", nil
	}
}

func literalSQL(value any, d dialect.Dialect) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return d.QuoteStringLiteral(v)
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return d.QuoteStringLiteral(fmt.Sprint(v))
	}
}

func binaryExprSQL(expr *pkgsemantic.BinaryExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	left, err := compileExpr(expr.Left, d, resolver, args, piiConfig, opt)
	if err != nil {
		return "", err
	}
	right, err := compileExpr(expr.Right, d, resolver, args, piiConfig, opt)
	if err != nil {
		return "", err
	}
	if expr.Op == pkgsemantic.OpConcat {
		switch d.Name() {
		case "mysql":
			return "CONCAT(" + left + ", " + right + ")", nil
		case "sqlserver":
			return "(" + left + " + " + right + ")", nil
		case "clickhouse":
			return "concat(" + left + ", " + right + ")", nil
		default:
			return "(" + left + " || " + right + ")", nil
		}
	}
	if expr.Op == pkgsemantic.OpDivide {
		// Multiply the dividend by a float literal before dividing so integer
		// operands (COUNT, integer columns) do not truncate to an integer
		// result — e.g. 5 / 2 must yield 2.5, not 2. Zero-guarding the divisor
		// stays the expression author's responsibility (wrap the right operand
		// in NULLIF(x, 0) when needed), matching existing derived-metric usage.
		return "(" + left + " * 1.0 / " + right + ")", nil
	}
	opSQL, err := binaryOpSQL(expr.Op)
	if err != nil {
		return "", err
	}
	return "(" + left + " " + opSQL + " " + right + ")", nil
}

func unaryExprSQL(expr *pkgsemantic.UnaryExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	inner, err := compileExpr(expr.Expr, d, resolver, args, piiConfig, opt)
	if err != nil {
		return "", err
	}
	switch expr.Op {
	case pkgsemantic.OpNot:
		return "(NOT " + inner + ")", nil
	case pkgsemantic.OpNegate:
		return "(-" + inner + ")", nil
	default:
		// Fail closed: the operator is emitted verbatim into SQL, so an unknown
		// request-supplied op is an injection sink.
		return "", fmt.Errorf("disallowed unary operator in expression: %s", expr.Op)
	}
}

func functionCallSQL(expr *pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	if sql, ok, err := dateTruncSQL(expr, d, resolver, piiConfig); ok || err != nil {
		if err != nil {
			return "", err
		}
		return sql, nil
	}

	funcName := strings.ToUpper(expr.Name)
	if pkgsemantic.IsAggregateFunction(funcName) {
		if !opt.AllowAggregates {
			return "", fmt.Errorf("disallowed function in expression: %s", expr.Name)
		}
		return aggregateFunctionCallSQL(expr, d, resolver, args, piiConfig, opt)
	}

	// Fail closed on any function not in the shared whitelist. The name is
	// emitted verbatim into SQL below, so an unchecked name (e.g. from a
	// request-supplied window expression) is a direct SQL-injection sink.
	if !pkgsemantic.FunctionAllowed(funcName, false) {
		return "", fmt.Errorf("disallowed function in expression: %s", expr.Name)
	}

	funcArgs := make([]string, 0, len(expr.Args))
	for _, arg := range expr.Args {
		compiled, err := compileExpr(arg, d, resolver, args, piiConfig, opt)
		if err != nil {
			return "", err
		}
		funcArgs = append(funcArgs, compiled)
	}
	name := functionNameSQL(expr.Name, d)
	return name + "(" + strings.Join(funcArgs, ", ") + ")", nil
}

func aggregateCountStarSQL(d dialect.Dialect) string {
	if d.Name() == "clickhouse" {
		return "count()"
	}
	return "COUNT(*)"
}

func isCountStarArg(arg pkgsemantic.ExprNode) bool {
	lit, ok := arg.(*pkgsemantic.LiteralExpr)
	if !ok {
		return false
	}
	s, ok := lit.Value.(string)
	return ok && s == "*"
}

func compileAggregateCountDistinctSQL(expr *pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	if len(expr.Args) != 1 {
		return "", fmt.Errorf("function COUNT_DISTINCT requires exactly 1 argument, got %d", len(expr.Args))
	}
	inner, err := compileExpr(expr.Args[0], d, resolver, args, piiConfig, opt)
	if err != nil {
		return "", err
	}
	return dialectAggregateSQL("count_distinct", inner, d), nil
}

func compileAggregateCountSQL(expr *pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	switch len(expr.Args) {
	case 0:
		return aggregateCountStarSQL(d), nil
	case 1:
		if isCountStarArg(expr.Args[0]) {
			return aggregateCountStarSQL(d), nil
		}
		inner, err := compileExpr(expr.Args[0], d, resolver, args, piiConfig, opt)
		if err != nil {
			return "", err
		}
		return dialectAggregateSQL("count", inner, d), nil
	default:
		return "", fmt.Errorf("function COUNT requires 0 or 1 arguments, got %d", len(expr.Args))
	}
}

func aggregateFunctionCallSQL(expr *pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	funcName := strings.ToUpper(expr.Name)
	switch funcName {
	case "COUNT_DISTINCT":
		return compileAggregateCountDistinctSQL(expr, d, resolver, args, piiConfig, opt)
	case "COUNT":
		return compileAggregateCountSQL(expr, d, resolver, args, piiConfig, opt)
	default:
		if len(expr.Args) != 1 {
			return "", fmt.Errorf("function %s requires exactly 1 argument, got %d", expr.Name, len(expr.Args))
		}
		inner, err := compileExpr(expr.Args[0], d, resolver, args, piiConfig, opt)
		if err != nil {
			return "", err
		}
		return dialectAggregateSQL(strings.ToLower(funcName), inner, d), nil
	}
}

func dialectAggregateSQL(fn, expr string, d dialect.Dialect) string {
	switch fn {
	case "count":
		if expr == "*" {
			if d.Name() == "clickhouse" {
				return "count()"
			}
			return "COUNT(*)"
		}
		if d.Name() == "clickhouse" {
			return "count(" + expr + ")"
		}
		return "COUNT(" + expr + ")"
	case "count_distinct":
		if d.Name() == "clickhouse" {
			return "uniq(" + expr + ")"
		}
		return "COUNT(DISTINCT " + expr + ")"
	case "sum":
		if d.Name() == "clickhouse" {
			return "sum(" + expr + ")"
		}
		return "SUM(" + expr + ")"
	case "avg":
		if d.Name() == "clickhouse" {
			return "avg(" + expr + ")"
		}
		return "AVG(" + expr + ")"
	case "min":
		if d.Name() == "clickhouse" {
			return "min(" + expr + ")"
		}
		return "MIN(" + expr + ")"
	case "max":
		if d.Name() == "clickhouse" {
			return "max(" + expr + ")"
		}
		return "MAX(" + expr + ")"
	default:
		return d.Aggregate(fn, expr)
	}
}

func dateTruncSQL(expr *pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver, piiConfig *PIIMaskingConfig) (string, bool, error) {
	if !strings.EqualFold(expr.Name, "DATE_TRUNC") {
		return "", false, nil
	}
	if len(expr.Args) != 2 {
		return "", true, errors.New("DATE_TRUNC requires grain and column arguments")
	}

	part, ok := literalString(expr.Args[0])
	if !ok {
		return "", true, errors.New("DATE_TRUNC grain must be a string literal")
	}
	part, ok = normalizeDateGrain(part)
	if !ok {
		return "", true, fmt.Errorf("unsupported DATE_TRUNC grain: %s", part)
	}

	ref, ok := expr.Args[1].(*pkgsemantic.ColumnRefExpr)
	if !ok {
		return "", true, errors.New("DATE_TRUNC column argument must be a column reference")
	}

	colPath := ref.Column
	if ref.Table != "" {
		colPath = ref.Table + "." + ref.Column
	}
	var physicalPath string
	if resolver != nil {
		physicalPath = resolver.PhysicalColumnRef(colPath)
	} else {
		physicalPath = colPath
	}

	if piiConfig != nil {
		access, _, found := piiConfig.lookup(colPath, physicalPath)
		if found && access != pii.AccessRaw && access != pii.AccessMasked {
			return pii.HiddenLiteral, true, nil
		}
	}

	return d.DateTrunc(part, physicalPath), true, nil
}

func normalizeDateGrain(part string) (string, bool) {
	part = strings.ToLower(strings.TrimSpace(part))
	switch part {
	case "hour", "day", "week", "month", "quarter", "year":
		return part, true
	default:
		return part, false
	}
}

func literalString(expr pkgsemantic.ExprNode) (string, bool) {
	lit, ok := expr.(*pkgsemantic.LiteralExpr)
	if !ok {
		return "", false
	}
	value, ok := lit.Value.(string)
	return value, ok
}

func caseExprSQL(expr *pkgsemantic.CaseExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig, opt ExprCompileOptions) (string, error) {
	chunks := make([]string, 0, 2+len(expr.Conditions)*4)
	chunks = append(chunks, "CASE")
	for _, condition := range expr.Conditions {
		compiledWhen, err := compileExpr(condition.When, d, resolver, args, piiConfig, opt)
		if err != nil {
			return "", err
		}
		compiledThen, err := compileExpr(condition.Then, d, resolver, args, piiConfig, opt)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, " WHEN ", compiledWhen, " THEN ", compiledThen)
	}
	if expr.ElseExpr != nil {
		compiledElse, err := compileExpr(expr.ElseExpr, d, resolver, args, piiConfig, opt)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, " ELSE ", compiledElse)
	}
	chunks = append(chunks, " END")
	return strings.Join(chunks, ""), nil
}

func binaryOpSQL(op pkgsemantic.BinaryOp) (string, error) {
	switch op {
	case pkgsemantic.OpAdd:
		return "+", nil
	case pkgsemantic.OpSubtract:
		return "-", nil
	case pkgsemantic.OpMultiply:
		return "*", nil
	case pkgsemantic.OpDivide:
		return "/", nil
	case pkgsemantic.OpModulo:
		return "%", nil
	case pkgsemantic.OpEq:
		return "=", nil
	case pkgsemantic.OpNeq:
		return "<>", nil
	case pkgsemantic.OpLt:
		return "<", nil
	case pkgsemantic.OpLte:
		return "<=", nil
	case pkgsemantic.OpGt:
		return ">", nil
	case pkgsemantic.OpGte:
		return ">=", nil
	case pkgsemantic.OpAnd:
		return "AND", nil
	case pkgsemantic.OpOr:
		return "OR", nil
	case pkgsemantic.OpConcat:
		return "||", nil
	default:
		return "", fmt.Errorf("unsupported binary operator %q", op)
	}
}

func functionNameSQL(name string, d dialect.Dialect) string {
	upper := strings.ToUpper(name)
	if byDialect, ok := dialectFunctions[upper]; ok {
		if mapped := byDialect[d.Name()]; mapped != "" {
			return mapped
		}
	}
	return upper
}

func maskColumnRef(colPath, physicalPath string, d dialect.Dialect, piiConfig *PIIMaskingConfig) (string, bool) {
	if piiConfig == nil {
		return "", false
	}
	access, piiType, found := piiConfig.lookup(colPath, physicalPath)
	if !found {
		return "", false
	}
	if access != pii.AccessRaw && access != pii.AccessMasked {
		piiConfig.Applied.recordHidden(physicalPath)
		return pii.HiddenLiteral, true
	}
	if access == pii.AccessMasked {
		piiConfig.Applied.recordMasked(physicalPath)
		colSQL := d.QuoteIdent(physicalPath)
		return piiConfig.strategy().MaskExpression(colSQL, piiType, d), true
	}
	return "", false
}
