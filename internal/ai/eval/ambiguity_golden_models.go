package eval

import (
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	ambiguityModelClarificationBasic = "clarification_basic"
	ambiguityModelRevenueSynonyms    = "revenue_synonyms"
	ambiguityModelGrossNetRevenue    = "gross_net_revenue"

	ambiguityGlossaryActiveCustomers = "active_customers"
)

func ambiguityGoldenModels() map[string]*semantic.SemanticModel {
	return map[string]*semantic.SemanticModel{
		ambiguityModelClarificationBasic: ambiguityClarificationBasicModel(),
		ambiguityModelRevenueSynonyms:    ambiguityRevenueSynonymsModel(),
		ambiguityModelGrossNetRevenue:    ambiguityGrossNetRevenueModel(),
	}
}

func ambiguityGoldenGlossaries() map[string][]prompt.GlossaryEntry {
	return map[string][]prompt.GlossaryEntry{
		ambiguityGlossaryActiveCustomers: ambiguityActiveCustomersGlossary(),
	}
}

func ambiguityClarificationBasicModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		ID: "ambiguity-clarification-basic",
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Type: string(semantic.DimensionTypeDate)},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count"},
			{Name: "revenue"},
		},
	}
}

func ambiguityRevenueSynonymsModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		ID: "ambiguity-revenue-synonyms",
		Dimensions: []semantic.Dimension{
			{Name: "customer_segment", Synonyms: []string{"CIRO"}},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
		},
	}
}

func ambiguityGrossNetRevenueModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		ID: "ambiguity-gross-net-revenue",
		Metrics: []semantic.Metric{
			{Name: "gross_revenue", Synonyms: []string{"ciro"}},
			{Name: "net_revenue", Synonyms: []string{"ciro"}},
		},
	}
}

func ambiguityActiveCustomersGlossary() []prompt.GlossaryEntry {
	return []prompt.GlossaryEntry{
		{Term: "aktif müşteriler", Definition: "Yakın dönemde sipariş veren müşteriler", MapsToType: "dimension", MapsToName: "recent_customer"},
		{Term: "aktif müşteriler", Definition: "Hesabı kapatılmamış müşteriler", MapsToType: "dimension", MapsToName: "enabled_customer"},
	}
}
