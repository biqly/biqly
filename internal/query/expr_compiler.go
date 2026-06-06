package query

import (
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

// CompileExpr emits safe SQL from a canonical semantic expression AST.
func CompileExpr(expr pkgsemantic.ExprNode, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig) (string, error) {
	d = normalizeExprDialect(d)
	sql, err := compileExpr(expr, d, resolver, args, piiConfig)
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

func normalizeExprDialect(d dialect.Dialect) dialect.Dialect {
	switch concrete := d.(type) {
	case dialect.PostgresDialect:
		if concrete.QuoteLeft == "" {
			return dialect.Postgres
		}
	case dialect.MySQLDialect:
		if concrete.QuoteLeft == "" {
			return dialect.MySQL
		}
	case dialect.SQLServerDialect:
		if concrete.QuoteLeft == "" {
			return dialect.SQLServer
		}
	case dialect.ClickHouseDialect:
		if concrete.QuoteLeft == "" {
			return dialect.ClickHouse
		}
	}
	return d
}

func compileExpr(expr pkgsemantic.ExprNode, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig) (string, error) {
	switch e := expr.(type) {
	case nil:
		return "", nil
	case pkgsemantic.LiteralExpr:
		if args != nil {
			*args = append(*args, e.Value)
			return d.Placeholder(len(*args)), nil
		}
		return literalSQL(e.Value), nil
	case pkgsemantic.ColumnRefExpr:
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
	case pkgsemantic.MetricRefExpr:
		return d.QuoteIdent(e.Name), nil
	case pkgsemantic.DimensionRefExpr:
		return d.QuoteIdent(e.Name), nil
	case pkgsemantic.BinaryExpr:
		return binaryExprSQL(e, d, resolver, args, piiConfig)
	case pkgsemantic.UnaryExpr:
		return unaryExprSQL(e, d, resolver, args, piiConfig)
	case pkgsemantic.FunctionCallExpr:
		return functionCallSQL(e, d, resolver, args, piiConfig)
	case pkgsemantic.CaseExpr:
		return caseExprSQL(e, d, resolver, args, piiConfig)
	default:
		return "", nil
	}
}

func literalSQL(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(v, "'", "''") + "'"
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
		return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
	}
}

func binaryExprSQL(expr pkgsemantic.BinaryExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig) (string, error) {
	left, err := compileExpr(expr.Left, d, resolver, args, piiConfig)
	if err != nil {
		return "", err
	}
	right, err := compileExpr(expr.Right, d, resolver, args, piiConfig)
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
	return "(" + left + " " + binaryOpSQL(expr.Op) + " " + right + ")", nil
}

func unaryExprSQL(expr pkgsemantic.UnaryExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig) (string, error) {
	inner, err := compileExpr(expr.Expr, d, resolver, args, piiConfig)
	if err != nil {
		return "", err
	}
	switch expr.Op {
	case pkgsemantic.OpNot:
		return "(NOT " + inner + ")", nil
	case pkgsemantic.OpNegate:
		return "(-" + inner + ")", nil
	default:
		return "(" + strings.ToUpper(string(expr.Op)) + " " + inner + ")", nil
	}
}

func functionCallSQL(expr pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig) (string, error) {
	if sql, ok := dateTruncSQL(expr, d, resolver, piiConfig); ok {
		return sql, nil
	}

	funcArgs := make([]string, 0, len(expr.Args))
	for _, arg := range expr.Args {
		compiled, err := compileExpr(arg, d, resolver, args, piiConfig)
		if err != nil {
			return "", err
		}
		funcArgs = append(funcArgs, compiled)
	}
	name := functionNameSQL(expr.Name, d)
	return name + "(" + strings.Join(funcArgs, ", ") + ")", nil
}

func dateTruncSQL(expr pkgsemantic.FunctionCallExpr, d dialect.Dialect, resolver *SchemaResolver, piiConfig *PIIMaskingConfig) (string, bool) {
	if !strings.EqualFold(expr.Name, "DATE_TRUNC") || len(expr.Args) != 2 {
		return "", false
	}

	part, ok := literalString(expr.Args[0])
	if !ok {
		return "", false
	}

	ref, ok := expr.Args[1].(pkgsemantic.ColumnRefExpr)
	if !ok {
		return "", false
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
			return pii.HiddenLiteral, true
		}
	}

	return d.DateTrunc(part, physicalPath), true
}

func literalString(expr pkgsemantic.ExprNode) (string, bool) {
	lit, ok := expr.(pkgsemantic.LiteralExpr)
	if !ok {
		return "", false
	}
	value, ok := lit.Value.(string)
	return value, ok
}

func caseExprSQL(expr pkgsemantic.CaseExpr, d dialect.Dialect, resolver *SchemaResolver, args *[]any, piiConfig *PIIMaskingConfig) (string, error) {
	chunks := make([]string, 0, 2+len(expr.Conditions)*4)
	chunks = append(chunks, "CASE")
	for _, condition := range expr.Conditions {
		compiledWhen, err := compileExpr(condition.When, d, resolver, args, piiConfig)
		if err != nil {
			return "", err
		}
		compiledThen, err := compileExpr(condition.Then, d, resolver, args, piiConfig)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, " WHEN ", compiledWhen, " THEN ", compiledThen)
	}
	if expr.ElseExpr != nil {
		compiledElse, err := compileExpr(expr.ElseExpr, d, resolver, args, piiConfig)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, " ELSE ", compiledElse)
	}
	chunks = append(chunks, " END")
	return strings.Join(chunks, ""), nil
}

func binaryOpSQL(op pkgsemantic.BinaryOp) string {
	switch op {
	case pkgsemantic.OpAdd:
		return "+"
	case pkgsemantic.OpSubtract:
		return "-"
	case pkgsemantic.OpMultiply:
		return "*"
	case pkgsemantic.OpDivide:
		return "/"
	case pkgsemantic.OpModulo:
		return "%"
	case pkgsemantic.OpEq:
		return "="
	case pkgsemantic.OpNeq:
		return "<>"
	case pkgsemantic.OpLt:
		return "<"
	case pkgsemantic.OpLte:
		return "<="
	case pkgsemantic.OpGt:
		return ">"
	case pkgsemantic.OpGte:
		return ">="
	case pkgsemantic.OpAnd:
		return "AND"
	case pkgsemantic.OpOr:
		return "OR"
	case pkgsemantic.OpConcat:
		return "||"
	default:
		return strings.ToUpper(string(op))
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
		return pii.HiddenLiteral, true
	}
	if access == pii.AccessMasked {
		colSQL := d.QuoteIdent(physicalPath)
		return piiConfig.strategy().MaskExpression(colSQL, piiType, d), true
	}
	return "", false
}
