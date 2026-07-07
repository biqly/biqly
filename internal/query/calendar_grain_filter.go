package query

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/errmsg"
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
func isDateOnlyCalendarValue(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func isRawDateTimestampDimension(dim *semantic.Dimension) bool {
	if dim == nil || strings.TrimSpace(dim.TimeGrain) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(dim.Type)) {
	case "date", "timestamp", "datetime":
		return true
	default:
		return false
	}
}

func rawDateColumnDayEqualityFilter(dim *semantic.Dimension, f Filter) bool {
	if !isRawDateTimestampDimension(dim) {
		return false
	}
	if f.Operator != OpEq && f.Operator != OpNeq {
		return false
	}
	return isDateOnlyCalendarValue(f.Value)
}

func calendarAnchorTime(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	s = strings.TrimSpace(s)
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

// numericFilterValueInt coerces a (possibly JSON-decoded) numeric filter value
// to a whole int. Returns false for non-numeric or fractional values.
func numericFilterValueInt(v any) (int, bool) {
	if !isNumericFilterValue(v) {
		return 0, false
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
			return 0, false
		}
	default:
		return 0, false
	}
	if math.Trunc(f) != f {
		return 0, false
	}
	return int(f), true
}

// bareIntegerMonthOrQuarter reports a 1–12 month or 1–4 quarter filter value
// that does not carry a calendar year and therefore needs a *_year sibling.
func bareIntegerMonthOrQuarter(grain string, v any) bool {
	iv, ok := numericFilterValueInt(v)
	if !ok {
		return false
	}
	switch grain {
	case TimeGrainMonth:
		return iv >= 1 && iv <= 12
	case TimeGrainQuarter:
		return iv >= 1 && iv <= 4
	default:
		return false
	}
}

// scalarFilterValues flattens a filter's comparable value(s): the scalar for
// eq/neq/gt/gte/lt/lte, both bounds for between, and every element for in/not_in.
// Returns false for ops that carry no comparable value (is_null etc.).
func scalarFilterValues(f Filter) ([]any, bool) {
	switch f.Operator {
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte:
		return []any{f.Value}, true
	case OpBetween, OpIn, OpNotIn:
		vals, ok := f.Value.([]any)
		if !ok {
			return nil, false
		}
		return vals, true
	default:
		return nil, false
	}
}

// dayGrainBareIntegerFilter reports a *_day grain filter whose every value is a
// bare day-of-month integer (1–31). The prompt's canonical single-day form is
// the integer trio (*_year eq + *_month eq + *_day eq), so these must compile
// as EXTRACT(DAY …) comparisons — the day grain's default DATE_TRUNC('day')
// shape compares against a timestamptz parameter and cannot bind an integer.
func dayGrainBareIntegerFilter(dim *semantic.Dimension, f Filter) bool {
	if dim == nil || strings.ToLower(strings.TrimSpace(dim.TimeGrain)) != TimeGrainDay {
		return false
	}
	vals, ok := scalarFilterValues(f)
	if !ok || len(vals) == 0 {
		return false
	}
	for _, v := range vals {
		iv, ok := numericFilterValueInt(v)
		if !ok || iv < 1 || iv > 31 {
			return false
		}
	}
	return true
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
func validateCalendarGrainYearCoverage(lq *LogicalQuery, model *semantic.SemanticModel) *ValidationError {
	dimByName := make(map[string]*semantic.Dimension, len(model.Dimensions))
	for i := range model.Dimensions {
		dimByName[model.Dimensions[i].Name] = &model.Dimensions[i]
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
			Field:               "filters",
			Code:                errmsg.CodeAmbiguousYearCoverage,
			Message:             fmt.Sprintf("ambiguous calendar %s filter on %q (numeric value without year): add a filter on dimension %q (e.g. eq 2026), or replace the %q filter value with an ISO calendar anchor like \"2026-04-01\"", g, f.Field, yearField, f.Field),
			Value:               f.Field,
			AllowedAlternatives: []string{yearField},
		}
	}
	return nil
}

