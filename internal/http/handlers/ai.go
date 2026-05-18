// Package handlers provides HTTP handlers for the BI query engine API.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type aiQueryPhase int

const (
	aiPhaseGenerate aiQueryPhase = iota
	aiPhasePreview
	aiPhaseRun
)

// AIHandler handles AI text-to-query operations.
type AIHandler struct {
	service     *ai.Service
	tableRouter *ai.TableRouter
	deps        *app.Dependencies
	metrics     AIMetricsRecorder
}

// SetAIMetricsRecorder wires process-level counters (e.g. Prometheus /metrics).
func (h *AIHandler) SetAIMetricsRecorder(m AIMetricsRecorder) {
	h.metrics = m
}

// NewAIHandler creates a new AI handler.
func NewAIHandler(deps *app.Dependencies) *AIHandler {
	queryCfg := deps.Config.AI.EffectiveQueryConfig()
	provider := deps.AIQueryClient
	if provider == nil {
		provider = deps.AIClient
	}
	svc := ai.NewServiceWithProvider(queryCfg, deps.Validator, provider)
	router := ai.NewTableRouterWithEmbeddings(
		deps.MetaRepo,
		deps.Embedder,
		deps.MetaRepo,
		deps.Config.AI.EmbeddingWeight,
	)
	router.SetRoutingLimits(ai.RoutingLimitsFromConfig(
		deps.Config.AI.RouteMaxDimensions,
		deps.Config.AI.RouteMaxMetrics,
		deps.Config.AI.RouteMaxColumnsPerTable,
		deps.Config.AI.RouteMaxDateGrainExtras,
		deps.Config.AI.RouteSlimNumericMetrics,
	))
	return &AIHandler{
		service:     svc,
		tableRouter: router,
		deps:        deps,
	}
}

type aiQueryRequest struct {
	DatasourceID string   `json:"datasource_id"`
	ModelID      string   `json:"model_id,omitempty"`
	Question     string   `json:"question"`
	Tables       []string `json:"tables,omitempty"`
	// IncludeBaseTables / IncludeViews default to true when omitted (backward compatible).
	IncludeBaseTables *bool `json:"include_base_tables,omitempty"`
	IncludeViews      *bool `json:"include_views,omitempty"`
	// PriorTurns are recent (question, logical_query) pairs from the active
	// conversation, sent by the frontend so follow-ups can be resolved in
	// context ("filter that to last quarter", "now group by region").
	PriorTurns []priorTurnPayload `json:"prior_turns,omitempty"`
	// ExampleIDs lets the frontend specify which few-shot examples to use
	ExampleIDs []string `json:"example_ids,omitempty"`
	// IncludePastQueries when true, attaches recent conversation turns as
	// few-shot examples (frontend-side toggle).
	IncludePastQueries bool `json:"include_past_queries,omitempty"`
}

// priorTurnPayload is the wire shape for one entry in aiQueryRequest.PriorTurns.
// LogicalQuery is sent as raw JSON if available; an empty value is fine
// (e.g. when the prior turn never produced a valid query).
type priorTurnPayload struct {
	Question     string          `json:"question"`
	LogicalQuery json.RawMessage `json:"logical_query,omitempty"`
	Note         string          `json:"note,omitempty"`
}

// maxPriorTurns caps how many turns we forward to the LLM. Older turns drop
// off so the prompt stays bounded regardless of how long the conversation runs.
const maxPriorTurns = 5

// priorTurnsForPrompt converts wire-format turns into the AI service's
// ConversationTurn slice, taking the most recent maxPriorTurns entries.
func priorTurnsForPrompt(payload []priorTurnPayload) []ai.ConversationTurn {
	if len(payload) == 0 {
		return nil
	}
	start := 0
	if len(payload) > maxPriorTurns {
		start = len(payload) - maxPriorTurns
	}
	out := make([]ai.ConversationTurn, 0, len(payload)-start)
	for _, t := range payload[start:] {
		out = append(out, ai.ConversationTurn{
			Question:     t.Question,
			LogicalQuery: string(t.LogicalQuery),
			Note:         t.Note,
		})
	}
	return out
}

