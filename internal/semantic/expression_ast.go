package semantic

import (
	"log/slog"
	"strings"
	"sync"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
	"github.com/bytedance/sonic"
)

var expressionParserRegistry struct {
	mu     sync.RWMutex
	parser func(expr string) (pkgsemantic.ExprNode, error)
}

// RegisterExpressionParser registers the parser used to hydrate semantic expression ASTs.
func RegisterExpressionParser(parser func(expr string) (pkgsemantic.ExprNode, error)) {
	expressionParserRegistry.mu.Lock()
	defer expressionParserRegistry.mu.Unlock()
	expressionParserRegistry.parser = parser
}

// CurrentExpressionParser returns the registered semantic expression parser.
func CurrentExpressionParser() func(expr string) (pkgsemantic.ExprNode, error) {
	expressionParserRegistry.mu.RLock()
	defer expressionParserRegistry.mu.RUnlock()
	return expressionParserRegistry.parser
}

func hydrateExpressionASTs(model *SemanticModel) {
	parser := CurrentExpressionParser()
	if model == nil || parser == nil {
		return
	}
	for i := range model.Dimensions {
		if model.Dimensions[i].CalculatedExpr != nil {
			continue
		}
		expr := strings.TrimSpace(model.Dimensions[i].CalculatedExpression)
		if expr == "" {
			continue
		}
		parsed, err := parser(expr)
		if err != nil {
			slog.Warn("semantic dimension calculated expression parse failed", "dimension", model.Dimensions[i].Name, "error", err)
			continue
		}
		model.Dimensions[i].CalculatedExpr = parsed
	}
	for i := range model.Metrics {
		if model.Metrics[i].Expr != nil {
			continue
		}
		expr := strings.TrimSpace(model.Metrics[i].Expression)
		if expr == "" || expr == "*" {
			continue
		}
		parsed, err := parser(expr)
		if err != nil {
			slog.Warn("semantic metric expression parse failed", "metric", model.Metrics[i].Name, "error", err)
			continue
		}
		model.Metrics[i].Expr = parsed
	}
}

func encodeExprNodeJSON(expr pkgsemantic.ExprNode) ([]byte, error) {
	if expr == nil {
		return nil, nil
	}
	return sonic.Marshal(expr)
}

func decodeExprNodeJSON(raw []byte) pkgsemantic.ExprNode {
	if len(raw) == 0 || strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return nil
	}
	expr, err := pkgsemantic.UnmarshalExprNode(raw)
	if err != nil {
		slog.Warn("semantic expression AST JSON decode failed", "error", err)
		return nil
	}
	return expr
}
