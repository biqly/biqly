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
	OpBetween, OpIsNull, OpIsNotNull,
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
//nolint:gocyclo // linear validation checks, each field validated independently
func (v *Validator) Validate(lq LogicalQuery, model *semantic.SemanticModel) error {
	var errs ValidationErrors

	// Build lookup maps from the semantic model — single source of truth.
	dimMap := make(map[string]bool, len(model.Dimensions))
	for _, d := range model.Dimensions {
		dimMap[d.Name] = true
	}

	metricRegistry := semantic.NewMetricRegistry(model.Metrics)

	for _, item := range lq.Select {
		switch item.Type {
		case SelectTypeDimension:
			if !dimMap[item.Name] {
				errs = append(errs, &ValidationError{
					Field:   "select",
					Message: errmsg.UnknownDimensionMsg(item.Name),
				})
			}
		case SelectTypeMetric:
			if !metricRegistry.Has(item.Name) {
				errs = append(errs, &ValidationError{
					Field:   "select",
					Message: errmsg.UnknownMetricMsg(item.Name),
				})
			}
		case SelectTypeWindow:
			errs = append(errs, validateWindowSelect(item, dimMap, metricRegistry)...)
		case SelectTypeCase:
			errs = append(errs, validateCaseSelect(item, dimMap)...)
		default:
			errs = append(errs, &ValidationError{
				Field:   "select",
				Message: "invalid select type: " + item.Type,
			})
		}
	}

	// HAVING — each field must be a metric (post-aggregation).
	for _, f := range lq.Having {
		if !metricRegistry.Has(f.Field) {
			errs = append(errs, &ValidationError{
				Field:   "having",
				Message: "having field must reference a metric: " + f.Field,
			})
		}
		if !slices.Contains(havingOps, f.Operator) {
			errs = append(errs, &ValidationError{
				Field:   "having",
				Message: "operator not supported in having: " + f.Operator,
			})
		}
	}

	// Check filters
	allowedFields := make(map[string]bool, len(model.Dimensions)+len(model.Metrics))
	for _, d := range model.Dimensions {
		allowedFields[d.Name] = true
	}
	for _, m := range model.Metrics {
		allowedFields[m.Name] = true
	}

	for _, f := range lq.Filters {
		if !allowedFields[f.Field] {
			errs = append(errs, &ValidationError{
				Field:   "filters",
				Message: errmsg.UnknownFieldMsg(f.Field),
			})
		}
		if !slices.Contains(validFilterOps, f.Operator) {
			errs = append(errs, &ValidationError{
				Field:   "filters",
				Message: "invalid operator: " + f.Operator,
			})
		}
		if f.Subquery != nil {
			errs = append(errs, validateSubqueryFilter(f)...)
		}
		if err := validateDateFilterValueType(f, model.Dimensions); err != nil {
			errs = append(errs, err)
		}
	}

	if lq.FromSubquery != nil && strings.TrimSpace(lq.FromCTE) != "" {
		errs = append(errs, &ValidationError{
			Field:   "from",
			Message: "from_subquery and from_cte are mutually exclusive",
		})
	}
	for _, cte := range lq.CTEs {
		if strings.TrimSpace(cte.Name) == "" {
			errs = append(errs, &ValidationError{Field: "ctes", Message: "cte name is required"})
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
				Message: "from_cte must match a defined cte name: " + lq.FromCTE,
			})
		}
	}

	if err := validateCalendarGrainYearCoverage(lq, model); err != nil {
		errs = append(errs, err)
	}

	// Date/timestamp dimensions are the only ones a time-grain can bucket.
	dimTypes := make(map[string]string, len(model.Dimensions))
	for _, d := range model.Dimensions {
		dimTypes[d.Name] = strings.ToLower(strings.TrimSpace(d.Type))
	}

	// Check group by
	for _, gb := range lq.GroupBy {
		if !dimMap[gb.Field] {
			errs = append(errs, &ValidationError{
				Field:   "group_by",
				Message: errmsg.UnknownDimensionMsg(gb.Field),
			})
			continue
		}
		if gb.TimeGrain == "" {
			continue
		}
		if !IsValidTimeGrain(gb.TimeGrain) {
			errs = append(errs, &ValidationError{
				Field:   "group_by.time_grain",
				Message: "invalid time_grain (expected day|week|month|quarter|year): " + gb.TimeGrain,
			})
			continue
		}
		if t := dimTypes[gb.Field]; t != "date" && t != "timestamp" && t != "datetime" {
			errs = append(errs, &ValidationError{
				Field:   "group_by.time_grain",
				Message: "time_grain only valid on date/timestamp dimensions: " + gb.Field,
			})
		}
	}

	// Check order by
	for _, ob := range lq.OrderBy {
		if !dimMap[ob.Field] && !metricRegistry.Has(ob.Field) {
			errs = append(errs, &ValidationError{
				Field:   "order_by",
				Message: errmsg.UnknownFieldMsg(ob.Field),
			})
		}
		if ob.Direction != "" && ob.Direction != OrderAsc && ob.Direction != OrderDesc {
			errs = append(errs, &ValidationError{
				Field:   "order_by",
				Message: "invalid direction: " + ob.Direction,
			})
		}
	}

	// Check limit
	if lq.Limit < 0 {
		errs = append(errs, &ValidationError{
			Field:   "limit",
			Message: "limit must be non-negative",
		})
	}
	if lq.Limit > v.maxRows {
		errs = append(errs, &ValidationError{
			Field:   "limit",
			Message: "limit exceeds maximum allowed rows (" + strconv.Itoa(v.maxRows) + ")",
		})
	}

	// Check offset
	if lq.Offset < 0 {
		errs = append(errs, &ValidationError{
			Field:   "offset",
			Message: "offset must be non-negative",
		})
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
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
	switch f.Operator {
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte:
	default:
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
	if strings.TrimSpace(dim.TimeGrain) != "" {
		return nil
	}
	if !isNumericFilterValue(f.Value) {
		return nil
	}
	return &ValidationError{
		Field: "filters",
		Message: "filter on raw date/timestamp dimension " + f.Field +
			" must use an ISO date string (\"YYYY-MM-DD\"); for integer year/month/day filters use the matching *_year, *_month, or *_day grain dimension",
	}
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
func validateWindowSelect(item SelectItem, dimMap map[string]bool, metricRegistry *semantic.MetricRegistry) ValidationErrors {
	var errs ValidationErrors
	if item.Window == nil {
		errs = append(errs, &ValidationError{Field: "select", Message: "window item missing window spec: " + item.Name})
		return errs
	}
	w := item.Window
	if w.Metric != "" && !metricRegistry.Has(w.Metric) {
		errs = append(errs, &ValidationError{Field: "select.window", Message: "unknown metric reference: " + w.Metric})
	}
	allowedAgg := map[string]bool{
		"sum": true, "avg": true, "count": true, "count_distinct": true,
		"min": true, "max": true,
		"row_number": true, "rank": true, "dense_rank": true, "ntile": true,
	}
	if !allowedAgg[strings.ToLower(strings.TrimSpace(w.Aggregation))] && w.Metric == "" {
		errs = append(errs, &ValidationError{Field: "select.window", Message: "unsupported window aggregation: " + w.Aggregation})
	}
	for _, p := range w.PartitionBy {
		if !dimMap[p] {
			errs = append(errs, &ValidationError{Field: "select.window.partition_by", Message: errmsg.UnknownDimensionMsg(p)})
		}
	}
	for _, ob := range w.OrderBy {
		if !dimMap[ob.Field] && !metricRegistry.Has(ob.Field) {
			errs = append(errs, &ValidationError{Field: "select.window.order_by", Message: errmsg.UnknownFieldMsg(ob.Field)})
		}
		if ob.Direction != "" && ob.Direction != OrderAsc && ob.Direction != OrderDesc {
			errs = append(errs, &ValidationError{Field: "select.window.order_by", Message: "invalid direction: " + ob.Direction})
		}
	}
	return errs
}

func validateCaseSelect(item SelectItem, dimMap map[string]bool) ValidationErrors {
	var errs ValidationErrors
	if item.Case == nil || len(item.Case.Branches) == 0 {
		errs = append(errs, &ValidationError{Field: "select.case", Message: "case item requires branches: " + item.Name})
		return errs
	}
	if strings.TrimSpace(item.Name) == "" && strings.TrimSpace(item.Alias) == "" {
		errs = append(errs, &ValidationError{Field: "select.case", Message: "case item requires name or alias"})
	}
	for i, br := range item.Case.Branches {
		if len(br.When) == 0 {
			errs = append(errs, &ValidationError{Field: "select.case", Message: fmt.Sprintf("case branch %d missing when filters", i)})
		}
		errs = append(errs, validateCaseThen(br.Then, dimMap, "select.case")...)
	}
	if item.Case.Else != nil {
		errs = append(errs, validateCaseThen(*item.Case.Else, dimMap, "select.case")...)
	}
	return errs
}

func validateCaseThen(then CaseThen, dimMap map[string]bool, field string) ValidationErrors {
	var errs ValidationErrors
	switch strings.ToLower(strings.TrimSpace(then.Type)) {
	case CaseThenTypeDimension, "":
		if then.Dimension == "" || !dimMap[then.Dimension] {
			errs = append(errs, &ValidationError{Field: field, Message: "unknown case then dimension: " + then.Dimension})
		}
	case CaseThenTypeLiteral:
		if then.Literal == nil {
			errs = append(errs, &ValidationError{Field: field, Message: "case then literal value required"})
		}
	default:
		errs = append(errs, &ValidationError{Field: field, Message: "invalid case then type: " + then.Type})
	}
	return errs
}

func validateSubqueryFilter(f Filter) ValidationErrors {
	var errs ValidationErrors
	if f.Operator != OpIn && f.Operator != OpNotIn {
		errs = append(errs, &ValidationError{Field: "filters.subquery", Message: "subquery filter requires in or not_in operator"})
	}
	if f.Subquery == nil {
		return errs
	}
	if strings.TrimSpace(f.Subquery.ResultField) == "" {
		errs = append(errs, &ValidationError{Field: "filters.subquery", Message: "subquery result_field is required"})
	}
	if len(f.Subquery.Body.Select) == 0 {
		errs = append(errs, &ValidationError{Field: "filters.subquery", Message: "subquery body requires select"})
	}
	return errs
}
