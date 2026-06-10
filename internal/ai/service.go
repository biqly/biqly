package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/errmsg"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// Service orchestrates the AI text-to-query flow.
type Service struct {
	client              providerpkg.Provider
	promptBuilder       *promptpkg.Builder
	validator           *query.Validator
	aiCfg               config.AIConfig
	queryModel          string
	maxPromptRunes      int
	maxRetries          int
	multiCandidateCount int
	baseTemperature     float64
	cache               ResponseCache
	ambiguityCache      sync.Map
}

// WithCache configures a cache for AI query responses.
func (s *Service) WithCache(cache ResponseCache) *Service {
	s.cache = cache
	return s
}

func newService(cfg *config.AIConfig, validator *query.Validator, provider providerpkg.Provider) *Service {
	maxR := cfg.Generation.MaxPromptInputRunes
	if maxR <= 0 {
		maxR = 80000
	}
	retries := max(cfg.Generation.MaxRetries, 0)
	queryView := cfg.ResolvedQuery()
	return &Service{
		client:              provider,
		promptBuilder:       &promptpkg.Builder{},
		validator:           validator,
		aiCfg:               queryView.Config,
		queryModel:          queryView.Config.Connection.Model,
		maxPromptRunes:      maxR,
		maxRetries:          retries,
		multiCandidateCount: cfg.Generation.MultiCandidateCount,
		baseTemperature:     cfg.Generation.Temperature,
	}
}

// NewService creates a new AI service. Returns an error if the configured
// provider is unknown — callers should surface this at startup.
func NewService(cfg *config.AIConfig, validator *query.Validator) *Service {
	// Fall back to the OpenAI client on unknown providers so test setups that
	// pass minimal configs (no Provider field) keep working. Production code
	// should call NewServiceWithProvider for explicit error handling.
	provider, err := providerpkg.NewProvider(*cfg)
	if err != nil {
		slog.Warn("AI provider configuration invalid, falling back to OpenAI client", "provider", cfg.Connection.Provider, "error", err)
		provider = providerpkg.NewClient(*cfg)
	}
	return newService(cfg, validator, provider)
}

// NewServiceWithProvider wires a service around an explicitly-supplied Provider.
// Use this in production wiring where unknown-provider should be a fatal error.
func NewServiceWithProvider(cfg *config.AIConfig, validator *query.Validator, provider providerpkg.Provider) *Service {
	return newService(cfg, validator, provider)
}

// LLMProvider returns the configured generation backend (for eval judge, etc.).
func (s *Service) LLMProvider() providerpkg.Provider {
	return s.client
}

// EvaluateQuestion adapts Service to the eval package without making eval
// depend on the root AI orchestration package.
func (s *Service) EvaluateQuestion(ctx context.Context, question string, model *semantic.SemanticModel) (*evalpkg.QuestionResult, error) {
	resp, err := s.ProcessQuestion(ctx, question, model)
	if err != nil {
		return nil, err
	}
	var lq *query.LogicalQuery
	var conf float64
	var tokenUsage *providerpkg.TokenUsage
	var promptTemplateVersions map[string]int
	var promptTemplateBundleVersion int

	if resp != nil {
		if resp.Result != nil {
			lq = resp.Result.LogicalQuery
			conf = resp.Result.Confidence
		}
		if resp.Metadata != nil {
			tokenUsage = resp.Metadata.TokenUsage
			promptTemplateVersions = resp.Metadata.PromptTemplateVersions
			promptTemplateBundleVersion = resp.Metadata.PromptTemplateBundleVersion
		}
	}

	return &evalpkg.QuestionResult{
		LogicalQuery:                lq,
		Confidence:                  conf,
		TokenUsage:                  tokenUsage,
		PromptTemplateVersions:      promptTemplateVersions,
		PromptTemplateBundleVersion: promptTemplateBundleVersion,
	}, nil
}

// SQLValidator dry-runs a compiled LogicalQuery against the live datasource
// (e.g. via EXPLAIN) and returns an error describing any planner-level issue.
// Returning nil means the query is safe to execute.
type SQLValidator func(ctx context.Context, lq *query.LogicalQuery) error

// ProcessOption customizes how ProcessQuestion runs. Use functional options to
// keep the call site backward compatible while enabling optional features.
type ProcessOption func(*processOptions)

// AmbiguityAnalysisObserver records rule-based and LLM ambiguity passes.
type AmbiguityAnalysisObserver func(latencyMs int64, source string, detected bool)

// AIStepObserver records per-step pipeline latency for Prometheus dashboards.
type AIStepObserver func(step string, latencyMs int64)

type processOptions struct {
	sqlValidator                 SQLValidator
	fewShot                      []promptpkg.FewShotExample
	samples                      []promptpkg.TableSample
	priorTurns                   []promptpkg.ConversationTurn
	deniedFields                 []string
	targetDialect                string
	glossary                     []promptpkg.GlossaryEntry
	ambiguityGlossary            []promptpkg.GlossaryEntry
	ambiguityCheck               bool
	ambiguitySynonymOnly         bool
	ambiguityInteractiveTier     bool
	ambiguityConfidenceThreshold float64
	ambiguityMaxOptions          int
	ambiguityLLMCheck            bool
	ambiguityObserver            AmbiguityAnalysisObserver
	ambiguityTierObserver        func(tier string)
	stepObserver                 AIStepObserver
}

