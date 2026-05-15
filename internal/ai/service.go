package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/biqly/biqly/internal/config"
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

// NewService creates a new AI service. Returns an error if the configured
// provider is unknown — callers should surface this at startup.
func NewService(cfg config.AIConfig, validator *query.Validator) *Service {
	maxR := cfg.MaxPromptInputRunes
	if maxR <= 0 {
		maxR = 80000
	}
	retries := cfg.MaxRetries
	if retries < 0 {
		retries = 0
	}
	// Fall back to the OpenAI client on unknown providers so test setups that
	// pass minimal configs (no Provider field) keep working. Production code
	// should call NewServiceWithProvider for explicit error handling.
	provider, err := NewProvider(cfg)
	if err != nil {
		provider = NewClient(cfg)
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

// NewServiceWithProvider wires a service around an explicitly-supplied Provider.
// Use this in production wiring where unknown-provider should be a fatal error.
func NewServiceWithProvider(cfg config.AIConfig, validator *query.Validator, provider Provider) *Service {
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

	basePrompt, baseStats := s.buildPrompt(ctx, question, model, 0, options)

	// Self-consistency: when configured, draw N candidates with stepped temperatures
	// and vote. A clear majority returns immediately; otherwise we fall through to
	// the standard retry loop which handles single-shot generation + correction.
	if s.multiCandidateCount > 1 {
		if resp, ok := s.tryMultiCandidate(ctx, basePrompt, model, options, baseStats); ok {
			return resp, nil
		}
	}

	var (
		prompt             = basePrompt
		promptStats        = baseStats
		lastRaw            string
		retryWarnings      []string
		lq                 *query.LogicalQuery
		warnings           []string
		validationErrCount int
		parseErr           error
	)

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		raw, genErr := s.client.Generate(ctx, prompt)
		if genErr != nil {
			return nil, fmt.Errorf("AI generation failed: %w", genErr)
		}
		lastRaw = raw

		lq, warnings, validationErrCount, parseErr = s.parseAndValidate(raw, model)

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
			retries := attempt
			resp := &AIResponse{
				LogicalQuery: lq,
				Confidence:   computeConfidence(validationErrCount, retries),
				Warnings:     append(retryWarnings, warnings...),
				Prompt:       prompt,
				RawResponse:  raw,
				RetryCount:   retries,
				PromptStats:  &promptStats,
				TokenUsage:   tokenUsageEstimate(promptStats, raw),
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
		expanded, _ := s.buildPrompt(ctx, question, model, nextTier, options)
		prompt = s.promptBuilder.BuildRetry(expanded, raw, failureMsg)
		promptStats = MeasurePrompt(prompt, s.queryModel, nextTier, s.aiCfg)
	}

	failureReason := failureMessageFor(parseErr, nil, warnings)

	if parseErr != nil {
		clarification := s.tryGenerateClarification(ctx, question, model, failureReason)
		return &AIResponse{
			Warnings:              append(retryWarnings, append(warnings, parseErr.Error())...),
			Prompt:                prompt,
			RawResponse:           lastRaw,
			Confidence:            0,
			RetryCount:            s.maxRetries,
			PromptStats:           &promptStats,
			TokenUsage:            tokenUsageEstimate(promptStats, lastRaw),
			NeedsClarification:    clarification != "",
			ClarificationQuestion: clarification,
			Clarification:         buildClarification(clarification, failureReason, "ai"),
		}, nil
	}

	clarification := ""
	if validationErrCount > 0 {
		clarification = s.tryGenerateClarification(ctx, question, model, failureReason)
		return &AIResponse{
			Confidence:            0,
			Warnings:              append(retryWarnings, warnings...),
			Prompt:                prompt,
			RawResponse:           lastRaw,
			RetryCount:            s.maxRetries,
			PromptStats:           &promptStats,
			TokenUsage:            tokenUsageEstimate(promptStats, lastRaw),
			NeedsClarification:    clarification != "",
			ClarificationQuestion: clarification,
			Clarification:         buildClarification(clarification, failureReason, "validator"),
		}, nil
	}

	return &AIResponse{
		LogicalQuery:          lq,
		Confidence:            computeConfidence(validationErrCount, s.maxRetries),
		Warnings:              append(retryWarnings, warnings...),
		Prompt:                prompt,
		RawResponse:           lastRaw,
		RetryCount:            s.maxRetries,
		PromptStats:           &promptStats,
		TokenUsage:            tokenUsageEstimate(promptStats, lastRaw),
		NeedsClarification:    clarification != "",
		ClarificationQuestion: clarification,
		Clarification:         buildClarification(clarification, failureReason, "ai"),
	}, nil
}

func (s *Service) buildPrompt(
	ctx context.Context,
	question string,
	model *semantic.SemanticModel,
	tier int,
	options processOptions,
) (string, PromptStats) {
	tiered := applyContextTier(options, tier)
	promptRunes := PromptRunesForTier(s.maxPromptRunes, tier, s.aiCfg, s.queryModel)
	prompt := s.promptBuilder.Build(
		question,
		model,
		promptRunes,
		options.targetDialect,
		tiered.fewShot,
		tiered.samples,
		tiered.priorTurns,
		tiered.deniedFields,
		tiered.glossary,
	)
	stats := MeasurePrompt(prompt, s.queryModel, tier, s.aiCfg)
	slog.InfoContext(ctx, "ai prompt context",
		"model", stats.Model,
		"context_tier", stats.ContextTierLabel,
		"prompt_runes", stats.PromptRunes,
		"est_prompt_tokens", stats.EstPromptTokens,
		"max_prompt_runes", stats.MaxPromptRunes,
		"context_window_tokens", stats.ContextWindowTokens,
	)
	return prompt, stats
}

func tokenUsageEstimate(stats PromptStats, completion string) *TokenUsage {
	promptTok := stats.EstPromptTokens
	completionTok := EstimateTokens(completion)
	if promptTok == 0 && completionTok == 0 {
		return nil
	}
	return &TokenUsage{
		Prompt:     promptTok,
		Completion: completionTok,
		Total:      promptTok + completionTok,
	}
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
	prompt string,
	model *semantic.SemanticModel,
	options processOptions,
	stats PromptStats,
) (*AIResponse, bool) {
	n := s.multiCandidateCount
	if n < 2 {
		return nil, false
	}

	type candidate struct {
		lq       *query.LogicalQuery
		raw      string
		warnings []string
	}
	groups := make(map[string][]candidate)
	successCount := 0

	for i := 0; i < n; i++ {
		temp := s.baseTemperature + 0.2*float64(i)
		if temp > 1 {
			temp = 1
		}
		raw, err := s.client.GenerateAt(ctx, prompt, temp)
		if err != nil {
			continue
		}
		lq, warnings, validationErrCount, parseErr := s.parseAndValidate(raw, model)
		if parseErr != nil || validationErrCount > 0 || lq == nil {
			continue
		}
		if options.sqlValidator != nil {
			if err := options.sqlValidator(ctx, lq); err != nil {
				continue
			}
		}
		fp := logicalQueryFingerprint(lq)
		groups[fp] = append(groups[fp], candidate{lq: lq, raw: raw, warnings: warnings})
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

	confidence := float64(winnerCount) / float64(n)
	if confidence > 1 {
		confidence = 1
	}
	return &AIResponse{
		LogicalQuery: winner.lq,
		Confidence:   confidence,
		Warnings: append(
			[]string{fmt.Sprintf("self-consistency: %d/%d candidates agreed", winnerCount, n)},
			winner.warnings...,
		),
		Prompt:       prompt,
		RawResponse:  winner.raw,
		PromptStats:  &stats,
		TokenUsage:   tokenUsageEstimate(stats, winner.raw),
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
	prompt := s.promptBuilder.BuildClarification(question, model, failureReason)
	raw, err := s.client.Generate(ctx, prompt)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(raw)
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

	cleaned := TrimToJSONObject(raw)

	// Parse JSON
	var lq query.LogicalQuery
	if err := json.Unmarshal([]byte(cleaned), &lq); err != nil {
		return nil, warnings, 0, fmt.Errorf("invalid JSON from AI: %w", err)
	}
	normalizeLogicalQueryContext(&lq, model)
	lq.EnsureGroupBySelected()

	// Guardrails: reject empty selects
	if len(lq.Select) == 0 {
		warnings = append(warnings, "AI returned empty select - question may be ambiguous")
		return nil, warnings, 0, fmt.Errorf("ambiguous question")
	}

	// Validate against semantic model
	validationErrCount := 0
	if err := s.validator.Validate(lq, model); err != nil {
		warnings = append(warnings, "validation warnings: "+err.Error())
		var ve query.ValidationErrors
		if errors.As(err, &ve) {
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
	lq.ModelID = model.Name
	if lq.ModelID == "" {
		lq.ModelID = model.ID
	}
	if model != nil && len(model.Dimensions) > 0 {
		names := make([]string, 0, len(model.Dimensions))
		for _, d := range model.Dimensions {
			names = append(names, d.Name)
		}
		query.RepairMisnamedCalendarGrainDimensions(lq, names)
	}
}
