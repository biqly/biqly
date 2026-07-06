package query

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/errmsg"
	"github.com/biqly/biqly/internal/semantic"
)

// havingOps lists the operators legal in a HAVING clause; restricted to
// scalar comparisons because HAVING only references aggregated metrics.
var havingOps = []string{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpBetween, OpIsNull, OpIsNotNull}

// validFilterOps lists every operator legal in a WHERE-clause filter.
var validFilterOps = []string{
	OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte,
	OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith,
	OpBetween, OpIsNull, OpIsNotNull, OpIsEmpty, OpIsNotEmpty,
}

// Validator validates a LogicalQuery against a semantic model and config.
type Validator struct {
	maxRows int
}

// NewValidator creates a new validator with the given max row limit.
func NewValidator(maxRows int) *Validator {
	return &Validator{maxRows: maxRows}
}

// Validate checks a LogicalQuery for correctness.
//
// validationLookups holds the per-model lookup tables shared across the
// individual Validate sub-checks, computed once per Validate call.
type validationLookups struct {
	dimMap         map[string]bool
	dimTypes       map[string]string
	allowedFields  map[string]bool
	metricRegistry *semantic.MetricRegistry
	dimensionNames []string
	metricNames    []string
	allFieldNames  []string
}

func newValidationLookups(model *semantic.SemanticModel) validationLookups {
	dimMap := make(map[string]bool, len(model.Dimensions))
	dimTypes := make(map[string]string, len(model.Dimensions))
	allowedFields := make(map[string]bool, len(model.Dimensions)+len(model.Metrics))
	for _, d := range model.Dimensions {
		dimMap[d.Name] = true
		dimTypes[d.Name] = strings.ToLower(strings.TrimSpace(d.Type))
		allowedFields[d.Name] = true
	}
	for _, m := range model.Metrics {
		allowedFields[m.Name] = true
	}
	return validationLookups{
		dimMap:         dimMap,
		dimTypes:       dimTypes,
		allowedFields:  allowedFields,
		metricRegistry: semantic.NewMetricRegistry(model.Metrics),
		dimensionNames: getDimensionNames(model),
		metricNames:    getMetricNames(model),
		allFieldNames:  getAllFieldNames(model),
	}
}

func (v *Validator) Validate(lq *LogicalQuery, model *semantic.SemanticModel) error {
	lk := newValidationLookups(model)

	var errs ValidationErrors
	errs = append(errs, validateSelectItems(lq, model, lk)...)
	errs = append(errs, validateHavingClause(lq, lk)...)
	errs = append(errs, validateFilterClauses(lq, model, lk)...)
	errs = append(errs, validateFromAndCTEs(lq)...)
	errs = append(errs, validateSchemaOverrides(lq, model)...)
	if err := validateCalendarGrainYearCoverage(lq, model); err != nil {
		errs = append(errs, err)
	}
	errs = append(errs, validateGroupByClauses(lq, lk)...)
	errs = append(errs, validateOrderByClauses(lq.OrderBy, lk.dimMap, lk.metricRegistry, lk.allFieldNames, "order_by")...)
	errs = append(errs, v.validateLimitOffset(lq)...)

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// validateSchemaOverrides rejects request-supplied DefaultSchema/TableSchemas
// values that name a schema the semantic model does not declare (its base
// schema or a join schema). Without this, a caller can redirect a joined table
// to another tenant's schema and read data through a model they were only
// authorized to query against their own schema.
func validateSchemaOverrides(lq *LogicalQuery, model *semantic.SemanticModel) ValidationErrors {
	if model == nil {
		return nil
	}
	if strings.TrimSpace(lq.DefaultSchema) == "" && len(lq.TableSchemas) == 0 {
		return nil
	}
	allowed := make(map[string]bool)
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			allowed[strings.ToLower(s)] = true
		}
	}
	add(model.BaseSchema)
	for _, j := range model.Joins {
		add(j.FromSchema)
		add(j.ToSchema)
	}
	var errs ValidationErrors
	check := func(field, schema string) {
		if s := strings.TrimSpace(schema); s != "" && !allowed[strings.ToLower(s)] {
			errs = append(errs, &ValidationError{
				Field:   field,
				Code:    "DISALLOWED_SCHEMA",
				Message: "schema not declared by the semantic model: " + schema,
				Value:   schema,
			})
		}
	}
	check("default_schema", lq.DefaultSchema)
	for tbl, schema := range lq.TableSchemas {
		check("table_schemas."+tbl, schema)
	}
	return errs
}

