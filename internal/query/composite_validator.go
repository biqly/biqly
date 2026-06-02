package query

import (
	"errors"
	"fmt"

	"github.com/biqly/biqly/internal/semantic"
)

// CompositeValidationResult captures the outcome of validating a LogicalQuery
// against a resolved composite model. Errors block compilation; Warnings are
// advisory (fanout risk, canonical date usage) and surfaced to callers.
type CompositeValidationResult struct {
	Errors      ValidationErrors `json:"errors,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
	FanoutRisk  string           `json:"fanout_risk,omitempty"`
	FanoutPlan  FanoutReport     `json:"fanout_report,omitempty"`
	HasCanonDim bool             `json:"has_canonical_dimension,omitempty"`
}

// Valid reports whether the query passed validation (no blocking errors).
func (r CompositeValidationResult) Valid() bool {
	return len(r.Errors) == 0
}

// ValidateCompositeQuery validates a LogicalQuery against a resolved composite
// model. It reuses the base Validator for field/aggregation checks and layers
// composite-specific analysis on top: dimension/metric name uniqueness in the
// merged model, fanout risk from cross-model joins, and canonical date usage.
func ValidateCompositeQuery(
	v *Validator,
	lq *LogicalQuery,
	resolved *semantic.SemanticModel,
	crossJoins []semantic.CrossModelJoin,
	canonicalDate *semantic.CanonicalDateRef,
) CompositeValidationResult {
	var result CompositeValidationResult

	if resolved == nil {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "composite_id",
			Message: "resolved composite model is nil",
		})
		return result
	}

	// Base field/aggregation/limit validation against the merged model.
	if v != nil {
		if err := v.Validate(lq, resolved); err != nil {
			var ve ValidationErrors
			if errors.As(err, &ve) {
				result.Errors = append(result.Errors, ve...)
			} else {
				result.Errors = append(result.Errors, &ValidationError{
					Field:   "composite",
					Message: err.Error(),
				})
			}
		}
	}

	result.Errors = append(result.Errors, validateMergedUniqueness(resolved)...)

	// Fanout risk analysis over active cross-model joins.
	if len(crossJoins) > 0 {
		report := NewCompositeFanoutDetector().AnalyzeFanoutRisk(resolved, crossJoins)
		result.FanoutRisk = report.RiskLevel
		result.FanoutPlan = report
		for _, f := range report.RiskFactors {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"fanout risk on join %q (%s): %s", f.JoinName, f.Cardinality, f.Impact))
		}
	}

	// Canonical date guidance: if the composite declares one, encourage its use
	// for cross-model time filtering; otherwise warn that time alignment is
	// undefined across components.
	result.HasCanonDim = canonicalDate != nil && canonicalDate.DimensionName != ""
	if !result.HasCanonDim {
		result.Warnings = append(result.Warnings,
			"composite has no canonical date dimension; cross-model time filters may be inconsistent")
	} else if !queryUsesCanonicalDate(lq, canonicalDate.DimensionName) {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"query does not reference canonical date dimension %q; consider time-bounding cross-model results",
			canonicalDate.DimensionName))
	}

	return result
}

// validateMergedUniqueness ensures the resolved model exposes each dimension and
// metric name exactly once. The resolver disambiguates duplicates, so a clash
// here indicates a resolution defect that must block compilation.
func validateMergedUniqueness(resolved *semantic.SemanticModel) ValidationErrors {
	var errs ValidationErrors
	seenDim := make(map[string]bool, len(resolved.Dimensions))
	for _, d := range resolved.Dimensions {
		if seenDim[d.Name] {
			errs = append(errs, &ValidationError{
				Field:   "dimension",
				Message: "duplicate dimension name in merged composite model: " + d.Name,
			})
		}
		seenDim[d.Name] = true
	}
	seenMetric := make(map[string]bool, len(resolved.Metrics))
	for _, m := range resolved.Metrics {
		if seenMetric[m.Name] {
			errs = append(errs, &ValidationError{
				Field:   "metric",
				Message: "duplicate metric name in merged composite model: " + m.Name,
			})
		}
		seenMetric[m.Name] = true
	}
	return errs
}

// queryUsesCanonicalDate reports whether the LogicalQuery references the named
// canonical date dimension in any select, filter, group-by or order-by clause.
func queryUsesCanonicalDate(lq *LogicalQuery, dimName string) bool {
	for _, s := range lq.Select {
		if s.Name == dimName {
			return true
		}
	}
	for _, f := range lq.Filters {
		if f.Field == dimName {
			return true
		}
	}
	for _, g := range lq.GroupBy {
		if g.Field == dimName {
			return true
		}
	}
	for _, o := range lq.OrderBy {
		if o.Field == dimName {
			return true
		}
	}
	return false
}