// parseAndRouteAIQuery decodes the request, validates required fields, loads the semantic
// model (and table routing). If it writes a response to w (bad request, model load error, or
// clarification-only response), ok is false.
func (h *AIHandler) parseAndRouteAIQuery(w http.ResponseWriter, r *http.Request) (aiQueryRequest, *semantic.SemanticModel, *ai.TableRoutingResult, bool) {
	req, ok := decodeJSON[aiQueryRequest](w, r)
	if !ok {
		return aiQueryRequest{}, nil, nil, false
	}
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return *req, nil, nil, false
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return *req, nil, nil, false
	}

	ctx := r.Context()
	model, routing, err := h.loadQueryModel(ctx, *req)
	if err != nil {
		h.writeModelLoadError(ctx, w, *req, err)
		return *req, nil, nil, false
	}
	if routing != nil && routing.NeedsClarification {
		resp := clarificationResponse(routing)
		h.observeAIRequest(ctx, *req, model, routing, resp, nil, 0)
		writeJSON(w, http.StatusOK, resp)
		return *req, nil, nil, false
	}
	return *req, model, routing, true
}

func (h *AIHandler) standardProcessOptions(ctx context.Context, req aiQueryRequest, model *semantic.SemanticModel) []ai.ProcessOption {
	return []ai.ProcessOption{
		ai.WithTargetDialect(h.datasourceDialectName(ctx, req.DatasourceID)),
		ai.WithFewShotExamples(h.loadFewShotExamples(ctx, model)),
		ai.WithPriorTurns(priorTurnsForPrompt(req.PriorTurns)),
		ai.WithGlossary(h.loadGlossaryForPrompt(ctx, model, req.Question)),
	}
}

func (h *AIHandler) processAIQuestion(
	ctx context.Context,
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routing *ai.TableRoutingResult,
	extra ...ai.ProcessOption,
) (*ai.Response, error) {
	start := time.Now()
	opts := h.standardProcessOptions(ctx, req, model)
	opts = append(opts, extra...)
	resp, err := h.service.ProcessQuestion(ctx, req.Question, model, opts...)
	if resp != nil {
		resp.ModelUsed = h.deps.Config.AI.EffectiveQueryConfig().Model
		resp.TableRouting = routing
	}
	if err != nil {
		h.observeAIRequest(ctx, req, model, routing, nil, err, time.Since(start).Milliseconds())
		return nil, err
	}
	return h.observeAIRequest(ctx, req, model, routing, resp, nil, time.Since(start).Milliseconds()), nil
}

// processAndObserve is the shared entry for Query, Preview, and Run: parse/route,
// optional Run-phase datasource pool, LLM process + telemetry, then phase-specific compile/execute.
func (h *AIHandler) processAndObserve(w http.ResponseWriter, r *http.Request, phase aiQueryPhase) {
	req, model, routing, ok := h.parseAndRouteAIQuery(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	var resolved *app.ResolvedDatasource
	var processOpts []ai.ProcessOption
	if phase == aiPhaseRun {
		var err error
		resolved, err = h.deps.ResolveDatasourceDB(ctx, req.DatasourceID)
		if err != nil {
			writeCoreServiceError(ctx, w, err)
			return
		}
		defer closeResolvedDatasource(ctx, resolved)

		driver := resolved.Driver
		db := resolved.DB
		processOpts = []ai.ProcessOption{
			ai.WithSQLValidator(newSQLDryRunValidator(h.deps.QueryService, db, driver, model)),
			ai.WithTargetDialect(driver.Dialect().Name()),
			ai.WithFewShotExamples(h.loadFewShotExamplesWithIDs(ctx, model, req.ExampleIDs, req.IncludePastQueries)),
			ai.WithSampleData(h.loadSampleData(ctx, db, driver.Dialect(), model)),
		}
	}

	resp, err := h.processAIQuestion(ctx, req, model, routing, processOpts...)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to process question", err)
		return
	}

	switch phase {
	case aiPhaseGenerate:
		writeJSON(w, http.StatusOK, resp)
	case aiPhasePreview:
		h.finishAIPreview(ctx, w, req, model, resp)
	case aiPhaseRun:
		h.finishAIRun(ctx, w, req, model, resp, resolved)
	}
}

func closeResolvedDatasource(ctx context.Context, resolved *app.ResolvedDatasource) {
	if resolved == nil || resolved.DB == nil {
		return
	}
	if closeErr := resolved.DB.Close(); closeErr != nil {
		slog.ErrorContext(ctx, "failed to close database connection", "error", closeErr)
	}
}

