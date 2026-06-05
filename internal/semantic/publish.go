package semantic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/biqly/biqly/internal/errmsg"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

const (
	ModelStatusDraft     = "draft"
	ModelStatusPublished = "published"
)

// CalculatedExpressionValidator is registered by internal/query to validate calculated expressions.
var CalculatedExpressionValidator func(expr string) error

// OnModelPublish is registered by external packages to perform actions (like cache invalidation) on model publish/rollback.
var OnModelPublish func(ctx context.Context, modelID string)

var reBracket = regexp.MustCompile(`\[([^\]]+)\]`)

// CatalogReader supplies datasource metadata needed to validate a semantic
// context before it is published to query runtime.
type CatalogReader interface {
	ListSemanticColumns(ctx context.Context, datasourceID string) ([]CatalogColumn, error)
	ListSemanticRelations(ctx context.Context, datasourceID string) ([]CatalogRelation, error)
	ListSemanticPolicies(ctx context.Context, datasourceID string) ([]CatalogPolicy, error)
}

type CatalogColumn struct {
	SchemaName string
	TableName  string
	ColumnName string
}

type CatalogRelation struct {
	FromSchema string
	FromTable  string
	FromColumn string
	ToSchema   string
	ToTable    string
	ToColumn   string
}

type CatalogPolicy struct {
	DeniedFields []string
	RowFilters   []CatalogRowFilter
}

type CatalogRowFilter struct {
	Field string
}

