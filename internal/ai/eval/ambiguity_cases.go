package eval

import (
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

// AmbiguityCase captures whether a question should stop for clarification
// before LogicalQuery generation.
type AmbiguityCase struct {
	ID                string
	Question          string
	Model             *semantic.SemanticModel
	Glossary          []prompt.GlossaryEntry
	ExpectedAmbiguous bool
}

// AmbiguityCases returns the built-in clarification evaluation set.
// JSON-backed regression cases live in testdata/ambiguity_golden.json; see
// AmbiguityGoldenCase and LoadDefaultAmbiguityGoldenCases for add-case steps.
func AmbiguityCases() []AmbiguityCase {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Type: string(semantic.DimensionTypeDate)},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count"},
			{Name: "revenue"},
		},
	}
	glossary := []prompt.GlossaryEntry{
		{Term: "aktif müşteriler", Definition: "Yakın dönemde sipariş veren müşteriler", MapsToType: "dimension", MapsToName: "recent_customer"},
		{Term: "aktif müşteriler", Definition: "Hesabı kapatılmamış müşteriler", MapsToType: "dimension", MapsToName: "enabled_customer"},
	}

	return []AmbiguityCase{
		{ID: "glossary-active-customers", Question: "aktif müşteriler", Model: model, Glossary: glossary, ExpectedAmbiguous: true},
		{ID: "scope-large-orders", Question: "büyük siparişler", Model: model, Glossary: glossary, ExpectedAmbiguous: true},
		{ID: "temporal-recent-orders", Question: "son zamanlarda sipariş veren müşteriler", Model: model, Glossary: glossary, ExpectedAmbiguous: true},
		{ID: "specific-last-30-days", Question: "son 30 günde sipariş veren müşteriler", Model: model, Glossary: glossary, ExpectedAmbiguous: false},
	}
}
