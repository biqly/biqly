package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// Service orchestrates the AI text-to-query flow.
type Service struct {
	client              Provider
	promptBuilder       *PromptBuilder
	validator           *query.Validator
	aiCfg               config.AIConfig
	queryModel          string
	maxPromptRunes      int
	maxRetries          int
	multiCandidateCount int
	baseTemperature     float64
}

func newService(cfg config.AIConfig, validator *query.Validator, provider Provider) *Service {
	maxR := cfg.MaxPromptInputRunes
	if maxR <= 0 {
		maxR = 80000
	}
	retries := cfg.MaxRetries
	if retries < 0 {
		retries = 0
	}
	effective := cfg.EffectiveQueryConfig()
	return &Service{
		client:              provider,
		promptBuilder:       &PromptBuilder{},
		validator:           validator,
		aiCfg:               effective,
		queryModel:          effective.Model,
		maxPromptRunes:      maxR,
		maxRetries:          retries,
		multiCandidateCount: cfg.MultiCandidateCount,
		baseTemperature:     cfg.Temperature,
	}
}

// NewService creates a new AI service. Returns an error if the configured
// provider is unknown — callers should surface this at startup.
func NewService(cfg config.AIConfig, validator *query.Validator) *Service {
	// Fall back to the OpenAI client on unknown providers so test setups that
	// pass minimal configs (no Provider field) keep working. Production code
	// should call NewServiceWithProvider for explicit error handling.
	provider, err := NewProvider(cfg)
	if err != nil {
		provider = NewClient(cfg)
	}
	return newService(cfg, validator, provider)
}

// NewServiceWithProvider wires a service around an explicitly-supplied Provider.
// Use this in production wiring where unknown-provider should be a fatal error.
func NewServiceWithProvider(cfg config.AIConfig, validator *query.Validator, provider Provider) *Service {
	return newService(cfg, validator, provider)
}

// LLMProvider returns the configured generation backend (for eval judge, etc.).
func (s *Service) LLMProvider() Provider {
	return s.client
}

// SQLValidator dry-runs a compiled LogicalQuery against the live datasource
// (e.g. via EXPLAIN) and returns an error describing any planner-level issue.
// Returning nil means the query is safe to execute.
type SQLValidator func(ctx context.Context, lq *query.LogicalQuery) error

// ProcessOption customizes how ProcessQuestion runs. Use functional options to
// keep the call site backward compatible while enabling optional features.
type ProcessOption func(*processOptions)

type processOptions struct {
	sqlValidator  SQLValidator
	fewShot       []FewShotExample
	samples       []TableSample
	priorTurns    []ConversationTurn
	deniedFields  []string
	targetDialect string
	glossary      []GlossaryEntry
}

// WithSQLValidator wires a dialect-aware dry-run check (e.g. EXPLAIN) into the
// retry loop: a failing dry-run is treated like a validation error and triggers
// a corrective re-prompt.
func WithSQLValidator(v SQLValidator) ProcessOption {
	return func(o *processOptions) { o.sqlValidator = v }
}

// WithFewShotExamples injects prior successful (question, logical_query) pairs
// into the prompt. Pass the most recent N high-confidence rows from history.
func WithFewShotExamples(examples []FewShotExample) ProcessOption {
	return func(o *processOptions) { o.fewShot = examples }
}

// WithSampleData attaches concrete row samples from the queried tables so the
// LLM sees actual values (not just column names). Cells should already be
// truncated by the caller via FetchTableSample's maxCellRunes parameter.
func WithSampleData(samples []TableSample) ProcessOption {
	return func(o *processOptions) { o.samples = samples }
}

// WithPriorTurns supplies recent turns from the active conversation so the
// model can resolve follow-ups in context. Pass the most recent N turns
// (caller should cap N — typically 3-5 — to keep the prompt bounded).
func WithPriorTurns(turns []ConversationTurn) ProcessOption {
	return func(o *processOptions) { o.priorTurns = turns }
}

