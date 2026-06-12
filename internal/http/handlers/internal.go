// Package handlers exposes Biqly's HTTP layer. This file owns the /internal/*
// surface used by peer services inside the cluster (Phase 1 of the
// microservice decomposition — see docs/microservice-decomposition.md).
//
// The /internal/* routes MUST NOT be reachable from outside the cluster.
// In Kubernetes the Cilium Gateway HTTPRoute only matches /api/*; in local
// development the router exposes them on the same port for convenience.
package handlers

import (
	"context"
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"
	"strings"
	"time"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/pkg/internalapi"
)

// InternalHandler serves /internal/* endpoints. It owns the read-only surface
// over the catalog (datasources, semantic models, tables, columns, relations,
// few-shot examples, business glossary) and the write surface for history
// persistence (ai-history, query-history). Compile/run/dry-run endpoints
// live on a sibling handler so the query subsystem can be lifted out
// without touching this struct.
type InternalHandler struct {
	meta        internalMetaRepo
	semantic    internalSemanticRepo
	eval        internalEvalRepo
	serviceName string
}

// NewInternalHandlerWithService returns an internal handler with a service
// name suitable for /internal/health responses.
func NewInternalHandlerWithService(deps *app.CatalogDeps, serviceName string) *InternalHandler {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "biqly-monolith"
	}
	return &InternalHandler{
		meta:        deps.MetaRepo,
		semantic:    deps.SemanticRepo,
		eval:        nil, // We will retrieve this from CatalogDeps if /internal ever needs it, but it only needs MetaRepo / SemanticRepo
		serviceName: serviceName,
	}
}

// Health reports the internal service health. Cheap — no DB ping — so
// liveness probes never block on a slow upstream. /ready does the deeper check.
func (h *InternalHandler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, internalapi.HealthResponse{
		Status:  "ok",
		Service: h.healthServiceName(),
	})
}

func (h *InternalHandler) healthServiceName() string {
	if h == nil || strings.TrimSpace(h.serviceName) == "" {
		return "biqly-monolith"
	}
	return h.serviceName
}

// GetDatasource returns the full datasource record including the encrypted
// DSN. The peer service is expected to share BI_ENCRYPTION_KEY and decrypt
// locally; we never ship plaintext credentials over the wire even inside the
// cluster. Returns 404 when the id is unknown.
func (h *InternalHandler) GetDatasource(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ds, err := h.meta.GetDatasource(r.Context(), id)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusNotFound,
			internalapi.CodeNotFound, "datasource not found", err)
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

// ListDatasources returns every datasource. DSNs remain encrypted; see
// GetDatasource for the reasoning.
func (h *InternalHandler) ListDatasources(w http.ResponseWriter, r *http.Request) {
	out, err := h.meta.ListDatasources(r.Context())
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to list datasources", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// GetFullModel returns the published semantic model with dimensions, metrics
// and joins inlined. AI Service relies on this to build prompts; "published"
// is implicit so callers never see drafts.
func (h *InternalHandler) GetFullModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	model, err := h.semantic.GetPublishedFullModel(r.Context(), id)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusNotFound,
			internalapi.CodeNotFound, "model not found", err)
		return
	}
	writeJSON(w, http.StatusOK, model)
}

// ListModels returns every semantic model header (no children) for a
// datasource. The datasource_id query parameter is required.
func (h *InternalHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireInternalQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	models, err := h.semantic.ListModels(r.Context(), datasourceID)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to list models", err)
		return
	}
	writeJSON(w, http.StatusOK, models)
}