func (h *AIHandler) finishAIPreview(ctx context.Context, w http.ResponseWriter, req aiQueryRequest, model *semantic.SemanticModel, resp *ai.Response) {
	if resp.LogicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resolved, err := h.deps.ResolveDatasourceDB(ctx, req.DatasourceID)
	if err != nil {
		writeCoreServiceError(ctx, w, err)
		return
	}
	defer closeResolvedDatasource(ctx, resolved)

	cq, se := h.deps.QueryService.CompileWithContext(ctx, *resp.LogicalQuery, model, resolved.Driver)
	if se != nil {
		slog.ErrorContext(ctx, "AI preview compilation failed", "error", core.LogCause(se))
		resp.Warnings = append(resp.Warnings, "compilation failed")
	} else {
		resp.SQL = cq.SQL
		resp.Args = cq.Args
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AIHandler) finishAIRun(ctx context.Context, w http.ResponseWriter, req aiQueryRequest, model *semantic.SemanticModel, resp *ai.Response, resolved *app.ResolvedDatasource) {
	if resp.LogicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	driver := resolved.Driver
	db := resolved.DB

	cq, se := h.deps.QueryService.CompileWithContext(ctx, *resp.LogicalQuery, model, driver)
	if se != nil {
		persistQueryHistory(ctx, h.deps.MetaRepo, "AI query history", *resp.LogicalQuery, model, nil, nil, queryStatusFailed, core.ErrAsError(se))
		writeServiceError(ctx, w, se)
		return
	}

	resp.SQL = cq.SQL
	resp.Args = cq.Args

	result, err := h.deps.Executor.Execute(ctx, db, cq)
	if err != nil {
		persistQueryHistory(ctx, h.deps.MetaRepo, "AI query history", *resp.LogicalQuery, model, cq, nil, queryStatusFailed, err)
		writeInternalError(ctx, w, http.StatusInternalServerError, "execution failed", err)
		return
	}

	query.EnrichResult(result, *resp.LogicalQuery, model)
	chartType, reason := query.VisualizationHintFromResult(result)
	resp.VisualizationHint = &ai.VisualizationHint{ChartType: chartType, Reason: reason}
	if anomalyWarnings := query.AnomalyWarningMessages(result); len(anomalyWarnings) > 0 {
		resp.Warnings = append(resp.Warnings, anomalyWarnings...)
	}
	resp.Result = result
	persistQueryHistory(ctx, h.deps.MetaRepo, "AI query history", *resp.LogicalQuery, model, cq, result, queryStatusSuccess, nil)
	writeJSON(w, http.StatusOK, resp)
}

// Query handles AI-powered natural language queries.
func (h *AIHandler) Query(w http.ResponseWriter, r *http.Request) {
	h.processAndObserve(w, r, aiPhaseGenerate)
}

// Preview handles AI query preview (compiles but does not execute).
func (h *AIHandler) Preview(w http.ResponseWriter, r *http.Request) {
	h.processAndObserve(w, r, aiPhasePreview)
}

// Run handles AI query execution (compiles and executes).
func (h *AIHandler) Run(w http.ResponseWriter, r *http.Request) {
	h.processAndObserve(w, r, aiPhaseRun)
}

// Describe runs the AI metadata describer over a single table and (optionally) writes
// the suggested table/column descriptions back into the metadata DB.
// Describe handles AI-powered table/column description generation.
func (h *AIHandler) Describe(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[ai.DescribeRequest](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" || req.Table == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and table are required")
		return
	}

	result, err := h.deps.AIDescriber.Describe(r.Context(), *req)
	if err != nil {
		writeCoreServiceError(r.Context(), w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type embedMetadataRequest struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id,omitempty"`
}

type embedMetadataResponse struct {
	DatasourceID string                `json:"datasource_id"`
	ModelID      string                `json:"model_id,omitempty"`
	Model        string                `json:"model"`
	Embedded     int                   `json:"embedded"`
	Skipped      int                   `json:"skipped"`
	Results      []ai.EmbedTableResult `json:"results,omitempty"`
}

// EmbedMetadata computes vector embeddings for every table and column in a
// datasource and stores them so the AI router can use hybrid retrieval.
// Idempotent — re-runs simply overwrite the prior vectors.
func (h *AIHandler) EmbedMetadata(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[embedMetadataRequest](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	if h.deps.AIEmbedMeta == nil || h.deps.Embedder == nil {
		writeError(w, http.StatusServiceUnavailable, "embeddings are not configured (set BI_AI_EMBEDDING_MODEL and API access: BI_AI_EMBEDDING_API_KEY or BI_AI_API_KEY; optional BI_AI_EMBEDDING_BASE_URL / BI_AI_BASE_URL)")
		return
	}
	ctx := r.Context()
	var (
		results []ai.EmbedTableResult
		err     error
	)
	if req.ModelID != "" {
		model, ferr := h.deps.SemanticRepo.GetFullModel(ctx, req.ModelID)
		if ferr != nil {
			writeError(w, http.StatusNotFound, "semantic model not found")
			return
		}
		allowed := tablesForModel(model)
		results, err = h.deps.AIEmbedMeta.EmbedForTables(ctx, req.DatasourceID, allowed)
	} else {
		results, err = h.deps.AIEmbedMeta.EmbedAllForDatasource(ctx, req.DatasourceID)
	}
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "embedding failed", err)
		return
	}
	embedded, skipped := 0, 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		} else {
			embedded++
		}
	}
	writeJSON(w, http.StatusOK, embedMetadataResponse{
		DatasourceID: req.DatasourceID,
		ModelID:      req.ModelID,
		Model:        h.deps.Embedder.Model(),
		Embedded:     embedded,
		Skipped:      skipped,
		Results:      results,
	})
}

