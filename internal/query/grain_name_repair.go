package query

import "strings"

// calendarGrainSuffixes lists longest-first suffixes so "_quarter" is not parsed as ending in "_ter".
var calendarGrainSuffixes = []string{
	"_year", "_quarter", "_month", "_day", "_hour",
}

// RepairMisnamedCalendarGrainDimensions fixes common LLM mistakes where a calendar
// grain dimension omits an infix on the underlying timestamp column (e.g.
// created_at_month when the model only defines created_at_ts_month).
// It only rewrites when the original name is absent and exactly one injected
// candidate exists in knownDimensions.
func RepairMisnamedCalendarGrainDimensions(lq *LogicalQuery, knownDimensions []string) {
	if lq == nil || len(knownDimensions) == 0 {
		return
	}
	dimSet := make(map[string]bool, len(knownDimensions))
	for _, n := range knownDimensions {
		dimSet[n] = true
	}

	repair := func(s string) string {
		return repairOneGrainFieldName(s, dimSet)
	}

	for i := range lq.Filters {
		lq.Filters[i].Field = repair(lq.Filters[i].Field)
	}
	for i := range lq.GroupBy {
		lq.GroupBy[i].Field = repair(lq.GroupBy[i].Field)
	}
	for i := range lq.OrderBy {
		lq.OrderBy[i].Field = repair(lq.OrderBy[i].Field)
	}
	for i := range lq.Having {
		lq.Having[i].Field = repair(lq.Having[i].Field)
	}
	for i := range lq.Select {
		if lq.Select[i].Type == SelectTypeDimension {
			lq.Select[i].Name = repair(lq.Select[i].Name)
		}
		if w := lq.Select[i].Window; w != nil {
			for j := range w.PartitionBy {
				w.PartitionBy[j] = repair(w.PartitionBy[j])
			}
			for j := range w.OrderBy {
				w.OrderBy[j].Field = repair(w.OrderBy[j].Field)
			}
		}
	}
	for ci := range lq.CTEs {
		cte := &lq.CTEs[ci]
		for i := range cte.Filters {
			cte.Filters[i].Field = repair(cte.Filters[i].Field)
		}
		for i := range cte.GroupBy {
			cte.GroupBy[i].Field = repair(cte.GroupBy[i].Field)
		}
		for i := range cte.OrderBy {
			cte.OrderBy[i].Field = repair(cte.OrderBy[i].Field)
		}
		for i := range cte.Select {
			if cte.Select[i].Type == SelectTypeDimension {
				cte.Select[i].Name = repair(cte.Select[i].Name)
			}
			if w := cte.Select[i].Window; w != nil {
				for j := range w.PartitionBy {
					w.PartitionBy[j] = repair(w.PartitionBy[j])
				}
				for j := range w.OrderBy {
					w.OrderBy[j].Field = repair(w.OrderBy[j].Field)
				}
			}
		}
	}
}

func repairOneGrainFieldName(field string, dimSet map[string]bool) string {
	if field == "" || dimSet[field] {
		return field
	}
	for _, suf := range calendarGrainSuffixes {
		if !strings.HasSuffix(field, suf) {
			continue
		}
		base := strings.TrimSuffix(field, suf)
		if base == "" {
			continue
		}
		// Common ORM / analytics naming: event time column is "<stem>_ts" while the
		// model incorrectly uses "<stem>_<grain>" (drops "_ts").
		if cand := base + "_ts" + suf; dimSet[cand] {
			return cand
		}
	}
	return field
}