type tieredProcessOptions struct {
	fewShot      []promptpkg.FewShotExample
	samples      []promptpkg.TableSample
	priorTurns   []promptpkg.ConversationTurn
	deniedFields []string
	glossary     []promptpkg.GlossaryEntry
}

func contextTierForAttempt(attempt int) int {
	return promptpkg.ContextTierForAttempt(attempt)
}

func applyContextTier(base *processOptions, tier int) tieredProcessOptions {
	return tieredProcessOptions{
		fewShot:      promptpkg.TailSlice(base.fewShot, promptpkg.FewShotCap(tier)),
		samples:      base.samples,
		priorTurns:   promptpkg.TailSlice(base.priorTurns, promptpkg.PriorTurnsCap(tier)),
		deniedFields: base.deniedFields,
		glossary:     promptpkg.TailGlossary(base.glossary, promptpkg.GlossaryCap(tier)),
	}
}

// WithSQLValidator wires a dialect-aware dry-run check (e.g. EXPLAIN) into the
// retry loop: a failing dry-run is treated like a validation error and triggers
// a corrective re-prompt.
func WithSQLValidator(v SQLValidator) ProcessOption {
	return func(o *processOptions) { o.sqlValidator = v }
}

// WithFewShotExamples injects prior successful (question, logical_query) pairs
// into the prompt. Pass the most recent N high-confidence rows from history.
func WithFewShotExamples(examples []promptpkg.FewShotExample) ProcessOption {
	return func(o *processOptions) { o.fewShot = examples }
}

// WithSampleData attaches concrete row samples from the queried tables so the
// LLM sees actual values (not just column names). Cells should already be
// truncated by the caller via FetchTableSample's maxCellRunes parameter.
func WithSampleData(samples []promptpkg.TableSample) ProcessOption {
	return func(o *processOptions) { o.samples = samples }
}

// WithPriorTurns supplies recent turns from the active conversation so the
// model can resolve follow-ups in context. Pass the most recent N turns
// (caller should cap N — typically 3-5 — to keep the prompt bounded).
func WithPriorTurns(turns []promptpkg.ConversationTurn) ProcessOption {
	return func(o *processOptions) { o.priorTurns = turns }
}

// WithTargetDialect sets the datasource SQL engine name for dialect-aware
// compilation examples in the prompt (postgres, mysql, sqlserver, clickhouse).
func WithTargetDialect(dialectName string) ProcessOption {
	return func(o *processOptions) { o.targetDialect = dialectName }
}

// WithGlossary injects business-term → field mappings into the prompt.
func WithGlossary(entries []promptpkg.GlossaryEntry) ProcessOption {
	return func(o *processOptions) { o.glossary = entries }
}

// WithAmbiguityGlossary preserves unmerged terms so collision checks see every mapping.
func WithAmbiguityGlossary(entries []promptpkg.GlossaryEntry) ProcessOption {
	return func(o *processOptions) { o.ambiguityGlossary = entries }
}

// WithAmbiguityCheck enables rule-based semantic clarification before prompting.
func WithAmbiguityCheck(enabled bool) ProcessOption {
	return func(o *processOptions) { o.ambiguityCheck = enabled }
}

// WithAmbiguitySynonymOnly limits rule-based checks to glossary/synonym detectors (tier 1).
func WithAmbiguitySynonymOnly(enabled bool) ProcessOption {
	return func(o *processOptions) { o.ambiguitySynonymOnly = enabled }
}

// WithAmbiguityInteractiveTier enables Tier 3 agent-driven disambiguation: full rule-based
// analysis plus LLM fallback with uncapped clarification options.
func WithAmbiguityInteractiveTier(enabled bool) ProcessOption {
	return func(o *processOptions) { o.ambiguityInteractiveTier = enabled }
}

// WithAmbiguityConfidenceThreshold sets the minimum interpretation confidence for ambiguity clarification.
func WithAmbiguityConfidenceThreshold(threshold float64) ProcessOption {
	return func(o *processOptions) { o.ambiguityConfidenceThreshold = threshold }
}

// WithAmbiguityMaxOptions caps the clarification choices returned to the user.
func WithAmbiguityMaxOptions(maxOptions int) ProcessOption {
	return func(o *processOptions) { o.ambiguityMaxOptions = maxOptions }
}

// WithLLMAmbiguityCheck enables provider-backed ambiguity detection after deterministic checks pass.
func WithLLMAmbiguityCheck(enabled bool) ProcessOption {
	return func(o *processOptions) { o.ambiguityLLMCheck = enabled }
}

// WithAmbiguityAnalysisObserver records ambiguity latency and source metrics.
func WithAmbiguityAnalysisObserver(observer AmbiguityAnalysisObserver) ProcessOption {
	return func(o *processOptions) { o.ambiguityObserver = observer }
}

// WithAmbiguityTierObserver records which ambiguity escalation tier ran.
func WithAmbiguityTierObserver(observer func(tier string)) ProcessOption {
	return func(o *processOptions) { o.ambiguityTierObserver = observer }
}

// WithAIStepObserver records per-step pipeline latency (prompt_build, llm_generate, etc.).
func WithAIStepObserver(observer AIStepObserver) ProcessOption {
	return func(o *processOptions) { o.stepObserver = observer }
}

