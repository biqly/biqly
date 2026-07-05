package eval

import (
	"time"

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
	cases = append(cases, complexTemporalCases(base)...)
	cases = append(cases, complexSegmentCases(base)...)
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

// complexTemporalCases covers relative-date windows and sub-day grains — the
// exact failure shapes seen in production ("dün ... saat dilimlerine göre").
// Yesterday is computed at suite-build time so goldens track the same
// Current Date/Time anchor the prompt embeds.
func complexTemporalCases(base *semantic.SemanticModel) []GoldenCase {
	return []GoldenCase{
		{
			ID:          "cx-yesterday-count",
			Question:    "dün kaç sipariş verildi",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Filters: yesterdayFilters(),
				Limit:   100,
			},
		},
		{
			ID:          "cx-yesterday-hourly",
			Question:    "dün saat dilimlerine göre sipariş adedi",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "order_date_hour"},
					{Type: "metric", Name: "row_count"},
				},
				Filters: yesterdayFilters(),
				GroupBy: []query.GroupBy{{Field: "order_date_hour"}},
				OrderBy: []query.OrderBy{{Field: "order_date_hour", Direction: "asc"}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-quarter-revenue",
			Question:    "2026 yılının 2. çeyreğinde toplam ciro ne kadardı",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "total_amount"}},
				Filters: []query.Filter{
					{Field: "order_date_year", Operator: "eq", Value: 2026},
					{Field: "order_date_quarter", Operator: "eq", Value: 2},
				},
				Limit: 100,
			},
		},
		{
			ID:          "cx-quarterly-breakdown",
			Question:    "çeyrek bazında sipariş adedi",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "order_date_quarter"},
					{Type: "metric", Name: "row_count"},
				},
				GroupBy: []query.GroupBy{{Field: "order_date_quarter"}},
				OrderBy: []query.OrderBy{{Field: "order_date_quarter", Direction: "asc"}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-multi-grain-revenue",
			Question:    "yıl ve ay bazında toplam ciro",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "order_date_year"},
					{Type: "dimension", Name: "order_date_month"},
					{Type: "metric", Name: "total_amount"},
				},
				GroupBy: []query.GroupBy{{Field: "order_date_year"}, {Field: "order_date_month"}},
				OrderBy: []query.OrderBy{
					{Field: "order_date_year", Direction: "asc"},
					{Field: "order_date_month", Direction: "asc"},
				},
				Limit: 100,
			},
		},
		{
			ID:          "cx-yoy-revenue-change",
			Question:    "2026 toplam cirosu 2025 yılına göre yüzde kaç değişti",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{
						Type: "formula",
						Name: "yoy_revenue_change",
						Formula: &query.FormulaSpec{
							Op:    "percent_change",
							Left:  query.MeasureRef{Metric: "total_amount", Filters: []query.Filter{{Field: "order_date_year", Operator: "eq", Value: 2026}}},
							Right: query.MeasureRef{Metric: "total_amount", Filters: []query.Filter{{Field: "order_date_year", Operator: "eq", Value: 2025}}},
						},
					},
				},
				Limit: 1,
			},
		},
		{
			ID:          "cx-top-country-in-may",
			Question:    "Mayıs 2026'da en fazla ciro yapan ülke hangisiydi",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "total_amount"},
				},
				Filters: append(monthFilters2026(5),
					query.Filter{Field: "country", Operator: "is_not_null", Value: nil},
					query.Filter{Field: "country", Operator: "neq", Value: ""},
				),
				GroupBy: []query.GroupBy{{Field: "country"}},
				OrderBy: []query.OrderBy{{Field: "total_amount", Direction: "desc"}},
				Limit:   1,
			},
		},
	}
}

// complexSegmentCases covers distinct counts, conditional aggregate columns,
// per-group extrema, ratios, and post-aggregate thresholds.
func complexSegmentCases(base *semantic.SemanticModel) []GoldenCase {
	return []GoldenCase{
		{
			ID:          "cx-distinct-customers-by-country",
			Question:    "ülke bazında kaç farklı müşteri var",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "distinct_customers"},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-conditional-columns",
			Question:    "her ülke için toplam sipariş sayısı ve shipped sipariş sayısı",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "row_count"},
					{Type: "metric", Name: "row_count", Alias: "shipped_count", Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}}},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-max-by-status",
			Question:    "statü bazında en yüksek sipariş tutarı",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "status"},
					{Type: "metric", Name: "max_amount"},
				},
				GroupBy: []query.GroupBy{{Field: "status"}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-shipped-pending-ratio",
			Question:    "shipped siparişlerin pending siparişlere oranı nedir",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{
						Type: "formula",
						Name: "shipped_pending_ratio",
						Formula: &query.FormulaSpec{
							Op:    "divide",
							Left:  query.MeasureRef{Metric: "row_count", Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}}},
							Right: query.MeasureRef{Metric: "row_count", Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "pending"}}},
						},
					},
				},
				Limit: 1,
			},
		},
		{
			ID:          "cx-having-avg-amount",
			Question:    "ortalama sipariş tutarı 100'ün üzerinde olan ülkeler",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "avg_amount"},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				Having:  []query.Filter{{Field: "avg_amount", Operator: "gt", Value: 100}},
				Limit:   100,
			},
		},
		{
			ID:          "cx-avg-amount-change",
			Question:    "ortalama sipariş tutarı Mayıs 2026'da Nisan 2026'ya kıyasla yüzde kaç değişti",
			Model:       base,
			LogicalOnly: true,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{
						Type: "formula",
						Name: "avg_amount_change",
						Formula: &query.FormulaSpec{
							Op:    "percent_change",
							Left:  query.MeasureRef{Metric: "avg_amount", Filters: monthFilters2026(5)},
							Right: query.MeasureRef{Metric: "avg_amount", Filters: monthFilters2026(4)},
						},
					},
				},
				Limit: 1,
			},
		},
	}
}

// yesterdayFilters returns the canonical integer grain trio for "dün",
// anchored to the same wall clock the prompt's Current Date/Time uses.
func yesterdayFilters() []query.Filter {
	y := time.Now().AddDate(0, 0, -1)
	return []query.Filter{
		{Field: "order_date_year", Operator: "eq", Value: y.Year()},
		{Field: "order_date_month", Operator: "eq", Value: int(y.Month())},
		{Field: "order_date_day", Operator: "eq", Value: y.Day()},
	}
}