// WithDeniedFields prevents listed field names (dimensions, metrics, columns)
// from appearing in the AI prompt — used in strict mode so the LLM cannot see
// or select permission-denied fields.
func WithDeniedFields(fields []string) ProcessOption {
	return func(o *processOptions) { o.deniedFields = fields }
}

// WithTargetDialect sets the datasource SQL engine name for dialect-aware
// compilation examples in the prompt (postgres, mysql, sqlserver, clickhouse).
func WithTargetDialect(dialectName string) ProcessOption {
	return func(o *processOptions) { o.targetDialect = dialectName }
}

// WithGlossary injects business-term → field mappings into the prompt.
func WithGlossary(entries []GlossaryEntry) ProcessOption {
	return func(o *processOptions) { o.glossary = entries }
}

// ProcessQuestion handles a natural language question. On parse or validation
// failure the LLM is re-prompted with the prior output and error message, up
// to s.maxRetries additional attempts.
func (s *Service) ProcessQuestion(ctx context.Context, question string, model *semantic.SemanticModel, opts ...ProcessOption) (*AIResponse, error) {
	options := processOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	filterSess := FilterSessionFromPriorTurns(options.priorTurns)
	followIntent := ClassifyFollowUpIntent(question, filterSess)

	basePrompt, baseStats := s.buildPrompt(ctx, question, model, 0, options, filterSess, followIntent)

	// Self-consistency: when configured, draw N candidates with stepped temperatures
	// and vote. A clear majority returns immediately; otherwise we fall through to
	// the standard retry loop which handles single-shot generation + correction.
	if s.multiCandidateCount > 1 {
		if resp, ok := s.tryMultiCandidate(ctx, question, basePrompt, model, options, baseStats, filterSess, followIntent); ok {
			return resp, nil
		}
	}

	var (
		prompt             = basePrompt
		promptStats        = baseStats
		lastRaw            string
		lastGen            GenerationResult
		retryWarnings      []string
		lq                 *query.LogicalQuery
		warnings           []string
		validationErrCount int
		parseErr           error
	)

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		gen, genErr := s.client.Generate(ctx, prompt)
		if genErr != nil {
			return nil, fmt.Errorf("AI generation failed: %w", genErr)
		}
		lastGen = gen
		lastRaw = gen.Content

		lq, warnings, validationErrCount, parseErr = s.parseAndValidate(gen.Content, model)

		// Dry-run check (e.g. EXPLAIN) only when the query parsed and passed
		// semantic validation; otherwise the SQL would not be compilable anyway.
		var sqlErr error
		if parseErr == nil && validationErrCount == 0 && options.sqlValidator != nil {
			sqlErr = options.sqlValidator(ctx, lq)
			if sqlErr != nil {
				warnings = append(warnings, "dry-run failed: "+sqlErr.Error())
			}
		}

		if parseErr == nil && validationErrCount == 0 && sqlErr == nil {
			if inheritNotes := ApplyFilterSession(lq, filterSess, followIntent); len(inheritNotes) > 0 {
				warnings = append(warnings, inheritNotes...)
			}
			retries := attempt
			templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)
			resp := &AIResponse{
				LogicalQuery:                lq,
				Confidence:                  computeConfidence(validationErrCount, retries),
				Warnings:                    append(retryWarnings, warnings...),
				Prompt:                      prompt,
				RawResponse:                 gen.Content,
				RetryCount:                  retries,
				PromptStats:                 &promptStats,
				TokenUsage:                  tokenUsageFromGeneration(promptStats, gen),
				PromptTemplateLocale:        templateLocale,
				PromptTemplateVersions:      templateVersions,
				PromptTemplateBundleVersion: bundleVersion,
			}
			return resp, nil
		}

		// Out of attempts — fall through to error/warning response.
		if attempt == s.maxRetries {
			break
		}

		// Re-prompt with the failure context for the next attempt.
		failureMsg := failureMessageFor(parseErr, sqlErr, warnings)
		retryWarnings = append(retryWarnings, fmt.Sprintf("retry %d (context %s): %s", attempt+1, contextTierLabel(contextTierForAttempt(attempt+1)), failureMsg))
		nextTier := contextTierForAttempt(attempt + 1)
		expanded, _ := s.buildPrompt(ctx, question, model, nextTier, options, filterSess, followIntent)
		prompt = s.promptBuilder.BuildRetry(ctx, PromptLocaleForQuestion(question, i18n.FromContext(ctx)), expanded, gen.Content, failureMsg)
		promptStats = MeasurePrompt(prompt, s.queryModel, nextTier, s.aiCfg)
	}

	failureReason := failureMessageFor(parseErr, nil, warnings)

	if parseErr != nil {
		clarification := s.tryGenerateClarification(ctx, question, model, failureReason)
		templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)
		return newClarificationResponse(clarificationInputs{
			LogicalQuery:                nil,
			Confidence:                  0,
			Warnings:                    append(retryWarnings, append(warnings, parseErr.Error())...),
			Prompt:                      prompt,
			RawResponse:                 lastRaw,
			RetryCount:                  s.maxRetries,
			PromptStats:                 promptStats,
			Gen:                         lastGen,
			Clarification:               clarification,
			FailureReason:               failureReason,
			Source:                      "ai",
			PromptTemplateLocale:        templateLocale,
			PromptTemplateVersions:      templateVersions,
			PromptTemplateBundleVersion: bundleVersion,
		}), nil
	}

	clarification := ""
	if validationErrCount > 0 {
		clarification = s.tryGenerateClarification(ctx, question, model, failureReason)
		templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)
		return newClarificationResponse(clarificationInputs{
			LogicalQuery:                nil,
			Confidence:                  0,
			Warnings:                    append(retryWarnings, warnings...),
			Prompt:                      prompt,
			RawResponse:                 lastRaw,
			RetryCount:                  s.maxRetries,
			PromptStats:                 promptStats,
			Gen:                         lastGen,
			Clarification:               clarification,
			FailureReason:               failureReason,
			Source:                      "validator",
			PromptTemplateLocale:        templateLocale,
			PromptTemplateVersions:      templateVersions,
			PromptTemplateBundleVersion: bundleVersion,
		}), nil
	}

	templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)
	return newClarificationResponse(clarificationInputs{
		LogicalQuery:                lq,
		Confidence:                  computeConfidence(validationErrCount, s.maxRetries),
		Warnings:                    append(retryWarnings, warnings...),
		Prompt:                      prompt,
		RawResponse:                 lastRaw,
		RetryCount:                  s.maxRetries,
		PromptStats:                 promptStats,
		Gen:                         lastGen,
		Clarification:               clarification,
		FailureReason:               failureReason,
		Source:                      "ai",
		PromptTemplateLocale:        templateLocale,
		PromptTemplateVersions:      templateVersions,
		PromptTemplateBundleVersion: bundleVersion,
	}), nil
}