// validateSelectItems checks each SELECT item against the model.
func validateSelectItems(lq *LogicalQuery, model *semantic.SemanticModel, lk validationLookups) ValidationErrors {
	var errs ValidationErrors
	for _, item := range lq.Select {
		switch item.Type {
		case SelectTypeDimension:
			if !lk.dimMap[item.Name] {
				errs = append(errs, &ValidationError{
					Field:               "select",
					Code:                errmsg.CodeUnknownDimension,
					Message:             errmsg.UnknownDimensionMsg(item.Name),
					Value:               item.Name,
					AllowedAlternatives: suggestAlternatives(item.Name, lk.dimensionNames),
				})
			}
		case SelectTypeMetric:
			if !lk.metricRegistry.Has(item.Name) {
				errs = append(errs, &ValidationError{
					Field:               "select",
					Code:                errmsg.CodeUnknownMetric,
					Message:             errmsg.UnknownMetricMsg(item.Name),
					Value:               item.Name,
					AllowedAlternatives: suggestAlternatives(item.Name, lk.metricNames),
				})
			}
			errs = append(errs, validateFilterList(item.Filters, model, lk, "select.filters")...)
		case SelectTypeWindow:
			errs = append(errs, validateWindowSelect(item, model, lk.dimMap, lk.metricRegistry, lk.dimensionNames, lk.metricNames, lk.allFieldNames)...)
		case SelectTypeCase:
			errs = append(errs, validateCaseSelect(item, lk.dimMap, lk.dimensionNames)...)
		case SelectTypeFormula:
			errs = append(errs, validateFormulaSelect(item, model, lk)...)
		default:
			errs = append(errs, &ValidationError{
				Field:               "select",
				Code:                errmsg.CodeInvalidSelectType,
				Message:             "invalid select type: " + item.Type,
				Value:               item.Type,
				AllowedAlternatives: []string{SelectTypeDimension, SelectTypeMetric, SelectTypeWindow, SelectTypeCase, SelectTypeFormula},
			})
		}
	}
	return errs
}

// validateHavingClause requires each HAVING field to be a metric with a
// HAVING-legal operator.
func validateHavingClause(lq *LogicalQuery, lk validationLookups) ValidationErrors {
	var errs ValidationErrors
	for _, f := range lq.Having {
		if !lk.metricRegistry.Has(f.Field) {
			errs = append(errs, &ValidationError{
				Field:               "having",
				Code:                errmsg.CodeUnknownMetric,
				Message:             "having field must reference a metric: " + f.Field,
				Value:               f.Field,
				AllowedAlternatives: suggestAlternatives(f.Field, lk.metricNames),
			})
		}
		if !slices.Contains(havingOps, f.Operator) {
			errs = append(errs, &ValidationError{
				Field:               "having",
				Code:                errmsg.CodeInvalidOperator,
				Message:             "operator not supported in having: " + f.Operator,
				Value:               f.Operator,
				AllowedAlternatives: havingOps,
			})
		}
	}
	return errs
}

// validateFilterClauses checks WHERE filters: known field, valid operator,
// subquery shape, and date/value-type compatibility.
func validateFilterClauses(lq *LogicalQuery, model *semantic.SemanticModel, lk validationLookups) ValidationErrors {
	return validateFilterList(lq.Filters, model, lk, "filters")
}

