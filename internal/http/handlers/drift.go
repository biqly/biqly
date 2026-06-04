package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// DriftHandler handles HTTP requests for schema drift reports.
type DriftHandler struct {
	deps *app.CatalogDeps
}

// NewDriftHandler constructs a DriftHandler.
func NewDriftHandler(deps *app.CatalogDeps) *DriftHandler {
	return &DriftHandler{deps: deps}
}

// ListForModel returns all unresolved drift reports for a semantic model.
func (h *DriftHandler) ListForModel(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	reports, err := h.deps.DriftRepo.ListUnresolvedByModel(ctx, modelID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list model drift reports", err)
		return
	}

	resp := make([]driftReportResponse, len(reports))
	for i, rpt := range reports {
		resp[i] = mapDriftReportResponse(rpt)
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListForDatasource returns all unresolved drift reports for a datasource.
func (h *DriftHandler) ListForDatasource(w http.ResponseWriter, r *http.Request) {
	dsID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	reports, err := h.deps.DriftRepo.ListUnresolvedByDatasource(ctx, dsID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list datasource drift reports", err)
		return
	}

	resp := make([]driftReportResponse, len(reports))
	for i, rpt := range reports {
		resp[i] = mapDriftReportResponse(rpt)
	}

	writeJSON(w, http.StatusOK, resp)
}

// Resolve marks a drift report as resolved.
func (h *DriftHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	reportID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	resolvedBy := bimw.UserID(ctx)
	if resolvedBy == "" {
		resolvedBy = "system"
	}

	err := h.deps.DriftRepo.ResolveReport(ctx, reportID, resolvedBy)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to resolve drift report", err)
		return
	}

	h.deps.AuditLogger.Log(ctx, audit.Event{
		UserID:    resolvedBy,
		EventType: audit.EventDriftResolved,
		Details: map[string]any{
			"drift_report_id": reportID,
		},
	})

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