// clarificationInputs collects the variable fields used to assemble an
// AIResponse for failure / partial-success paths that may need to request
// clarification from the user.
type clarificationInputs struct {
	LogicalQuery                *query.LogicalQuery
	Confidence                  float64
	Warnings                    []string
	Prompt                      string
	RawResponse                 string
	RetryCount                  int
	PromptStats                 PromptStats
	Gen                         GenerationResult
	Clarification               string
	FailureReason               string
	Source                      string
	PromptTemplateLocale        string
	PromptTemplateVersions      map[string]int
	PromptTemplateBundleVersion int
}

func newClarificationResponse(in clarificationInputs) *AIResponse {
	stats := in.PromptStats
	return &AIResponse{
		LogicalQuery:                in.LogicalQuery,
		Confidence:                  in.Confidence,
		Warnings:                    in.Warnings,
		Prompt:                      in.Prompt,
		RawResponse:                 in.RawResponse,
		RetryCount:                  in.RetryCount,
		PromptStats:                 &stats,
		TokenUsage:                  tokenUsageFromGeneration(stats, in.Gen),
		NeedsClarification:          in.Clarification != "",
		ClarificationQuestion:       in.Clarification,
		Clarification:               buildClarification(in.Clarification, in.FailureReason, in.Source),
		PromptTemplateLocale:        in.PromptTemplateLocale,
		PromptTemplateVersions:      in.PromptTemplateVersions,
		PromptTemplateBundleVersion: in.PromptTemplateBundleVersion,
	}
}

