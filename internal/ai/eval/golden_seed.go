package eval

import (
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// OrdersModel DefaultGoldenCases returns the built-in text-to-SQL golden set used by HTTP
// eval endpoints and tests. Keep cases small and self-consistent — expand over
// time or load from testdata in a follow-up.
// OrdersModel returns the mock Orders semantic model used by the built-in seed cases.
func OrdersModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		ID:           "orders",
		DatasourceID: "ds-1",
		Name:         "public.orders",
		BaseSchema:   "public",
		BaseTable:    "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", Type: "text", ColumnRef: "orders.country"},
			{Name: "status", Type: "text", ColumnRef: "orders.status"},
			{Name: "customer_id", Type: "text", ColumnRef: "orders.customer_id"},
			{Name: "order_date", Type: "date", ColumnRef: "orders.order_date"},
			{Name: "order_date_year", Type: "date", ColumnRef: "orders.order_date", TimeGrain: "year"},
			{Name: "order_date_quarter", Type: "date", ColumnRef: "orders.order_date", TimeGrain: "quarter"},
			{Name: "order_date_month", Type: "date", ColumnRef: "orders.order_date", TimeGrain: "month"},
			{Name: "order_date_day", Type: "date", ColumnRef: "orders.order_date", TimeGrain: "day"},
			{Name: "order_date_hour", Type: "date", ColumnRef: "orders.order_date", TimeGrain: "hour"},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Aggregation: "count", Expression: "*"},
			{Name: "total_amount", Aggregation: "sum", Expression: "orders.amount"},
			{Name: "avg_amount", Aggregation: "avg", Expression: "orders.amount"},
			{Name: "max_amount", Aggregation: "max", Expression: "orders.amount"},
			{Name: "distinct_customers", Aggregation: "count_distinct", Expression: "orders.customer_id"},
		},
	}
}

func DefaultGoldenCases() []GoldenCase {
	ordersModel := OrdersModel()

	return []GoldenCase{
		{
			ID:       "count-orders",
			Question: "kaç sipariş var",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
				Limit:  100,
			},
		},
		{
			ID:       "orders-by-country",
			Question: "ülkeye göre sipariş sayısı",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "country"},
					{Type: "metric", Name: "row_count"},
				},
				GroupBy: []query.GroupBy{{Field: "country"}},
				Limit:   100,
			},
		},
		{
			ID:       "shipped-orders-total",
			Question: "shipped olan siparişlerin toplam tutarı",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select:  []query.SelectItem{{Type: "metric", Name: "total_amount"}},
				Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}},
				Limit:   100,
			},
		},
		{
			ID:       "sum-total-amount",
			Question: "tüm siparişlerin tutar toplamı",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{{Type: "metric", Name: "total_amount"}},
				Limit:  100,
			},
		},
		{
			ID:       "orders-by-status-count",
			Question: "duruma göre sipariş adedi",
			Model:    ordersModel,
			Expected: query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "status"},
					{Type: "metric", Name: "row_count"},
				},
				GroupBy: []query.GroupBy{{Field: "status"}},
				Limit:   100,
			},
		},
	}
}