// PublishValidationResult is returned by validate/publish endpoints.
type PublishValidationResult struct {
	Valid               bool     `json:"valid"`
	Errors              []string `json:"errors,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
	EstimatedPromptSize int      `json:"estimated_prompt_size"`
}

func (r PublishValidationResult) HasError(needle string) bool {
	return containsMessage(r.Errors, needle)
}

func (r PublishValidationResult) HasWarning(needle string) bool {
	return containsMessage(r.Warnings, needle)
}

func containsMessage(messages []string, needle string) bool {
	for _, msg := range messages {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// ValidateContext checks whether a draft semantic model can be safely
// published. Errors block publish; warnings describe risky but valid context.
//
//nolint:gocyclo,gocognit,funlen // aggregates dimension, metric, join, and catalog consistency checks
func ValidateContext(ctx context.Context, model SemanticModel, catalog CatalogReader) PublishValidationResult {
	result := PublishValidationResult{
		Valid:               true,
		EstimatedPromptSize: estimatePromptSize(model),
	}
	addError := func(format string, args ...any) {
		result.Errors = append(result.Errors, fmt.Sprintf(format, args...))
		result.Valid = false
	}
	addWarning := func(format string, args ...any) {
		result.Warnings = append(result.Warnings, fmt.Sprintf(format, args...))
	}

	if strings.TrimSpace(model.Name) == "" {
		addError("model name is required")
	}
	if strings.TrimSpace(model.BaseTable) == "" {
		addError("base table is required")
	}
	if len(model.Dimensions) == 0 {
		addError("model has no active dimensions; AI cannot generate queries without at least one dimension")
	}

	columns, err := catalog.ListSemanticColumns(ctx, model.DatasourceID)
	if err != nil {
		addError("load datasource columns: %s", err)
		return result
	}
	relations, err := catalog.ListSemanticRelations(ctx, model.DatasourceID)
	if err != nil {
		addError("load datasource relations: %s", err)
		return result
	}
	policies, err := catalog.ListSemanticPolicies(ctx, model.DatasourceID)
	if err != nil {
		addError("load permission policies: %s", err)
		return result
	}
	columnSet := buildColumnSet(columns)

	validateDuplicateNames("dimension", model.Dimensions, func(d Dimension) string { return d.Name }, addError)
	validateDuplicateNames("metric", model.Metrics, func(m Metric) string { return m.Name }, addError)
	validateDuplicateNames("relationship", model.Joins, func(j Join) string { return j.Name }, addError)

	// Build map of allowed dimensions and metrics for reference checking
	allowedDims := make(map[string]bool, len(model.Dimensions))
	for _, d := range model.Dimensions {
		allowedDims[strings.ToLower(d.Name)] = true
	}
	allowedMets := make(map[string]bool, len(model.Metrics))
	for _, m := range model.Metrics {
		allowedMets[strings.ToLower(m.Name)] = true
	}

	for _, dim := range model.Dimensions {
		if dim.CalculatedExpression != "" { //nolint:nestif
			// Validate calculated expression
			if err := validateCalculatedExpression(dim.CalculatedExpression, columnSet, model.BaseSchema); err != nil {
				addError("dimension %q calculated expression invalid: %s", dim.Name, err)
			}
			expr := getOrParseExpr(dim.CalculatedExpression, dim.CalculatedExpr)
			if expr != nil {
				if err := pkgsemantic.ValidateExprStrict(expr, columnSet, allowedMets, allowedDims, false, 0); err != nil {
					addError("dimension %q calculated expression invalid: %s", dim.Name, err)
				}
			}
		} else if !columnSet.has(model.BaseSchema, dim.ColumnRef) {
			addError("%s: %s", errmsg.DimensionUnknownColumn, dim.ColumnRef)
		}
	}

	for _, metric := range model.Metrics {
		fnLower := strings.ToLower(strings.TrimSpace(metric.Aggregation))
		if fnLower == "custom" || strings.Contains(metric.Expression, "[") { //nolint:nestif
			matches := reBracket.FindAllStringSubmatch(metric.Expression, -1)
			for _, match := range matches {
				token := strings.TrimSpace(match[1])
				isDimOrMetric := false
				for _, d := range model.Dimensions {
					if strings.EqualFold(d.Name, token) {
						isDimOrMetric = true
						break
					}
				}
				for _, m := range model.Metrics {
					if strings.EqualFold(m.Name, token) {
						isDimOrMetric = true
						break
					}
				}
				if isDimOrMetric {
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
		} else {
			for _, ref := range columnRefsInExpression(metric.Expression) {
				if !columnSet.has(model.BaseSchema, ref) {
					addError("%s: %s", errmsg.MetricExpressionUnknownColumn, ref)
				}
			}
		}

		expr := getOrParseExpr(metric.Expression, metric.Expr)
		if expr != nil {
			allowMets := strings.ToLower(strings.TrimSpace(metric.Aggregation)) == "custom"
			if err := pkgsemantic.ValidateExprStrict(expr, columnSet, allowedMets, allowedDims, allowMets, 0); err != nil {
				addError("metric %q expression invalid: %s", metric.Name, err)
			}
		}
	}

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
		switch strings.ToLower(strings.TrimSpace(join.Relationship)) {
		case "", RelationshipOneToOne, RelationshipManyToOne:
		case RelationshipOneToMany, RelationshipManyToMany:
			addWarning("join can fan out aggregations: %s uses %s", join.Name, join.Relationship)
		default:
			addError("join has invalid relationship type: %s", join.Relationship)
		}
	}

	// Circular dependency validation
	for _, cycleErr := range checkCircularDependencies(model) {
		addError("%s", cycleErr)
	}

	validatePolicies(model, policies, addError)

	if result.EstimatedPromptSize > 60000 {
		addWarning("semantic context prompt estimate is high: %d runes", result.EstimatedPromptSize)
	}

	for _, msg := range EnforceBudget(model, DefaultContextBudget(), result.EstimatedPromptSize) {
		addWarning("%s", msg)
	}

	return result
}

func validatePolicies(model SemanticModel, policies []CatalogPolicy, addError func(string, ...any)) {
	fields := make(map[string]bool, len(model.Dimensions)+len(model.Metrics))
	for _, dimension := range model.Dimensions {
		fields[strings.ToLower(dimension.Name)] = true
		fields[strings.ToLower(model.Name+"."+dimension.Name)] = true
	}
	for _, metric := range model.Metrics {
		fields[strings.ToLower(metric.Name)] = true
		fields[strings.ToLower(model.Name+"."+metric.Name)] = true
	}
	for _, policy := range policies {
		for _, field := range policy.DeniedFields {
			field = strings.ToLower(strings.TrimSpace(field))
			if field == "" || fields[field] {
				continue
			}
			addError("%s: %s", errmsg.PermissionPolicyUnknownField, field)
		}
		for _, filter := range policy.RowFilters {
			field := strings.ToLower(strings.TrimSpace(filter.Field))
			if field == "" || fields[field] {
				continue
			}
			addError("%s: %s", errmsg.PermissionRowFilterUnknownField, field)
		}
	}
}

func validateDuplicateNames[T any](kind string, values []T, nameOf func(T) string, addError func(string, ...any)) {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		name := strings.TrimSpace(nameOf(value))
		if name == "" {
			addError("%s name is required", kind)
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			addError("duplicate %s name: %s", kind, name)
			continue
		}
		seen[key] = true
	}
}

type datasourceColumnSet map[string]bool

func buildColumnSet(columns []CatalogColumn) datasourceColumnSet {
	set := make(datasourceColumnSet, len(columns)*2)
	for _, col := range columns {
		full := normalizeColumnRef(col.SchemaName + "." + col.TableName + "." + col.ColumnName)
		short := normalizeColumnRef(col.TableName + "." + col.ColumnName)
		set[full] = true
		set[short] = true
	}
	return set
}

func (s datasourceColumnSet) has(defaultSchema, ref string) bool {
	ref = normalizeColumnRef(ref)
	if s[ref] {
		return true
	}
	parts := strings.Split(ref, ".")
	if len(parts) == 2 && defaultSchema != "" {
		return s[normalizeColumnRef(defaultSchema+"."+ref)]
	}
	return false
}

func (s datasourceColumnSet) hasTableColumn(defaultSchema, table, column string) bool {
	return s.has(defaultSchema, strings.TrimSpace(table)+"."+strings.TrimSpace(column))
}

func normalizeColumnRef(ref string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(ref), `"`))
}

