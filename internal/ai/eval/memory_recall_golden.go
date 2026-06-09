package eval

import (
	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/query"
)

const memoryRecallFewShotMarker = "## Examples — Successful Past Queries"

// MemoryRecallGoldenCase returns a regression case where the stub provider
// emits the correct LogicalQuery only when confirmed-query few-shot recall
// is present in the prompt.
func MemoryRecallGoldenCase() GoldenCase {
	ordersModel := OrdersModel()
	return GoldenCase{
		ID:       "memory-recall-shipped-total",
		Question: "shipped olan siparişlerin toplam tutarı",
		Model:    ordersModel,
		Expected: query.LogicalQuery{
			Select:  []query.SelectItem{{Type: "metric", Name: "total_amount"}},
			Filters: []query.Filter{{Field: "status", Operator: "eq", Value: "shipped"}},
			Limit:   100,
		},
	}
}

// MemoryRecallFewShotExample is the confirmed-query exemplar injected in the
// memory-recall regression gate.
func MemoryRecallFewShotExample() prompt.FewShotExample {
	c := MemoryRecallGoldenCase()
	raw, err := sonic.ConfigStd.Marshal(c.Expected)
	if err != nil {
		return prompt.FewShotExample{}
	}
	return prompt.FewShotExample{
		Question:     "kargoya verilen siparişlerin tutar toplamı",
		LogicalQuery: string(raw),
	}
}