func tablesForModel(model *semantic.SemanticModel) map[string]bool {
	allowed := map[string]bool{}
	if model == nil {
		return allowed
	}
	add := func(schema, table string) {
		if table == "" {
			return
		}
		if schema == "" {
			schema = model.BaseSchema
		}
		allowed[schema+"."+table] = true
	}
	add(model.BaseSchema, model.BaseTable)
	for _, j := range model.Joins {
		if j.IsActive {
			add(j.FromSchema, j.FromTable)
			add(j.ToSchema, j.ToTable)
		}
	}
	for _, d := range model.Dimensions {
		if !d.IsActive {
			continue
		}
		if s, t := splitColumnRef(d.ColumnRef, model.BaseSchema); t != "" {
			add(s, t)
		}
	}
	for _, m := range model.Metrics {
		if !m.IsActive {
			continue
		}
		if s, t := splitColumnRef(m.Expression, model.BaseSchema); t != "" {
			add(s, t)
		}
	}
	return allowed
}

func splitColumnRef(ref, baseSchema string) (schema, table string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 2:
		return baseSchema, parts[0]
	case 3:
		return parts[0], parts[1]
	default:
		return "", ""
	}
}

func (h *AIHandler) loadQueryModel(
	ctx context.Context,
	req aiQueryRequest,
) (*semantic.SemanticModel, *ai.TableRoutingResult, error) {
	if req.ModelID != "" {
		model, err := h.loadModel(ctx, req.DatasourceID, req.ModelID)
		if err != nil {
			return nil, nil, err
		}
		return model, routingForSemanticModel(model, 1), nil
	}
	if len(nonEmptyStringSlice(req.Tables)) == 0 {
		if model, routing, ok := h.loadPreferredSemanticModel(ctx, req.DatasourceID, req.Question); ok {
			return model, routing, nil
		}
	}
	base, views, err := typeScopeFromAIQueryRequest(req)
	if err != nil {
		return nil, nil, err
	}
	return h.tableRouter.Route(ctx, req.DatasourceID, req.Question, req.Tables, base, views)
}

func (h *AIHandler) loadPreferredSemanticModel(ctx context.Context, datasourceID, question string) (*semantic.SemanticModel, *ai.TableRoutingResult, bool) {
	models, err := h.deps.SemanticRepo.ListModels(ctx, datasourceID)
	if err != nil {
		slog.WarnContext(ctx, "list semantic models failed; falling back to auto context", "datasource_id", datasourceID, "error", err)
		return nil, nil, false
	}
	model, ok := chooseSemanticModelForQuestion(models, question)
	if !ok {
		return nil, nil, false
	}
	full, err := h.loadModel(ctx, datasourceID, model.Name)
	if err != nil {
		slog.WarnContext(ctx, "load preferred semantic model failed; falling back to auto context", "datasource_id", datasourceID, "model", model.Name, "error", err)
		return nil, nil, false
	}
	return full, routingForSemanticModel(full, semanticModelConfidence(models, model, question)), true
}