// ProcessQuestion handles a natural language question. On parse or validation
// failure the LLM is re-prompted with the prior output and error message, up
// to s.maxRetries additional attempts.
func (s *Service) ProcessQuestion(ctx context.Context, question string, model *semantic.SemanticModel, opts ...ProcessOption) (*AIResponse, error) {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.ProcessQuestion")
	defer span.End()
	span.SetAttributes(
		attribute.String("ai.model", s.queryModel),
		attribute.Int("question.length", len(question)),
	)
	if model != nil {
		span.SetAttributes(attribute.String("model.id", model.ID))
	}

	options := processOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	if resp, done := s.checkAmbiguity(ctx, question, model, &options); done {
		return resp, nil
	}

	filterSess := FilterSessionFromPriorTurns(options.priorTurns)
	followIntent := ClassifyFollowUpIntent(question, filterSess)

	var cacheKey string
	if s.cache != nil && model != nil {
		cacheKey = GenerateCacheKey(question, model.ID, options.deniedFields)
		if cachedResp, err := s.cache.Get(ctx, cacheKey); err == nil && cachedResp != nil {
			slog.InfoContext(ctx, "LLM Response Cache hit", "question", question, "key", cacheKey)
			observability.Default().RecordLLMResponseCacheHit()
			return cachedResp, nil
		}
		observability.Default().RecordLLMResponseCacheMiss()
	}

	basePrompt, baseStats := s.buildPrompt(ctx, question, model, 0, &options, filterSess, followIntent)

	// Self-consistency: when configured, draw N candidates with stepped temperatures
	// and vote. A clear majority returns immediately; otherwise we fall through to
	// the standard retry loop which handles single-shot generation + correction.
	if s.multiCandidateCount > 1 {
		if resp, ok := s.tryMultiCandidate(ctx, question, basePrompt, model, &options, &baseStats, filterSess, followIntent); ok {
			applyTemporalFilterPostCheck(ctx, question, model, resp)
			s.cacheResponse(ctx, cacheKey, resp)
			return resp, nil
		}
	}

	resp, state, err := s.generateWithRetries(ctx, span, question, model, &options, basePrompt, baseStats, filterSess, followIntent)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		applyTemporalFilterPostCheck(ctx, question, model, resp)
		s.cacheResponse(ctx, cacheKey, resp)
		return resp, nil
	}
	failure := s.buildFailureResponse(ctx, question, model, state)
	applyTemporalFilterPostCheck(ctx, question, model, failure)
	return failure, nil
}

// checkAmbiguity runs the optional pre-LLM ambiguity analysis. It returns
// (response, true) when the question is ambiguous and the caller should return
// the clarification response immediately, or (nil, false) to continue with
// generation.
func (s *Service) checkAmbiguity(ctx context.Context, question string, model *semantic.SemanticModel, options *processOptions) (*AIResponse, bool) {
	if !options.ambiguityCheck {
		return nil, false
	}
	glossary := options.ambiguityGlossary
	if glossary == nil {
		glossary = options.glossary
	}
	cacheKey := ambiguityAnalysisCacheKey(question, model, glossary, options.ambiguityConfidenceThreshold, options.ambiguityLLMCheck, options.ambiguitySynonymOnly, options.ambiguityInteractiveTier)
	result, source, cached := s.getCachedAmbiguityAnalysis(cacheKey)
	if cached {
		if options.ambiguityObserver != nil {
			options.ambiguityObserver(0, source, result.IsAmbiguous)
		}
	} else {
		result = s.analyzeAmbiguity(ctx, question, model, glossary, options, cacheKey)
	}
	if result.IsAmbiguous {
		maxOptions := options.ambiguityMaxOptions
		if options.ambiguityInteractiveTier {
			maxOptions = 0
		}
		return ambiguityClarificationResponse(i18n.FromContext(ctx), result, maxOptions), true
	}
	return nil, false
}

// analyzeAmbiguity runs the deterministic ambiguity check and, when configured,
// the LLM fallback, caching the result unless the LLM call failed.
func (s *Service) analyzeAmbiguity(ctx context.Context, question string, model *semantic.SemanticModel, glossary []promptpkg.GlossaryEntry, options *processOptions, cacheKey string) ambiguitypkg.Result {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.AmbiguityAnalyze")
	defer span.End()
	if model != nil {
		span.SetAttributes(attribute.String("model.id", model.ID))
	}

	ambiguityStart := time.Now()
	defer func() {
		if options.stepObserver != nil {
			options.stepObserver("ambiguity", time.Since(ambiguityStart).Milliseconds())
		}
	}()

	source := "rule_based"
	start := time.Now()
	if options.ambiguityTierObserver != nil && !options.ambiguityInteractiveTier {
		options.ambiguityTierObserver("1")
	}
	var result ambiguitypkg.Result
	if options.ambiguitySynonymOnly && !options.ambiguityInteractiveTier {
		result = ambiguitypkg.AnalyzeSynonymHomonym(ctx, question, model, glossary, options.ambiguityConfidenceThreshold)
	} else {
		result = ambiguitypkg.Analyze(ctx, question, model, glossary, options.ambiguityConfidenceThreshold)
	}
	if options.ambiguityObserver != nil {
		options.ambiguityObserver(time.Since(start).Milliseconds(), source, result.IsAmbiguous)
	}

	cacheable := true
	llmCheck := options.ambiguityLLMCheck || options.ambiguityInteractiveTier
	if !result.IsAmbiguous && llmCheck {
		if options.ambiguityTierObserver != nil {
			options.ambiguityTierObserver("2")
		}
		analysisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		start = time.Now()
		llmResult, err := ambiguitypkg.NewLLMAnalyzer(s.client).Analyze(analysisCtx, i18n.FromContext(ctx), question, model, glossary)
		cancel()
		source = "llm"
		yield := "empty"
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			yield = "timeout"
		case err != nil:
			yield = "error"
		case llmResult.IsAmbiguous:
			yield = "found"
		}
		observability.Default().RecordAmbiguityLLMTierYield(yield)
		if err != nil {
			cacheable = false
			slog.WarnContext(ctx, "LLM ambiguity analysis failed", "error", err)
		} else {
			result = llmResult
		}
		if options.ambiguityObserver != nil {
			options.ambiguityObserver(time.Since(start).Milliseconds(), source, result.IsAmbiguous)
		}
	}
	if cacheable {
		s.cacheAmbiguityAnalysis(cacheKey, result, source)
	}
	span.SetAttributes(
		attribute.String("ai.ambiguity.source", source),
		attribute.Bool("ai.ambiguity.detected", result.IsAmbiguous),
	)
	return result
}

