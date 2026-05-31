package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	promptStaticReserveRunes = 14000 // question, rules, output format, date
	maxSynonymsPerLine       = 6
	maxEnumValuesPerLine     = 20 // cap coded-value lists so prompt stays bounded
)

const defaultLayout = `{{.SystemRules}}

## Current Date/Time: {{.CurrentDateTime}}

## Semantic Model: {{.ModelName}}
{{if .ModelLabel}}Label: {{.ModelLabel}}
{{end}}{{if .ModelDescription}}Description: {{.ModelDescription}}
{{end}}Base table: {{.BaseTable}}
{{if .ModelSynonyms}}Model synonyms: {{.ModelSynonyms}}
{{end}}
## Available Dimensions
{{.Dimensions}}
{{.Note}}
## Available Metrics
{{.Metrics}}

{{if .Joins}}## Available Joins
{{.Joins}}
{{end}}
## Supported Filter Operators
{{.FilterOperators}}
{{.Glossary}}{{.DialectGuide}}{{.FailureExamples}}{{.PlanningSteps}}
## User Question
{{.Question}}

{{.OutputFormat}}
{{.SampleData}}
{{.Examples}}
{{.PriorTurns}}`

var promptBuilderPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// withPooledBuffer borrows a *bytes.Buffer from promptBuilderPool, runs fn to
// fill it, copies its contents to a string, returns the buffer to the pool,
// and yields the string to the caller. Used by Build to avoid eleven fresh
// bytes.Buffer allocations per prompt.
func withPooledBuffer(fn func(*bytes.Buffer)) string {
	buf, _ := promptBuilderPool.Get().(*bytes.Buffer)
	buf.Reset()
	fn(buf)
	out := buf.String()
	promptBuilderPool.Put(buf)
	return out
}

// PromptConfig contains options to configure prompt construction.
type PromptConfig struct {
	MaxRunes     int
	Locale       i18n.Locale
	Dialect      string
	Examples     []FewShotExample
	Samples      []TableSample
	PriorTurns   []ConversationTurn
	DeniedFields []string
	Glossary     []GlossaryEntry
}

// PromptBuilder constructs the AI prompt with semantic context.
type PromptBuilder struct{}

