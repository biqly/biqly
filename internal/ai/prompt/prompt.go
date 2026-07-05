// Package prompt implements the prompt builder service.
package prompt

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/query"
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
{{end}}{{.CompositeContext}}
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
{{.Instructions}}{{.Glossary}}{{.Memories}}{{.DialectGuide}}{{.FailureExamples}}{{.PlanningSteps}}
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
	buf, ok := promptBuilderPool.Get().(*bytes.Buffer)
	if !ok {
		buf = new(bytes.Buffer)
	}
	buf.Reset()
	fn(buf)
	out := buf.String()
	promptBuilderPool.Put(buf)
	return out
}

// Config PromptConfig contains options to configure prompt construction.
type Config struct {
	MaxRunes     int
	Locale       i18n.Locale
	Dialect      string
	Examples     []FewShotExample
	Samples      []TableSample
	PriorTurns   []ConversationTurn
	DeniedFields []string
	Glossary     []GlossaryEntry
	// Memories are durable, user-curated facts (preferences, definitions,
	// prior resolutions) injected as remembered context.
	Memories []string
	// Instructions are admin-curated free-form business rules for the datasource,
	// injected as a "## Business Rules" block.
	Instructions []Instruction
	// Composite, when non-nil, marks this prompt as targeting a cross-domain
	// composite model and supplies the extra context (component domains,
	// cross-model joins, canonical date, renamed duplicate dimensions) the LLM
	// needs to reason across merged domains.
	Composite *CompositeContext
}

// CompositeContext describes the cross-domain shape of a composite semantic
// model so the prompt can explain to the LLM how the merged model was assembled.
// The merged SemanticModel already carries the flattened dimensions/metrics/joins;
// this struct only adds the domain-level narrative the flattened model loses.
type CompositeContext struct {
	// Name is the composite model's display name.
	Name string
	// Components lists the participating domains as "alias: model label" hints.
	Components []CompositeComponentHint
	// CrossModelJoins describes how the component domains connect.
	CrossModelJoins []CompositeJoinHint
	// CanonicalDate, when set, is the shared date dimension used to align time
	// grains across domains (e.g. "order_date").
	CanonicalDate string
	// RenamedDimensions lists duplicate dimensions that were alias-prefixed to
	// disambiguate them across domains (e.g. "sales_name vs customer_name").
	RenamedDimensions []string
}

// CompositeComponentHint names one component domain in a composite model.
type CompositeComponentHint struct {
	Alias string
	Label string
}

// CompositeJoinHint describes one cross-model join in human-readable terms.
type CompositeJoinHint struct {
	FromModel    string
	ToModel      string
	Relationship string
}

// Builder PromptBuilder constructs the AI prompt with semantic context.
type Builder struct{}

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
	Question      string
	LogicalQuery  string // raw JSON if available, else empty
	Note          string // e.g. "executed", "user rejected, asked to refine"
	ResultSummary string // compact previous answer/result summary, if available
}

type promptCatalogSections struct {
	dimensions string
	metrics    string
	note       string
	joins      string
}

func deniedFieldSet(deniedFields []string) map[string]bool {
	deniedSet := make(map[string]bool, len(deniedFields))
	for _, f := range deniedFields {
		deniedSet[strings.ToLower(f)] = true
	}
	return deniedSet
}

func filterAllowedDimensions(model *semantic.SemanticModel, denied map[string]bool) []semantic.Dimension {
	allowed := make([]semantic.Dimension, 0, len(model.Dimensions))
	for _, d := range model.Dimensions {
		if !denied[strings.ToLower(d.ColumnRef)] && !denied[strings.ToLower(d.Name)] {
			allowed = append(allowed, d)
		}
	}
	return allowed
}

func filterAllowedMetrics(model *semantic.SemanticModel, denied map[string]bool) []semantic.Metric {
	allowed := make([]semantic.Metric, 0, len(model.Metrics))
	for _, m := range model.Metrics {
		if !denied[strings.ToLower(m.Expression)] && !denied[strings.ToLower(m.Name)] {
			allowed = append(allowed, m)
		}
	}
	return allowed
}

func filterAllowedJoins(model *semantic.SemanticModel, denied map[string]bool) []semantic.Join {
	allowed := make([]semantic.Join, 0, len(model.Joins))
	for _, j := range model.Joins {
		if !denied[strings.ToLower(j.FromTable+"."+j.FromColumn)] && !denied[strings.ToLower(j.ToTable+"."+j.ToColumn)] {
			allowed = append(allowed, j)
		}
	}
	return allowed
}

