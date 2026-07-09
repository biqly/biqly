package query

import (
	"errors"
	"fmt"
	"strings"
)

// StructuredMetricQuery is a programmatic metric query that does not require
// natural language. It compiles to a LogicalQuery against a semantic model.
type StructuredMetricQuery struct {
	DatasourceID  string             `json:"datasource_id"`
	ModelID       string             `json:"model_id"`
	Measures      []string           `json:"measures"`
	Dimensions    []string           `json:"dimensions,omitempty"`
	TimeDimension *TimeDimension     `json:"time_dimension,omitempty"`
	Filters       []StructuredFilter `json:"filters,omitempty"`
	Sort          []StructuredSort   `json:"sort,omitempty"`
	Limit         int                `json:"limit,omitempty"`
	Offset        int                `json:"offset,omitempty"`
}

// TimeDimension buckets a date dimension by grain and optionally filters a range.
type TimeDimension struct {
	Dimension string     `json:"dimension"`
	Grain     string     `json:"grain"` // day, week, month, quarter, year, hour
	DateRange *DateRange `json:"date_range,omitempty"`
}

// DateRange is an inclusive start/end filter on a time dimension.
type DateRange struct {
	Start any `json:"start,omitempty"`
	End   any `json:"end,omitempty"`
}

// StructuredFilter is a field/operator/value predicate.
type StructuredFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

// StructuredSort orders by a measure or dimension field.
type StructuredSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction,omitempty"` // asc|desc; default asc
}

// ToLogicalQuery converts a structured metric query into a LogicalQuery.
// It does not resolve names against a model — that happens in the normal
// compile/validate pipeline.
func (q StructuredMetricQuery) ToLogicalQuery() (LogicalQuery, error) {
	if err := q.validateRequired(); err != nil {
		return LogicalQuery{}, err
	}

	lq := LogicalQuery{
		Version:      CurrentLogicalQueryVersion,
		DatasourceID: q.DatasourceID,
		ModelID:      q.ModelID,
		Select:       make([]SelectItem, 0, len(q.Measures)+len(q.Dimensions)+1),
		Filters:      make([]Filter, 0, len(q.Filters)+2),
		GroupBy:      make([]GroupBy, 0, len(q.Dimensions)+1),
		OrderBy:      make([]OrderBy, 0, len(q.Sort)),
		Limit:        q.Limit,
		Offset:       q.Offset,
	}
	seen := map[string]bool{}

	for _, d := range q.Dimensions {
		if err := appendDimension(&lq, seen, d, ""); err != nil {
			return LogicalQuery{}, err
		}
	}
	if err := appendTimeDimension(&lq, seen, q.TimeDimension); err != nil {
		return LogicalQuery{}, err
	}
	if err := appendMeasures(&lq, seen, q.Measures); err != nil {
		return LogicalQuery{}, err
	}
	filters, err := buildStructuredFilters(q.Filters)
	if err != nil {
		return LogicalQuery{}, err
	}
	lq.Filters = append(lq.Filters, filters...)
	orderBy, err := buildStructuredSort(q.Sort)
	if err != nil {
		return LogicalQuery{}, err
	}
	lq.OrderBy = orderBy
	if lq.Limit <= 0 {
		lq.Limit = 1000
	}
	return lq, nil
}

func (q StructuredMetricQuery) validateRequired() error {
	if strings.TrimSpace(q.DatasourceID) == "" {
		return errors.New("datasource_id is required")
	}
	if strings.TrimSpace(q.ModelID) == "" {
		return errors.New("model_id is required")
	}
	hasTime := q.TimeDimension != nil && strings.TrimSpace(q.TimeDimension.Dimension) != ""
	if len(q.Measures) == 0 && len(q.Dimensions) == 0 && !hasTime {
		return errors.New("at least one measure or dimension is required")
	}
	return nil
}

func appendDimension(lq *LogicalQuery, seen map[string]bool, name, grain string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("dimension name is empty")
	}
	key := strings.ToLower(name)
	if !seen[key] {
		lq.Select = append(lq.Select, SelectItem{Type: SelectTypeDimension, Name: name})
		seen[key] = true
	}
	gb := GroupBy{Field: name}
	if grain != "" {
		if !IsValidTimeGrain(grain) {
			return fmt.Errorf("invalid time grain %q", grain)
		}
		gb.TimeGrain = grain
	}
	for _, existing := range lq.GroupBy {
		if strings.EqualFold(existing.Field, name) {
			return nil
		}
	}
	lq.GroupBy = append(lq.GroupBy, gb)
	return nil
}

func appendTimeDimension(lq *LogicalQuery, seen map[string]bool, td *TimeDimension) error {
	if td == nil || strings.TrimSpace(td.Dimension) == "" {
		return nil
	}
	if err := appendDimension(lq, seen, td.Dimension, td.Grain); err != nil {
		return err
	}
	lq.Filters = append(lq.Filters, dateRangeFilters(td.Dimension, td.DateRange)...)
	return nil
}

func dateRangeFilters(field string, dr *DateRange) []Filter {
	if dr == nil {
		return nil
	}
	field = strings.TrimSpace(field)
	if dr.Start != nil && dr.End != nil {
		return []Filter{{Field: field, Operator: OpBetween, Value: []any{dr.Start, dr.End}}}
	}
	out := make([]Filter, 0, 2)
	if dr.Start != nil {
		out = append(out, Filter{Field: field, Operator: OpGte, Value: dr.Start})
	}
	if dr.End != nil {
		out = append(out, Filter{Field: field, Operator: OpLte, Value: dr.End})
	}
	return out
}

func appendMeasures(lq *LogicalQuery, seen map[string]bool, measures []string) error {
	for _, m := range measures {
		m = strings.TrimSpace(m)
		if m == "" {
			return errors.New("measure name is empty")
		}
		key := strings.ToLower(m)
		if seen[key] {
			continue
		}
		lq.Select = append(lq.Select, SelectItem{Type: SelectTypeMetric, Name: m})
		seen[key] = true
	}
	return nil
}

func buildStructuredFilters(filters []StructuredFilter) ([]Filter, error) {
	out := make([]Filter, 0, len(filters))
	for _, f := range filters {
		field := strings.TrimSpace(f.Field)
		op := strings.TrimSpace(f.Operator)
		if field == "" || op == "" {
			return nil, errors.New("filter field and operator are required")
		}
		if !isValidStructuredOperator(op) {
			return nil, fmt.Errorf("unsupported filter operator %q", op)
		}
		out = append(out, Filter{Field: field, Operator: op, Value: f.Value})
	}
	return out, nil
}

func buildStructuredSort(sort []StructuredSort) ([]OrderBy, error) {
	out := make([]OrderBy, 0, len(sort))
	for _, s := range sort {
		field := strings.TrimSpace(s.Field)
		if field == "" {
			return nil, errors.New("sort field is empty")
		}
		dir := strings.ToLower(strings.TrimSpace(s.Direction))
		if dir == "" {
			dir = OrderAsc
		}
		if dir != OrderAsc && dir != OrderDesc {
			return nil, fmt.Errorf("invalid sort direction %q", s.Direction)
		}
		out = append(out, OrderBy{Field: field, Direction: dir})
	}
	return out, nil
}

func isValidStructuredOperator(op string) bool {
	switch op {
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte,
		OpIn, OpNotIn, OpContains, OpStartsWith, OpEndsWith,
		OpBetween, OpIsNull, OpIsNotNull, OpIsEmpty, OpIsNotEmpty:
		return true
	default:
		return false
	}
}