// ListTables returns introspected tables for a datasource. Optional
// schema_name query parameter narrows the result; when omitted every schema
// is returned.
func (h *InternalHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireInternalQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	schemaName := strings.TrimSpace(r.URL.Query().Get("schema_name"))
	tables, err := h.meta.ListTables(r.Context(), datasourceID, schemaName)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to list tables", err)
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

// ListColumns returns columns for a datasource. Optional schema_name and
// table_name query parameters narrow the result; when omitted every column
// is returned (large responses — callers should always scope to a table).
func (h *InternalHandler) ListColumns(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireInternalQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	q := r.URL.Query()
	cols, err := h.meta.ListColumns(
		r.Context(),
		datasourceID,
		strings.TrimSpace(q.Get("schema_name")),
		strings.TrimSpace(q.Get("table_name")),
	)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to list columns", err)
		return
	}
	writeJSON(w, http.StatusOK, cols)
}

// ListRelations returns foreign-key relations for a datasource.
func (h *InternalHandler) ListRelations(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireInternalQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	rels, err := h.meta.ListRelations(r.Context(), datasourceID)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to list relations", err)
		return
	}
	writeJSON(w, http.StatusOK, rels)
}

// ListFewShot returns curated few-shot examples for a datasource (and
// optionally a single model). AI Service uses this to seed prompts.
func (h *InternalHandler) ListFewShot(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireInternalQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	modelID := strings.TrimSpace(r.URL.Query().Get("model_id"))
	rows, err := h.meta.ListFewShotCurated(r.Context(), datasourceID, modelID)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to list few-shot examples", err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// ListGlossary returns business glossary entries for a datasource (and
// optionally a single model). AI Service injects these into prompts so the
// LLM can map business jargon to declared dimensions/metrics.
func (h *InternalHandler) ListGlossary(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireInternalQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	modelID := strings.TrimSpace(r.URL.Query().Get("model_id"))
	rows, err := h.meta.ListBusinessGlossary(r.Context(), datasourceID, modelID)
	if err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to list glossary", err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

type historyEntryRequest interface {
	HistoryDatasourceID() string
	HistoryID() string
}

func recordHistoryEntry[T any, TResp any](
	w http.ResponseWriter,
	r *http.Request,
	persist func(context.Context, *T) error,
	response func(string) TResp,
	failMsg string,
) {
	req, ok := decodeJSON[T](w, r)
	if !ok {
		return
	}
	hist, ok := any(req).(historyEntryRequest)
	if !ok {
		writeInternalAPIErrorMsg(w, http.StatusInternalServerError, internalapi.CodeInternal,
			"history entry request type mismatch")
		return
	}
	if strings.TrimSpace(hist.HistoryDatasourceID()) == "" {
		writeInternalAPIErrorMsg(w, http.StatusBadRequest, internalapi.CodeInvalidRequest,
			"entry.datasource_id is required")
		return
	}
	if err := persist(r.Context(), req); err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, failMsg, err)
		return
	}
	writeJSON(w, http.StatusCreated, response(hist.HistoryID()))
}

// CreateAIHistory persists one AI query history row. Returns the assigned
// row id so callers can correlate later /api/ai/feedback submissions.
func (h *InternalHandler) CreateAIHistory(w http.ResponseWriter, r *http.Request) {
	recordHistoryEntry(w, r,
		func(ctx context.Context, req *internalapi.AIHistoryRequest) error {
			return h.meta.CreateAIQueryHistory(ctx, &req.Entry)
		},
		func(id string) internalapi.AIHistoryResponse { return internalapi.AIHistoryResponse{ID: id} },
		"failed to record ai history",
	)
}

// CreateQueryHistory persists one query history row. Returns the assigned
// row id so the caller can attach this id to downstream telemetry.
func (h *InternalHandler) CreateQueryHistory(w http.ResponseWriter, r *http.Request) {
	recordHistoryEntry(w, r,
		func(ctx context.Context, req *internalapi.QueryHistoryRequest) error {
			return h.meta.CreateQueryHistory(ctx, &req.Entry)
		},
		func(id string) internalapi.QueryHistoryResponse { return internalapi.QueryHistoryResponse{ID: id} },
		"failed to record query history",
	)
}