func (b *Builder) buildPromptCatalogSections(
	model *semantic.SemanticModel,
	deniedSet map[string]bool,
	headRunes, maxPromptRunes int,
) promptCatalogSections {
	remaining := maxPromptRunes - headRunes - promptStaticReserveRunes
	if remaining < 16000 {
		remaining = 16000
	}

	allowedDims := filterAllowedDimensions(model, deniedSet)
	var omittedDims int
	dimensionsStr := withPooledBuffer(func(buf *bytes.Buffer) {
		omittedDims = b.writeDimensions(buf, allowedDims, remaining/2)
	})

	metricsBudget := maxPromptRunes - (headRunes + utf8.RuneCountInString(dimensionsStr)) - promptStaticReserveRunes/2
	if metricsBudget < 4000 {
		metricsBudget = 4000
	}
	allowedMetrics := filterAllowedMetrics(model, deniedSet)
	var omittedMetrics int
	metricsStr := withPooledBuffer(func(buf *bytes.Buffer) {
		omittedMetrics = b.writeMetrics(buf, allowedMetrics, metricsBudget)
	})

	var noteStr string
	if omittedDims > 0 || omittedMetrics > 0 {
		noteStr = fmt.Sprintf("## Note\nSome catalog entries were omitted to fit the model context window (%d dimensions, %d metrics skipped). Narrow **Tables** scope in the UI or define a smaller semantic model if a field is missing.\n\n",
			omittedDims, omittedMetrics)
	}

	allowedJoins := filterAllowedJoins(model, deniedSet)
	var joinsStr string
	if len(allowedJoins) > 0 {
		joinsStr = withPooledBuffer(func(buf *bytes.Buffer) {
			for _, j := range allowedJoins {
				writePromptf(buf, "- %s: %s.%s → %s.%s (%s, %s)\n",
					j.Name, j.FromTable, j.FromColumn, j.ToTable, j.ToColumn, j.JoinType, j.Relationship)
			}
		})
	}

	return promptCatalogSections{
		dimensions: dimensionsStr,
		metrics:    metricsStr,
		note:       noteStr,
		joins:      joinsStr,
	}
}

func promptHeadRunes(ctx context.Context, locale i18n.Locale, model *semantic.SemanticModel) int {
	rules := promptTemplate(ctx, locale, "system_rules")
	headBuf := new(bytes.Buffer)
	writePromptf(headBuf, "## Current Date/Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	writePromptf(headBuf, "## Semantic Model: %s\n", model.Name)
	if model.Label != nil {
		writePromptf(headBuf, "Label: %s\n", *model.Label)
	}
	if model.Description != nil {
		writePromptf(headBuf, "Description: %s\n", *model.Description)
	}
	writePromptf(headBuf, "Base table: %s.%s\n\n", model.BaseSchema, model.BaseTable)
	if len(model.Synonyms) > 0 {
		writePromptf(headBuf, "Model synonyms: %s\n\n", strings.Join(model.Synonyms, ", "))
	}
	return utf8.RuneCountInString(rules) + utf8.RuneCount(headBuf.Bytes())
}

func (b *Builder) buildPromptTemplateData(
	ctx context.Context,
	question string,
	model *semantic.SemanticModel,
	cfg Config,
	catalog promptCatalogSections,
) map[string]any {
	locale := LocaleForQuestion(question, cfg.Locale)
	rules := promptTemplate(ctx, locale, "system_rules")
	outputFmt := promptTemplate(ctx, locale, "output_format")

	glossaryStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeBusinessGlossary(buf, cfg.Glossary) })
	memoriesStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeMemories(buf, cfg.Memories) })
	instructionsStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeInstructions(buf, cfg.Instructions) })
	dialectStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeDialectCompilationGuide(buf, cfg.Dialect) })
	failureStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeFailureExamples(buf) })
	planningStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writePlanningSteps(buf) })
	sampleStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeSampleData(buf, cfg.Samples) })
	exampleStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeFewShotExamples(buf, cfg.Examples, locale) })
	priorStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writePriorTurns(buf, cfg.PriorTurns) })
	compositeStr := withPooledBuffer(func(buf *bytes.Buffer) { b.writeCompositeContext(buf, cfg.Composite) })

	var labelStr, descStr, modelSynonymsStr string
	if model.Label != nil {
		labelStr = *model.Label
	}
	if model.Description != nil {
		descStr = *model.Description
	}
	if len(model.Synonyms) > 0 {
		modelSynonymsStr = strings.Join(model.Synonyms, ", ")
	}

	return map[string]any{
		"SystemRules":      rules,
		"CurrentDateTime":  time.Now().Format("2006-01-02 15:04:05 UTC"),
		"ModelName":        model.Name,
		"ModelLabel":       labelStr,
		"ModelDescription": descStr,
		"BaseTable":        fmt.Sprintf("%s.%s", model.BaseSchema, model.BaseTable),
		"ModelSynonyms":    modelSynonymsStr,
		"CompositeContext": compositeStr,
		"Dimensions":       catalog.dimensions,
		"Metrics":          catalog.metrics,
		"Note":             catalog.note,
		"Joins":            catalog.joins,
		"FilterOperators":  "eq, neq, gt, gte, lt, lte, in, not_in, contains, starts_with, ends_with, between, is_null, is_not_null\n\n",
		"Glossary":         glossaryStr,
		"Memories":         memoriesStr,
		"Instructions":     instructionsStr,
		"DialectGuide":     dialectStr,
		"FailureExamples":  failureStr,
		"PlanningSteps":    planningStr,
		"Question":         question,
		"OutputFormat":     outputFmt,
		"SampleData":       sampleStr,
		"Examples":         exampleStr,
		"PriorTurns":       priorStr,
	}
}

