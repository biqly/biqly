package eval

import (
	"github.com/biqly/biqly/internal/query"
)

// RepairGoldenCase pairs a golden case with a deliberately invalid first LLM
// response. It exercises the query repair loop end-to-end: a scripted provider
// returns BadFirstResponse on the first attempt (which fails semantic
// validation with a structured error code), and the repair loop must recover
// to the canonical Expected query on a subsequent attempt.
type RepairGoldenCase struct {
	GoldenCase
	// BadFirstResponse is the raw LogicalQuery JSON the model "emits" first;
	// it must fail validation so the repair path is triggered.
	BadFirstResponse string
}

// RepairGoldenCases returns golden scenarios whose first generation is wrong in
// a way that the structured-error repair loop is designed to fix: an unknown
// metric, an unknown dimension, and an invalid filter operator. All cases use
// the built-in Orders model so they run without a live datasource.
func RepairGoldenCases() []RepairGoldenCase {
	ordersModel := OrdersModel()

	return []RepairGoldenCase{
		{
			// "ciro" is colloquial for revenue; the model first guesses it as a
			// metric name (UNKNOWN_METRIC) before being repaired to total_amount.
			GoldenCase: GoldenCase{
				ID:       "repair-unknown-metric",
				Question: "toplam ciro ne kadar",
				Model:    ordersModel,
				Expected: query.LogicalQuery{
					Select: []query.SelectItem{{Type: "metric", Name: "total_amount"}},
					Limit:  100,
				},
			},
			BadFirstResponse: `{"select":[{"type":"metric","name":"ciro"}],"limit":100}`,
		},
		{
			// First guess uses a non-existent dimension name (UNKNOWN_DIMENSION),
			// repaired to the canonical "country" dimension.
			GoldenCase: GoldenCase{
				ID:       "repair-unknown-dimension",
				Question: "ülkeye göre toplam tutar",
				Model:    ordersModel,
				Expected: query.LogicalQuery{
					Select: []query.SelectItem{
						{Type: "dimension", Name: "country"},
						{Type: "metric", Name: "total_amount"},
					},
					GroupBy: []query.GroupBy{{Field: "country"}},
					Limit:   100,
				},
			},
			BadFirstResponse: `{"select":[{"type":"dimension","name":"ulke"},{"type":"metric","name":"total_amount"}],"group_by":[{"field":"ulke"}],"limit":100}`,
		},
		{
			// First guess uses an unsupported filter operator (INVALID_OPERATOR),
			// repaired to "eq".
			GoldenCase: GoldenCase{
				ID:       "repair-invalid-operator",
				Question: "shipped siparişlerin sayısı",
				Model:    ordersModel,
				Expected: query.LogicalQuery{
					Select:  []query.SelectItem{{Type: "metric", Name: "row_count"}},
					Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}},
					Limit:   100,
				},
			},
			BadFirstResponse: `{"select":[{"type":"metric","name":"row_count"}],"filters":[{"field":"status","operator":"like","value":"shipped"}],"limit":100}`,
		},
	}
}