// validateFilterList runs the WHERE-filter checks (known field, valid operator,
// subquery shape, date/value-type) over an arbitrary filter slice. Shared by the
// query-level WHERE and by per-measure filters on metric / ratio select items;
// fieldPath names the offending location in error messages.
func validateFilterList(filters []Filter, model *semantic.SemanticModel, lk validationLookups, fieldPath string) ValidationErrors {
	var errs ValidationErrors
	for _, f := range filters {
		if !lk.allowedFields[f.Field] {
			errs = append(errs, &ValidationError{
				Field:               fieldPath,
				Code:                errmsg.CodeUnknownField,
				Message:             errmsg.UnknownFieldMsg(f.Field),
				Value:               f.Field,
				AllowedAlternatives: suggestAlternatives(f.Field, lk.allFieldNames),
			})
		}
		if !slices.Contains(validFilterOps, f.Operator) {
			errs = append(errs, &ValidationError{
				Field:               fieldPath,
				Code:                errmsg.CodeInvalidOperator,
				Message:             "invalid operator: " + f.Operator,
				Value:               f.Operator,
				AllowedAlternatives: validFilterOps,
			})
		}
		if f.Subquery != nil {
			errs = append(errs, validateSubqueryFilter(f)...)
		}
		if err := validateDateFilterValueType(f, model.Dimensions); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// validateFormulaSelect checks a `formula` select item: spec present, name/alias
// available, a supported operator, and that both sides reference known metrics
// with well-formed per-measure filters.
func validateFormulaSelect(item SelectItem, model *semantic.SemanticModel, lk validationLookups) ValidationErrors {
	var errs ValidationErrors
	if item.Formula == nil {
		return append(errs, &ValidationError{
			Field:   "select.formula",
			Code:    "MISSING_FORMULA_SPEC",
			Message: "formula item requires a formula spec: " + item.Name,
		})
	}
	if strings.TrimSpace(item.Name) == "" && strings.TrimSpace(item.Alias) == "" {
		errs = append(errs, &ValidationError{
			Field:   "select.formula",
			Code:    "MISSING_FORMULA_NAME",
			Message: "formula item requires name or alias",
		})
	}
	if !IsValidFormulaOp(item.Formula.Op) {
		errs = append(errs, &ValidationError{
			Field:               "select.formula",
			Code:                "INVALID_FORMULA_OP",
			Message:             "unsupported formula op: " + item.Formula.Op,
			Value:               item.Formula.Op,
			AllowedAlternatives: []string{FormulaOpAdd, FormulaOpSubtract, FormulaOpDivide, FormulaOpPercentOf, FormulaOpPercentChange},
		})
	}
	errs = append(errs, validateMeasureRef(item.Formula.Left, model, lk, "select.formula.left")...)
	errs = append(errs, validateMeasureRef(item.Formula.Right, model, lk, "select.formula.right")...)
	return errs
}

// validateMeasureRef checks a MeasureRef names a known metric and that its
// per-measure filters are well-formed.
func validateMeasureRef(m MeasureRef, model *semantic.SemanticModel, lk validationLookups, fieldPath string) ValidationErrors {
	var errs ValidationErrors
	if !lk.metricRegistry.Has(m.Metric) {
		errs = append(errs, &ValidationError{
			Field:               fieldPath,
			Code:                errmsg.CodeUnknownMetric,
			Message:             errmsg.UnknownMetricMsg(m.Metric),
			Value:               m.Metric,
			AllowedAlternatives: suggestAlternatives(m.Metric, lk.metricNames),
		})
	}
	errs = append(errs, validateFilterList(m.Filters, model, lk, fieldPath+".filters")...)
	return errs
}

// validateFromAndCTEs enforces from_subquery/from_cte exclusivity, non-empty CTE
// names, and that from_cte references a defined CTE.
func validateFromAndCTEs(lq *LogicalQuery) ValidationErrors {
	var errs ValidationErrors
	if lq.FromSubquery != nil && strings.TrimSpace(lq.FromCTE) != "" {
		errs = append(errs, &ValidationError{
			Field:   "from",
			Code:    "MUTUALLY_EXCLUSIVE_FROM",
			Message: "from_subquery and from_cte are mutually exclusive",
		})
	}
	for _, cte := range lq.CTEs {
		if strings.TrimSpace(cte.Name) == "" {
			errs = append(errs, &ValidationError{
				Field:   "ctes",
				Code:    "MISSING_CTE_NAME",
				Message: "cte name is required",
			})
		}
	}
	if strings.TrimSpace(lq.FromCTE) != "" {
		found := false
		for _, cte := range lq.CTEs {
			if cte.Name == lq.FromCTE {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, &ValidationError{
				Field:   "from_cte",
				Code:    "UNKNOWN_CTE",
				Message: "from_cte must match a defined cte name: " + lq.FromCTE,
				Value:   lq.FromCTE,
			})
		}
	}
	return errs
}

// validateGroupByClauses checks each GROUP BY field is a known dimension and that
// any time_grain is valid and applied only to a date/timestamp dimension.
func validateGroupByClauses(lq *LogicalQuery, lk validationLookups) ValidationErrors {
	var errs ValidationErrors
	for _, gb := range lq.GroupBy {
		if !lk.dimMap[gb.Field] {
			errs = append(errs, &ValidationError{
				Field:               "group_by",
				Code:                errmsg.CodeUnknownDimension,
				Message:             errmsg.UnknownDimensionMsg(gb.Field),
				Value:               gb.Field,
				AllowedAlternatives: suggestAlternatives(gb.Field, lk.dimensionNames),
			})
			continue
		}
		if gb.TimeGrain == "" {
			continue
		}
		if !IsValidTimeGrain(gb.TimeGrain) {
			errs = append(errs, &ValidationError{
				Field:               "group_by.time_grain",
				Code:                errmsg.CodeInvalidTimeGrain,
				Message:             "invalid time_grain (expected hour|day|week|month|quarter|year): " + gb.TimeGrain,
				Value:               gb.TimeGrain,
				AllowedAlternatives: []string{"hour", "day", "week", "month", "quarter", "year"},
			})
			continue
		}
		if t := lk.dimTypes[gb.Field]; t != "date" && t != "timestamp" && t != "datetime" {
			errs = append(errs, &ValidationError{
				Field:   "group_by.time_grain",
				Code:    errmsg.CodeTimeGrainOnNonDate,
				Message: "time_grain only valid on date/timestamp dimensions: " + gb.Field,
				Value:   gb.Field,
			})
		}
	}
	return errs
}

// validateLimitOffset checks non-negative limit/offset and the max-rows ceiling.
func (v *Validator) validateLimitOffset(lq *LogicalQuery) ValidationErrors {
	var errs ValidationErrors
	if lq.Limit < 0 {
		errs = append(errs, &ValidationError{
			Field:   "limit",
			Code:    "NEGATIVE_LIMIT",
			Message: "limit must be non-negative",
		})
	}
	if lq.Limit > v.maxRows {
		errs = append(errs, &ValidationError{
			Field:   "limit",
			Code:    errmsg.CodeRowLimitExceeded,
			Message: "limit exceeds maximum allowed rows (" + strconv.Itoa(v.maxRows) + ")",
			Value:   strconv.Itoa(lq.Limit),
		})
	}
	if lq.Offset < 0 {
		errs = append(errs, &ValidationError{
			Field:   "offset",
			Code:    errmsg.CodeNegativeOffset,
			Message: "offset must be non-negative",
		})
	}
	return errs
}

// validateDateFilterValueType catches the common AI mistake of comparing a raw
// timestamp/date dimension to an integer year/month/day (e.g. "created_at = 2026").
// PostgreSQL rejects this at bind time with a confusing encode error; we surface
// it during validation instead so the retry loop has a clearer message that
// points at the matching `*_year` / `*_month` / `*_day` grain dimension.
//
// Scope is intentionally narrow: only scalar comparison operators with a numeric
// value on a raw (non-time-grain) date/timestamp dimension. Bucketed grain
// dimensions (TimeGrain set) accept integers — those project to ints via
// CalendarPart. Array operators (in/between) and string values pass through.
func validateDateFilterValueType(f Filter, dimensions []semantic.Dimension) *ValidationError {
	vals, ok := scalarFilterValues(f)
	if !ok || len(vals) == 0 {
		return nil
	}
	var dim *semantic.Dimension
	for i := range dimensions {
		if dimensions[i].Name == f.Field {
			dim = &dimensions[i]
			break
		}
	}
	if dim == nil {
		return nil
	}
	t := strings.ToLower(strings.TrimSpace(dim.Type))
	if t != "date" && t != "timestamp" && t != "datetime" {
		return nil
	}
	grain := strings.ToLower(strings.TrimSpace(dim.TimeGrain))
	for _, v := range vals {
		if !isNumericFilterValue(v) {
			continue
		}
		// whole=false means a fractional numeric (e.g. 5.5) — never a valid
		// calendar part, so it fails the day range check below.
		iv, whole := numericFilterValueInt(v)
		switch grain {
		case "":
			return &ValidationError{
				Field:               "filters",
				Code:                errmsg.CodeDateValueTypeMismatch,
				Message:             "filter on raw date/timestamp dimension " + f.Field + " must use an ISO date string (\"YYYY-MM-DD\"); for integer year/month/day filters use the matching *_year, *_month, or *_day grain dimension",
				Value:               f.Field,
				AllowedAlternatives: []string{f.Field + "_year", f.Field + "_month", f.Field + "_day"},
			}
		case TimeGrainDay:
			if !whole || iv < 1 || iv > 31 {
				return &ValidationError{
					Field:   "filters",
					Code:    errmsg.CodeDateValueTypeMismatch,
					Message: "day grain filter on " + f.Field + " accepts day-of-month integers 1-31 or ISO date strings (\"YYYY-MM-DD\")",
					Value:   f.Field,
				}
			}
		case TimeGrainWeek:
			return &ValidationError{
				Field:   "filters",
				Code:    errmsg.CodeDateValueTypeMismatch,
				Message: "week grain filter on " + f.Field + " must use an ISO date string (\"YYYY-MM-DD\") anchoring the week; integers are not supported",
				Value:   f.Field,
			}
		}
	}
	return nil
}

func isNumericFilterValue(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case json.Number:
		return true
	}
	return false
}

// validateWindowSelect ensures a window SelectItem is well-formed: spec is
// present, the aggregation is recognised, partition_by and order_by fields
// resolve in the semantic model, and any referenced metric exists.
func validateOrderByClauses(
	orderBy []OrderBy,
	dimMap map[string]bool,
	metricRegistry *semantic.MetricRegistry,
	allFieldNames []string,
	field string,
) ValidationErrors {
	var errs ValidationErrors
	for _, ob := range orderBy {
		if !dimMap[ob.Field] && !metricRegistry.Has(ob.Field) {
			errs = append(errs, &ValidationError{
				Field:               field,
				Code:                errmsg.CodeUnknownField,
				Message:             errmsg.UnknownFieldMsg(ob.Field),
				Value:               ob.Field,
				AllowedAlternatives: suggestAlternatives(ob.Field, allFieldNames),
			})
		}
		if ob.Direction != "" && ob.Direction != OrderAsc && ob.Direction != OrderDesc {
			errs = append(errs, &ValidationError{
				Field:               field,
				Code:                "INVALID_DIRECTION",
				Message:             "invalid direction: " + ob.Direction,
				Value:               ob.Direction,
				AllowedAlternatives: []string{OrderAsc, OrderDesc},
			})
		}
	}
	return errs
}

func validateWindowSelect(
	item SelectItem,
	_ *semantic.SemanticModel,
	dimMap map[string]bool,
	metricRegistry *semantic.MetricRegistry,
	dimensionNames []string,
	metricNames []string,
	allFieldNames []string,
) ValidationErrors {
	var errs ValidationErrors
	if item.Window == nil {
		errs = append(errs, &ValidationError{
			Field:   "select",
			Code:    "MISSING_WINDOW_SPEC",
			Message: "window item missing window spec: " + item.Name,
		})
		return errs
	}
	w := item.Window

	if w.Metric != "" && !metricRegistry.Has(w.Metric) {
		errs = append(errs, &ValidationError{
			Field:               "select.window",
			Code:                errmsg.CodeUnknownMetric,
			Message:             "unknown metric reference: " + w.Metric,
			Value:               w.Metric,
			AllowedAlternatives: suggestAlternatives(w.Metric, metricNames),
		})
	}
	agg := strings.ToLower(strings.TrimSpace(w.Aggregation))
	// Aggregate family (windowed totals/averages) plus the analytic functions.
	// count_distinct is intentionally excluded: COUNT(DISTINCT) is not a legal
	// window function in PostgreSQL, MySQL, or SQL Server.
	aggregateWindow := map[string]bool{"sum": true, "avg": true, "count": true, "min": true, "max": true}
	if agg == "count_distinct" {
		errs = append(errs, &ValidationError{
			Field:   "select.window",
			Code:    "INVALID_WINDOW_AGGREGATION",
			Message: "count_distinct cannot be used as a window function; use a plain count window or a distinct subquery",
			Value:   w.Aggregation,
		})
	} else if !aggregateWindow[agg] && !analyticWindowFuncs[agg] && w.Metric == "" {
		errs = append(errs, &ValidationError{
			Field:               "select.window",
			Code:                "INVALID_WINDOW_AGGREGATION",
			Message:             "unsupported window aggregation: " + w.Aggregation,
			Value:               w.Aggregation,
			AllowedAlternatives: []string{"sum", "avg", "count", "min", "max", "row_number", "rank", "dense_rank", "percent_rank", "cume_dist", "ntile", "lag", "lead", "first_value", "last_value"},
		})
	}
	// Ranking, ntile, lag/lead and first/last value are order-sensitive and
	// SQL Server *requires* ORDER BY for them; enforce it for deterministic,
	// portable output.
	if requiresWindowOrderBy(agg) && len(w.OrderBy) == 0 {
		errs = append(errs, &ValidationError{
			Field:   "select.window.order_by",
			Code:    "MISSING_WINDOW_ORDER_BY",
			Message: "window function " + agg + " requires order_by",
			Value:   agg,
		})
	}
	// lag/lead/first_value/last_value read a value: they need an expression or metric.
	if windowReadsValue(agg) && strings.TrimSpace(w.Expression) == "" && w.Expr == nil && strings.TrimSpace(w.Metric) == "" {
		errs = append(errs, &ValidationError{
			Field:   "select.window",
			Code:    "MISSING_WINDOW_VALUE",
			Message: "window function " + agg + " requires a metric or expression to read",
			Value:   agg,
		})
	}
	for _, p := range w.PartitionBy {
		if !dimMap[p] {
			errs = append(errs, &ValidationError{
				Field:               "select.window.partition_by",
				Code:                errmsg.CodeUnknownDimension,
				Message:             errmsg.UnknownDimensionMsg(p),
				Value:               p,
				AllowedAlternatives: suggestAlternatives(p, dimensionNames),
			})
		}
	}
	errs = append(errs, validateOrderByClauses(w.OrderBy, dimMap, metricRegistry, allFieldNames, "select.window.order_by")...)
	return errs
}

// requiresWindowOrderBy reports whether a window function is order-sensitive and
// must be given an ORDER BY (also a hard requirement in SQL Server).
func requiresWindowOrderBy(agg string) bool {
	switch agg {
	case "row_number", "rank", "dense_rank", "percent_rank", "cume_dist", "ntile", "lag", "lead", "first_value", "last_value":
		return true
	}
	return false
}

// windowReadsValue reports whether a window function reads a per-row value (so
// it needs an expression/metric), as opposed to ranking functions that don't.
func windowReadsValue(agg string) bool {
	switch agg {
	case "lag", "lead", "first_value", "last_value":
		return true
	}
	return false
}

func validateCaseSelect(item SelectItem, dimMap map[string]bool, dimensionNames []string) ValidationErrors {
	var errs ValidationErrors
	if item.Case == nil || len(item.Case.Branches) == 0 {
		errs = append(errs, &ValidationError{
			Field:   "select.case",
			Code:    "MISSING_CASE_BRANCHES",
			Message: "case item requires branches: " + item.Name,
		})
		return errs
	}
	if strings.TrimSpace(item.Name) == "" && strings.TrimSpace(item.Alias) == "" {
		errs = append(errs, &ValidationError{
			Field:   "select.case",
			Code:    "MISSING_CASE_NAME",
			Message: "case item requires name or alias",
		})
	}
	for i, br := range item.Case.Branches {
		if len(br.When) == 0 {
			errs = append(errs, &ValidationError{
				Field:   "select.case",
				Code:    "MISSING_CASE_WHEN",
				Message: fmt.Sprintf("case branch %d missing when filters", i),
			})
		}
		errs = append(errs, validateCaseThen(br.Then, dimMap, dimensionNames)...)
	}
	if item.Case.Else != nil {
		errs = append(errs, validateCaseThen(*item.Case.Else, dimMap, dimensionNames)...)
	}
	return errs
}

func validateCaseThen(then CaseThen, dimMap map[string]bool, dimensionNames []string) ValidationErrors {
	const field = "select.case"
	var errs ValidationErrors
	switch strings.ToLower(strings.TrimSpace(then.Type)) {
	case CaseThenTypeDimension, "":
		if then.Dimension == "" || !dimMap[then.Dimension] {
			errs = append(errs, &ValidationError{
				Field:               field,
				Code:                errmsg.CodeUnknownDimension,
				Message:             "unknown case then dimension: " + then.Dimension,
				Value:               then.Dimension,
				AllowedAlternatives: suggestAlternatives(then.Dimension, dimensionNames),
			})
		}
	case CaseThenTypeLiteral:
		if then.Literal == nil {
			errs = append(errs, &ValidationError{
				Field:   field,
				Code:    "MISSING_CASE_THEN_LITERAL",
				Message: "case then literal value required",
			})
		}
	default:
		errs = append(errs, &ValidationError{
			Field:               field,
			Code:                "INVALID_CASE_THEN_TYPE",
			Message:             "invalid case then type: " + then.Type,
			Value:               then.Type,
			AllowedAlternatives: []string{CaseThenTypeDimension, CaseThenTypeLiteral},
		})
	}
	return errs
}

func validateSubqueryFilter(f Filter) ValidationErrors {
	var errs ValidationErrors
	if f.Operator != OpIn && f.Operator != OpNotIn {
		errs = append(errs, &ValidationError{
			Field:               "filters.subquery",
			Code:                "INVALID_SUBQUERY_OPERATOR",
			Message:             "subquery filter requires in or not_in operator",
			Value:               f.Operator,
			AllowedAlternatives: []string{"in", "not_in"},
		})
	}
	if f.Subquery == nil {
		return errs
	}
	if strings.TrimSpace(f.Subquery.ResultField) == "" {
		errs = append(errs, &ValidationError{
			Field:   "filters.subquery",
			Code:    "MISSING_SUBQUERY_RESULT_FIELD",
			Message: "subquery result_field is required",
		})
	}
	if len(f.Subquery.Body.Select) == 0 {
		errs = append(errs, &ValidationError{
			Field:   "filters.subquery",
			Code:    "MISSING_SUBQUERY_SELECT",
			Message: "subquery body requires select",
		})
	}
	return errs
}