// Build creates the full prompt for the AI.
func (b *Builder) Build(ctx context.Context, question string, model *semantic.SemanticModel, cfg Config) string {
	maxPromptRunes := cfg.MaxRunes
	if maxPromptRunes <= 0 {
		maxPromptRunes = 80000
	}
	locale := LocaleForQuestion(question, cfg.Locale)
	deniedSet := deniedFieldSet(cfg.DeniedFields)
	headRunes := promptHeadRunes(ctx, locale, model)
	catalog := b.buildPromptCatalogSections(model, deniedSet, headRunes, maxPromptRunes)
	data := b.buildPromptTemplateData(ctx, question, model, cfg, catalog)

	layoutTmpl := promptTemplate(ctx, locale, "prompt_layout")
	if layoutTmpl == "" {
		layoutTmpl = defaultLayout
	}
	return renderPromptTemplate(layoutTmpl, data)
}

// writeCompositeContext appends a cross-domain narrative for composite models.
// The merged SemanticModel flattens everything into one dimension/metric/join
// list, which hides that the data spans several domains joined through shared
// keys. This block restores that domain-level context so the LLM understands
// the model is cross-domain, how the domains connect, which date dimension
// aligns time grains, and which dimensions were renamed to avoid collisions.
func (b *Builder) writeCompositeContext(sb *bytes.Buffer, cc *CompositeContext) { //nolint:revive // keeps PromptBuilder method grouping
	if cc == nil {
		return
	}
	_, _ = sb.WriteString("\n## Composite Model\n")
	_, _ = sb.WriteString("This is a cross-domain composite model: its dimensions and metrics come from multiple business domains merged into one model. Treat all listed fields as a single model — the backend resolves the cross-domain joins automatically.\n")
	if len(cc.Components) > 0 {
		_, _ = sb.WriteString("\nComponent domains:\n")
		for _, comp := range cc.Components {
			label := strings.TrimSpace(comp.Label)
			if label == "" {
				writePromptf(sb, "- %s\n", comp.Alias)
				continue
			}
			writePromptf(sb, "- %s: %s\n", comp.Alias, label)
		}
	}
	if len(cc.CrossModelJoins) > 0 {
		_, _ = sb.WriteString("\nCross-domain connections:\n")
		for _, j := range cc.CrossModelJoins {
			rel := strings.TrimSpace(j.Relationship)
			if rel == "" {
				writePromptf(sb, "- %s ↔ %s\n", j.FromModel, j.ToModel)
				continue
			}
			writePromptf(sb, "- %s ↔ %s (%s)\n", j.FromModel, j.ToModel, rel)
		}
	}
	if date := strings.TrimSpace(cc.CanonicalDate); date != "" {
		writePromptf(sb, "\nCanonical date: use **%s** for any date filter or time grouping so results align across domains.\n", date)
	}
	if len(cc.RenamedDimensions) > 0 {
		writePromptf(sb, "\nDisambiguated dimensions (renamed to avoid collisions across domains): %s\n", strings.Join(cc.RenamedDimensions, ", "))
	}
	_, _ = sb.WriteString("\n")
}