// CreateEvalResults persists one eval run's per-case results and aggregate
// summary. AI Service calls this after running the golden suite.
func (h *InternalHandler) CreateEvalResults(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[internalapi.EvalResultsRequest](w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		writeInternalAPIErrorMsg(w, http.StatusBadRequest, internalapi.CodeInvalidRequest,
			"run_id is required")
		return
	}
	if strings.TrimSpace(req.Provider) == "" {
		writeInternalAPIErrorMsg(w, http.StatusBadRequest, internalapi.CodeInvalidRequest,
			"provider is required")
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		writeInternalAPIErrorMsg(w, http.StatusBadRequest, internalapi.CodeInvalidRequest,
			"model is required")
		return
	}
	if h.eval == nil {
		writeInternalAPIErrorMsg(w, http.StatusServiceUnavailable, internalapi.CodeUpstream,
			"eval repository is not configured")
		return
	}
	if req.ContextUpdatedAt.IsZero() {
		req.ContextUpdatedAt = time.Now().UTC()
	}
	if err := h.eval.SaveRunResults(
		r.Context(),
		req.RunID,
		req.Provider,
		req.Model,
		req.ContextVersion,
		req.ContextUpdatedAt,
		evalResultsFromWire(req.Results),
	); err != nil {
		writeInternalAPIError(r.Context(), w, http.StatusInternalServerError,
			internalapi.CodeInternal, "failed to record eval results", err)
		return
	}
	writeJSON(w, http.StatusCreated, internalapi.EvalResultsResponse{
		RunID:      req.RunID,
		TotalCases: len(req.Results),
	})
}

func evalResultsFromWire(results []internalapi.EvalResultMetrics) []evalpkg.ResultWithMetrics {
	out := make([]evalpkg.ResultWithMetrics, 0, len(results))
	for _, result := range results {
		out = append(out, evalpkg.ResultWithMetrics{
			Result: evalpkg.Result{
				Case: evalpkg.GoldenCase{
					ID:       result.Case.ID,
					Question: result.Case.Question,
					Expected: result.Case.Expected,
				},
				Got:    result.Got,
				Match:  result.Match,
				Reason: result.Reason,
			},
			Confidence:                  result.Confidence,
			LatencyMs:                   result.LatencyMs,
			TokenCount:                  result.TokenCount,
			PromptTemplateVersions:      result.PromptTemplateVersions,
			PromptTemplateBundleVersion: result.PromptTemplateBundleVersion,
		})
	}
	return out
}

// --- helpers (scoped to the /internal/* surface) ----------------------------

// requireInternalQueryParam writes a 400 and returns ok=false when the named query
// parameter is missing or blank. Mirrors requireURLParam's contract.
//
// future required params (model_id, schema_name, ...) without touching call
// sites.
//
//nolint:unparam // name is always "datasource_id" today but kept generic for
func requireInternalQueryParam(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		writeInternalAPIErrorMsg(w, http.StatusBadRequest, internalapi.CodeInvalidRequest,
			name+" query parameter is required")
		return "", false
	}
	return v, true
}

// writeInternalAPIErrorMsg emits the canonical internalapi.Error envelope.
func writeInternalAPIErrorMsg(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := internalapi.Error{Code: code, Error: message}
	if err := sonic.ConfigStd.NewEncoder(w).Encode(body); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// writeInternalAPIError logs (via slog) and emits the canonical error
// envelope. publicMsg is what the peer sees; err goes only to the structured log.
func writeInternalAPIError(ctx context.Context, w http.ResponseWriter,
	status int, code, publicMsg string, err error,
) {
	if err != nil {
		allArgs := []any{"error", err}
		allArgs = appendRequestLogArgs(ctx, allArgs)
		slog.ErrorContext(ctx, publicMsg, allArgs...)
	}
	writeInternalAPIErrorMsg(w, status, code, publicMsg)
}