// genLoopState captures the terminal state of the generate/validate/repair loop
// when no attempt produced a usable query, so the clarification builders can
// assemble the failure response.
type genLoopState struct {
	lq                 *query.LogicalQuery
	lastRaw            string
	lastGen            providerpkg.GenerationResult
	retryWarnings      []string
	warnings           []string
	validationErrCount int
	parseErr           error
	prompt             string
	promptStats        promptpkg.Stats
	repairDetails      []RepairDetail
	llmGenerateMs      int64
}

func (s *Service) generateAtWithSpan(ctx context.Context, prompt string, temperature float64, attempt int, options *processOptions) (providerpkg.GenerationResult, int64, error) {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.LLMGenerate")
	defer span.End()
	span.SetAttributes(
		attribute.String("ai.model", s.queryModel),
		attribute.Int("ai.attempt", attempt),
		attribute.Float64("ai.temperature", temperature),
	)
	start := time.Now()
	gen, err := s.client.GenerateAt(ctx, prompt, temperature)
	llmMs := time.Since(start).Milliseconds()
	if options != nil && options.stepObserver != nil {
		options.stepObserver("llm_generate", llmMs)
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return providerpkg.GenerationResult{}, llmMs, err
	}
	if usage := providerpkg.TokenUsageFromGeneration(promptpkg.Stats{}, gen); usage != nil {
		observability.SetAITokenAttributes(span, usage.Prompt, usage.Completion, usage.Total)
	}
	return gen, llmMs, nil
}

func (s *Service) generateWithSpan(ctx context.Context, prompt string, attempt int, options *processOptions) (providerpkg.GenerationResult, int64, error) {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.LLMGenerate")
	defer span.End()
	span.SetAttributes(
		attribute.String("ai.model", s.queryModel),
		attribute.Int("ai.attempt", attempt),
	)
	start := time.Now()
	gen, err := s.client.Generate(ctx, prompt)
	llmMs := time.Since(start).Milliseconds()
	if options != nil && options.stepObserver != nil {
		options.stepObserver("llm_generate", llmMs)
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return providerpkg.GenerationResult{}, llmMs, err
	}
	if usage := providerpkg.TokenUsageFromGeneration(promptpkg.Stats{}, gen); usage != nil {
		observability.SetAITokenAttributes(span, usage.Prompt, usage.Completion, usage.Total)
	}
	return gen, llmMs, nil
}

// generateWithRetries runs single-shot generation plus the repair/retry loop. It
// returns a fully-formed success response, or (nil, state, nil) with the terminal
// loop state for the caller to assemble a clarification response, or a non-nil
// error when the provider call itself fails.
func (s *Service) generateWithRetries(
	ctx context.Context,
	span trace.Span,
	question string,
	model *semantic.SemanticModel,
	options *processOptions,
	basePrompt string,
	baseStats promptpkg.Stats,
	filterSess *FilterSessionState,
	followIntent FollowUpIntent,
) (*AIResponse, *genLoopState, error) {
	st := &genLoopState{prompt: basePrompt, promptStats: baseStats}
	var validationErrors query.ValidationErrors

	for attempt := range s.maxRetries + 1 {
		gen, llmMs, genErr := s.generateWithSpan(ctx, st.prompt, attempt, options)
		st.llmGenerateMs += llmMs
		if genErr != nil {
			span.RecordError(genErr)
			span.SetStatus(codes.Error, genErr.Error())
			return nil, nil, fmt.Errorf("AI generation failed: %w", genErr)
		}
		st.lastGen = gen
		st.lastRaw = gen.Content

		parseStart := time.Now()
		st.lq, st.warnings, st.validationErrCount, validationErrors, st.parseErr = s.parseAndValidate(gen.Content, model)
		if st.parseErr != nil && gen.FinishReason == "length" {
			st.parseErr = fmt.Errorf("%w (completion truncated by max_tokens, finish_reason=length)", st.parseErr)
		}
		if options.stepObserver != nil {
			options.stepObserver("parse_validate", time.Since(parseStart).Milliseconds())
		}

		// Dry-run check (e.g. EXPLAIN) only when the query parsed and passed
		// semantic validation; otherwise the SQL would not be compilable anyway.
		var sqlErr error
		if st.parseErr == nil && st.validationErrCount == 0 && options.sqlValidator != nil {
			sqlStart := time.Now()
			sqlErr = options.sqlValidator(ctx, st.lq)
			if options.stepObserver != nil {
				options.stepObserver("sql_dry_run", time.Since(sqlStart).Milliseconds())
			}
			if sqlErr != nil {
				st.warnings = append(st.warnings, "dry-run failed: "+sqlErr.Error())
			}
		}

		if st.parseErr == nil && st.validationErrCount == 0 && sqlErr == nil {
			if inheritNotes := ApplyFilterSession(st.lq, filterSess, followIntent); len(inheritNotes) > 0 {
				st.warnings = append(st.warnings, inheritNotes...)
			}
			span.SetAttributes(attribute.Int("ai.retries", attempt))
			return s.buildSuccessResponse(ctx, question, st, attempt), nil, nil
		}

		// Out of attempts — fall through to the failure/clarification path.
		if attempt == s.maxRetries {
			break
		}

		// Re-prompt with the failure context for the next attempt.
		repairCtx, repairSpan := otel.Tracer("biqly/ai").Start(ctx, "ai.Repair")
		repairSpan.SetAttributes(attribute.Int("ai.repair.attempt", attempt+1))
		failureMsg := failureMessageFor(st.parseErr, sqlErr, st.warnings)
		st.retryWarnings = append(st.retryWarnings, fmt.Sprintf("retry %d (context %s): %s", attempt+1, promptpkg.ContextTierLabel(contextTierForAttempt(attempt+1)), failureMsg))
		nextTier := contextTierForAttempt(attempt + 1)
		expanded, _ := s.buildPrompt(repairCtx, question, model, nextTier, options, filterSess, followIntent)
		locale := promptpkg.LocaleForQuestion(question, i18n.FromContext(ctx))

		st.prompt = s.buildNextAttemptPrompt(repairCtx, locale, expanded, gen.Content, failureMsg, validationErrors, attempt, st)
		st.promptStats = promptpkg.MeasurePrompt(st.prompt, s.queryModel, nextTier, s.aiCfg)
		repairSpan.End()
	}

	return nil, st, nil
}