var expressionColumnRefRE = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b`)

func columnRefsInExpression(expression string) []string {
	expression = strings.TrimSpace(expression)
	if expression == "" || expression == "*" {
		return nil
	}
	matches := expressionColumnRefRE.FindAllString(expression, -1)
	if len(matches) == 0 {
		return []string{expression}
	}
	return matches
}

func relationExists(defaultSchema string, join Join, relations []CatalogRelation) bool {
	fromSchema, fromTable := splitSemanticTable(defaultSchema, join.FromTable)
	toSchema, toTable := splitSemanticTable(defaultSchema, join.ToTable)
	for _, rel := range relations {
		if sameRelationSide(rel.FromSchema, rel.FromTable, rel.FromColumn, fromSchema, fromTable, join.FromColumn) &&
			sameRelationSide(rel.ToSchema, rel.ToTable, rel.ToColumn, toSchema, toTable, join.ToColumn) {
			return true
		}
		if sameRelationSide(rel.FromSchema, rel.FromTable, rel.FromColumn, toSchema, toTable, join.ToColumn) &&
			sameRelationSide(rel.ToSchema, rel.ToTable, rel.ToColumn, fromSchema, fromTable, join.FromColumn) {
			return true
		}
	}
	return false
}

func sameRelationSide(schemaA, tableA, columnA, schemaB, tableB, columnB string) bool {
	return strings.EqualFold(schemaA, schemaB) &&
		strings.EqualFold(tableA, tableB) &&
		strings.EqualFold(columnA, columnB)
}

func splitSemanticTable(defaultSchema, table string) (string, string) {
	parts := strings.Split(strings.TrimSpace(table), ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return defaultSchema, table
}

func estimatePromptSize(model SemanticModel) int {
	size := len(model.Name) + len(model.BaseSchema) + len(model.BaseTable)
	if model.Label != nil {
		size += len(*model.Label)
	}
	if model.Description != nil {
		size += len(*model.Description)
	}
	size += len(model.Dimensions) * 180
	size += len(model.Metrics) * 160
	size += len(model.Joins) * 120
	return size
}

// validateCalculatedExpression checks a dimension's calculated_expression
// against the allowed function/operator whitelist and verifies that all
// referenced columns exist in the datasource catalog.
func validateCalculatedExpression(expr string, columnSet datasourceColumnSet, defaultSchema string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return errors.New("calculated expression is empty")
	}

	if CalculatedExpressionValidator != nil {
		if err := CalculatedExpressionValidator(expr); err != nil {
			return err
		}
	}

	// Extract column references (table.column or bare column patterns)
	cols := columnRefsInExpression(expr)
	for _, col := range cols {
		// Skip literals and numbers
		if isSQLLiteral(col) {
			continue
		}
		if !columnSet.has(defaultSchema, col) {
			return fmt.Errorf("unknown column reference: %s", col)
		}
	}

	// Check for disallowed patterns (subqueries, DML). Use word-boundary
	// matching on a copy that has had string literals and comments stripped
	// so that legitimate column names like `select_count` or string content
	// `'delete me'` cannot mask or false-trigger the guard.
	if kw, ok := containsDMLKeyword(expr); ok {
		return fmt.Errorf("calculated expressions must not contain DML/DDL keyword: %s", kw)
	}

	return nil
}

// dmlKeywordPatterns gates calculated expressions to a SELECT-fragment-only
// world. Compiled once at package init.
var dmlKeywordPatterns = func() []*regexp.Regexp {
	keywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "ALTER",
		"TRUNCATE", "CREATE", "GRANT", "REVOKE", "MERGE", "CALL",
		"EXEC", "EXECUTE",
	}
	out := make([]*regexp.Regexp, len(keywords))
	for i, kw := range keywords {
		out[i] = regexp.MustCompile(`(?i)\b` + kw + `\b`)
	}
	return out
}()

func containsDMLKeyword(expr string) (string, bool) {
	cleaned := stripCalcExprLiteralsAndComments(expr)
	for _, re := range dmlKeywordPatterns {
		if loc := re.FindStringIndex(cleaned); loc != nil {
			return cleaned[loc[0]:loc[1]], true
		}
	}
	return "", false
}

// stripCalcExprLiteralsAndComments removes single-quoted strings, double-quoted
// identifiers, line comments (--), and block comments (/* */) from a SQL
// fragment so the remainder can be scanned for DML keywords without false
// positives from values or comment text.
//
//nolint:gocognit
func stripCalcExprLiteralsAndComments(sql string) string {
	var out strings.Builder
	out.Grow(len(sql))
	i := 0
	n := len(sql)
	for i < n {
		c := sql[i]
		if c == '-' && i+1 < n && sql[i+1] == '-' {
			for i < n && sql[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < n && sql[i+1] == '*' {
			i += 2
			for i+1 < n && (sql[i] != '*' || sql[i+1] != '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			continue
		}
		if c == '\'' {
			i++
			for i < n {
				if sql[i] == '\'' {
					if i+1 < n && sql[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}
		if c == '"' {
			i++
			for i < n {
				if sql[i] == '"' {
					if i+1 < n && sql[i+1] == '"' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}
		if err := out.WriteByte(c); err != nil {
			_ = err
		}
		i++
	}
	return out.String()
}

// isSQLLiteral returns true if s looks like a SQL literal (number, string, NULL, etc.).
func isSQLLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	// Numbers
	if s == "0" || s == "1" || s == "NULL" || s == "null" || s == "TRUE" || s == "FALSE" {
		return true
	}
	// String literals
	if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") {
		return true
	}
	// Pure numbers
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

//nolint:gocognit
func checkCircularDependencies(model SemanticModel) []string {
	var errs []string
	adj := make(map[string][]string)

	for _, d := range model.Dimensions {
		expr := getOrParseExpr(d.CalculatedExpression, d.CalculatedExpr)
		if expr != nil {
			_, mets, dims := pkgsemantic.ExprDependencies(expr)
			nodeKey := "dim:" + strings.ToLower(d.Name)
			for _, depDim := range dims {
				adj[nodeKey] = append(adj[nodeKey], "dim:"+strings.ToLower(depDim))
			}
			for _, depMet := range mets {
				adj[nodeKey] = append(adj[nodeKey], "met:"+strings.ToLower(depMet))
			}
		}
	}

	for _, m := range model.Metrics {
		expr := getOrParseExpr(m.Expression, m.Expr)
		if expr != nil {
			_, mets, dims := pkgsemantic.ExprDependencies(expr)
			nodeKey := "met:" + strings.ToLower(m.Name)
			for _, depDim := range dims {
				adj[nodeKey] = append(adj[nodeKey], "dim:"+strings.ToLower(depDim))
			}
			for _, depMet := range mets {
				adj[nodeKey] = append(adj[nodeKey], "met:"+strings.ToLower(depMet))
			}
		}
	}

	visited := make(map[string]int) // 0 = unvisited, 1 = visiting, 2 = visited
	var path []string

	var dfs func(u string) bool
	dfs = func(u string) bool {
		visited[u] = 1
		path = append(path, u)

		for _, v := range adj[u] {
			if visited[v] == 1 {
				cycleStartIdx := -1
				for i, p := range path {
					if p == v {
						cycleStartIdx = i
						break
					}
				}
				if cycleStartIdx != -1 {
					cycle := make([]string, 0, len(path)-cycleStartIdx+1)
					for _, p := range path[cycleStartIdx:] {
						cycle = append(cycle, cleanNodeName(p))
					}
					cycle = append(cycle, cleanNodeName(v))
					errs = append(errs, "circular dependency detected: "+strings.Join(cycle, " -> "))
				}
				return true
			} else if visited[v] == 0 {
				if dfs(v) {
					return true
				}
			}
		}

		path = path[:len(path)-1]
		visited[u] = 2
		return false
	}

	for k := range adj {
		if visited[k] == 0 {
			dfs(k)
		}
	}

	return errs
}

func getOrParseExpr(exprStr string, ast pkgsemantic.ExprNode) pkgsemantic.ExprNode {
	if ast != nil {
		return ast
	}
	exprStr = strings.TrimSpace(exprStr)
	if exprStr == "" || exprStr == "*" || ExpressionParser == nil {
		return nil
	}
	parsed, err := ExpressionParser(exprStr)
	if err != nil {
		return nil
	}
	return parsed
}

func cleanNodeName(s string) string {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return s
}
