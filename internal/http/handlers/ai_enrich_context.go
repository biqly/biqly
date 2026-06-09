package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/ai/enrichcontext"
	"github.com/biqly/biqly/internal/app"
)

// EnrichContextHandler exposes admin enrich-context analyze/apply endpoints.
type EnrichContextHandler struct {
	deps    *app.AIDeps
	svc     *enrichcontext.Service
	metrics AIMetricsRecorder
}

// NewEnrichContextHandler creates the handler.
func NewEnrichContextHandler(deps *app.AIDeps) *EnrichContextHandler {
	svc := enrichcontext.NewService(
		deps.MetaRepo,
		deps.SemanticRepo,
		deps.AIClient,
		deps.DriverReg,
		deps.PoolCache,
		deps.Encryptor,
	)
	return &EnrichContextHandler{deps: deps, svc: svc}
}

// SetAIMetricsRecorder wires Prometheus counters.
func (h *EnrichContextHandler) SetAIMetricsRecorder(m AIMetricsRecorder) {
	h.metrics = m
}

// Analyze detects context gaps and optional AI suggestions.
func (h *EnrichContextHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[enrichcontext.AnalyzeRequest](w, r)
	if !ok {
		return
	}
	result, err := h.svc.Analyze(r.Context(), *input)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusBadRequest, "enrich-context analyze failed", err)
		return
	}
	if h.metrics != nil {
		h.metrics.RecordEnrichContextGaps(len(result.Gaps))
	}
	writeJSON(w, http.StatusOK, result)
}

// Apply persists user-approved enrichments.
func (h *EnrichContextHandler) Apply(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[enrichcontext.ApplyRequest](w, r)
	if !ok {
		return
	}
	result, err := h.svc.Apply(r.Context(), *input)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusBadRequest, "enrich-context apply failed", err)
		return
	}
	if h.metrics != nil && result.Applied > 0 {
		h.metrics.RecordEnrichContextApplied(result.Applied)
	}
	writeJSON(w, http.StatusOK, result)
}