// buildNextAttemptPrompt selects between a validation-error repair prompt and a
// generic retry prompt, recording a RepairDetail on st for the former.
func (s *Service) buildNextAttemptPrompt(ctx context.Context, locale i18n.Locale, expanded, lastResponse, failureMsg string, validationErrors query.ValidationErrors, attempt int, st *genLoopState) string {
	if len(validationErrors) == 0 {
		return s.promptBuilder.BuildRetry(ctx, locale, expanded, lastResponse, failureMsg)
	}

	filteredErrors := filterValidationErrors(validationErrors, attempt)

	var errorCodes []string
	for _, ve := range filteredErrors {
		if ve.Code != "" {
			errorCodes = append(errorCodes, ve.Code)
		}
	}

	// Record the same strategy text the repair prompt will use (1-indexed attempt).
	strategyStr := promptpkg.RepairStrategy(locale, attempt+1)
	st.repairDetails = append(st.repairDetails, RepairDetail{
		Attempt:    attempt + 1,
		ErrorCodes: errorCodes,
		ErrorsJSON: filteredErrors.ToRepairJSON(),
		Strategy:   strategyStr,
	})

	return s.promptBuilder.BuildRepairPrompt(ctx, locale, expanded, lastResponse, filteredErrors, attempt+1)
}

// filterValidationErrors narrows the first repair attempt to unknown-field /
// dimension / metric errors (the highest-signal ones), falling back to the full
// set when none match or on later attempts.
func filterValidationErrors(validationErrors query.ValidationErrors, attempt int) query.ValidationErrors {
	if attempt != 0 {
		return validationErrors
	}
	var filtered query.ValidationErrors
	for _, ve := range validationErrors {
		if ve.Code == errmsg.CodeUnknownField || ve.Code == errmsg.CodeUnknownDimension || ve.Code == errmsg.CodeUnknownMetric {
			filtered = append(filtered, ve)
		}
	}
	if len(filtered) == 0 {
		return validationErrors
	}
	return filtered
}

// buildSuccessResponse assembles the AIResponse for an attempt that produced a
// valid, compilable query.
func (*Service) buildSuccessResponse(ctx context.Context, question string, st *genLoopState, retries int) *AIResponse {
	templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)
	return &AIResponse{
		Result: &AIResult{
			LogicalQuery: st.lq,
			Confidence:   computeConfidence(st.validationErrCount, retries),
			Warnings:     joinWarnings(st.retryWarnings, st.warnings),
		},
		Metadata: &AIMetadata{
			Prompt:                      st.prompt,
			RawResponse:                 st.lastGen.Content,
			RetryCount:                  retries,
			LLMGenerateDurationMs:       int(st.llmGenerateMs),
			PromptStats:                 &st.promptStats,
			TokenUsage:                  providerpkg.TokenUsageFromGeneration(st.promptStats, st.lastGen),
			PromptTemplateLocale:        templateLocale,
			PromptTemplateVersions:      templateVersions,
			PromptTemplateBundleVersion: bundleVersion,
			RepairDetails:               st.repairDetails,
		},
	}
}

