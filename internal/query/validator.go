package query

import (
	"slices"
	"strconv"

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
func (v *Validator) Validate(lq LogicalQuery, model *semantic.SemanticModel) error {
	var errs ValidationErrors

	// Check select items
	dimMap := make(map[string]bool)
	for _, d := range model.Dimensions {
		dimMap[d.Name] = true
	}

	metricMap := make(map[string]bool)
	for _, m := range model.Metrics {
		metricMap[m.Name] = true
	}

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
			if !metricMap[item.Name] {
				errs = append(errs, &ValidationError{
					Field:   "select",
					Message: "unknown metric: " + item.Name,
				})
			}
		default:
			errs = append(errs, &ValidationError{
				Field:   "select",
				Message: "invalid select type: " + item.Type,
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
