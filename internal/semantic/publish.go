package semantic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/biqly/biqly/internal/errmsg"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

const (
	ModelStatusDraft     = "draft"
	ModelStatusPublished = "published"
)

var calculatedExpressionValidatorRegistry struct {
	mu        sync.RWMutex
	validator func(expr string) error
}

var modelPublishHooks struct {
	mu    sync.RWMutex
	hooks []func(ctx context.Context, modelID string)
}

// RegisterCalculatedExpressionValidator registers the calculated-expression validator.
func RegisterCalculatedExpressionValidator(validator func(expr string) error) {
	calculatedExpressionValidatorRegistry.mu.Lock()
	defer calculatedExpressionValidatorRegistry.mu.Unlock()
	calculatedExpressionValidatorRegistry.validator = validator
}

func currentCalculatedExpressionValidator() func(expr string) error {
	calculatedExpressionValidatorRegistry.mu.RLock()
	defer calculatedExpressionValidatorRegistry.mu.RUnlock()
	return calculatedExpressionValidatorRegistry.validator
}

// RegisterModelPublishHook registers a callback for semantic model publish/rollback events.
func RegisterModelPublishHook(hook func(ctx context.Context, modelID string)) {
	if hook == nil {
		return
	}
	modelPublishHooks.mu.Lock()
	defer modelPublishHooks.mu.Unlock()
	modelPublishHooks.hooks = append(modelPublishHooks.hooks, hook)
}