// FewShotExample is a successful prior (question, logical_query) pair injected
// into the prompt to steer the model toward the project's house style.
type FewShotExample struct {
	Question     string
	LogicalQuery string // raw JSON, already validated when stored
	// Locale is the language of the question text (e.g. "tr", "en"). Empty
	// means locale-agnostic — always eligible regardless of the active locale.
	Locale string
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

// Build creates the full prompt for the AI.
func (b *PromptBuilder) Build(ctx context.Context, question string, model *semantic.SemanticModel, cfg PromptConfig) string {
	maxPromptRunes := cfg.MaxRunes
	if maxPromptRunes <= 0 {
		maxPromptRunes = 80000
	}
	locale := PromptLocaleForQuestion(question, cfg.Locale)
	targetDialect := cfg.Dialect
	examples := cfg.Examples
	samples := cfg.Samples
	priorTurns := cfg.PriorTurns
	deniedFields := cfg.DeniedFields
	glossary := cfg.Glossary

	// Build set of denied field names for fast lookup
	deniedSet := make(map[string]bool, len(deniedFields))
	for _, f := range deniedFields {
		deniedSet[strings.ToLower(f)] = true
	}

	rules := promptTemplate(ctx, locale, "system_rules")

	headBuf := new(bytes.Buffer)
	fmt.Fprintf(headBuf, "## Current Date/Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(headBuf, "## Semantic Model: %s\n", model.Name)
	if model.Label != nil {
		fmt.Fprintf(headBuf, "Label: %s\n", *model.Label)
	}
	if model.Description != nil {
		fmt.Fprintf(headBuf, "Description: %s\n", *model.Description)
	}
	fmt.Fprintf(headBuf, "Base table: %s.%s\n\n", model.BaseSchema, model.BaseTable)
	if len(model.Synonyms) > 0 {
		fmt.Fprintf(headBuf, "Model synonyms: %s\n\n", strings.Join(model.Synonyms, ", "))
	}

	headRunes := utf8.RuneCountInString(rules) + utf8.RuneCount(headBuf.Bytes())
	remaining := maxPromptRunes - headRunes - promptStaticReserveRunes
	if remaining < 16000 {
		remaining = 16000
	}

	// Filter out denied fields from dimensions
	allowedDims := make([]semantic.Dimension, 0, len(model.Dimensions))
	for _, d := range model.Dimensions {
		if !deniedSet[strings.ToLower(d.ColumnRef)] && !deniedSet[strings.ToLower(d.Name)] {
			allowedDims = append(allowedDims, d)
		}
	}

	// Filter out denied fields from metrics
	allowedMetrics := make([]semantic.Metric, 0, len(model.Metrics))
	for _, m := range model.Metrics {
		if !deniedSet[strings.ToLower(m.Expression)] && !deniedSet[strings.ToLower(m.Name)] {
			allowedMetrics = append(allowedMetrics, m)
		}
	}

	var omittedDims int
	dimensionsStr := withPooledBuffer(func(buf *bytes.Buffer) {
		omittedDims = b.writeDimensions(buf, allowedDims, remaining/2)
	})

	metricsBudget := maxPromptRunes - (headRunes + utf8.RuneCountInString(dimensionsStr)) - promptStaticReserveRunes/2
	if metricsBudget < 4000 {
		metricsBudget = 4000
	}
	var omittedMetrics int
	metricsStr := withPooledBuffer(func(buf *bytes.Buffer) {
		omittedMetrics = b.writeMetrics(buf, allowedMetrics, metricsBudget)
	})

	var noteStr string
	if omittedDims > 0 || omittedMetrics > 0 {
		noteStr = fmt.Sprintf("## Note\nSome catalog entries were omitted to fit the model context window (%d dimensions, %d metrics skipped). Narrow **Tables** scope in the UI or define a smaller semantic model if a field is missing.\n\n",
			omittedDims, omittedMetrics)
	}

	// Filter out denied fields from joins
	allowedJoins := make([]semantic.Join, 0, len(model.Joins))
	for _, j := range model.Joins {
		if !deniedSet[strings.ToLower(j.FromTable+"."+j.FromColumn)] && !deniedSet[strings.ToLower(j.ToTable+"."+j.ToColumn)] {
			allowedJoins = append(allowedJoins, j)
		}
	}

	var joinsStr string
	if len(allowedJoins) > 0 {
		joinsStr = withPooledBuffer(func(buf *bytes.Buffer) {
			for _, j := range allowedJoins {
				fmt.Fprintf(buf, "- %s: %s.%s → %s.%s (%s, %s)\n",
					j.Name, j.FromTable, j.FromColumn, j.ToTable, j.ToColumn, j.JoinType, j.Relationship)
			}
		})
	}

	filterOpsStr := "eq, neq, gt, gte, lt, lte, in, not_in, contains, starts_with, ends_with, between, is_null, is_not_null\n\n"

	glossaryStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeBusinessGlossary(buf, glossary) })
	dialectStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeDialectCompilationGuide(buf, targetDialect) })
	failureStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeFailureExamples(buf) })
	planningStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writePlanningSteps(buf) })

	outputFmt := promptTemplate(ctx, locale, "output_format")

	sampleStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeSampleData(buf, samples) })
	exampleStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeFewShotExamples(buf, examples, locale) })
	priorStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writePriorTurns(buf, priorTurns) })

	var labelStr string
	if model.Label != nil {
		labelStr = *model.Label
	}
	var descStr string
	if model.Description != nil {
		descStr = *model.Description
	}
	var modelSynonymsStr string
	if len(model.Synonyms) > 0 {
		modelSynonymsStr = strings.Join(model.Synonyms, ", ")
	}

	data := map[string]any{
		"SystemRules":      rules,
		"CurrentDateTime":  time.Now().Format("2006-01-02 15:04:05 UTC"),
		"ModelName":        model.Name,
		"ModelLabel":       labelStr,
		"ModelDescription": descStr,
		"BaseTable":        fmt.Sprintf("%s.%s", model.BaseSchema, model.BaseTable),
		"ModelSynonyms":    modelSynonymsStr,
		"Dimensions":       dimensionsStr,
		"Metrics":          metricsStr,
		"Note":             noteStr,
		"Joins":            joinsStr,
		"FilterOperators":  filterOpsStr,
		"Glossary":         glossaryStr,
		"DialectGuide":     dialectStr,
		"FailureExamples":  failureStr,
		"PlanningSteps":    planningStr,
		"Question":         question,
		"OutputFormat":     outputFmt,
		"SampleData":       sampleStr,
		"Examples":         exampleStr,
		"PriorTurns":       priorStr,
	}

	layoutTmpl := promptTemplate(ctx, locale, "prompt_layout")
	if layoutTmpl == "" {
		layoutTmpl = defaultLayout
	}

	return renderPromptTemplate(layoutTmpl, data)
}

