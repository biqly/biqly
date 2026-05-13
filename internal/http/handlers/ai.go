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

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// AIHandler handles AI text-to-query operations.
type AIHandler struct {
	service     *ai.Service
	tableRouter *ai.TableRouter
	deps        *app.Dependencies
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
	var req aiQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return req, nil, nil, false
	}
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return req, nil, nil, false
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return req, nil, nil, false
	}

	ctx := r.Context()
	model, routing, err := h.loadQueryModel(ctx, req)
	if err != nil {
		h.writeModelLoadError(w, req, err)
		return req, nil, nil, false
	}
	if routing != nil && routing.NeedsClarification {
		resp := clarificationResponse(routing)
		h.recordAIHistory(ctx, req, model, routing, resp)
		writeJSON(w, http.StatusOK, resp)
		return req, nil, nil, false
	}
	return req, model, routing, true
}

// Query handles AI-powered natural language queries.
func (h *AIHandler) Query(w http.ResponseWriter, r *http.Request) {
	req, model, routing, ok := h.parseAndRouteAIQuery(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	resp, err := h.service.ProcessQuestion(ctx, req.Question, model,
		ai.WithFewShotExamples(h.loadFewShotExamples(ctx, model)),
		ai.WithPriorTurns(priorTurnsForPrompt(req.PriorTurns)),
	)
	if err != nil {
		h.recordAIHistory(ctx, req, model, routing, failedAIResponse(err))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.ModelUsed = h.deps.Config.AI.EffectiveQueryConfig().Model
	resp.TableRouting = routing
	h.recordAIHistory(ctx, req, model, routing, resp)

	writeJSON(w, http.StatusOK, resp)
}

// Preview handles AI query preview (compiles but does not execute).
func (h *AIHandler) Preview(w http.ResponseWriter, r *http.Request) {
	req, model, routing, ok := h.parseAndRouteAIQuery(w, r)
	if !ok {
		return
	}

	ctx := r.Context()

	// Get AI response
	resp, err := h.service.ProcessQuestion(ctx, req.Question, model,
		ai.WithFewShotExamples(h.loadFewShotExamples(ctx, model)),
		ai.WithPriorTurns(priorTurnsForPrompt(req.PriorTurns)),
	)
	if err != nil {
		h.recordAIHistory(ctx, req, model, routing, failedAIResponse(err))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.ModelUsed = h.deps.Config.AI.EffectiveQueryConfig().Model
	resp.TableRouting = routing
	h.recordAIHistory(ctx, req, model, routing, resp)

	if resp.LogicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Compile to SQL for preview
	ds, err := h.deps.MetaRepo.GetDatasource(ctx, req.DatasourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	cq, err := h.deps.QueryService.CompileWithContext(ctx, *resp.LogicalQuery, model, driver)
	if err != nil {
		resp.Warnings = append(resp.Warnings, "compilation failed: "+err.Error())
	} else {
		resp.SQL = cq.SQL
		resp.Args = cq.Args
	}

	writeJSON(w, http.StatusOK, resp)
}

// Run handles AI query execution (compiles and executes).
func (h *AIHandler) Run(w http.ResponseWriter, r *http.Request) {
	req, model, routing, ok := h.parseAndRouteAIQuery(w, r)
	if !ok {
		return
	}

	ctx := r.Context()

	// Open the datasource up front so the dry-run validator (and later the
	// real Execute call) share a single connection. Both happen before we
	// stream a response, so a failure aborts cleanly.
	ds, err := h.deps.MetaRepo.GetDatasource(ctx, req.DatasourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}
	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}
	dsn, err := security.ConnectionDSN(h.deps.Encryptor, ds.DSNEncrypted)
	if err != nil {
		slog.ErrorContext(ctx, "decrypt datasource DSN failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read datasource credentials")
		return
	}
	db, err := driver.Open(ctx, dsn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("connection failed: %s", err.Error()))
		return
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("failed to close database connection", "error", closeErr)
		}
	}()
	dryRun := newSQLDryRunValidator(h.deps.QueryService, db, driver, model)

	// Get AI response (with EXPLAIN dry-run + few-shot history + sample rows)
	resp, err := h.service.ProcessQuestion(ctx, req.Question, model,
		ai.WithSQLValidator(dryRun),
		ai.WithFewShotExamples(h.loadFewShotExamplesWithIDs(ctx, model, req.ExampleIDs, req.IncludePastQueries)),
		ai.WithSampleData(h.loadSampleData(ctx, db, driver.Dialect(), model)),
		ai.WithPriorTurns(priorTurnsForPrompt(req.PriorTurns)),
	)
	if err != nil {
		h.recordAIHistory(ctx, req, model, routing, failedAIResponse(err))
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.TableRouting = routing
	resp.ModelUsed = h.deps.Config.AI.EffectiveQueryConfig().Model
	h.recordAIHistory(ctx, req, model, routing, resp)

	if resp.LogicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Compile (final, for execution — the dry-run already succeeded above)
	cq, err := h.deps.QueryService.CompileWithContext(ctx, *resp.LogicalQuery, model, driver)
	if err != nil {
		h.recordAIQueryHistory(ctx, *resp.LogicalQuery, model, nil, nil, queryStatusFailed, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", err.Error()))
		return
	}

	resp.SQL = cq.SQL
	resp.Args = cq.Args

	result, err := h.deps.Executor.Execute(ctx, db, cq)
	if err != nil {
		h.recordAIQueryHistory(ctx, *resp.LogicalQuery, model, cq, nil, queryStatusFailed, err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("execution failed: %s", err.Error()))
		return
	}

	resp.Result = result
	h.recordAIQueryHistory(ctx, *resp.LogicalQuery, model, cq, result, queryStatusSuccess, nil)
	writeJSON(w, http.StatusOK, resp)
}

// Describe runs the AI metadata describer over a single table and (optionally) writes
// the suggested table/column descriptions back into the metadata DB.
// Describe handles AI-powered table/column description generation.
func (h *AIHandler) Describe(w http.ResponseWriter, r *http.Request) {
	var req ai.DescribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DatasourceID == "" || req.Table == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and table are required")
		return
	}

	result, err := h.deps.AIDescriber.Describe(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type embedMetadataRequest struct {
	DatasourceID string `json:"datasource_id"`
}

type embedMetadataResponse struct {
	DatasourceID string                `json:"datasource_id"`
	Model        string                `json:"model"`
	Embedded     int                   `json:"embedded"`
	Skipped      int                   `json:"skipped"`
	Results      []ai.EmbedTableResult `json:"results,omitempty"`
}

// EmbedMetadata computes vector embeddings for every table and column in a
// datasource and stores them so the AI router can use hybrid retrieval.
// Idempotent — re-runs simply overwrite the prior vectors.
func (h *AIHandler) EmbedMetadata(w http.ResponseWriter, r *http.Request) {
	var req embedMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
	results, err := h.deps.AIEmbedMeta.EmbedAllForDatasource(r.Context(), req.DatasourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		Model:        h.deps.Embedder.Model(),
		Embedded:     embedded,
		Skipped:      skipped,
		Results:      results,
	})
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

func (h *AIHandler) writeModelLoadError(w http.ResponseWriter, req aiQueryRequest, err error) {
	if errors.Is(err, ai.ErrTypeScopeEmpty) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ai.ErrTableScopeInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ModelID == "" {
		writeError(w, http.StatusInternalServerError, "failed to route table scope")
		return
	}
	writeError(w, http.StatusNotFound, "semantic model not found")
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

// loadFewShotExamples returns recent high-confidence (question, logical_query)
// pairs for this datasource+model. Errors are non-fatal — we just log and skip.
func (h *AIHandler) loadFewShotExamples(ctx context.Context, model *semantic.SemanticModel) []ai.FewShotExample {
	if model == nil {
		return nil
	}
	var modelName *string
	if model.Name != "" {
		n := model.Name
		modelName = &n
	}
	rows, err := h.deps.MetaRepo.ListSuccessfulAIQueries(ctx, model.DatasourceID, modelName, fewShotLimit)
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

func (h *AIHandler) loadModel(ctx context.Context, datasourceID, modelID string) (*semantic.SemanticModel, error) {
	model, err := h.deps.SemanticRepo.GetPublishedModelByName(ctx, datasourceID, modelID)
	if err != nil {
		slog.ErrorContext(ctx, "load semantic model failed", "datasource_id", datasourceID, "model_id", modelID, "error", err)
		return nil, err
	}
	return model, nil
}
