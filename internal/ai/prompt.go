package ai

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/semantic"
)

const (
	promptStaticReserveRunes = 14000 // question, rules, output format, date
	maxSynonymsPerLine       = 6
)

// PromptBuilder constructs the AI prompt with semantic context.
type PromptBuilder struct{}

// Build creates the full prompt for the AI. maxPromptRunes caps the total size (0 = default 80000)
// so providers with finite context windows do not receive huge auto-generated semantic models.
func (b *PromptBuilder) Build(question string, model *semantic.SemanticModel, maxPromptRunes int) string {
	if maxPromptRunes <= 0 {
		maxPromptRunes = 80000
	}

	var sb strings.Builder
	write := func(s string) {
		sb.WriteString(s)
	}

	write("You are a Business Intelligence query engine. Convert the user's natural language question into a LogicalQuery JSON object.\n\n")
	write("## Rules\n")
	write("- Output ONLY valid JSON. No markdown, no code blocks, no explanation.\n")
	write("- Do NOT invent fields that don't exist in the semantic layer.\n")
	write("- Use ONLY the dimensions, metrics, and fields listed below.\n")
	write("- A single select array may combine multiple dimensions AND multiple metrics; include every column the question asks for.\n")
	write("- When the question groups results by a dimension (e.g. \"per customer\", \"by month\"), put that dimension in both `select` and `group_by`.\n")
	write("- Match metric names by their listed synonyms (e.g. \"latest date\"/\"en son tarih\" → max_<date_column>; \"how many\"/\"adet\" → row_count).\n")
	write("- When the question refers to an entity generically (e.g. \"customer\"/\"müşteri\", \"product\"/\"ürün\") without naming a column, prefer the human-readable display dimension whose synonyms include that entity (typically a name/title/label column) over identifier columns like *_id.\n")
	write("- For date filters, use ISO 8601 format (YYYY-MM-DD).\n")
	write("- Only return an empty select if NO listed dimension or metric can answer the question; never use empty select to avoid combining fields.\n\n")

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

	headRunes := utf8.RuneCountInString(sb.String())
	remaining := maxPromptRunes - headRunes - promptStaticReserveRunes
	if remaining < 16000 {
		remaining = 16000
	}

	write("## Available Dimensions\n")
	omittedDims := b.writeDimensions(&sb, model.Dimensions, remaining/2)
	write("\n")

	write("## Available Metrics\n")
	metricsBudget := maxPromptRunes - utf8.RuneCountInString(sb.String()) - promptStaticReserveRunes/2
	if metricsBudget < 4000 {
		metricsBudget = 4000
	}
	omittedMetrics := b.writeMetrics(&sb, model.Metrics, metricsBudget)
	write("\n")

	if omittedDims > 0 || omittedMetrics > 0 {
		fmt.Fprintf(&sb, "## Note\nSome catalog entries were omitted to fit the model context window (%d dimensions, %d metrics skipped). Narrow **Tables** scope in the UI or define a smaller semantic model if a field is missing.\n\n",
			omittedDims, omittedMetrics)
	}

	if len(model.Joins) > 0 {
		write("## Available Joins\n")
		for _, j := range model.Joins {
			fmt.Fprintf(&sb, "- %s: %s.%s → %s.%s (%s, %s)\n",
				j.Name, j.FromTable, j.FromColumn, j.ToTable, j.ToColumn, j.JoinType, j.Relationship)
		}
		write("\n")
	}

	write("## Supported Filter Operators\n")
	write("eq, neq, gt, gte, lt, lte, in, not_in, contains, starts_with, ends_with, between, is_null, is_not_null\n\n")

	write("## User Question\n")
	write(question)
	write("\n\n")

	write("## Output Format\n")
	write("Output ONLY the JSON object matching this structure:\n")
	write(`{"select": [{"type": "dimension|metric", "name": "..."}], "filters": [{"field": "...", "operator": "...", "value": ...}], "group_by": [{"field": "..."}], "order_by": [{"field": "...", "direction": "asc|desc"}], "limit": 100}`)
	write("\n\n## Example — multi-metric grouping\n")
	write(`Question: "orders per customer name and the most recent order date"` + "\n")
	write(`{"select":[{"type":"dimension","name":"name"},{"type":"metric","name":"row_count"},{"type":"metric","name":"max_order_date"}],"group_by":[{"field":"name"}],"order_by":[{"field":"row_count","direction":"desc"}],"limit":100}`)

	return sb.String()
}

func (b *PromptBuilder) writeDimensions(sb *strings.Builder, dims []semantic.Dimension, budgetRunes int) int {
	if budgetRunes <= 0 {
		fmt.Fprintf(sb, "(dimensions omitted — size budget)\n")
		return len(dims)
	}
	used := 0
	omitted := 0
	for i, d := range dims {
		syn := joinSynonymsCap(d.Synonyms, maxSynonymsPerLine)
		line := fmt.Sprintf("- %s (type: %s, column: %s)", d.Name, d.Type, d.ColumnRef)
		if syn != "" {
			line += fmt.Sprintf(", synonyms: %s", syn)
		}
		line += "\n"
		r := utf8.RuneCountInString(line)
		if used+r > budgetRunes {
			omitted = len(dims) - i
			break
		}
		sb.WriteString(line)
		used += r
	}
	return omitted
}

func (b *PromptBuilder) writeMetrics(sb *strings.Builder, metrics []semantic.Metric, budgetRunes int) int {
	if budgetRunes <= 0 {
		fmt.Fprintf(sb, "(metrics omitted — size budget)\n")
		return len(metrics)
	}
	used := 0
	omitted := 0
	for i, m := range metrics {
		syn := joinSynonymsCap(m.Synonyms, maxSynonymsPerLine)
		line := fmt.Sprintf("- %s (aggregation: %s, expression: %s)", m.Name, m.Aggregation, m.Expression)
		if syn != "" {
			line += fmt.Sprintf(", synonyms: %s", syn)
		}
		line += "\n"
		r := utf8.RuneCountInString(line)
		if used+r > budgetRunes {
			omitted = len(metrics) - i
			break
		}
		sb.WriteString(line)
		used += r
	}
	return omitted
}

func joinSynonymsCap(s []string, maxN int) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= maxN {
		return strings.Join(s, ", ")
	}
	return strings.Join(s[:maxN], ", ") + ", …"
}
