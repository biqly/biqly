package eval

import (
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// ComplexCases returns logical-only compositional questions for the nightly
// live-LLM suite. Execution is skipped because MemoryResultExecutor does not
// yet evaluate grains, formulas, or HAVING clauses.
func ComplexCases() []GoldenCase {
	base := OrdersModel()
	cases := complexFormulaCases(base)
	cases = append(cases, complexBreakdownCases(base)...)
	cases = append(cases, complexRankingCases(base)...)
	return cases
}

func complexFormulaCases(base *semantic.SemanticModel) []GoldenCase {
	return []GoldenCase{
		{
			ID:          "cx-month-diff",
			Question:    "Mayıs 2026 ile Nisan 2026 arasındaki sipariş adedi farkı",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{
						Type: "formula",
						Name: "order_count_diff",
						Formula: &query.FormulaSpec{
							Op:    "subtract",
							Left:  query.MeasureRef{Metric: "row_count", Filters: monthFilters2026(5)},
							Right: query.MeasureRef{Metric: "row_count", Filters: monthFilters2026(4)},
						},
					},
				},
				Limit: 1,
			},
		},
		{
			ID:          "cx-month-growth",
			Question:    "Nisan 2026'ya göre Mayıs 2026 cirosu yüzde kaç değişti",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{
						Type: "formula",
						Name: "revenue_percent_change",
						Formula: &query.FormulaSpec{
							Op:    "percent_change",
							Left:  query.MeasureRef{Metric: "total_amount", Filters: monthFilters2026(5)},
							Right: query.MeasureRef{Metric: "total_amount", Filters: monthFilters2026(4)},
						},
					},
				},
				Limit: 1,
			},
		},
		{
			ID:          "cx-shipped-share",
			Question:    "shipped siparişlerin tüm siparişler içindeki payı yüzde kaç",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{
						Type: "formula",
						Name: "shipped_order_share",
						Formula: &query.FormulaSpec{
							Op:    "percent_of",
							Left:  query.MeasureRef{Metric: "row_count", Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}}},
							Right: query.MeasureRef{Metric: "row_count"},
						},
					},
				},
				Limit: 1,
			},
		},
	}
}

func complexBreakdownCases(base *semantic.SemanticModel) []GoldenCase {
	return []GoldenCase{
		{
			ID:          "cx-monthly-revenue",
			Question:    "aylara göre ciro",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "order_date_month"},
					{Type: "metric", Name: "total_amount"},
				},
				GroupBy: []query.GroupBy{{Field: "order_date_month"}},
				OrderBy: []query.OrderBy{{Field: "order_date_month", Direction: "asc"}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-monthly-count-2026",
			Question:    "2026 yılında ay bazında sipariş adedi",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "order_date_month"},
					{Type: "metric", Name: "row_count"},
				},
				Filters: []query.Filter{{Field: "order_date_year", Operator: "eq", Value: 2026}},
				GroupBy: []query.GroupBy{{Field: "order_date_month"}},
				OrderBy: []query.OrderBy{{Field: "order_date_month", Direction: "asc"}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-having-countries",
			Question:    "toplam cirosu 150'yi geçen ülkeler",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "total_amount"},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				Having:  []query.Filter{{Field: "total_amount", Operator: "gt", Value: 150}},
				Limit:   100,
			},
		},
	}
}

func complexRankingCases(base *semantic.SemanticModel) []GoldenCase {
	return []GoldenCase{
		{
			ID:          "cx-top2-countries",
			Question:    "ciroya göre ilk 2 ülke",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "total_amount"},
				},
				Filters: []query.Filter{
					{Field: "country", Operator: "is_not_null", Value: nil},
					{Field: "country", Operator: "neq", Value: ""},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				OrderBy: []query.OrderBy{{Field: "total_amount", Direction: "desc"}},
				Limit:   2,
			},
		},
		{
			ID:          "cx-busiest-day",
			Question:    "Mayıs 2026'da en çok sipariş alınan gün hangisi",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "order_date_day"},
					{Type: "metric", Name: "row_count"},
				},
				Filters: []query.Filter{
					{Field: "order_date_year", Operator: "eq", Value: 2026},
					{Field: "order_date_month", Operator: "eq", Value: 5},
					{Field: "order_date_day", Operator: "is_not_null", Value: nil},
				},
				GroupBy: []query.GroupBy{{Field: "order_date_day"}},
				OrderBy: []query.OrderBy{{Field: "row_count", Direction: "desc"}},
				Limit:   1,
			},
		},
		{
			ID:          "cx-avg-by-country-sorted",
			Question:    "ülke bazında ortalama sipariş tutarı, en yüksekten düşüğe",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "avg_amount"},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				OrderBy: []query.OrderBy{{Field: "avg_amount", Direction: "desc"}},
				Limit:   100,
			},
		},
	}
}

func monthFilters2026(month int) []query.Filter {
	return []query.Filter{
		{Field: "order_date_year", Operator: "eq", Value: 2026},
		{Field: "order_date_month", Operator: "eq", Value: month},
	}
}