func chooseSemanticModelForQuestion(models []semantic.SemanticModel, question string) (semantic.SemanticModel, bool) {
	active := make([]semantic.SemanticModel, 0, len(models))
	for _, model := range models {
		status := model.Status
		if status == "" {
			status = semantic.ModelStatusPublished
		}
		if !model.IsActive || status != semantic.ModelStatusPublished || strings.HasPrefix(model.Name, "auto:") {
			continue
		}
		active = append(active, model)
	}
	if len(active) == 0 {
		return semantic.SemanticModel{}, false
	}
	if len(active) == 1 {
		return active[0], true
	}

	tokens := handlerTokenSet(question)
	var (
		best      semantic.SemanticModel
		bestScore float64
	)
	for _, model := range active {
		score := scoreSemanticModelForQuestion(model, tokens)
		if score > bestScore {
			best = model
			bestScore = score
		}
	}
	if bestScore == 0 {
		return semantic.SemanticModel{}, false
	}
	return best, true
}

func semanticModelConfidence(models []semantic.SemanticModel, selected semantic.SemanticModel, question string) float64 {
	activeCount := 0
	for _, model := range models {
		status := model.Status
		if status == "" {
			status = semantic.ModelStatusPublished
		}
		if model.IsActive && status == semantic.ModelStatusPublished && !strings.HasPrefix(model.Name, "auto:") {
			activeCount++
		}
	}
	if activeCount <= 1 {
		return 1
	}
	score := scoreSemanticModelForQuestion(selected, handlerTokenSet(question))
	if score <= 0 {
		return 0.65
	}
	if score > 10 {
		return 0.95
	}
	return 0.75
}

func scoreSemanticModelForQuestion(model semantic.SemanticModel, tokens map[string]bool) float64 {
	score := weightedHandlerTokenScore(tokens, model.Name, 4)
	score += weightedHandlerTokenScore(tokens, model.BaseTable, 3)
	if model.Label != nil {
		score += weightedHandlerTokenScore(tokens, *model.Label, 2)
	}
	if model.Description != nil {
		score += weightedHandlerTokenScore(tokens, *model.Description, 1.5)
	}
	for _, synonym := range model.Synonyms {
		score += weightedHandlerTokenScore(tokens, synonym, 3)
	}
	return score
}

func routingForSemanticModel(model *semantic.SemanticModel, confidence float64) *ai.TableRoutingResult {
	if model == nil {
		return nil
	}
	selectedTables := selectedTablesForSemanticModel(model)
	routing := &ai.TableRoutingResult{
		SelectedModels:     []string{model.Name},
		SelectedTables:     selectedTables,
		SelectedDimensions: semanticDimensionNames(model.Dimensions),
		SelectedMetrics:    semanticMetricNames(model.Metrics),
		JoinPaths:          semanticJoinPaths(model),
		Confidence:         confidence,
		ContextSource:      "semantic_model",
		ContextKey:         model.ID,
		RankingMethod:      "semantic",
		Candidates: []ai.TableCandidate{{
			Table:      qualifySemanticTable(model, model.BaseTable),
			Score:      confidence,
			TotalScore: confidence,
			Selected:   true,
		}},
		Debug: &ai.TableRoutingDebug{
			RelationExpansion: semanticJoinPaths(model),
		},
	}
	if routing.ContextKey == "" {
		routing.ContextKey = model.Name
	}
	if !model.UpdatedAt.IsZero() {
		t := model.UpdatedAt
		routing.ContextUpdatedAt = &t
	}
	return routing
}

func selectedTablesForSemanticModel(model *semantic.SemanticModel) []string {
	seen := map[string]bool{}
	var out []string
	add := func(table string) {
		table = qualifySemanticTable(model, table)
		if table == "" || seen[table] {
			return
		}
		seen[table] = true
		out = append(out, table)
	}
	add(model.BaseTable)
	for _, join := range model.Joins {
		add(join.FromTable)
		add(join.ToTable)
	}
	return out
}

func semanticJoinPaths(model *semantic.SemanticModel) []string {
	var out []string
	for _, join := range model.Joins {
		out = append(out, fmt.Sprintf(
			"%s.%s = %s.%s",
			qualifySemanticTable(model, join.FromTable),
			join.FromColumn,
			qualifySemanticTable(model, join.ToTable),
			join.ToColumn,
		))
	}
	return out
}

func qualifySemanticTable(model *semantic.SemanticModel, table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		return ""
	}
	if strings.Contains(table, ".") || model.BaseSchema == "" {
		return table
	}
	return model.BaseSchema + "." + table
}

