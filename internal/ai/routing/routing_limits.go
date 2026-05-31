package routing

// RoutingLimits caps auto-generated semantic models before they reach the LLM prompt.
// Zero values fall back to DefaultRoutingLimits().
type RoutingLimits struct {
	MaxDimensions      int
	MaxMetrics         int
	MaxColumnsPerTable int
	MaxDateGrainExtras int
	// SlimNumericMetrics emits only sum_ and max_ per numeric column (skips avg_/min_)
	// to cut metric fan-out on wide auto-routed tables.
	SlimNumericMetrics bool
}

// DefaultRoutingLimits returns conservative caps tuned for NL→query prompts.
func DefaultRoutingLimits() RoutingLimits {
	return RoutingLimits{
		MaxDimensions:      56,
		MaxMetrics:         32,
		MaxColumnsPerTable: 14,
		MaxDateGrainExtras: 20,
		SlimNumericMetrics: true,
	}
}

func (l RoutingLimits) withDefaults() RoutingLimits {
	d := DefaultRoutingLimits()
	if l.MaxDimensions <= 0 {
		l.MaxDimensions = d.MaxDimensions
	}
	if l.MaxMetrics <= 0 {
		l.MaxMetrics = d.MaxMetrics
	}
	if l.MaxColumnsPerTable <= 0 {
		l.MaxColumnsPerTable = d.MaxColumnsPerTable
	}
	if l.MaxDateGrainExtras <= 0 {
		l.MaxDateGrainExtras = d.MaxDateGrainExtras
	}
	return l
}

// RoutingLimitsFromConfig maps BI_AI_ROUTE_* env settings onto RoutingLimits.
func RoutingLimitsFromConfig(maxDims, maxMetrics, maxCols, maxDateGrains int, slimNumeric bool) RoutingLimits {
	return RoutingLimits{
		MaxDimensions:      maxDims,
		MaxMetrics:         maxMetrics,
		MaxColumnsPerTable: maxCols,
		MaxDateGrainExtras: maxDateGrains,
		SlimNumericMetrics: slimNumeric,
	}.withDefaults()
}
