package semantic

import (
	"context"
	"errors"
	"strings"

	"github.com/biqly/biqly/internal/errmsg"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

type validationCatalog struct {
	columns   datasourceColumnSet
	relations []CatalogRelation
	policies  []CatalogPolicy
}

func validateModelRequiredFields(model SemanticModel, addError func(string, ...any)) {
	if strings.TrimSpace(model.Name) == "" {
		addError("model name is required")
	}
	if strings.TrimSpace(model.BaseTable) == "" {
		addError("base table is required")
	}
	if len(model.Dimensions) == 0 {
		addError("model has no active dimensions; AI cannot generate queries without at least one dimension")
	}
}

func loadValidationCatalog(
	ctx context.Context,
	model SemanticModel,
	catalog CatalogReader,
	addError func(string, ...any),
) (validationCatalog, bool) {
	var vc validationCatalog

	columns, err := catalog.ListSemanticColumns(ctx, model.DatasourceID)
	if err != nil {
		addError("load datasource columns: %s", err)
		return vc, false
	}
	relations, err := catalog.ListSemanticRelations(ctx, model.DatasourceID)
	if err != nil {
		addError("load datasource relations: %s", err)
		return vc, false
	}
	policies, err := catalog.ListSemanticPolicies(ctx, model.DatasourceID)
	if err != nil {
		addError("load permission policies: %s", err)
		return vc, false
	}

	vc.columns = buildColumnSet(columns)
	vc.relations = relations
	vc.policies = policies
	return vc, true
}

func allowedFieldMaps(model SemanticModel) (map[string]bool, map[string]bool) {
	allowedDims := make(map[string]bool, len(model.Dimensions))
	for _, d := range model.Dimensions {
		allowedDims[strings.ToLower(d.Name)] = true
	}
	allowedMets := make(map[string]bool, len(model.Metrics))
	for _, m := range model.Metrics {
		allowedMets[strings.ToLower(m.Name)] = true
	}
	return allowedDims, allowedMets
}

func validateContextDimensions(
	model SemanticModel,
	columnSet datasourceColumnSet,
	allowedDims, allowedMets map[string]bool,
	addError func(string, ...any),
) {
	for _, dim := range model.Dimensions {
		if dim.CalculatedExpression != "" {
			validateCalculatedDimension(model, dim, columnSet, allowedDims, allowedMets, addError)
			continue
		}
		if !columnSet.has(model.BaseSchema, dim.ColumnRef) {
			addError("%s: %s", errmsg.DimensionUnknownColumn, dim.ColumnRef)
		}
	}
}

func validateCalculatedDimension(
	model SemanticModel,
	dim Dimension,
	columnSet datasourceColumnSet,
	allowedDims, allowedMets map[string]bool,
	addError func(string, ...any),
) {
	if err := validateCalculatedExpression(dim.CalculatedExpression, columnSet, model.BaseSchema); err != nil {
		addError("dimension %q calculated expression invalid: %s", dim.Name, err)
	}
	expr, err := getOrParseExpr(dim.CalculatedExpression, dim.CalculatedExpr)
	if errors.Is(err, errNoExpr) {
		return
	}
	if err != nil {
		addError("dimension %q calculated expression parse error: %s", dim.Name, err)
		return
	}
	if err := pkgsemantic.ValidateExprStrict(expr, columnSet, allowedMets, allowedDims, false, 0); err != nil {
		addError("dimension %q calculated expression invalid: %s", dim.Name, err)
	}
}

func validateContextMetrics(
	model SemanticModel,
	columnSet datasourceColumnSet,
	allowedDims, allowedMets map[string]bool,
	addError func(string, ...any),
) {
	for _, metric := range model.Metrics {
		validateMetricColumns(model, metric, columnSet, addError)
		validateMetricExpressionAST(metric, columnSet, allowedDims, allowedMets, addError)
	}
}

func validateMetricColumns(model SemanticModel, metric Metric, columnSet datasourceColumnSet, addError func(string, ...any)) {
	fnLower := strings.ToLower(strings.TrimSpace(metric.Aggregation))
	if fnLower == "custom" || strings.Contains(metric.Expression, "[") {
		validateCustomMetricExpression(model, metric, columnSet, addError)
		return
	}
	for _, ref := range columnRefsInExpression(metric.Expression) {
		if !columnSet.has(model.BaseSchema, ref) {
			addError("%s: %s", errmsg.MetricExpressionUnknownColumn, ref)
		}
	}
}

func validateCustomMetricExpression(model SemanticModel, metric Metric, columnSet datasourceColumnSet, addError func(string, ...any)) {
	matches := reBracket.FindAllStringSubmatch(metric.Expression, -1)
	for _, match := range matches {
		token := strings.TrimSpace(match[1])
		if isModelFieldName(model, token) {
			continue
		}
		ref := token
		if !strings.Contains(token, ".") {
			ref = model.BaseTable + "." + token
		}
		if !columnSet.has(model.BaseSchema, ref) {
			addError("%s: %s", errmsg.MetricExpressionUnknownColumn, token)
		}
	}
}

func isModelFieldName(model SemanticModel, token string) bool {
	for _, d := range model.Dimensions {
		if strings.EqualFold(d.Name, token) {
			return true
		}
	}
	for _, m := range model.Metrics {
		if strings.EqualFold(m.Name, token) {
			return true
		}
	}
	return false
}

func validateMetricExpressionAST(
	metric Metric,
	columnSet datasourceColumnSet,
	allowedDims, allowedMets map[string]bool,
	addError func(string, ...any),
) {
	expr, err := getOrParseExpr(metric.Expression, metric.Expr)
	if errors.Is(err, errNoExpr) {
		return
	}
	if err != nil {
		addError("metric %q expression parse error: %s", metric.Name, err)
		return
	}
	allowMets := strings.ToLower(strings.TrimSpace(metric.Aggregation)) == "custom"
	if err := pkgsemantic.ValidateExprStrict(expr, columnSet, allowedMets, allowedDims, allowMets, 0); err != nil {
		addError("metric %q expression invalid: %s", metric.Name, err)
	}
}

func validateContextJoins(
	model SemanticModel,
	columnSet datasourceColumnSet,
	relations []CatalogRelation,
	addError, addWarning func(string, ...any),
) {
	for _, join := range model.Joins {
		if !columnSet.hasTableColumn(model.BaseSchema, join.FromTable, join.FromColumn) {
			addError("%s: %s.%s", errmsg.JoinUnknownFromColumn, join.FromTable, join.FromColumn)
		}
		if !columnSet.hasTableColumn(model.BaseSchema, join.ToTable, join.ToColumn) {
			addError("%s: %s.%s", errmsg.JoinUnknownToColumn, join.ToTable, join.ToColumn)
		}
		if !relationExists(model.BaseSchema, join, relations) {
			addWarning("join does not match datasource metadata relation: %s (manual join)", join.Name)
		}
		validateJoinRelationship(join, addError, addWarning)
	}
}

func validateJoinRelationship(join Join, addError, addWarning func(string, ...any)) {
	switch strings.ToLower(strings.TrimSpace(join.Relationship)) {
	case "", RelationshipOneToOne, RelationshipManyToOne:
	case RelationshipOneToMany, RelationshipManyToMany:
		addWarning("join can fan out aggregations: %s uses %s", join.Name, join.Relationship)
	default:
		addError("join has invalid relationship type: %s", join.Relationship)
	}
}

func appendContextBudgetWarnings(model SemanticModel, result *PublishValidationResult, addWarning func(string, ...any)) {
	if result.EstimatedPromptSize > 60000 {
		addWarning("semantic context prompt estimate is high: %d runes", result.EstimatedPromptSize)
	}
	for _, msg := range EnforceBudget(model, DefaultContextBudget(), result.EstimatedPromptSize) {
		addWarning("%s", msg)
	}
}
