package query

import (
	"github.com/biqly/biqly/internal/semantic"
)

// RepairRawTimestampDayEqualityFilters rewrites eq/neq filters on a raw
// date/timestamp dimension with a YYYY-MM-DD value to the sibling *_day grain
// dimension when the model defines one (e.g. created_at_ts → created_at_ts_day).
func RepairRawTimestampDayEqualityFilters(lq *LogicalQuery, model *semantic.SemanticModel) {
	if lq == nil || model == nil {
		return
	}
	dimByName := make(map[string]*semantic.Dimension, len(model.Dimensions))
	for i := range model.Dimensions {
		dimByName[model.Dimensions[i].Name] = &model.Dimensions[i]
	}
	for i := range lq.Filters {
		repairRawTimestampDayEqualityFilter(&lq.Filters[i], dimByName)
	}
	for ci := range lq.CTEs {
		for i := range lq.CTEs[ci].Filters {
			repairRawTimestampDayEqualityFilter(&lq.CTEs[ci].Filters[i], dimByName)
		}
	}
}

func repairRawTimestampDayEqualityFilter(f *Filter, dimByName map[string]*semantic.Dimension) {
	if f == nil {
		return
	}
	if f.Operator != OpEq && f.Operator != OpNeq {
		return
	}
	if !isDateOnlyCalendarValue(f.Value) {
		return
	}
	dim, ok := dimByName[f.Field]
	if !ok || !isRawDateTimestampDimension(dim) {
		return
	}
	dayField := f.Field + "_day"
	if _, hasDay := dimByName[dayField]; hasDay {
		f.Field = dayField
	}
}
