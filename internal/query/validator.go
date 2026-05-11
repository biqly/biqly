package query

import (
	"slices"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

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
	dimMap := make(map[string]bool)
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
					Message: "unknown dimension: " + item.Name,
				})
			}
		case SelectTypeMetric:
			if !metricRegistry.Has(item.Name) {
				errs = append(errs, &ValidationError{
					Field:   "select",
					Message: "unknown metric: " + item.Name,
				})
			}
		case SelectTypeWindow:
			errs = append(errs, validateWindowSelect(item, dimMap, metricRegistry)...)
		default:
			errs = append(errs, &ValidationError{
				Field:   "select",
				Message: "invalid select type: " + item.Type,
			})
		}
	}

	// HAVING — each field must be a metric (post-aggregation).
	havingOps := []string{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpBetween, OpIsNull, OpIsNotNull}
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
	allowedFields := make(map[string]bool)
	for _, d := range model.Dimensions {
		allowedFields[d.Name] = true
	}
	for _, m := range model.Metrics {
		allowedFields[m.Name] = true
	}

	validOps := []string{
		OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte,
		OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith,
		OpBetween, OpIsNull, OpIsNotNull,
	}

	for _, f := range lq.Filters {
		if !allowedFields[f.Field] {
			errs = append(errs, &ValidationError{
				Field:   "filters",
				Message: "unknown field: " + f.Field,
			})
		}
		if !slices.Contains(validOps, f.Operator) {
			errs = append(errs, &ValidationError{
				Field:   "filters",
				Message: "invalid operator: " + f.Operator,
			})
		}
	}

	// Check group by
	for _, gb := range lq.GroupBy {
		if !dimMap[gb.Field] {
			errs = append(errs, &ValidationError{
				Field:   "group_by",
				Message: "unknown dimension: " + gb.Field,
			})
		}
	}

	// Check order by
	for _, ob := range lq.OrderBy {
		if !dimMap[ob.Field] && !metricRegistry.Has(ob.Field) {
			errs = append(errs, &ValidationError{
				Field:   "order_by",
				Message: "unknown field: " + ob.Field,
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
			errs = append(errs, &ValidationError{Field: "select.window.partition_by", Message: "unknown dimension: " + p})
		}
	}
	for _, ob := range w.OrderBy {
		if !dimMap[ob.Field] && !metricRegistry.Has(ob.Field) {
			errs = append(errs, &ValidationError{Field: "select.window.order_by", Message: "unknown field: " + ob.Field})
		}
		if ob.Direction != "" && ob.Direction != OrderAsc && ob.Direction != OrderDesc {
			errs = append(errs, &ValidationError{Field: "select.window.order_by", Message: "invalid direction: " + ob.Direction})
		}
	}
	return errs
}
