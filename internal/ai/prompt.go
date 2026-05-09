package ai

import (
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/semantic"
)

// PromptBuilder constructs the AI prompt with semantic context.
type PromptBuilder struct{}

// Build creates the full prompt for the AI.
func (b *PromptBuilder) Build(question string, model *semantic.SemanticModel) string {
	var sb strings.Builder

	sb.WriteString("You are a Business Intelligence query engine. Convert the user's natural language question into a LogicalQuery JSON object.\n\n")
	sb.WriteString("## Rules\n")
	sb.WriteString("- Output ONLY valid JSON. No markdown, no code blocks, no explanation.\n")
	sb.WriteString("- Do NOT invent fields that don't exist in the semantic layer.\n")
	sb.WriteString("- Use ONLY the dimensions, metrics, and fields listed below.\n")
	sb.WriteString("- For date filters, use ISO 8601 format (YYYY-MM-DD).\n")
	sb.WriteString("- If the question is ambiguous or cannot be answered, set an empty select array and add a warning.\n\n")

	fmt.Fprintf(&sb, "## Current Date/Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))

	fmt.Fprintf(&sb, "## Semantic Model: %s\n", model.Name)
	if model.Label != nil {
		fmt.Fprintf(&sb, "Label: %s\n", *model.Label)
	}
	if model.Description != nil {
		fmt.Fprintf(&sb, "Description: %s\n", *model.Description)
	}
	fmt.Fprintf(&sb, "Base table: %s.%s\n\n", model.BaseSchema, model.BaseTable)

	if len(model.Synonyms) > 0 {
		fmt.Fprintf(&sb, "Model synonyms: %s\n\n", strings.Join(model.Synonyms, ", "))
	}

	sb.WriteString("## Available Dimensions\n")
	for _, d := range model.Dimensions {
		fmt.Fprintf(&sb, "- %s (type: %s, column: %s)", d.Name, d.Type, d.ColumnRef)
		if len(d.Synonyms) > 0 {
			fmt.Fprintf(&sb, ", synonyms: %s", strings.Join(d.Synonyms, ", "))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Available Metrics\n")
	for _, m := range model.Metrics {
		fmt.Fprintf(&sb, "- %s (aggregation: %s, expression: %s)", m.Name, m.Aggregation, m.Expression)
		if len(m.Synonyms) > 0 {
			fmt.Fprintf(&sb, ", synonyms: %s", strings.Join(m.Synonyms, ", "))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	if len(model.Joins) > 0 {
		sb.WriteString("## Available Joins\n")
		for _, j := range model.Joins {
			fmt.Fprintf(&sb, "- %s: %s.%s → %s.%s (%s, %s)\n",
				j.Name, j.FromTable, j.FromColumn, j.ToTable, j.ToColumn, j.JoinType, j.Relationship)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Supported Filter Operators\n")
	sb.WriteString("eq, neq, gt, gte, lt, lte, in, not_in, contains, starts_with, ends_with, between, is_null, is_not_null\n\n")

	sb.WriteString("## User Question\n")
	sb.WriteString(question)
	sb.WriteString("\n\n")

	sb.WriteString("## Output Format\n")
	sb.WriteString("Output ONLY the JSON object matching this structure:\n")
	sb.WriteString(`{"select": [{"type": "dimension|metric", "name": "..."}], "filters": [{"field": "...", "operator": "...", "value": ...}], "group_by": [{"field": "..."}], "order_by": [{"field": "...", "direction": "asc|desc"}], "limit": 100}`)

	return sb.String()
}