func promptTemplateTrace(ctx context.Context, question string) (string, map[string]int, int) {
	loc := PromptLocaleForQuestion(question, i18n.FromContext(ctx))
	versions := PromptTemplateBundleVersions(ctx, loc)
	maxVersion := 0
	for _, v := range versions {
		if v > maxVersion {
			maxVersion = v
		}
	}
	return string(loc), versions, maxVersion
}

func (s *Service) buildPrompt(
	ctx context.Context,
	question string,
	model *semantic.SemanticModel,
	tier int,
	options processOptions,
	filterSess *FilterSessionState,
	followIntent FollowUpIntent,
) (string, PromptStats) {
	tiered := applyContextTier(options, tier)
	promptRunes := PromptRunesForTier(s.maxPromptRunes, tier, s.aiCfg, s.queryModel)
	start := time.Now()
	prompt := s.promptBuilder.Build(
		ctx,
		question,
		model,
		promptRunes,
		i18n.FromContext(ctx),
		options.targetDialect,
		tiered.fewShot,
		tiered.samples,
		tiered.priorTurns,
		tiered.deniedFields,
		tiered.glossary,
	)
	if block := ActiveFilterInstructions(filterSess, followIntent); block != "" {
		prompt += block
	}
	buildDurationMs := time.Since(start).Milliseconds()
	stats := MeasurePrompt(prompt, s.queryModel, tier, s.aiCfg)
	stats.PromptBuildDurationMs = buildDurationMs
	slog.InfoContext(ctx, "ai prompt context",
		"model", stats.Model,
		"context_tier", stats.ContextTierLabel,
		"prompt_runes", stats.PromptRunes,
		"est_prompt_tokens", stats.EstPromptTokens,
		"max_prompt_runes", stats.MaxPromptRunes,
		"context_window_tokens", stats.ContextWindowTokens,
		"prompt_build_ms", stats.PromptBuildDurationMs,
	)
	return prompt, stats
}

func tokenUsageEstimate(stats PromptStats, completion string) *TokenUsage {
	promptTok := stats.EstPromptTokens
	completionTok := EstimateTokens(completion)
	if promptTok == 0 && completionTok == 0 {
		return nil
	}
	return newTokenUsage(promptTok, completionTok, 0)
}

// buildClarification wraps a free-text clarification question into the
// structured Clarification envelope. Returns nil when there is nothing to ask.
// Options are populated by callers that have discrete candidates (router,
// validator with ambiguous metric names); the AI fallback path leaves them
// empty.
func buildClarification(question, reason, source string) *Clarification {
	if question == "" {
		return nil
	}
	return &Clarification{
		Status:   ClarificationStatusNeeded,
		Question: question,
		Reason:   reason,
		Source:   source,
	}
}

