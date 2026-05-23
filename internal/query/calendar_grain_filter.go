package query

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/semantic"
)

// grainStemBeforeSuffix returns ("foo_created_at_ts", true) for suffix "_month" and name "foo_created_at_ts_month".
func grainStemBeforeSuffix(dimName, suffix string) (stem string, ok bool) {
	if !strings.HasSuffix(dimName, suffix) {
		return "", false
	}
	return strings.TrimSuffix(dimName, suffix), true
}

// calendarAnchorTime parses filter values that pin a specific calendar month or
// instant (RFC3339, YYYY-MM-DD, YYYY-MM, etc.). Used to compile month/quarter
// grain filters as DATE_TRUNC instead of bare EXTRACT parts.
func calendarAnchorTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return time.Time{}, false
		}
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02 15:04:05",
			"2006-01-02",
			"2006-01",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

// isScalarCompareOp reports whether a filter op is a single-scalar comparison.
func isScalarCompareOp(op string) bool {
	switch op {
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte:
		return true
	default:
		return false
	}
}

// bareIntegerMonthOrQuarter reports a 1–12 month or 1–4 quarter filter value
// that does not carry a calendar year and therefore needs a *_year sibling.
func bareIntegerMonthOrQuarter(grain string, v any) bool {
	if !isNumericFilterValue(v) {
		return false
	}
	var f float64
	switch n := v.(type) {
	case int:
		f = float64(n)
	case int32:
		f = float64(n)
	case int64:
		f = float64(n)
	case float64:
		f = n
	case json.Number:
		var err error
		f, err = n.Float64()
		if err != nil {
			return false
		}
	default:
		return false
	}
	if math.Trunc(f) != f {
		return false
	}
	iv := int(f)
	switch grain {
	case TimeGrainMonth:
		return iv >= 1 && iv <= 12
	case TimeGrainQuarter:
		return iv >= 1 && iv <= 4
	default:
		return false
	}
}

func filterTouchesField(filters []Filter, field string) bool {
	for i := range filters {
		if filters[i].Field == field {
			return true
		}
	}
	return false
}

// validateCalendarGrainYearCoverage rejects EXTRACT(month)=4 style filters when
// no *_year filter is present — "April 2026" cannot be represented by month
// integer alone.
func validateCalendarGrainYearCoverage(lq LogicalQuery, model *semantic.SemanticModel) *ValidationError {
	dimByName := make(map[string]semantic.Dimension, len(model.Dimensions))
	for i := range model.Dimensions {
		d := model.Dimensions[i]
		dimByName[d.Name] = d
	}
	for _, f := range lq.Filters {
		dim, ok := dimByName[f.Field]
		if !ok || !isScalarCompareOp(f.Operator) {
			continue
		}
		g := strings.ToLower(strings.TrimSpace(dim.TimeGrain))
		if g != TimeGrainMonth && g != TimeGrainQuarter {
			continue
		}
		if _, ok := calendarAnchorTime(f.Value); ok {
			continue
		}
		if !bareIntegerMonthOrQuarter(g, f.Value) {
			continue
		}
		var suffix string
		switch g {
		case TimeGrainMonth:
			suffix = "_month"
		case TimeGrainQuarter:
			suffix = "_quarter"
		default:
			continue
		}
		stem, ok := grainStemBeforeSuffix(f.Field, suffix)
		if !ok || stem == "" {
			continue
		}
		yearField := stem + "_year"
		if _, hasYearDim := dimByName[yearField]; !hasYearDim {
			continue
		}
		if filterTouchesField(lq.Filters, yearField) {
			continue
		}
		return &ValidationError{
			Field: "filters",
			Message: fmt.Sprintf(
				"ambiguous calendar %s filter on %q (numeric value without year): add a filter on dimension %q (e.g. eq 2026), "+
					"or replace the %q filter value with an ISO calendar anchor like \"2026-04-01\"",
				g, f.Field, yearField, f.Field,
			),
		}
	}
	return nil
}

// monthGrainFilterUsesDateTrunc reports whether we should compare
// DATE_TRUNC('month', col) to a timestamptz parameter instead of EXTRACT(MONTH).
func monthGrainFilterUsesDateTrunc(dim semantic.Dimension, f Filter) bool {
	if strings.ToLower(strings.TrimSpace(dim.TimeGrain)) != TimeGrainMonth {
		return false
	}
	if !isScalarCompareOp(f.Operator) {
		return false
	}
	_, ok := calendarAnchorTime(f.Value)
	return ok
}

// quarterGrainFilterUsesDateTrunc is the quarter analogue of monthGrainFilterUsesDateTrunc.
func quarterGrainFilterUsesDateTrunc(dim semantic.Dimension, f Filter) bool {
	if strings.ToLower(strings.TrimSpace(dim.TimeGrain)) != TimeGrainQuarter {
		return false
	}
	if !isScalarCompareOp(f.Operator) {
		return false
	}
	_, ok := calendarAnchorTime(f.Value)
	return ok
}

func (c *Compiler) dateTruncCompareExpr(part, columnRef, op string, argIndex int) (string, error) {
	part = strings.ToLower(strings.TrimSpace(part))
	if part != "month" && part != "quarter" {
		return "", fmt.Errorf("date_trunc compare: unsupported part %q", part)
	}
	lhs := c.dialect.DateTrunc(part, columnRef)
	rhs := c.dialect.DateTruncPlaceholder(part, c.dialect.Placeholder(argIndex))
	cmp := sqlComparator(op)
	return fmt.Sprintf("%s %s %s", lhs, cmp, rhs), nil
}