// writePriorTurns appends recent turns from the active conversation so the
// model can resolve follow-ups like "now filter to last quarter" or "same
// breakdown but by region". Each turn shows the user's question and, if
// available, the prior LogicalQuery the assistant produced.
func (*Builder) writePriorTurns(sb *bytes.Buffer, turns []ConversationTurn) {
	if len(turns) == 0 {
		return
	}
	_, _ = sb.WriteString("\n\n## Prior Turns in This Conversation\n")
	_, _ = sb.WriteString("Use these to resolve references like \"that\", \"now\", \"also\", \"instead\". The latest user question (above) takes precedence.\n")
	for i, t := range turns {
		q := strings.TrimSpace(t.Question)
		if q == "" {
			continue
		}
		writePromptf(sb, "\nTurn %d — Question: %q\n", i+1, q)
		if lq := strings.TrimSpace(t.LogicalQuery); lq != "" {
			writePromptf(sb, "Previous LogicalQuery: %s\n", lq)
		}
		if note := strings.TrimSpace(t.Note); note != "" {
			writePromptf(sb, "Note: %s\n", note)
		}
		if result := strings.TrimSpace(t.ResultSummary); result != "" {
			writePromptf(sb, "Result: %s\n", truncatePriorTurnResult(result, 300))
		}
	}
}

func truncatePriorTurnResult(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes-3]) + "..."
}

// writeSampleData appends a compact JSON block of concrete rows so the LLM can
// see actual values (formats, casing, enum-like fields). Skipped silently if
// no samples were provided.
func (*Builder) writeSampleData(sb *bytes.Buffer, samples []TableSample) {
	if len(samples) == 0 {
		return
	}
	writePromptString(sb, "\n\n## Sample Data\n")
	for _, s := range samples {
		if len(s.Rows) == 0 {
			continue
		}
		writePromptf(sb, "### %s.%s\n", s.Schema, s.Table)
		enc := sonic.ConfigStd.NewEncoder(sb)
		if err := enc.Encode(s.Rows); err != nil {
			continue
		}
		writePromptString(sb, "\n")
	}
}

// writeFewShotExamples appends previously-successful (question, logical_query)
// pairs to the prompt as additional examples. Locale-tagged rows are preferred
// when matching the active locale; locale-empty rows are always eligible and
// rows tagged for a different locale are filtered out. Skipped silently if no
// rows survive the filter.
func (*Builder) writeFewShotExamples(sb *bytes.Buffer, examples []FewShotExample, locale i18n.Locale) {
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
	writePromptString(sb, "\n\n## Examples — Successful Past Queries\n")
	for _, ex := range filtered {
		writePromptf(sb, "Question: %q\n%s\n\n", strings.TrimSpace(ex.Question), strings.TrimSpace(ex.LogicalQuery))
	}
}