// tryMultiCandidate draws s.multiCandidateCount completions at stepped
// temperatures and votes on a structurally-equivalent LogicalQuery. A strict
// majority returns immediately; ties or no successful candidates fall through
// to the standard single-shot + retry path. Best-effort: any provider error
// falls back rather than aborting the whole request.
func (s *Service) tryMultiCandidate(
	ctx context.Context,
	question string,
	prompt string,
	model *semantic.SemanticModel,
	options processOptions,
	stats PromptStats,
	filterSess *FilterSessionState,
	followIntent FollowUpIntent,
) (*AIResponse, bool) {
	n := s.multiCandidateCount
	if n < 2 {
		return nil, false
	}

	type candidate struct {
		idx      int
		lq       *query.LogicalQuery
		gen      GenerationResult
		warnings []string
		fp       string
	}

	// Run all N candidate generations concurrently. Each call talks to an
	// LLM API which is typically several hundred ms — running them serially
	// multiplied total latency by N. Errors and validation failures are
	// best-effort and do not cancel siblings.
	results := make([]*candidate, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		idx := i
		go func() {
			defer wg.Done()
			temp := s.baseTemperature + 0.2*float64(idx)
			if temp > 1 {
				temp = 1
			}
			gen, err := s.client.GenerateAt(ctx, prompt, temp)
			if err != nil {
				return
			}
			lq, warnings, validationErrCount, parseErr := s.parseAndValidate(gen.Content, model)
			if parseErr != nil || validationErrCount > 0 || lq == nil {
				return
			}
			if options.sqlValidator != nil {
				if err := options.sqlValidator(ctx, lq); err != nil {
					return
				}
			}
			results[idx] = &candidate{
				idx:      idx,
				lq:       lq,
				gen:      gen,
				warnings: warnings,
				fp:       logicalQueryFingerprint(lq),
			}
		}()
	}
	wg.Wait()

	groups := make(map[string][]candidate)
	successCount := 0
	for _, c := range results {
		if c == nil {
			continue
		}
		groups[c.fp] = append(groups[c.fp], *c)
		successCount++
	}

	if successCount == 0 {
		return nil, false
	}

	var winnerKey string
	var winnerCount int
	for k, group := range groups {
		if len(group) > winnerCount {
			winnerKey = k
			winnerCount = len(group)
		}
	}
	// Strict majority among successful samples.
	if winnerCount*2 <= successCount {
		return nil, false
	}
	winner := groups[winnerKey][0]
	if inheritNotes := ApplyFilterSession(winner.lq, filterSess, followIntent); len(inheritNotes) > 0 {
		winner.warnings = append(winner.warnings, inheritNotes...)
	}

	confidence := float64(winnerCount) / float64(n)
	if confidence > 1 {
		confidence = 1
	}
	templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)
	return &AIResponse{
		LogicalQuery: winner.lq,
		Confidence:   confidence,
		Warnings: append(
			[]string{fmt.Sprintf("self-consistency: %d/%d candidates agreed", winnerCount, n)},
			winner.warnings...,
		),
		Prompt:                      prompt,
		RawResponse:                 winner.gen.Content,
		PromptStats:                 &stats,
		TokenUsage:                  tokenUsageFromGeneration(stats, winner.gen),
		PromptTemplateLocale:        templateLocale,
		PromptTemplateVersions:      templateVersions,
		PromptTemplateBundleVersion: bundleVersion,
	}, true
}

// logicalQueryFingerprint returns a canonical string identifying the structure
// of a LogicalQuery so different completions can be voted as equivalent.
// Datasource/model identifiers and free-form descriptions are excluded.
func logicalQueryFingerprint(lq *query.LogicalQuery) string {
	if lq == nil {
		return ""
	}
	type fp struct {
		Select  []query.SelectItem `json:"select"`
		Filters []query.Filter     `json:"filters"`
		GroupBy []query.GroupBy    `json:"group_by"`
		OrderBy []query.OrderBy    `json:"order_by"`
		Limit   int                `json:"limit"`
		Offset  int                `json:"offset"`
	}
	out, err := json.Marshal(fp{
		Select:  lq.Select,
		Filters: lq.Filters,
		GroupBy: lq.GroupBy,
		OrderBy: lq.OrderBy,
		Limit:   lq.Limit,
		Offset:  lq.Offset,
	})
	if err != nil {
		return ""
	}
	return string(out)
}