// writePriorTurns appends recent turns from the active conversation so the
// model can resolve follow-ups like "now filter to last quarter" or "same
// breakdown but by region". Each turn shows the user's question and, if
// available, the prior LogicalQuery the assistant produced.
func (b *PromptBuilder) writePriorTurns(sb *bytes.Buffer, turns []ConversationTurn) {
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
func (b *PromptBuilder) writeSampleData(sb *bytes.Buffer, samples []TableSample) {
	if len(samples) == 0 {
		return
	}
	sb.WriteString("\n\n## Sample Data\n")
	for _, s := range samples {
		if len(s.Rows) == 0 {
			continue
		}
		fmt.Fprintf(sb, "### %s.%s\n", s.Schema, s.Table)
		enc := json.NewEncoder(sb)
		if err := enc.Encode(s.Rows); err != nil {
			continue
		}
		sb.WriteString("\n")
	}
}

// writeFewShotExamples appends previously-successful (question, logical_query)
// pairs to the prompt as additional examples. Locale-tagged rows are preferred
// when matching the active locale; locale-empty rows are always eligible and
// rows tagged for a different locale are filtered out. Skipped silently if no
// rows survive the filter.
func (b *PromptBuilder) writeFewShotExamples(sb *bytes.Buffer, examples []FewShotExample, locale i18n.Locale) {
	if len(examples) == 0 {
		return
	}
	target := string(locale)
	filtered := make([]FewShotExample, 0, len(examples))
	for _, ex := range examples {
		q := strings.TrimSpace(ex.Question)
		lq := strings.TrimSpace(ex.LogicalQuery)
		if q == "" || lq == "" {
			continue
		}
		loc := strings.ToLower(strings.TrimSpace(ex.Locale))
		if loc != "" && target != "" && loc != target {
			continue
		}
		filtered = append(filtered, ex)
	}
	if len(filtered) == 0 {
		return
	}
	sb.WriteString("\n\n## Examples — Successful Past Queries\n")
	for _, ex := range filtered {
		fmt.Fprintf(sb, "Question: %q\n%s\n\n", strings.TrimSpace(ex.Question), strings.TrimSpace(ex.LogicalQuery))
	}
}

func (b *PromptBuilder) writeDimensions(sb *bytes.Buffer, dims []semantic.Dimension, budgetRunes int) int {
	if budgetRunes <= 0 {
		fmt.Fprintf(sb, "(dimensions omitted — size budget)\n")
		return len(dims)
	}

	// Surface display dimensions first — these are the human-readable columns
	// (name, title, label) that users expect when asking for "list X", "names",
	// or readable labels.
	displayDims := make([]semantic.Dimension, 0, len(dims))
	otherDims := make([]semantic.Dimension, 0, len(dims))
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
			tg := ""
			if d.TimeGrain != "" {
				tg = fmt.Sprintf(", time_grain: %s", d.TimeGrain)
			}
			sy := ""
			if syn != "" {
				sy = fmt.Sprintf(", synonyms: %s", syn)
			}
			line := fmt.Sprintf("- %s (type: %s, column: %s%s%s%s)\n", d.Name, d.Type, d.ColumnRef, tg, sy, formatEnumValues(d.EnumValues))
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
		tg := ""
		if d.TimeGrain != "" {
			tg = fmt.Sprintf(", time_grain: %s", d.TimeGrain)
		}
		sy := ""
		if syn != "" {
			sy = fmt.Sprintf(", synonyms: %s", syn)
		}
		line := fmt.Sprintf("- %s (type: %s, column: %s%s%s%s)\n", d.Name, d.Type, d.ColumnRef, tg, sy, formatEnumValues(d.EnumValues))
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

func (b *PromptBuilder) writeMetrics(sb *bytes.Buffer, metrics []semantic.Metric, budgetRunes int) int {
	if budgetRunes <= 0 {
		fmt.Fprintf(sb, "(metrics omitted — size budget)\n")
		return len(metrics)
	}
	used := 0
	omitted := 0
	for i, m := range metrics {
		syn := joinSynonymsCap(m.Synonyms, maxSynonymsPerLine)
		sy := ""
		if syn != "" {
			sy = fmt.Sprintf(", synonyms: %s", syn)
		}
		line := fmt.Sprintf("- %s (aggregation: %s, expression: %s%s)\n", m.Name, m.Aggregation, m.Expression, sy)
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
func (b *PromptBuilder) BuildClarification(ctx context.Context, locale i18n.Locale, question string, model *semantic.SemanticModel, failureReason string) string {
	locale = PromptLocaleForQuestion(question, locale)
	tmpl := promptTemplate(ctx, locale, "clarification")
	if tmpl != "" {
		if model == nil {
			model = &semantic.SemanticModel{}
		}
		names := make([]string, 0, len(model.Dimensions))
		for _, d := range model.Dimensions {
			names = append(names, d.Name)
		}
		metricNames := make([]string, 0, len(model.Metrics))
		for _, m := range model.Metrics {
			metricNames = append(metricNames, m.Name)
		}
		return renderPromptTemplate(tmpl, map[string]any{
			"Question":      question,
			"FailureReason": failureReason,
			"ModelName":     model.Name,
			"Dimensions":    strings.Join(names, ", "),
			"Metrics":       strings.Join(metricNames, ", "),
		})
	}

	sb := promptBuilderPool.Get().(*bytes.Buffer)
	sb.Reset()
	defer promptBuilderPool.Put(sb)
	sb.WriteString("You are a Business Intelligence assistant. The user asked a question that could not be answered with the current semantic model.\n\n")
	fmt.Fprintf(sb, "## User Question\n%s\n\n", question)
	if failureReason != "" {
		fmt.Fprintf(sb, "## Why It Was Ambiguous\n%s\n\n", failureReason)
	}
	fmt.Fprintf(sb, "## Semantic Model: %s\n", model.Name)
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
func (b *PromptBuilder) BuildRetry(ctx context.Context, locale i18n.Locale, originalPrompt, lastResponse, validationError string) string {
	tmpl := promptTemplate(ctx, locale, "retry")
	if tmpl != "" {
		return renderPromptTemplate(tmpl, map[string]any{
			"OriginalPrompt":   originalPrompt,
			"LastResponse":     lastResponse,
			"ValidationError":  validationError,
			"ValidationErrors": validationError,
		})
	}

	sb := promptBuilderPool.Get().(*bytes.Buffer)
	sb.Reset()
	defer promptBuilderPool.Put(sb)
	sb.WriteString(originalPrompt)
	sb.WriteString("\n\n## Previous Attempt (incorrect)\n")
	sb.WriteString("Your previous response was:\n")
	sb.WriteString(lastResponse)
	sb.WriteString("\n\n## Why It Failed\n")
	sb.WriteString(validationError)
	sb.WriteString("\n\n## Required Action\n")
	sb.WriteString("Re-run the **Planning Steps** from the original prompt, then re-emit a corrected LogicalQuery JSON object using dimensions, metrics, and operators that exist in the semantic model above. Do not repeat the previous mistake. Optional `## Reasoning` prefix allowed; final output must include valid JSON — no markdown fences.\n")
	return sb.String()
}

// formatEnumValues renders a coded dimension's raw→label pairs as an inline
// ", values: 1=pending, 2=shipped" segment so the LLM can translate user
// phrasing into the stored codes. Returns "" when the dimension has no enums.
func formatEnumValues(vals []semantic.EnumMapping) string {
	if len(vals) == 0 {
		return ""
	}
	n := len(vals)
	truncated := false
	if n > maxEnumValuesPerLine {
		n = maxEnumValuesPerLine
		truncated = true
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, fmt.Sprintf("%s=%s", vals[i].RawValue, vals[i].Label))
	}
	out := ", values: " + strings.Join(parts, ", ")
	if truncated {
		out += ", …"
	}
	return out
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
