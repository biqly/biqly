package ai

import (
	"strings"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// BenchmarkCases returns a BIRD-style NL→LogicalQuery→result pipeline benchmark set.
// All cases use the same in-memory orders semantic model as DefaultGoldenCases.
func BenchmarkCases() []GoldenCase {
	base := ordersGoldenModel()
	cases := DefaultGoldenCases()
	extra := []GoldenCase{
		{
			ID:       "bench-top-country-by-revenue",
			Question: "en yüksek tutarlı ülke hangisi",
			Model:    base,
			Expected: benchmarkQuery(
				[]query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "total_amount"},
				},
				[]query.GroupBy{{Field: "country"}},
				nil,
				[]query.OrderBy{{Field: "total_amount", Direction: "desc"}},
				1,
			),
		},
		{
			ID:       "bench-pending-count",
			Question: "bekleyen sipariş sayısı",
			Model:    base,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "pending"}},
				Limit:   100,
			},
		},
		{
			ID:       "bench-cancelled-total",
			Question: "iptal edilen siparişlerin tutarı",
			Model:    base,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "total_amount"}},
				Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "cancelled"}},
				Limit:   100,
			},
		},
		{
			ID:       "bench-shipped-count-by-country",
			Question: "ülkeye göre shipped sipariş adedi",
			Model:    base,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "row_count"},
				},
				Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}},
				GroupBy: []query.GroupBy{{Field: "country"}},
				Limit:   100,
			},
		},
		{
			ID:       "bench-de-shipped-sum",
			Question: "Almanya shipped sipariş tutarı",
			Model:    base,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "total_amount"}},
				Filters: []query.Filter{
					{Field: "country", Operator: "eq", Value: "DE"},
					{Field: "status", Operator: "eq", Value: "shipped"},
				},
				Limit: 100,
			},
		},
	}
	return append(cases, extra...)
}

func ordersGoldenModel() *semantic.SemanticModel {
	cases := DefaultGoldenCases()
	if len(cases) > 0 && cases[0].Model != nil {
		return cases[0].Model
	}
	return nil
}

func benchmarkQuery(selects []query.SelectItem, groupBy []query.GroupBy, filters []query.Filter, orderBy []query.OrderBy, limit int) query.LogicalQuery {
	if limit <= 0 {
		limit = 100
	}
	return query.LogicalQuery{
		Select:  selects,
		Filters: filters,
		GroupBy: groupBy,
		OrderBy: orderBy,
		Limit:   limit,
	}
}

// BenchmarkCaseIDs returns stable IDs for CI reporting.
func BenchmarkCaseIDs() []string {
	cases := BenchmarkCases()
	ids := make([]string, len(cases))
	for i, c := range cases {
		ids[i] = c.ID
	}
	return ids
}

// BenchmarkSuiteName is the identifier for full-pipeline benchmark runs.
const BenchmarkSuiteName = "biqly-benchmark-v1"

// NormalizeBenchmarkQuestion trims whitespace for stable matching in reports.
func NormalizeBenchmarkQuestion(q string) string {
	return strings.TrimSpace(q)
}