// buildFailureResponse assembles a clarification response after the retry loop
// exhausted without a usable query, branching on parse vs validation failure.
func (s *Service) buildFailureResponse(ctx context.Context, question string, model *semantic.SemanticModel, st *genLoopState) *AIResponse {
	failureReason := failureMessageFor(st.parseErr, nil, st.warnings)
	templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)

	in := &clarificationInputs{
		Prompt:                      st.prompt,
		RawResponse:                 st.lastRaw,
		RetryCount:                  s.maxRetries,
		LLMGenerateDurationMs:       int(st.llmGenerateMs),
		PromptStats:                 st.promptStats,
		Gen:                         st.lastGen,
		FailureReason:               failureReason,
		PromptTemplateLocale:        templateLocale,
		PromptTemplateVersions:      templateVersions,
		PromptTemplateBundleVersion: bundleVersion,
		RepairDetails:               st.repairDetails,
	}

	switch {
	case st.parseErr != nil:
		in.Warnings = joinWarnings(st.retryWarnings, st.warnings, []string{st.parseErr.Error()})
		in.Clarification = s.tryGenerateClarification(ctx, question, model, failureReason)
		in.Source = "ai"
	case st.validationErrCount > 0:
		in.Warnings = joinWarnings(st.retryWarnings, st.warnings)
		in.Clarification = s.tryGenerateClarification(ctx, question, model, failureReason)
		in.Source = "validator"
	default:
		in.LogicalQuery = st.lq
		in.Confidence = computeConfidence(st.validationErrCount, s.maxRetries)
		in.Warnings = joinWarnings(st.retryWarnings, st.warnings)
		in.Source = "ai"
	}
	return newClarificationResponse(in)
}

