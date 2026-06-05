package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/security/pii"
)

// PIIHandler manages PII detection scans and column annotations.
type PIIHandler struct {
	deps *app.CatalogDeps
}

// NewPIIHandler creates a new PII handler.
func NewPIIHandler(deps *app.CatalogDeps) *PIIHandler {
	return &PIIHandler{deps: deps}
}

// runPIIScan executes a PII detection scan over a resolved datasource using
// its live connection for sample data. Threshold and sample limit come from
// the PII config section; zero values fall back to package defaults.
func runPIIScan(ctx context.Context, deps *app.CatalogDeps, resolved *app.ResolvedDatasource) (*pii.ScanSummary, error) {
	threshold := pii.DefaultThreshold
	sampleLimit := pii.DefaultSampleLimit
	if deps.Config != nil {
		if deps.Config.PII.DetectionThreshold > 0 {
			threshold = deps.Config.PII.DetectionThreshold
		}
		if deps.Config.PII.SampleDataLimit > 0 {
			sampleLimit = deps.Config.PII.SampleDataLimit
		}
	}
	detector := pii.NewDetector(threshold)
	scanner := pii.NewScanner(detector, deps.MetaRepo, sampleLimit)
	fetch := pii.NewDBSampleFetcher(resolved.DB, resolved.Driver.Dialect())

	summary, err := scanner.ScanDatasource(ctx, resolved.Record.ID, fetch)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "pii scan completed",
		"datasource_id", resolved.Record.ID,
		"scanned_columns", summary.ScannedColumns,
		"detected", summary.Detected)
	if deps.AuditLogger != nil {
		detected := make(map[string]any, len(summary.Detected))
		for k, v := range summary.Detected {
			detected[k] = v
		}
		deps.AuditLogger.Log(ctx, audit.Event{
			UserID:       bimw.UserID(ctx),
			EventType:    audit.EventPIIScanCompleted,
			DatasourceID: resolved.Record.ID,
			Details: map[string]any{
				"scanned_columns": summary.ScannedColumns,
				"detected":        detected,
			},
		})
	}
	return summary, nil
}

// piiEnabled reports whether the PII subsystem is on (default true).
func piiEnabled(deps *app.CatalogDeps) bool {
	return deps.Config == nil || deps.Config.PII.Enabled
}

// Scan triggers a PII detection scan on an existing datasource without a
// full metadata sync. POST /api/datasources/{id}/scan-pii
func (h *PIIHandler) Scan(w http.ResponseWriter, r *http.Request) {
	if !piiEnabled(h.deps) {
		writeError(w, http.StatusServiceUnavailable, "pii detection is disabled")
		return
	}
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	resolved, err := h.deps.ResolveDatasourceDB(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to open connection", err)
		return
	}
	defer func() { _ = resolved.DB.Close() }()

	summary, err := runPIIScan(ctx, h.deps, resolved)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "pii scan failed", err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// piiColumnResponse is the wire shape for a PII-annotated column.
type piiColumnResponse struct {
	ColumnID        string   `json:"column_id"`
	Schema          string   `json:"schema"`
	Table           string   `json:"table"`
	Column          string   `json:"column"`
	PIIType         string   `json:"pii_type"`
	Confidence      *float64 `json:"confidence"`
	MaskingStrategy *string  `json:"masking_strategy"`
	ReviewedBy      *string  `json:"reviewed_by"`
}

// ListColumns returns all PII-annotated columns for a datasource.
// GET /api/datasources/{id}/pii-columns
func (h *PIIHandler) ListColumns(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	cols, err := h.deps.MetaRepo.ListPIIColumns(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list pii columns", err)
		return
	}

	resp := make([]piiColumnResponse, 0, len(cols))
	for _, c := range cols {
		piiType := ""
		if c.PIIType != nil {
			piiType = *c.PIIType
		}
		resp = append(resp, piiColumnResponse{
			ColumnID:        c.ID,
			Schema:          c.SchemaName,
			Table:           c.TableName,
			Column:          c.ColumnName,
			PIIType:         piiType,
			Confidence:      c.PIIConfidence,
			MaskingStrategy: c.PIIMaskingStrategy,
			ReviewedBy:      c.PIIReviewedBy,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

type updateColumnPIIRequest struct {
	PIIType            string  `json:"pii_type"`
	PIIMaskingStrategy *string `json:"pii_masking_strategy,omitempty"`
	PIIReviewedBy      string  `json:"pii_reviewed_by"`
}

// UpdateColumn manually sets/overrides the PII annotation on a column with
// full confidence. PATCH /api/metadata/columns/{id}/pii
func (h *PIIHandler) UpdateColumn(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[updateColumnPIIRequest](w, r)
	if !ok {
		return
	}
	if !pii.IsValidType(req.PIIType) {
		writeError(w, http.StatusBadRequest, "invalid pii_type")
		return
	}
	if req.PIIReviewedBy == "" {
		writeError(w, http.StatusBadRequest, "pii_reviewed_by is required")
		return
	}

	ctx := r.Context()
	if _, err := h.deps.MetaRepo.GetColumn(ctx, id); err != nil {
		writeEntityNotFound(w, "column")
		return
	}
	if err := h.deps.MetaRepo.SetColumnPIIReview(ctx, id, req.PIIType, req.PIIMaskingStrategy, req.PIIReviewedBy); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update pii annotation", err)
		return
	}
	col, err := h.deps.MetaRepo.GetColumn(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load column", err)
		return
	}
	writeJSON(w, http.StatusOK, col)
}

// DeleteColumn clears the PII annotation from a column, recording the
// reviewer so future scans don't re-flag it.
// DELETE /api/metadata/columns/{id}/pii?reviewed_by=admin@biqly.com
func (h *PIIHandler) DeleteColumn(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	if _, err := h.deps.MetaRepo.GetColumn(ctx, id); err != nil {
		writeEntityNotFound(w, "column")
		return
	}
	reviewedBy := r.URL.Query().Get("reviewed_by")
	if err := h.deps.MetaRepo.ClearColumnPII(ctx, id, reviewedBy); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to clear pii annotation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ComplianceSummary reports per-datasource PII detection/review counts.
// GET /api/compliance/pii-summary[?format=csv]
func (h *PIIHandler) ComplianceSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	summaries, err := h.deps.MetaRepo.PIIComplianceSummary(ctx)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to build pii compliance summary", err)
		return
	}

	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="pii-compliance-summary.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"datasource_id", "datasource_name", "total_columns", "pii_detected", "reviewed", "unreviewed", "by_type"})
		for _, s := range summaries {
			byType, err := json.Marshal(s.ByType)
			if err != nil {
				continue
			}
			_ = cw.Write([]string{
				s.DatasourceID,
				s.DatasourceName,
				strconv.Itoa(s.TotalColumns),
				strconv.Itoa(s.PIIDetected),
				strconv.Itoa(s.Reviewed),
				strconv.Itoa(s.Unreviewed),
				string(byType),
			})
		}
		cw.Flush()
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}
