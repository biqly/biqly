package ai

import (
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	GenerationTraceAmbiguityPassed              = "passed"
	GenerationTraceAmbiguityClarificationNeeded = "clarification_needed"
)

// ColumnResolution records how a user-facing term mapped to a semantic field.
type ColumnResolution struct {
	Term     string `json:"term,omitempty"`
	Resolved string `json:"resolved"`
	Source   string `json:"source,omitempty"`
}

// GenerationTrace summarizes routing, ambiguity, and field resolution for transparency.
type GenerationTrace struct {
	RoutedTable     string             `json:"routed_table,omitempty"`
	RouteConfidence float64            `json:"route_confidence,omitempty"`
	ColumnsResolved []ColumnResolution `json:"columns_resolved,omitempty"`
	AmbiguityResult string             `json:"ambiguity_result,omitempty"`
	AmbiguityDetail string             `json:"ambiguity_detail,omitempty"`
}

// BuildGenerationTrace assembles trace metadata from routing and the AI response.
func BuildGenerationTrace(route *routing.TableRoutingResult, model *semantic.SemanticModel, resp *Response) *GenerationTrace {
	if route == nil && resp == nil {
		return nil
	}
	trace := &GenerationTrace{}
	if route != nil {
		trace.RoutedTable = formatRoutedTable(route)
		trace.RouteConfidence = route.Confidence
		trace.ColumnsResolved = append(trace.ColumnsResolved, columnsFromRouting(route)...)
	}
	if resp == nil {
		return trace
	}
	needsClarification := resp.Clarification != nil && resp.Clarification.NeedsClarification
	if needsClarification {
		trace.AmbiguityResult = GenerationTraceAmbiguityClarificationNeeded
		trace.AmbiguityDetail = formatClarificationTraceDetail(resp.Clarification)
		if resp.Clarification.Clarification != nil && resp.Clarification.Clarification.AmbiguityDetail != nil {
			trace.ColumnsResolved = mergeColumnResolutions(
				trace.ColumnsResolved,
				columnsFromAmbiguityDetail(resp.Clarification.Clarification.AmbiguityDetail),
			)
		}
		return trace
	}
	if resp.Result != nil && resp.Result.LogicalQuery != nil {
		trace.AmbiguityResult = GenerationTraceAmbiguityPassed
		trace.ColumnsResolved = mergeColumnResolutions(
			trace.ColumnsResolved,
			columnsFromLogicalQuery(resp.Result.LogicalQuery, model),
		)
	}
	return trace
}

func formatRoutedTable(route *routing.TableRoutingResult) string {
	if route == nil {
		return ""
	}
	if len(route.SelectedModels) > 0 {
		return strings.Join(route.SelectedModels, ", ")
	}
	if len(route.SelectedTables) > 0 {
		return strings.Join(route.SelectedTables, ", ")
	}
	for _, candidate := range route.Candidates {
		if candidate.Selected {
			return candidate.Table
		}
	}
	if len(route.Candidates) > 0 {
		return route.Candidates[0].Table
	}
	return ""
}

func columnsFromRouting(route *routing.TableRoutingResult) []ColumnResolution {
	if route == nil {
		return nil
	}
	out := make([]ColumnResolution, 0, len(route.SelectedDimensions)+len(route.SelectedMetrics))
	for _, name := range route.SelectedDimensions {
		out = append(out, ColumnResolution{Term: name, Resolved: name, Source: "routing"})
	}
	for _, name := range route.SelectedMetrics {
		out = append(out, ColumnResolution{Term: name, Resolved: name, Source: "routing"})
	}
	return out
}

func columnsFromAmbiguityDetail(detail *AmbiguityDetail) []ColumnResolution {
	if detail == nil || len(detail.Ambiguities) == 0 {
		return nil
	}
	var out []ColumnResolution
	for _, item := range detail.Ambiguities {
		for _, interpretation := range item.Interpretations {
			resolved := interpretation.SemanticMapping.Name
			if resolved == "" {
				resolved = interpretation.Label
			}
			out = append(out, ColumnResolution{
				Term:     item.Term,
				Resolved: resolved,
				Source:   item.Type,
			})
		}
	}
	return out
}

func columnsFromLogicalQuery(lq *query.LogicalQuery, model *semantic.SemanticModel) []ColumnResolution {
	if lq == nil {
		return nil
	}
	dimByName := map[string]semantic.Dimension{}
	metricByName := map[string]semantic.Metric{}
	if model != nil {
		for _, dimension := range model.Dimensions {
			dimByName[dimension.Name] = dimension
		}
		for _, metric := range model.Metrics {
			metricByName[metric.Name] = metric
		}
	}
	out := make([]ColumnResolution, 0, len(lq.Select))
	for _, item := range lq.Select {
		switch item.Type {
		case "metric":
			if metric, ok := metricByName[item.Name]; ok {
				out = append(out, ColumnResolution{
					Term:     item.Name,
					Resolved: metric.Aggregation + "(" + metric.Expression + ")",
					Source:   "metric",
				})
				continue
			}
		case "dimension", "":
			if dimension, ok := dimByName[item.Name]; ok {
				resolved := dimension.ColumnRef
				if dimension.CalculatedExpression != "" {
					resolved = dimension.CalculatedExpression
				}
				out = append(out, ColumnResolution{
					Term:     item.Name,
					Resolved: resolved,
					Source:   "dimension",
				})
				continue
			}
		}
		out = append(out, ColumnResolution{
			Term:     item.Name,
			Resolved: item.Name,
			Source:   item.Type,
		})
	}
	return out
}

func formatClarificationTraceDetail(clar *ClarificationResponse) string {
	if clar == nil || clar.Clarification == nil {
		return ""
	}
	c := clar.Clarification
	if c.AmbiguityDetail != nil && len(c.AmbiguityDetail.Ambiguities) > 0 {
		parts := make([]string, 0, len(c.AmbiguityDetail.Ambiguities))
		for _, item := range c.AmbiguityDetail.Ambiguities {
			labels := make([]string, 0, len(item.Interpretations))
			for _, interpretation := range item.Interpretations {
				name := interpretation.SemanticMapping.Name
				if name == "" {
					name = interpretation.Label
				}
				labels = append(labels, name)
			}
			parts = append(parts, fmt.Sprintf("%s (%s): %s", item.Term, item.Type, strings.Join(labels, " vs ")))
		}
		return strings.Join(parts, "; ")
	}
	if c.Source == "router" {
		return "routing: multiple table candidates"
	}
	return c.Reason
}

func mergeColumnResolutions(groups ...[]ColumnResolution) []ColumnResolution {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	if total == 0 {
		return nil
	}
	out := make([]ColumnResolution, 0, total)
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}
