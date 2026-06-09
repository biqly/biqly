package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/ai/ambiguity"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/require"
)

func TestBuildGenerationTrace_SuccessMapsLogicalQueryFields(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{{Name: "order_date", ColumnRef: "orders.order_date"}},
		Metrics:    []semantic.Metric{{Name: "revenue", Expression: "orders.total_amount", Aggregation: "sum"}},
	}
	route := &routing.TableRoutingResult{
		SelectedTables: []string{"orders"},
		Confidence:     0.92,
	}
	resp := &Response{
		Result: &AIResult{
			LogicalQuery: &query.LogicalQuery{
				Select: []query.SelectItem{
					{Type: "dimension", Name: "order_date"},
					{Type: "metric", Name: "revenue"},
				},
			},
		},
	}

	trace := BuildGenerationTrace(route, model, resp)
	require.NotNil(t, trace)
	require.Equal(t, "orders", trace.RoutedTable)
	require.InDelta(t, 0.92, trace.RouteConfidence, 0.001)
	require.Equal(t, GenerationTraceAmbiguityPassed, trace.AmbiguityResult)
	require.Len(t, trace.ColumnsResolved, 2)
	require.Equal(t, "orders.order_date", trace.ColumnsResolved[0].Resolved)
	require.Equal(t, "sum(orders.total_amount)", trace.ColumnsResolved[1].Resolved)
}

func TestBuildGenerationTrace_AmbiguityClarificationDetail(t *testing.T) {
	resp := &Response{
		Clarification: &ClarificationResponse{
			NeedsClarification: true,
			Clarification: &Clarification{
				Source: "ambiguity_analyzer",
				AmbiguityDetail: &AmbiguityDetail{
					Ambiguities: []ambiguity.Item{
						{
							Term: "revenue",
							Type: "synonym",
							Interpretations: []ambiguity.Interpretation{
								{Label: "Gross revenue", SemanticMapping: ambiguity.SemanticMapping{Name: "gross_revenue", Type: "metric"}},
								{Label: "Net revenue", SemanticMapping: ambiguity.SemanticMapping{Name: "net_revenue", Type: "metric"}},
							},
						},
					},
				},
			},
		},
	}

	trace := BuildGenerationTrace(nil, nil, resp)
	require.NotNil(t, trace)
	require.Equal(t, GenerationTraceAmbiguityClarificationNeeded, trace.AmbiguityResult)
	require.Contains(t, trace.AmbiguityDetail, "revenue")
	require.Contains(t, trace.AmbiguityDetail, "gross_revenue")
	require.Len(t, trace.ColumnsResolved, 2)
	require.Equal(t, "revenue", trace.ColumnsResolved[0].Term)
	require.Equal(t, "gross_revenue", trace.ColumnsResolved[0].Resolved)
}
