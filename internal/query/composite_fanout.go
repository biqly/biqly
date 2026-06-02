package query

import (
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

// Composite fanout risk levels.
const (
	FanoutRiskLow      = "low"
	FanoutRiskMedium   = "medium"
	FanoutRiskHigh     = "high"
	FanoutRiskCritical = "critical"
)

// RiskFactor describes a single contributor to composite fanout risk.
type RiskFactor struct {
	JoinName        string   `json:"join_name"`
	Cardinality     string   `json:"cardinality"`
	Impact          string   `json:"impact"`
	AffectedMetrics []string `json:"affected_metrics,omitempty"`
}

// FanoutReport summarises the multiplication risk of a composite model's
// cross-model joins.
type FanoutReport struct {
	RiskLevel            string       `json:"risk_level"`
	RiskFactors          []RiskFactor `json:"risk_factors,omitempty"`
	SuggestedMitigations []string     `json:"suggested_mitigations,omitempty"`
}

// CompositeFanoutDetector analyses cross-model joins on a merged model to
// estimate aggregation inflation risk.
type CompositeFanoutDetector struct{}

// NewCompositeFanoutDetector constructs a CompositeFanoutDetector.
func NewCompositeFanoutDetector() *CompositeFanoutDetector {
	return &CompositeFanoutDetector{}
}

// AnalyzeFanoutRisk evaluates the cross-model joins of a composite model
// against the merged model's metrics and returns a risk report.
func (d *CompositeFanoutDetector) AnalyzeFanoutRisk(
	resolved *semantic.SemanticModel,
	crossJoins []semantic.CrossModelJoin,
) FanoutReport {
	var factors []RiskFactor
	oneToManyCount := 0
	manyToManyCount := 0

	metricNames := make([]string, 0, len(resolved.Metrics))
	for _, m := range resolved.Metrics {
		metricNames = append(metricNames, m.Name)
	}

	for _, cj := range crossJoins {
		if !cj.IsActive {
			continue
		}
		switch cj.Relationship {
		case semantic.RelationshipManyToMany:
			manyToManyCount++
			factors = append(factors, RiskFactor{
				JoinName:        cj.Name,
				Cardinality:     cj.Relationship,
				Impact:          "row multiplication across both sides inflates all additive metrics",
				AffectedMetrics: metricNames,
			})
		case semantic.RelationshipOneToMany:
			oneToManyCount++
			factors = append(factors, RiskFactor{
				JoinName:        cj.Name,
				Cardinality:     cj.Relationship,
				Impact:          "duplicates primary-side rows; additive metrics may be over-counted",
				AffectedMetrics: metricNames,
			})
		}
	}

	report := FanoutReport{
		RiskLevel:   classifyFanoutRisk(oneToManyCount, manyToManyCount),
		RiskFactors: factors,
	}
	report.SuggestedMitigations = suggestMitigations(oneToManyCount, manyToManyCount)
	return report
}

func classifyFanoutRisk(oneToMany, manyToMany int) string {
	switch {
	case manyToMany > 0:
		return FanoutRiskCritical
	case oneToMany > 1:
		return FanoutRiskHigh
	case oneToMany == 1:
		return FanoutRiskMedium
	default:
		return FanoutRiskLow
	}
}

func suggestMitigations(oneToMany, manyToMany int) []string {
	var out []string
	if manyToMany > 0 {
		out = append(out,
			"pre-aggregate one side in a CTE before joining",
			"use COUNT(DISTINCT ...) for entity counts to avoid inflation",
		)
	}
	if oneToMany > 1 {
		out = append(out,
			fmt.Sprintf("avoid combining %d one-to-many cross-model joins in a single aggregation", oneToMany),
			"aggregate each branch separately, then join the aggregates",
		)
	}
	return out
}

// FanoutReportSummary renders a one-line human-readable summary, useful for
// warnings surfaced to callers.
func FanoutReportSummary(r FanoutReport) string {
	if len(r.RiskFactors) == 0 {
		return "cross-model fanout risk: low"
	}
	names := make([]string, 0, len(r.RiskFactors))
	for _, f := range r.RiskFactors {
		names = append(names, f.JoinName)
	}
	return fmt.Sprintf("cross-model fanout risk: %s [%s]", r.RiskLevel, strings.Join(names, ", "))
}
