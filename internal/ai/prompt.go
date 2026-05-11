package ai

import (
	"encoding/json"
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

// FewShotExample is a successful prior (question, logical_query) pair injected
// into the prompt to steer the model toward the project's house style.
type FewShotExample struct {
	Question     string
	LogicalQuery string // raw JSON, already validated when stored
}

// ConversationTurn captures one earlier exchange in the active conversation so
// follow-up questions ("filter that to last quarter", "now group by region")
// can be resolved in context. Distinct from FewShotExample: those are
// hand-picked exemplars from history; turns are the live thread.
type ConversationTurn struct {
	Question     string
	LogicalQuery string // raw JSON if available, else empty
	Note         string // e.g. "executed", "user rejected, asked to refine"
}

// Build creates the full prompt for the AI. maxPromptRunes caps the total size (0 = default 80000)
// so providers with finite context windows do not receive huge auto-generated semantic models.
// examples are dynamic few-shot pairs from the project's history; pass nil for none.
// samples are concrete rows from queried tables; pass nil for none.
// deniedFields is an optional list of qualified field names (e.g. "model.secret_field") that
// must NOT appear in the prompt — used in strict mode to enforce row-level security at prompt time.
func (b *PromptBuilder) Build(question string, model *semantic.SemanticModel, maxPromptRunes int, examples []FewShotExample, samples []TableSample, priorTurns []ConversationTurn, deniedFields []string) string {
	if maxPromptRunes <= 0 {
		maxPromptRunes = 80000
	}

	// Build set of denied field names for fast lookup
	deniedSet := make(map[string]bool, len(deniedFields))
	for _, f := range deniedFields {
		deniedSet[strings.ToLower(f)] = true
	}

	var sb strings.Builder
	write := func(s string) {
		sb.WriteString(s)
	}

	write("You are a Business Intelligence query engine. Convert the user's natural language question into a LogicalQuery JSON object.\n\n")
	write("## Rules\n")
	write("- Output ONLY valid JSON. No markdown, no code blocks, no explanation.\n")
	write("- Use strict JSON (RFC 8259): every property name MUST be double-quoted. Never use JavaScript object syntax (unquoted keys like {select: [...]}).\n")
	write("- Do NOT invent fields that don't exist in the semantic layer.\n")
	write("- Use ONLY the dimensions, metrics, and fields listed below.\n")
	write("- In `select`, `group_by`, `filters`, `having`, and `order_by`, field/name values MUST be exact listed dimension or metric names. Do NOT write expressions such as `year(orderdate)`, `table.column`, SQL functions, or aliases unless they are listed names.\n")
	write("- A single select array may combine multiple dimensions AND multiple metrics; include every column the question asks for.\n")
	write("- When the question groups results by a dimension (e.g. \"per customer\", \"by month\"), put that dimension in both `select` and `group_by`.\n")
	write("- Match metric names by their listed synonyms (e.g. \"latest date\"/\"en son tarih\" → max_<date_column>; \"how many\"/\"adet\" → row_count).\n")
	write("- When the question refers to an entity generically (e.g. \"customer\"/\"müşteri\", \"product\"/\"ürün\") without naming a column, prefer the human-readable display dimension whose synonyms include that entity (typically a name/title/label column) over identifier columns like *_id.\n")
	write("- Do NOT put raw identifier dimensions in `select` (e.g. columns ending in _id, *id, *key, businessentityid, departmentid) unless the user explicitly asks for ids, keys, codes, or identifiers. For requests like \"names\", \"list X with Y\", or readable labels, use `name`, `title`, `label`, or other descriptive text dimensions from the joined tables instead.\n")
	write("- For date filters, use ISO 8601 format (YYYY-MM-DD).\n")
	write("- For grouping or listing by calendar period, use the matching time-grain dimension: `*_year` for \"by year\" / \"yearly\" / \"yıllık\", `*_month` for \"by month\", `*_quarter` for \"by quarter\" — not the raw timestamp/date column (avoid daily buckets when the user asked for years or months).\n")
	write("- For ranking questions (\"which … highest/largest/most revenue\", \"top …\"): put the breakdown dimension in `select` and `group_by`, put the revenue/count metric in `select`, `order_by` that metric `desc`, and `limit` 1 (or a small N) — never return an empty `select` if the model lists usable dimensions and metrics.\n")
	write("- For threshold questions on aggregates (\"customers with more than 10 orders\", \"products averaging > 100\"), use `having` (post-aggregation) — not `filters` (pre-aggregation). Each `having` entry is `{\"field\":\"<metric_name>\", \"operator\":\"gt|gte|lt|lte|eq|neq|between\", \"value\": ...}`.\n")
	write("- For running totals, ranks per group, or moving averages (\"running total of revenue by month\", \"rank customers within each region\"), emit a window select item: `{\"type\":\"window\",\"name\":\"<alias>\",\"window\":{\"aggregation\":\"sum|avg|count|min|max|row_number|rank|dense_rank|ntile\",\"metric\":\"<metric_name>\" OR \"expression\":\"<table.column>\",\"partition_by\":[\"<dim>\",...],\"order_by\":[{\"field\":\"<dim_or_metric>\",\"direction\":\"asc|desc\"}]}}`. Ranking functions (row_number/rank/dense_rank) ignore `expression`; `ntile` puts the bucket count in `expression` (e.g. \"4\").\n")
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

	// Filter out denied fields from dimensions
	var allowedDims []semantic.Dimension
	for _, d := range model.Dimensions {
		if !deniedSet[strings.ToLower(d.ColumnRef)] && !deniedSet[strings.ToLower(d.Name)] {
			allowedDims = append(allowedDims, d)
		}
	}

	// Filter out denied fields from metrics
	var allowedMetrics []semantic.Metric
	for _, m := range model.Metrics {
		if !deniedSet[strings.ToLower(m.Expression)] && !deniedSet[strings.ToLower(m.Name)] {
			allowedMetrics = append(allowedMetrics, m)
		}
	}

	write("## Available Dimensions\n")
	omittedDims := b.writeDimensions(&sb, allowedDims, remaining/2)
	write("\n")

	write("## Available Metrics\n")
	metricsBudget := maxPromptRunes - utf8.RuneCountInString(sb.String()) - promptStaticReserveRunes/2
	if metricsBudget < 4000 {
		metricsBudget = 4000
	}
	omittedMetrics := b.writeMetrics(&sb, allowedMetrics, metricsBudget)
	write("\n")

	if omittedDims > 0 || omittedMetrics > 0 {
		fmt.Fprintf(&sb, "## Note\nSome catalog entries were omitted to fit the model context window (%d dimensions, %d metrics skipped). Narrow **Tables** scope in the UI or define a smaller semantic model if a field is missing.\n\n",
			omittedDims, omittedMetrics)
	}

	// Filter out denied fields from joins
	var allowedJoins []semantic.Join
	for _, j := range model.Joins {
		if !deniedSet[strings.ToLower(j.FromTable+"."+j.FromColumn)] && !deniedSet[strings.ToLower(j.ToTable+"."+j.ToColumn)] {
			allowedJoins = append(allowedJoins, j)
		}
	}

	if len(allowedJoins) > 0 {
		write("## Available Joins\n")
		for _, j := range allowedJoins {
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

	b.writeSampleData(&sb, samples)
	b.writeFewShotExamples(&sb, examples)
	b.writePriorTurns(&sb, priorTurns)

	return sb.String()
}

// writePriorTurns appends recent turns from the active conversation so the
// model can resolve follow-ups like "now filter to last quarter" or "same
// breakdown but by region". Each turn shows the user's question and, if
// available, the prior LogicalQuery the assistant produced.
func (b *PromptBuilder) writePriorTurns(sb *strings.Builder, turns []ConversationTurn) {
	if len(turns) == 0 {
		return
	}
	sb.WriteString("\n\n## Prior Turns in This Conversation\n")
	sb.WriteString("Use these to resolve references like \"that\", \"now\", \"also\", \"instead\". The latest user question (above) takes precedence.\n")
	for i, t := range turns {
		q := strings.TrimSpace(t.Question)
		if q == "" {
			continue
		}
		fmt.Fprintf(sb, "\nTurn %d — Question: %q\n", i+1, q)
		if lq := strings.TrimSpace(t.LogicalQuery); lq != "" {
			fmt.Fprintf(sb, "Previous LogicalQuery: %s\n", lq)
		}
		if note := strings.TrimSpace(t.Note); note != "" {
			fmt.Fprintf(sb, "Note: %s\n", note)
		}
	}
}

// writeSampleData appends a compact JSON block of concrete rows so the LLM can
// see actual values (formats, casing, enum-like fields). Skipped silently if
// no samples were provided.
func (b *PromptBuilder) writeSampleData(sb *strings.Builder, samples []TableSample) {
	if len(samples) == 0 {
		return
	}
	sb.WriteString("\n\n## Sample Data\n")
	for _, s := range samples {
		if len(s.Rows) == 0 {
			continue
		}
		fmt.Fprintf(sb, "### %s.%s\n", s.Schema, s.Table)
		data, err := json.Marshal(s.Rows)
		if err != nil {
			continue
		}
		sb.Write(data)
		sb.WriteString("\n")
	}
}

// writeFewShotExamples appends previously-successful (question, logical_query)
// pairs to the prompt as additional examples. Skipped silently if empty.
func (b *PromptBuilder) writeFewShotExamples(sb *strings.Builder, examples []FewShotExample) {
	if len(examples) == 0 {
		return
	}
	sb.WriteString("\n\n## Examples — Successful Past Queries\n")
	for _, ex := range examples {
		q := strings.TrimSpace(ex.Question)
		lq := strings.TrimSpace(ex.LogicalQuery)
		if q == "" || lq == "" {
			continue
		}
		fmt.Fprintf(sb, "Question: %q\n%s\n\n", q, lq)
	}
}

func (b *PromptBuilder) writeDimensions(sb *strings.Builder, dims []semantic.Dimension, budgetRunes int) int {
	if budgetRunes <= 0 {
		fmt.Fprintf(sb, "(dimensions omitted — size budget)\n")
		return len(dims)
	}

	// Surface display dimensions first — these are the human-readable columns
	// (name, title, label) that users expect when asking for "list X", "names",
	// or readable labels.
	var displayDims, otherDims []semantic.Dimension
	for _, d := range dims {
		if d.IsDisplay {
			displayDims = append(displayDims, d)
		} else {
			otherDims = append(otherDims, d)
		}
	}

	if len(displayDims) > 0 {
		sb.WriteString("### Display Dimensions (preferred for SELECT when asking for names/labels)\n")
		for _, d := range displayDims {
			syn := joinSynonymsCap(d.Synonyms, maxSynonymsPerLine)
			line := fmt.Sprintf("- %s (type: %s, column: %s)", d.Name, d.Type, d.ColumnRef)
			if syn != "" {
				line += fmt.Sprintf(", synonyms: %s", syn)
			}
			line += "\n"
			r := utf8.RuneCountInString(line)
			if r > budgetRunes {
				return len(dims)
			}
			sb.WriteString(line)
			budgetRunes -= r
		}
		sb.WriteString("\n")
	}

	used := 0
	omitted := 0
	for i, d := range otherDims {
		syn := joinSynonymsCap(d.Synonyms, maxSynonymsPerLine)
		line := fmt.Sprintf("- %s (type: %s, column: %s)", d.Name, d.Type, d.ColumnRef)
		if syn != "" {
			line += fmt.Sprintf(", synonyms: %s", syn)
		}
		line += "\n"
		r := utf8.RuneCountInString(line)
		if used+r > budgetRunes {
			omitted = len(otherDims) - i + len(displayDims)
			break
		}
		sb.WriteString(line)
		used += r
	}

	// Reinforce the display-dimension priority rule
	if len(displayDims) > 0 {
		sb.WriteString("\n**IMPORTANT**: When the user asks for names, labels, or readable identifiers, ALWAYS use the display dimensions above — NOT the *_id or *_key columns.\n")
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

// BuildClarification asks the LLM to produce a single, user-facing clarifying
// question explaining what is ambiguous about the original request, given the
// available semantic model. Output is plain text (no JSON) — short, natural.
func (b *PromptBuilder) BuildClarification(question string, model *semantic.SemanticModel, failureReason string) string {
	var sb strings.Builder
	sb.WriteString("You are a Business Intelligence assistant. The user asked a question that could not be answered with the current semantic model.\n\n")
	fmt.Fprintf(&sb, "## User Question\n%s\n\n", question)
	if failureReason != "" {
		fmt.Fprintf(&sb, "## Why It Was Ambiguous\n%s\n\n", failureReason)
	}
	fmt.Fprintf(&sb, "## Semantic Model: %s\n", model.Name)
	if len(model.Dimensions) > 0 {
		sb.WriteString("Dimensions: ")
		names := make([]string, 0, len(model.Dimensions))
		for _, d := range model.Dimensions {
			names = append(names, d.Name)
		}
		sb.WriteString(strings.Join(names, ", "))
		sb.WriteString("\n")
	}
	if len(model.Metrics) > 0 {
		sb.WriteString("Metrics: ")
		names := make([]string, 0, len(model.Metrics))
		for _, m := range model.Metrics {
			names = append(names, m.Name)
		}
		sb.WriteString(strings.Join(names, ", "))
		sb.WriteString("\n")
	}
	sb.WriteString("\n## Required Output\n")
	sb.WriteString("Reply with ONE short clarifying question (max 200 chars) the user can answer to disambiguate their request. ")
	sb.WriteString("Match the user's language. No prefixes, no JSON, no quotes — just the question itself.\n")
	return sb.String()
}

// BuildRetry constructs a corrective prompt to send back to the LLM when the
// previous attempt produced unparseable or semantically-invalid output. The
// original prompt is reused as the source of truth for the schema; the
// addendum carries the model's failed response and the validation error.
func (b *PromptBuilder) BuildRetry(originalPrompt, lastResponse, validationError string) string {
	var sb strings.Builder
	sb.WriteString(originalPrompt)
	sb.WriteString("\n\n## Previous Attempt (incorrect)\n")
	sb.WriteString("Your previous response was:\n")
	sb.WriteString(lastResponse)
	sb.WriteString("\n\n## Why It Failed\n")
	sb.WriteString(validationError)
	sb.WriteString("\n\n## Required Action\n")
	sb.WriteString("Re-emit ONLY a corrected JSON object using dimensions, metrics, and operators that exist in the semantic model above. Do not repeat the previous mistake. Output JSON only — no prose, no markdown, no code fences.\n")
	return sb.String()
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