// dayGrainFilterUsesDateTrunc reports whether a *_day grain filter should compare
// DATE_TRUNC('day', col) to a truncated bind parameter instead of a bare string.
func dayGrainFilterUsesDateTrunc(dim *semantic.Dimension, f Filter) bool {
	if strings.ToLower(strings.TrimSpace(dim.TimeGrain)) != TimeGrainDay {
		return false
	}
	if !isScalarCompareOp(f.Operator) {
		return false
	}
	_, ok := calendarAnchorTime(f.Value)
	return ok
}

// monthGrainFilterUsesDateTrunc reports whether we should compare
// DATE_TRUNC('month', col) to a timestamptz parameter instead of EXTRACT(MONTH).
func monthGrainFilterUsesDateTrunc(dim *semantic.Dimension, f Filter) bool {
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
func quarterGrainFilterUsesDateTrunc(dim *semantic.Dimension, f Filter) bool {
	if strings.ToLower(strings.TrimSpace(dim.TimeGrain)) != TimeGrainQuarter {
		return false
	}
	if !isScalarCompareOp(f.Operator) {
		return false
	}
	_, ok := calendarAnchorTime(f.Value)
	return ok
}

// hourGrainFilterUsesDateTrunc reports whether an *_hour grain filter carries a
// calendar anchor value (date or timestamp). The hour grain renders its
// dimension as EXTRACT(HOUR FROM col) (an integer), so binding a date/timestamp
// string to it fails with 22P02 ("invalid input syntax for type integer"). This
// happens when a GROUP BY hour override is applied to a dimension that also
// carries a "yesterday"-style date filter (e.g. "hourly breakdown of yesterday").
// Such filters must compare via DATE_TRUNC instead — see calendarGrainFilterExpr
// for the day-vs-hour truncation choice.
func hourGrainFilterUsesDateTrunc(dim *semantic.Dimension, f Filter) bool {
	if strings.ToLower(strings.TrimSpace(dim.TimeGrain)) != TimeGrainHour {
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
	if part != "day" && part != "month" && part != "quarter" && part != "hour" {
		return "", fmt.Errorf("date_trunc compare: unsupported part %q", part)
	}
	lhs := c.dialect.DateTrunc(part, columnRef)
	rhs := c.dialect.DateTruncPlaceholder(part, c.dialect.Placeholder(argIndex))
	cmp, err := sqlComparator(op)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s %s", lhs, cmp, rhs), nil
}

func (c *Compiler) castColumnAsDate(columnRef string) string {
	quoted := c.dialect.QuoteIdent(columnRef)
	switch c.dialect.Name() {
	case "mysql":
		return fmt.Sprintf("DATE(%s)", quoted)
	case "clickhouse":
		return fmt.Sprintf("toDate(%s)", quoted)
	case "sqlite":
		return fmt.Sprintf("date(%s)", quoted)
	default:
		return fmt.Sprintf("CAST(%s AS %s)", quoted, c.dialect.CastType("date"))
	}
}

func (c *Compiler) rawDateDayFilterExpr(
	f Filter,
	dimMap map[string]*semantic.Dimension,
	resolver *SchemaResolver,
	args *[]any,
) (string, bool, error) {
	dim, ok := dimMap[f.Field]
	if !ok || !rawDateColumnDayEqualityFilter(dim, f) {
		return "", false, nil
	}
	colRef := resolver.PhysicalColumnRef(dim.ColumnRef)
	lhs := c.castColumnAsDate(colRef)
	cmp, err := sqlComparator(f.Operator)
	if err != nil {
		return "", false, err
	}
	*args = append(*args, f.Value)
	ph := c.dialect.Placeholder(len(*args))
	return fmt.Sprintf("%s %s %s", lhs, cmp, ph), true, nil
}