func (*Builder) writeDimensions(sb *bytes.Buffer, dims []semantic.Dimension, budgetRunes int) int {
	if budgetRunes <= 0 {
		writePromptString(sb, "(dimensions omitted — size budget)\n")
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
				tg = ", time_grain: " + d.TimeGrain
			}
			sy := ""
			if syn != "" {
				sy = ", synonyms: " + syn
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
			tg = ", time_grain: " + d.TimeGrain
		}
		sy := ""
		if syn != "" {
			sy = ", synonyms: " + syn
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

func (*Builder) writeMetrics(sb *bytes.Buffer, metrics []semantic.Metric, budgetRunes int) int {
	if budgetRunes <= 0 {
		writePromptString(sb, "(metrics omitted — size budget)\n")
		return len(metrics)
	}
	used := 0
	omitted := 0
	for i, m := range metrics {
		syn := joinSynonymsCap(m.Synonyms, maxSynonymsPerLine)
		sy := ""
		if syn != "" {
			sy = ", synonyms: " + syn
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

// BuildAmbiguityAnalysis asks the LLM to identify unclear terms before
// LogicalQuery generation. The result is structured JSON consumed by the
// ambiguity analyzer, never SQL.
func (*Builder) BuildAmbiguityAnalysis(ctx context.Context, locale i18n.Locale, question string, model *semantic.SemanticModel, glossary []GlossaryEntry) string {
	locale = LocaleForQuestion(question, locale)
	if model == nil {
		model = &semantic.SemanticModel{}
	}

	names := make([]string, 0, len(model.Dimensions))
	for _, dimension := range model.Dimensions {
		names = append(names, dimension.Name)
	}
	metricNames := make([]string, 0, len(model.Metrics))
	for _, metric := range model.Metrics {
		metricNames = append(metricNames, metric.Name)
	}
	glossaryEntries := make([]string, 0, len(glossary))
	for _, entry := range glossary {
		glossaryEntries = append(glossaryEntries, entry.Term+": "+entry.Definition)
	}

	return renderPromptTemplate(promptTemplate(ctx, locale, "ambiguity"), map[string]any{
		"Question":   question,
		"ModelName":  model.Name,
		"Dimensions": strings.Join(names, ", "),
		"Metrics":    strings.Join(metricNames, ", "),
		"Glossary":   strings.Join(glossaryEntries, "; "),
	})
}

// BuildClarification asks the LLM to produce a single, user-facing clarifying
// question explaining what is ambiguous about the original request, given the
// available semantic model. Output is plain text (no JSON) — short, natural.
func (*Builder) BuildClarification(ctx context.Context, locale i18n.Locale, question string, model *semantic.SemanticModel, failureReason string) string {
	locale = LocaleForQuestion(question, locale)
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

	return withPooledBuffer(func(sb *bytes.Buffer) {
		sb.WriteString("You are a Business Intelligence assistant. The user asked a question that could not be answered with the current semantic model.\n\n")
		writePromptf(sb, "## User Question\n%s\n\n", question)
		if failureReason != "" {
			writePromptf(sb, "## Why It Was Ambiguous\n%s\n\n", failureReason)
		}
		writePromptf(sb, "## Semantic Model: %s\n", model.Name)
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
	})
}

// BuildRetry constructs a corrective prompt to send back to the LLM when the
// previous attempt produced unparseable or semantically-invalid output. The
// original prompt is reused as the source of truth for the schema; the
// addendum carries the model's failed response and the validation error.
func (*Builder) BuildRetry(ctx context.Context, locale i18n.Locale, originalPrompt, lastResponse, validationError string) string {
	tmpl := promptTemplate(ctx, locale, "retry")
	if tmpl != "" {
		return renderPromptTemplate(tmpl, map[string]any{
			"OriginalPrompt":   originalPrompt,
			"LastResponse":     lastResponse,
			"ValidationError":  validationError,
			"ValidationErrors": validationError,
		})
	}

	return withPooledBuffer(func(sb *bytes.Buffer) {
		sb.WriteString(originalPrompt)
		sb.WriteString("\n\n## Previous Attempt (incorrect)\n")
		sb.WriteString("Your previous response was:\n")
		sb.WriteString(lastResponse)
		sb.WriteString("\n\n## Why It Failed\n")
		sb.WriteString(validationError)
		sb.WriteString("\n\n## Required Action\n")
		sb.WriteString("Re-run the **Planning Steps** from the original prompt, then re-emit a corrected LogicalQuery JSON object using dimensions, metrics, and operators that exist in the semantic model above. Do not repeat the previous mistake. Optional `## Reasoning` prefix allowed; final output must include valid JSON — no markdown fences.\n")
	})
}

// BuildEmptyRetry constructs a corrective prompt for the specific case where the
// previous attempt returned a blank completion. Unlike BuildRetry it does not
// echo the (empty) last response; it reuses the original prompt as the schema
// source of truth and appends a JSON-only emphasis so the model stops emitting
// reasoning/prose and returns the LogicalQuery object directly.
func (*Builder) BuildEmptyRetry(ctx context.Context, locale i18n.Locale, originalPrompt, failureReason string) string {
	tmpl := promptTemplate(ctx, locale, "empty_retry")
	if tmpl != "" {
		return renderPromptTemplate(tmpl, map[string]any{
			"OriginalPrompt": originalPrompt,
			"FailureReason":  failureReason,
		})
	}

	return withPooledBuffer(func(sb *bytes.Buffer) {
		sb.WriteString(originalPrompt)
		sb.WriteString("\n\n## Previous Attempt (empty)\n")
		if locale == i18n.LocaleTR {
			sb.WriteString("Önceki yanıtınız boştu. YALNIZCA LogicalQuery JSON nesnesini döndürün — hiçbir açıklama, gerekçe, düşünce veya markdown olmadan. Yanıtınıza doğrudan `{` ile başlayın ve `}` ile bitirin.\n")
		} else {
			sb.WriteString("Your previous response was empty. Output ONLY the LogicalQuery JSON object — no reasoning, no prose, no markdown. Start your response with `{` and end with `}`.\n")
		}
	})
}

// RepairStrategy returns the locale-specific repair instruction for a 1-indexed
// repair attempt. Attempt 1 is a minimal, surgical fix; attempt 2 re-evaluates
// structure; attempt 3+ regenerates the whole query. Shared by BuildRepairPrompt
// and the service repair loop so the prompt text and recorded telemetry stay in sync.
func RepairStrategy(locale i18n.Locale, attempt int) string {
	if locale == i18n.LocaleTR {
		switch attempt {
		case 1:
			return "Hata listesinde belirtilen geçersiz boyut/metrik adlarını düzeltmeye odaklanın. Sorgunun geri kalanını değiştirmeyin."
		case 2:
			return "Sorgu yapısını, join yollarını ve seçilen alanları tekrar değerlendirin; ancak kullanıcının amacına sadık kalın."
		default:
			return "Tüm LogicalQuery'yi sıfırdan oluşturun; önceki hatalardan kaçındığınızdan emin olun."
		}
	}
	switch attempt {
	case 1:
		return "Focus strictly on fixing the invalid dimension/metric names highlighted in the error list. Keep the rest of the query identical."
	case 2:
		return "Re-evaluate the query structure, including join paths and selected fields, while keeping the user's intent intact."
	default:
		return "Regenerate the entire LogicalQuery from scratch; ensure you avoid all previous errors."
	}
}

// BuildRepairPrompt builds a highly-focused corrective prompt to guide the LLM to fix the validation errors.
func (*Builder) BuildRepairPrompt(ctx context.Context, locale i18n.Locale, originalPrompt, lastResponse string, errs query.ValidationErrors, attempt int) string {
	tmpl := promptTemplate(ctx, locale, "repair")
	if tmpl == "" {
		tmpl = promptTemplate(ctx, locale, "retry")
	}

	strategyStr := RepairStrategy(locale, attempt)

	var explanation strings.Builder
	for _, e := range errs {
		if e.Code != "" {
			writePromptf(&explanation, "- Field: %s, Error Code: %s, Message: %s", e.Field, e.Code, e.Message)
		} else {
			writePromptf(&explanation, "- Field: %s, Message: %s", e.Field, e.Message)
		}
		if len(e.AllowedAlternatives) > 0 {
			writePromptf(&explanation, " (did you mean one of: %s?)", strings.Join(e.AllowedAlternatives, ", "))
		}
		writePromptString(&explanation, "\n")
	}

	if tmpl != "" {
		return renderPromptTemplate(tmpl, map[string]any{
			"OriginalPrompt":      originalPrompt,
			"LastResponse":        lastResponse,
			"ValidationError":     explanation.String(),
			"ValidationErrors":    errs.ToRepairJSON(),
			"Attempt":             attempt,
			"StrategyInstruction": strategyStr,
		})
	}

	return withPooledBuffer(func(sb *bytes.Buffer) {
		sb.WriteString(originalPrompt)
		sb.WriteString("\n\n## Previous Attempt (incorrect)\n")
		sb.WriteString("Your previous response was:\n")
		sb.WriteString(lastResponse)
		sb.WriteString("\n\n## Why It Failed\n")
		sb.WriteString("The previous LogicalQuery had validation errors:\n")
		sb.WriteString(explanation.String())
		sb.WriteString("\n## Required Action\n")
		writePromptf(sb, "Repair Strategy: %s\n", strategyStr)
		sb.WriteString("Re-run the **Planning Steps** from the original prompt, and fix each validation error above. Replace incorrect dimensions, metrics, or operators with allowed alternatives. Do not repeat the previous mistake. Final output must include valid JSON - no markdown fences.\n")
	})
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
	for i := range n {
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
