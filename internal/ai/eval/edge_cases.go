package eval

import (
	"github.com/biqly/biqly/internal/query"
)

// EdgeCases returns dialect variants and operator edge scenarios on the orders model.
func EdgeCases() []GoldenCase {
	base := ordersGoldenModel()
	return []GoldenCase{
		{
			ID:       "edge-multistatus-in",
			Question: "pending veya cancelled sipariş sayısı",
			Model:    base,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Filters: []query.Filter{
					{Field: "status", Operator: "in", Value: []any{"pending", "cancelled"}},
				},
				Limit: 100,
			},
		},
		{
			ID:       "edge-english-total-count",
			Question: "how many orders are there",
			Model:    base,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Limit:  100,
			},
		},
		{
			ID:       "edge-colloquial-count",
			Question: "toplam kaç tane sipariş",
			Model:    base,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Limit:  100,
			},
		},
		{
			ID:       "edge-lowest-country-amount",
			Question: "en düşük tutarlı ülke hangisi",
			Model:    base,
			Expected: benchmarkQuery(
				[]query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "total_amount"},
				},
				[]query.GroupBy{{Field: "country"}},
				nil,
				[]query.OrderBy{{Field: "total_amount", Direction: "asc"}},
				1,
			),
		},
		{
			ID:       "edge-tr-shipped-colloquial",
			Question: "kargolanan siparişlerin toplam tutarı",
			Model:    base,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "total_amount"}},
				Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}},
				Limit:   100,
			},
		},
		{
			ID:       "edge-country-starts-with-d",
			Question: "ülkesi D ile başlayan sipariş sayısı",
			Model:    base,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Filters: []query.Filter{{Field: "country", Operator: "starts_with", Value: "D"}},
				Limit:   100,
			},
		},
		{
			ID:       "edge-status-contains-ship",
			Question: "durumu ship içeren sipariş adedi",
			Model:    base,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Filters: []query.Filter{{Field: "status", Operator: "contains", Value: "ship"}},
				Limit:   100,
			},
		},
	}
}

// NightlyCases is the full live-LLM nightly suite: benchmark + dialect/edge cases.
func NightlyCases() []GoldenCase {
	return append(BenchmarkCases(), EdgeCases()...)
}

// NightlySuiteName identifies persisted runs and baseline snapshots for nightly eval.
const NightlySuiteName = "biqly-nightly-v1"