func notifyModelPublished(ctx context.Context, modelID string) {
	modelPublishHooks.mu.RLock()
	hooks := append([]func(context.Context, string){}, modelPublishHooks.hooks...)
	modelPublishHooks.mu.RUnlock()
	for _, hook := range hooks {
		hook(ctx, modelID)
	}
}

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
func ValidateContext(ctx context.Context, model SemanticModel, catalog CatalogReader) PublishValidationResult {
	sink := newValidationSinkWithPromptSize(model)
	validateModelRequiredFields(model, sink.addError)

	vc, ok := loadValidationCatalog(ctx, model, catalog, sink.addError)
	if !ok {
		return sink.finish()
	}

	validateDuplicateNames("dimension", model.Dimensions, func(d Dimension) string { return d.Name }, sink.addError)
	validateDuplicateNames("metric", model.Metrics, func(m Metric) string { return m.Name }, sink.addError)
	validateDuplicateNames("relationship", model.Joins, func(j Join) string { return j.Name }, sink.addError)

	allowedDims, allowedMets := allowedFieldMaps(model)
	validateContextDimensions(model, vc.columns, allowedDims, allowedMets, sink.addError)
	validateContextMetrics(model, vc.columns, allowedDims, allowedMets, sink.addError)
	validateContextJoins(model, vc.columns, vc.relations, sink.addError, sink.addWarning)

	for _, cycleErr := range checkCircularDependencies(model) {
		sink.addError("%s", cycleErr)
	}
	validatePolicies(model, vc.policies, sink.addError)
	appendContextBudgetWarnings(model, &sink.result, sink.addWarning)
	return sink.finish()
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

	if validator := currentCalculatedExpressionValidator(); validator != nil {
		if err := validator(expr); err != nil {
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
func stripCalcExprLiteralsAndComments(sql string) string {
	var out strings.Builder
	out.Grow(len(sql))
	i := 0
	n := len(sql)
	for i < n {
		switch {
		case calcExprLineCommentAt(sql, i, n):
			i = skipCalcExprLineComment(sql, i, n)
		case calcExprBlockCommentAt(sql, i, n):
			i = skipCalcExprBlockComment(sql, i, n)
		case sql[i] == '\'':
			i = skipCalcExprQuoted(sql, i, n, '\'')
		case sql[i] == '"':
			i = skipCalcExprQuoted(sql, i, n, '"')
		default:
			_ = out.WriteByte(sql[i])
			i++
		}
	}
	return out.String()
}

func calcExprLineCommentAt(sql string, i, n int) bool {
	return sql[i] == '-' && i+1 < n && sql[i+1] == '-'
}

func skipCalcExprLineComment(sql string, i, n int) int {
	for i < n && sql[i] != '\n' {
		i++
	}
	return i
}

func calcExprBlockCommentAt(sql string, i, n int) bool {
	return sql[i] == '/' && i+1 < n && sql[i+1] == '*'
}

func skipCalcExprBlockComment(sql string, i, n int) int {
	i += 2
	for i+1 < n && (sql[i] != '*' || sql[i+1] != '/') {
		i++
	}
	if i+1 < n {
		return i + 2
	}
	return n
}

func skipCalcExprQuoted(sql string, i, n int, quote byte) int {
	i++
	for i < n {
		if sql[i] == quote {
			if i+1 < n && sql[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
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

func checkCircularDependencies(model SemanticModel) []string {
	return findCalcExprCycles(buildCalcExprDependencyAdj(model))
}

func buildCalcExprDependencyAdj(model SemanticModel) map[string][]string {
	adj := make(map[string][]string)
	for _, d := range model.Dimensions {
		addCalcExprDeps(adj, "dim:"+strings.ToLower(d.Name), d.CalculatedExpression, d.CalculatedExpr)
	}
	for _, m := range model.Metrics {
		addCalcExprDeps(adj, "met:"+strings.ToLower(m.Name), m.Expression, m.Expr)
	}
	return adj
}

func addCalcExprDeps(adj map[string][]string, nodeKey, exprStr string, ast pkgsemantic.ExprNode) {
	expr, _ := getOrParseExpr(exprStr, ast)
	if expr == nil {
		return
	}
	_, mets, dims := pkgsemantic.ExprDependencies(expr)
	for _, depDim := range dims {
		adj[nodeKey] = append(adj[nodeKey], "dim:"+strings.ToLower(depDim))
	}
	for _, depMet := range mets {
		adj[nodeKey] = append(adj[nodeKey], "met:"+strings.ToLower(depMet))
	}
}

func findCalcExprCycles(adj map[string][]string) []string {
	visited := make(map[string]int)
	path := make([]string, 0, len(adj))
	errs := make([]string, 0)

	var dfs func(u string)
	dfs = func(u string) {
		visited[u] = 1
		path = append(path, u)

		for _, v := range adj[u] {
			switch visited[v] {
			case 1:
				if cycle := formatCalcExprCycle(path, v); cycle != "" {
					errs = append(errs, cycle)
				}
			case 0:
				dfs(v)
			}
		}

		path = path[:len(path)-1]
		visited[u] = 2
	}

	for k := range adj {
		if visited[k] == 0 {
			dfs(k)
		}
	}
	return errs
}

func formatCalcExprCycle(path []string, repeat string) string {
	cycleStartIdx := -1
	for i, p := range path {
		if p == repeat {
			cycleStartIdx = i
			break
		}
	}
	if cycleStartIdx == -1 {
		return ""
	}
	cycle := make([]string, 0, len(path)-cycleStartIdx+1)
	for _, p := range path[cycleStartIdx:] {
		cycle = append(cycle, cleanNodeName(p))
	}
	cycle = append(cycle, cleanNodeName(repeat))
	return "circular dependency detected: " + strings.Join(cycle, " -> ")
}

var errNoExpr = errors.New("no expression")

func getOrParseExpr(exprStr string, ast pkgsemantic.ExprNode) (pkgsemantic.ExprNode, error) {
	if ast != nil {
		return ast, nil
	}
	exprStr = strings.TrimSpace(exprStr)
	parser := CurrentExpressionParser()
	if exprStr == "" || exprStr == "*" || parser == nil {
		return nil, errNoExpr
	}
	parsed, err := parser(exprStr)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func cleanNodeName(s string) string {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return s
}
