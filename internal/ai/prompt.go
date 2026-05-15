package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/i18n"
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

// Build creates the full prompt for the AI. maxPromptRunes caps the total size (0 = default 80000)
// so providers with finite context windows do not receive huge auto-generated semantic models.
// locale selects which prompt template bundle is rendered. The LLM-facing rules
// stay English (model accuracy is sensitive to phrasing), but the template
// indirection keeps the door open for locale-specific wording where it is safe.
// targetDialect is the datasource engine (postgres, mysql, sqlserver, clickhouse); empty defaults to postgres.
// examples are dynamic few-shot pairs from the project's history; pass nil for none.
// samples are concrete rows from queried tables; pass nil for none.
// deniedFields is an optional list of qualified field names (e.g. "model.secret_field") that
// must NOT appear in the prompt — used in strict mode to enforce row-level security at prompt time.
func (b *PromptBuilder) Build(question string, model *semantic.SemanticModel, maxPromptRunes int, locale i18n.Locale, targetDialect string, examples []FewShotExample, samples []TableSample, priorTurns []ConversationTurn, deniedFields []string, glossary []GlossaryEntry) string {
	if maxPromptRunes <= 0 {
		maxPromptRunes = 80000
	}
	if locale == "" {
		locale = i18n.DefaultLocale
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

	rules := promptTemplate(locale, "system_rules")
	if rules == "" {
		rules = promptTemplate(i18n.DefaultLocale, "system_rules")
	}
	write(rules)
	write("\n")

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

	b.writeBusinessGlossary(&sb, glossary)

	b.writeDialectCompilationGuide(&sb, targetDialect)
	b.writeFailureExamples(&sb)
	b.writePlanningSteps(&sb)

	write("## User Question\n")
	write(question)
	write("\n\n")

	outputFmt := promptTemplate(locale, "output_format")
	if outputFmt == "" {
		outputFmt = promptTemplate(i18n.DefaultLocale, "output_format")
	}
	write(outputFmt)

	b.writeSampleData(&sb, samples)
	b.writeFewShotExamples(&sb, examples, locale)
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
// pairs to the prompt as additional examples. Locale-tagged rows are preferred
// when matching the active locale; locale-empty rows are always eligible and
// rows tagged for a different locale are filtered out. Skipped silently if no
// rows survive the filter.
func (b *PromptBuilder) writeFewShotExamples(sb *strings.Builder, examples []FewShotExample, locale i18n.Locale) {
	if len(examples) == 0 {
		return
	}
	target := string(locale)
	var filtered []FewShotExample
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
	sb.WriteString("Re-run the **Planning Steps** from the original prompt, then re-emit a corrected LogicalQuery JSON object using dimensions, metrics, and operators that exist in the semantic model above. Do not repeat the previous mistake. Optional `## Reasoning` prefix allowed; final output must include valid JSON — no markdown fences.\n")
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