func semanticDimensionNames(dimensions []semantic.Dimension) []string {
	out := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		out = append(out, dimension.Name)
	}
	return out
}

func semanticMetricNames(metrics []semantic.Metric) []string {
	out := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		out = append(out, metric.Name)
	}
	return out
}

func nonEmptyStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func weightedHandlerTokenScore(questionTokens map[string]bool, text string, weight float64) float64 {
	textTokens := handlerTokenSet(text)
	score := 0.0
	for token := range questionTokens {
		if textTokens[token] {
			score += weight
		}
	}
	return score
}

func handlerTokenSet(text string) map[string]bool {
	text = strings.ToLower(strings.NewReplacer(
		"İ", "i", "I", "i", "ı", "i",
		"Ş", "s", "ş", "s",
		"Ğ", "g", "ğ", "g",
		"Ü", "u", "ü", "u",
		"Ö", "o", "ö", "o",
		"Ç", "c", "ç", "c",
	).Replace(text))
	var normalized strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			normalized.WriteRune(r)
			continue
		}
		normalized.WriteRune(' ')
	}
	tokens := make(map[string]bool)
	for _, token := range strings.Fields(normalized.String()) {
		tokens[token] = true
		if strings.HasSuffix(token, "s") && len(token) > 3 {
			tokens[strings.TrimSuffix(token, "s")] = true
		}
	}
	return tokens
}

func typeScopeFromAIQueryRequest(req aiQueryRequest) (includeBase, includeViews bool, err error) {
	includeBase = true
	includeViews = true
	if req.IncludeBaseTables != nil {
		includeBase = *req.IncludeBaseTables
	}
	if req.IncludeViews != nil {
		includeViews = *req.IncludeViews
	}
	if !includeBase && !includeViews {
		return false, false, ai.ErrTypeScopeEmpty
	}
	return includeBase, includeViews, nil
}