// joinWarnings concatenates warning groups into a fresh slice, avoiding aliasing
// the first group's backing array.
func joinWarnings(groups ...[]string) []string {
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	out := make([]string, 0, total)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func ambiguityClarificationResponse(locale i18n.Locale, result ambiguitypkg.Result, maxOptions int) *AIResponse {
	clarification := ClarificationFromAmbiguityWithMaxOptions(locale, result, maxOptions)
	if clarification == nil {
		return nil
	}
	options := make([]string, 0, len(clarification.Options))
	for _, option := range clarification.Options {
		options = append(options, option.Label)
	}
	return &AIResponse{
		Result: &AIResult{
			Warnings:   []string{i18n.T(locale, "clarification.needs_clarification_warning")},
			Confidence: 0,
		},
		Clarification: &ClarificationResponse{
			NeedsClarification:    true,
			ClarificationQuestion: clarification.Question,
			ClarificationOptions:  options,
			Clarification:         clarification,
		},
	}
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
	LLMGenerateDurationMs       int
	PromptStats                 promptpkg.Stats
	Gen                         providerpkg.GenerationResult
	Clarification               string
	FailureReason               string
	Source                      string
	PromptTemplateLocale        string
	PromptTemplateVersions      map[string]int
	PromptTemplateBundleVersion int
	RepairDetails               []RepairDetail
}

func newClarificationResponse(in *clarificationInputs) *AIResponse {
	stats := in.PromptStats
	return &AIResponse{
		Result: &AIResult{
			LogicalQuery: in.LogicalQuery,
			Confidence:   in.Confidence,
			Warnings:     in.Warnings,
		},
		Metadata: &AIMetadata{
			Prompt:                      in.Prompt,
			RawResponse:                 in.RawResponse,
			RetryCount:                  in.RetryCount,
			LLMGenerateDurationMs:       in.LLMGenerateDurationMs,
			PromptStats:                 &stats,
			TokenUsage:                  providerpkg.TokenUsageFromGeneration(stats, in.Gen),
			PromptTemplateLocale:        in.PromptTemplateLocale,
			PromptTemplateVersions:      in.PromptTemplateVersions,
			PromptTemplateBundleVersion: in.PromptTemplateBundleVersion,
			RepairDetails:               in.RepairDetails,
		},
		Clarification: &ClarificationResponse{
			NeedsClarification:    in.Clarification != "",
			ClarificationQuestion: in.Clarification,
			Clarification:         buildClarification(in.Clarification, in.FailureReason, in.Source),
		},
	}
}

func promptTemplateTrace(ctx context.Context, question string) (locale string, versions map[string]int, bundleVersion int) {
	loc := promptpkg.LocaleForQuestion(question, i18n.FromContext(ctx))
	versions = promptpkg.TemplateBundleVersions(ctx, loc)
	for _, v := range versions {
		if v > bundleVersion {
			bundleVersion = v
		}
	}
	locale = string(loc)
	return locale, versions, bundleVersion
}

func (s *Service) buildPrompt(
	ctx context.Context,
	question string,
	model *semantic.SemanticModel,
	tier int,
	options *processOptions,
	filterSess *FilterSessionState,
	followIntent FollowUpIntent,
) (string, promptpkg.Stats) {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.PromptBuild")
	defer span.End()
	span.SetAttributes(
		attribute.String("ai.model", s.queryModel),
		attribute.Int("ai.context_tier", tier),
		attribute.Int("ai.few_shot.count", len(options.fewShot)),
	)
	if model != nil {
		span.SetAttributes(attribute.String("model.id", model.ID))
	}

	tiered := applyContextTier(options, tier)
	promptRunes := promptpkg.RunesForTier(s.maxPromptRunes, tier, s.aiCfg, s.queryModel)
	start := time.Now()
	prompt := s.promptBuilder.Build(
		ctx,
		question,
		model,
		promptpkg.Config{
			MaxRunes:     promptRunes,
			Locale:       i18n.FromContext(ctx),
			Dialect:      options.targetDialect,
			Examples:     tiered.fewShot,
			Samples:      tiered.samples,
			PriorTurns:   tiered.priorTurns,
			DeniedFields: tiered.deniedFields,
			Glossary:     tiered.glossary,
		},
	)
	if block := ActiveFilterInstructions(filterSess, followIntent); block != "" {
		prompt += block
	}
	buildDurationMs := time.Since(start).Milliseconds()
	if options.stepObserver != nil {
		options.stepObserver("prompt_build", buildDurationMs)
	}
	stats := promptpkg.MeasurePrompt(prompt, s.queryModel, tier, s.aiCfg)
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

type multiCandidate struct {
	idx      int
	lq       *query.LogicalQuery
	gen      providerpkg.GenerationResult
	warnings []string
	fp       string
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
	options *processOptions,
	stats *promptpkg.Stats,
	filterSess *FilterSessionState,
	followIntent FollowUpIntent,
) (*AIResponse, bool) {
	n := s.multiCandidateCount
	if n < 2 {
		return nil, false
	}

	multiStart := time.Now()
	defer func() {
		if options.stepObserver != nil {
			options.stepObserver("multi_candidate", time.Since(multiStart).Milliseconds())
		}
	}()

	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.MultiCandidate")
	defer span.End()
	span.SetAttributes(
		attribute.String("ai.model", s.queryModel),
		attribute.Int("ai.candidate_count", n),
	)
	if model != nil {
		span.SetAttributes(attribute.String("model.id", model.ID))
	}

	results := s.runMultiCandidateWorkers(ctx, prompt, model, options, n)
	winner, winnerCount, ok := pickMultiCandidateWinner(results)
	if !ok {
		return nil, false
	}

	if inheritNotes := ApplyFilterSession(winner.lq, filterSess, followIntent); len(inheritNotes) > 0 {
		winner.warnings = append(winner.warnings, inheritNotes...)
	}

	return buildMultiCandidateResponse(ctx, question, prompt, stats, winner, winnerCount, n), true
}

func (s *Service) runMultiCandidateWorkers(
	ctx context.Context,
	prompt string,
	model *semantic.SemanticModel,
	options *processOptions,
	n int,
) []*multiCandidate {
	results := make([]*multiCandidate, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = s.generateMultiCandidate(ctx, prompt, model, options, idx)
		}()
	}
	wg.Wait()
	return results
}

func (s *Service) generateMultiCandidate(
	ctx context.Context,
	prompt string,
	model *semantic.SemanticModel,
	options *processOptions,
	idx int,
) *multiCandidate {
	temp := min(s.baseTemperature+0.2*float64(idx), 1)

	type genResult struct {
		gen providerpkg.GenerationResult
		err error
	}
	ch := make(chan genResult, 1)
	go func() {
		g, _, err := s.generateAtWithSpan(ctx, prompt, temp, idx, options)
		ch <- genResult{g, err}
	}()

	var res genResult
	select {
	case <-ctx.Done():
		return nil
	case res = <-ch:
	}
	if res.err != nil || ctx.Err() != nil {
		return nil
	}

	lq, warnings, validationErrCount, _, parseErr := s.parseAndValidate(res.gen.Content, model)
	if parseErr != nil || validationErrCount > 0 || lq == nil || ctx.Err() != nil {
		return nil
	}
	if options.sqlValidator != nil {
		if err := options.sqlValidator(ctx, lq); err != nil {
			return nil
		}
	}
	return &multiCandidate{
		idx:      idx,
		lq:       lq,
		gen:      res.gen,
		warnings: warnings,
		fp:       logicalQueryFingerprint(lq),
	}
}

func pickMultiCandidateWinner(results []*multiCandidate) (multiCandidate, int, bool) {
	groups := make(map[string][]multiCandidate)
	successCount := 0
	for _, c := range results {
		if c == nil {
			continue
		}
		groups[c.fp] = append(groups[c.fp], *c)
		successCount++
	}
	if successCount == 0 {
		return multiCandidate{}, 0, false
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
		return multiCandidate{}, 0, false
	}
	return groups[winnerKey][0], winnerCount, true
}

func buildMultiCandidateResponse(
	ctx context.Context,
	question, prompt string,
	stats *promptpkg.Stats,
	winner multiCandidate,
	winnerCount, n int,
) *AIResponse {
	confidence := float64(winnerCount) / float64(n)
	if confidence > 1 {
		confidence = 1
	}
	templateLocale, templateVersions, bundleVersion := promptTemplateTrace(ctx, question)
	return &AIResponse{
		Result: &AIResult{
			LogicalQuery: winner.lq,
			Confidence:   confidence,
			Warnings: append(
				[]string{fmt.Sprintf("self-consistency: %d/%d candidates agreed", winnerCount, n)},
				winner.warnings...,
			),
		},
		Metadata: &AIMetadata{
			Prompt:                      prompt,
			RawResponse:                 winner.gen.Content,
			PromptStats:                 stats,
			TokenUsage:                  providerpkg.TokenUsageFromGeneration(*stats, winner.gen),
			PromptTemplateLocale:        templateLocale,
			PromptTemplateVersions:      templateVersions,
			PromptTemplateBundleVersion: bundleVersion,
		},
	}
}

// logicalQueryFingerprint returns a canonical string identifying the structure
// of a LogicalQuery so different completions can be voted as equivalent.
// Datasource/model identifiers and free-form descriptions are excluded.
func logicalQueryFingerprint(lq *query.LogicalQuery) string {
	if lq == nil {
		return ""
	}
	var sb strings.Builder
	sb.Grow(512)

	sb.WriteString("select:[")
	for i, item := range lq.Select {
		if i > 0 {
			_ = sb.WriteByte(',')
		}
		sb.WriteString(item.Type)
		_ = sb.WriteByte(':')
		sb.WriteString(item.Name)
		if item.Alias != "" {
			sb.WriteString(" as ")
			sb.WriteString(item.Alias)
		}
		if item.Window != nil {
			sb.WriteString(" over(")
			sb.WriteString(item.Window.Aggregation)
			_ = sb.WriteByte(':')
			sb.WriteString(item.Window.Expression)
			_ = sb.WriteByte(':')
			sb.WriteString(item.Window.Metric)
			sb.WriteString(" partition_by:[")
			sb.WriteString(strings.Join(item.Window.PartitionBy, ","))
			sb.WriteString("] order_by:[")
			for j, ob := range item.Window.OrderBy {
				if j > 0 {
					_ = sb.WriteByte(',')
				}
				sb.WriteString(ob.Field)
				_ = sb.WriteByte(':')
				sb.WriteString(ob.Direction)
			}
			sb.WriteString("] frame:")
			sb.WriteString(item.Window.Frame)
			sb.WriteByte(')')
		}
	}
	sb.WriteString("] filters:[")
	for i, f := range lq.Filters {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(f.Field)
		sb.WriteByte(':')
		sb.WriteString(f.Operator)
		sb.WriteByte(':')
		sb.WriteString(formatFingerprintValue(f.Value))
	}
	sb.WriteString("] group_by:[")
	for i, gb := range lq.GroupBy {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(gb.Field)
		sb.WriteByte(':')
		sb.WriteString(gb.TimeGrain)
	}
	sb.WriteString("] order_by:[")
	for i, ob := range lq.OrderBy {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(ob.Field)
		sb.WriteByte(':')
		sb.WriteString(ob.Direction)
	}
	sb.WriteString("] limit:")
	sb.WriteString(strconv.Itoa(lq.Limit))
	sb.WriteString(" offset:")
	sb.WriteString(strconv.Itoa(lq.Offset))

	return sb.String()
}

func formatFingerprintValue(val any) string {
	if val == nil {
		return "null"
	}
	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any:
		var sb strings.Builder
		sb.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(formatFingerprintValue(item))
		}
		sb.WriteByte(']')
		return sb.String()
	case []string:
		return strings.Join(v, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// tryGenerateClarification asks the LLM for one short clarifying question to
// surface to the user. Failures are swallowed: clarification is best-effort
// and must never mask the underlying validation/parse error to the caller.
func (s *Service) tryGenerateClarification(ctx context.Context, question string, model *semantic.SemanticModel, failureReason string) string {
	loc := promptpkg.LocaleForQuestion(question, i18n.FromContext(ctx))
	prompt := s.promptBuilder.BuildClarification(ctx, loc, question, model, failureReason)
	gen, err := s.client.Generate(ctx, prompt)
	var content string
	if err == nil {
		content = strings.TrimSpace(gen.Content)
	} else {
		slog.DebugContext(ctx, "failed to generate clarification question", "error", err)
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

func (s *Service) cacheResponse(ctx context.Context, key string, resp *AIResponse) {
	if s.cache == nil || key == "" || resp == nil || resp.Result == nil {
		return
	}
	if resp.Result.Confidence < 0.85 {
		return
	}
	ttl := time.Duration(s.aiCfg.Cache.ResponseTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}
	if err := s.cache.Put(ctx, key, resp, ttl); err != nil {
		slog.WarnContext(ctx, "failed to put response in cache", "error", err)
	}
}

func (s *Service) parseAndValidate(raw string, model *semantic.SemanticModel) (lq *query.LogicalQuery, warnings []string, validationErrCount int, validationErrors query.ValidationErrors, err error) {
	parsed, parseErr := parseLogicalQueryFromRaw(raw)
	if parseErr != nil {
		return nil, warnings, 0, nil, fmt.Errorf("invalid JSON from AI: %w", parseErr)
	}
	lq = &parsed
	normalizeLogicalQueryContext(lq, model)
	lq.EnsureGroupBySelected()
	ensureTimeSeriesOrderBy(lq, model)

	// Guardrails: reject empty selects
	if len(lq.Select) == 0 {
		warnings = append(warnings, "AI returned empty select - question may be ambiguous")
		return nil, warnings, 0, nil, errors.New("ambiguous question")
	}

	// Validate against semantic model
	if err := s.validator.Validate(lq, model); err != nil {
		warnings = append(warnings, "validation warnings: "+err.Error())
		if ve, ok := errors.AsType[query.ValidationErrors](err); ok {
			validationErrors = ve
			validationErrCount = len(ve)
		} else {
			validationErrCount = 1
		}
		// Still return the query but with warnings
	}

	return lq, warnings, validationErrCount, validationErrors, nil
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
	if len(model.Dimensions) > 0 {
		names := make([]string, 0, len(model.Dimensions))
		for i := range model.Dimensions {
			names = append(names, model.Dimensions[i].Name)
		}
		query.RepairMisnamedCalendarGrainDimensions(lq, names)
	}
}

func ensureTimeSeriesOrderBy(lq *query.LogicalQuery, model *semantic.SemanticModel) {
	if lq == nil || model == nil || len(lq.GroupBy) == 0 || len(lq.OrderBy) > 0 {
		return
	}
	grainByDimension := make(map[string]string, len(model.Dimensions))
	for i := range model.Dimensions {
		grainByDimension[model.Dimensions[i].Name] = model.Dimensions[i].TimeGrain
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