// tryGenerateClarification asks the LLM for one short clarifying question to
// surface to the user. Failures are swallowed: clarification is best-effort
// and must never mask the underlying validation/parse error to the caller.
func (s *Service) tryGenerateClarification(ctx context.Context, question string, model *semantic.SemanticModel, failureReason string) string {
	loc := PromptLocaleForQuestion(question, i18n.FromContext(ctx))
	prompt := s.promptBuilder.BuildClarification(ctx, loc, question, model, failureReason)
	gen, err := s.client.Generate(ctx, prompt)
	var content string
	if err == nil {
		content = strings.TrimSpace(gen.Content)
	}
	if content == "" {
		if loc == i18n.LocaleTR {
			return "Ne istediğinizi tam olarak anlayamadım, lütfen sorunuzu biraz daha detaylandırabilir misiniz?"
		}
		return "I couldn't quite understand what you wanted. Could you please provide more details or clarify your question?"
	}
	return content
}

// failureMessageFor renders a concise, LLM-readable description of why the
// previous attempt failed: parse error first (most fundamental), then SQL
// dry-run error (planner-level), then accumulated validation warnings.
func failureMessageFor(parseErr, sqlErr error, warnings []string) string {
	if parseErr != nil {
		return parseErr.Error()
	}
	if sqlErr != nil {
		return "dry-run failed: " + sqlErr.Error()
	}
	if len(warnings) == 0 {
		return "previous output was rejected"
	}
	return strings.Join(warnings, "; ")
}

func (s *Service) parseAndValidate(raw string, model *semantic.SemanticModel) (*query.LogicalQuery, []string, int, error) {
	var warnings []string

	lq, err := parseLogicalQueryFromRaw(raw)
	if err != nil {
		return nil, warnings, 0, fmt.Errorf("invalid JSON from AI: %w", err)
	}
	normalizeLogicalQueryContext(&lq, model)
	lq.EnsureGroupBySelected()
	ensureTimeSeriesOrderBy(&lq, model)

	// Guardrails: reject empty selects
	if len(lq.Select) == 0 {
		warnings = append(warnings, "AI returned empty select - question may be ambiguous")
		return nil, warnings, 0, fmt.Errorf("ambiguous question")
	}

	// Validate against semantic model
	validationErrCount := 0
	if err := s.validator.Validate(lq, model); err != nil {
		warnings = append(warnings, "validation warnings: "+err.Error())
		if ve, ok := errors.AsType[query.ValidationErrors](err); ok {
			validationErrCount = len(ve)
		} else {
			validationErrCount = 1
		}
		// Still return the query but with warnings
	}

	return &lq, warnings, validationErrCount, nil
}

// computeConfidence produces a [0.0, 1.0] confidence score for a generated
// LogicalQuery. Each semantic validation error reduces confidence; each retry
// attempt also reduces it (the model needed correction to arrive here).
func computeConfidence(validationErrCount, retries int) float64 {
	const (
		base                 = 0.9
		penaltyPerValidation = 0.15
		penaltyPerRetry      = 0.1
	)
	score := base - penaltyPerValidation*float64(validationErrCount) - penaltyPerRetry*float64(retries)
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func normalizeLogicalQueryContext(lq *query.LogicalQuery, model *semantic.SemanticModel) {
	lq.EnsureVersion()
	lq.DatasourceID = model.DatasourceID
	lq.ModelID = model.ID
	if lq.ModelID == "" {
		lq.ModelID = model.Name
	}
	if model != nil && len(model.Dimensions) > 0 {
		names := make([]string, 0, len(model.Dimensions))
		for _, d := range model.Dimensions {
			names = append(names, d.Name)
		}
		query.RepairMisnamedCalendarGrainDimensions(lq, names)
	}
}

func ensureTimeSeriesOrderBy(lq *query.LogicalQuery, model *semantic.SemanticModel) {
	if lq == nil || model == nil || len(lq.GroupBy) == 0 || len(lq.OrderBy) > 0 {
		return
	}
	grainByDimension := make(map[string]string, len(model.Dimensions))
	for _, dim := range model.Dimensions {
		grainByDimension[dim.Name] = dim.TimeGrain
	}
	for _, gb := range lq.GroupBy {
		if gb.TimeGrain == "" && grainByDimension[gb.Field] == "" {
			continue
		}
		lq.OrderBy = append(lq.OrderBy, query.OrderBy{
			Field:     gb.Field,
			Direction: query.OrderAsc,
		})
	}
}