func (h *AIHandler) writeModelLoadError(ctx context.Context, w http.ResponseWriter, req aiQueryRequest, err error) {
	switch {
	case errors.Is(err, ai.ErrTypeScopeEmpty):
		writeError(w, http.StatusBadRequest, "at least one of include_base_tables or include_views must be true")
	case errors.Is(err, ai.ErrTableScopeInvalid):
		writeError(w, http.StatusBadRequest, "table scope invalid")
	case errors.Is(err, core.ErrLoadSemanticModel):
		writeError(w, http.StatusNotFound, "semantic model not found")
	case req.ModelID == "":
		slog.ErrorContext(ctx, "failed to route table scope", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to route table scope")
	default:
		slog.ErrorContext(ctx, "failed to load query model", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load query model")
	}
}

// fewShotLimit caps how many prior successful queries we attach to the prompt.
// Higher values dilute attention and inflate tokens; lower values miss context.
const fewShotLimit = 5

// sampleRowLimit and sampleCellRunes shape the prompt-side sample-data block:
// few rows, short cells. The goal is to show value formats, not full data.
const (
	sampleRowLimit   = 3
	sampleCellRunes  = 80
	sampleColumnsMax = 30 // skip super-wide tables to keep tokens bounded
)

// loadSampleData fetches a small sample of rows from the model's base table to
// embed in the prompt. Errors are non-fatal — sampling is purely advisory.
func (h *AIHandler) loadSampleData(ctx context.Context, db *sql.DB, d dialect.Dialect, model *semantic.SemanticModel) []ai.TableSample {
	if model == nil || db == nil || model.BaseTable == "" {
		return nil
	}
	cols, err := h.deps.MetaRepo.ListColumns(ctx, model.DatasourceID, model.BaseSchema, model.BaseTable)
	if err != nil {
		slog.WarnContext(ctx, "load columns for sample failed", "error", err)
		return nil
	}
	if len(cols) == 0 || len(cols) > sampleColumnsMax {
		return nil
	}
	rows, err := ai.FetchTableSample(ctx, db, d, cols, model.BaseSchema, model.BaseTable, sampleRowLimit, sampleCellRunes)
	if err != nil {
		slog.WarnContext(ctx, "fetch sample rows failed", "error", err)
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	return []ai.TableSample{{Schema: model.BaseSchema, Table: model.BaseTable, Rows: rows}}
}

// loadFewShotExamplesWithIDs returns few-shot examples with optional explicit
// example_ids and an opt-in to include recent successful queries. When both
// inputs are empty/false this matches loadFewShotExamples — used by Run() so
// the frontend can override which exemplars hit the prompt without breaking
// the simpler Query/Preview paths.
func (h *AIHandler) loadFewShotExamplesWithIDs(ctx context.Context, model *semantic.SemanticModel, exampleIDs []string, includePastQueries bool) []ai.FewShotExample {
	// Explicit IDs are not yet wired through to the example store; fall back
	// to the default loader plus the past-queries opt-in. The explicit-IDs
	// branch can be plumbed without changing call sites.
	_ = exampleIDs
	if !includePastQueries && len(exampleIDs) == 0 {
		return nil
	}
	return h.loadFewShotExamples(ctx, model)
}

// datasourceDialectName returns the driver type for prompt dialect examples.
// Failures are non-fatal — empty string defaults to postgres in the prompt builder.
func (h *AIHandler) datasourceDialectName(ctx context.Context, datasourceID string) string {
	if datasourceID == "" {
		return ""
	}
	ds, err := h.deps.MetaRepo.GetDatasource(ctx, datasourceID)
	if err != nil {
		slog.WarnContext(ctx, "load datasource for dialect hint failed", "error", err)
		return ""
	}
	return ds.Type
}

// loadGlossaryForPrompt merges catalog synonyms with curated glossary terms and
// selects entries relevant to the user question.
func (h *AIHandler) loadGlossaryForPrompt(ctx context.Context, model *semantic.SemanticModel, question string) []ai.GlossaryEntry {
	if model == nil {
		return nil
	}
	catalog := ai.GlossaryFromSemanticModel(model)
	var ext []ai.ExternalGlossaryInput
	rows, err := h.deps.MetaRepo.ListBusinessGlossary(ctx, model.DatasourceID, model.ID)
	if err != nil {
		slog.WarnContext(ctx, "load business glossary failed", "error", err)
	} else {
		for _, r := range rows {
			ext = append(ext, ai.ExternalGlossaryInput{
				Term:       r.Term,
				Definition: r.Definition,
				MapsToType: r.MapsToType,
				MapsToName: r.MapsToName,
				Aliases:    r.Aliases,
			})
		}
	}
	merged := ai.MergeGlossaryEntries(catalog, ai.GlossaryFromExternal(ext))
	return ai.SelectGlossaryForQuestion(question, merged, model)
}

// loadFewShotExamples returns recent high-confidence (question, logical_query)
// pairs for this datasource+model. Errors are non-fatal — we just log and skip.
func (h *AIHandler) loadFewShotExamples(ctx context.Context, model *semantic.SemanticModel) []ai.FewShotExample {
	if model == nil {
		return nil
	}
	var modelID *string
	if model.ID != "" {
		id := model.ID
		modelID = &id
	}
	rows, err := h.deps.MetaRepo.ListSuccessfulAIQueries(ctx, model.DatasourceID, modelID, fewShotLimit)
	if err != nil {
		slog.WarnContext(ctx, "load few-shot examples failed", "error", err)
		return nil
	}
	out := make([]ai.FewShotExample, 0, len(rows))
	for _, r := range rows {
		out = append(out, ai.FewShotExample{Question: r.Question, LogicalQuery: string(r.LogicalQuery)})
	}
	return out
}

func clarificationResponse(routing *ai.TableRoutingResult) *ai.Response {
	question := "Which table or topic do you want to query? Please pick one or more from the candidates."
	return &ai.Response{
		Warnings: []string{
			"could not confidently choose the relevant table scope; select one or more tables and try again",
		},
		Confidence:            0,
		TableRouting:          routing,
		NeedsClarification:    true,
		ClarificationQuestion: question,
		Clarification:         ai.ClarificationFromRouting(routing, question),
	}
}

func failedAIResponse(err error) *ai.Response {
	return &ai.Response{
		Warnings:   []string{err.Error()},
		Confidence: 0,
	}
}

func (h *AIHandler) loadModel(ctx context.Context, datasourceID, modelRef string) (*semantic.SemanticModel, error) {
	if model, err := h.deps.SemanticRepo.GetPublishedFullModel(ctx, modelRef); err == nil {
		return model, nil
	}
	model, err := h.deps.SemanticRepo.GetPublishedModelByName(ctx, datasourceID, modelRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrLoadSemanticModel, err)
	}
	return model, nil
}
