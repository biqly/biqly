package semantic

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	ModelStatusDraft     = "draft"
	ModelStatusPublished = "published"
)

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

	for _, dim := range model.Dimensions {
		if dim.CalculatedExpression != "" {
			// Validate calculated expression
			if err := validateCalculatedExpression(dim.CalculatedExpression, columnSet, model.BaseSchema); err != nil {
				addError("dimension %q calculated expression invalid: %s", dim.Name, err)
			}
		} else if !columnSet.has(model.BaseSchema, dim.ColumnRef) {
			addError("dimension references unknown column: %s", dim.ColumnRef)
		}
	}

	for _, metric := range model.Metrics {
		for _, ref := range columnRefsInExpression(metric.Expression) {
			if !columnSet.has(model.BaseSchema, ref) {
				addError("metric expression references unknown column: %s", ref)
			}
		}
	}

	for _, join := range model.Joins {
		if !columnSet.hasTableColumn(model.BaseSchema, join.FromTable, join.FromColumn) {
			addError("join references unknown from column: %s.%s", join.FromTable, join.FromColumn)
		}
		if !columnSet.hasTableColumn(model.BaseSchema, join.ToTable, join.ToColumn) {
			addError("join references unknown to column: %s.%s", join.ToTable, join.ToColumn)
		}
		if !relationExists(model.BaseSchema, join, relations) {
			addError("join does not match datasource metadata relation: %s", join.Name)
		}
		switch strings.ToLower(strings.TrimSpace(join.Relationship)) {
		case "", "one_to_one", "many_to_one":
		case "one_to_many", "many_to_many":
			addWarning("join can fan out aggregations: %s uses %s", join.Name, join.Relationship)
		default:
			addError("join has invalid relationship type: %s", join.Relationship)
		}
	}

	validatePolicies(model, policies, addError)

	if result.EstimatedPromptSize > 60000 {
		addWarning("semantic context prompt estimate is high: %d runes", result.EstimatedPromptSize)
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
			addError("permission policy references unknown field: %s", field)
		}
		for _, filter := range policy.RowFilters {
			field := strings.ToLower(strings.TrimSpace(filter.Field))
			if field == "" || fields[field] {
				continue
			}
			addError("permission row filter references unknown field: %s", field)
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

// allowedCalculatedFunctions is the whitelist of functions permitted in
// dimension calculated expressions. This keeps expressions portable across
// dialects (Postgres, MySQL, SQL Server, ClickHouse).
var allowedCalculatedFunctions = map[string]bool{
	"COALESCE":    true,
	"CONCAT":      true,
	"UPPER":       true,
	"LOWER":       true,
	"ROUND":       true,
	"TRIM":        true,
	"NULLIF":      true,
	"CAST":        true,
	"EXTRACT":     true,
	"DATE_TRUNC":  true,
	"TO_CHAR":     true,
	"TO_DATE":     true,
	"LENGTH":      true,
	"SUBSTRING":   true,
	"REPLACE":     true,
	"ABS":         true,
	"CEIL":        true,
	"FLOOR":       true,
	"SIGN":        true,
}

// allowedCalculatedOperators is the whitelist of operators permitted in
// dimension calculated expressions.
var allowedCalculatedOperators = map[string]bool{
	"+":  true,
	"-":  true,
	"*":  true,
	"/":  true,
	"||": true, // string concatenation (SQL standard)
	"=":  true,
	"!=": true,
	">":  true,
	"<":  true,
	">=": true,
	"<=": true,
}

// validateCalculatedExpression checks a dimension's calculated_expression
// against the allowed function/operator whitelist and verifies that all
// referenced columns exist in the datasource catalog.
func validateCalculatedExpression(expr string, columnSet datasourceColumnSet, defaultSchema string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("calculated expression is empty")
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

	// Check for disallowed patterns (subqueries, etc.)
	upper := strings.ToUpper(expr)
	if strings.Contains(upper, "SELECT") || strings.Contains(upper, "INSERT") ||
		strings.Contains(upper, "UPDATE") || strings.Contains(upper, "DELETE") {
		return fmt.Errorf("calculated expressions must not contain DML statements")
	}

	return nil
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
