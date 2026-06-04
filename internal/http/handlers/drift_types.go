package handlers

import (
	"time"

	"github.com/biqly/biqly/internal/semantic/drift"
)

type driftItemResponse struct {
	Type        string `json:"type"`
	Field       string `json:"field"`
	ColumnRef   string `json:"column_ref"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
	Description string `json:"description"`
}

type driftReportResponse struct {
	ID           string              `json:"id"`
	ModelID      string              `json:"model_id"`
	DatasourceID string              `json:"datasource_id"`
	SyncEventID  *string             `json:"sync_event_id,omitempty"`
	Severity     string              `json:"severity"`
	Drifts       []driftItemResponse `json:"drifts"`
	Resolved     bool                `json:"resolved"`
	ResolvedBy   *string             `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time          `json:"resolved_at,omitempty"`
	DetectedAt   time.Time           `json:"detected_at"`
}

func mapDriftReportResponse(r drift.DriftReport) driftReportResponse {
	items := make([]driftItemResponse, len(r.Drifts))
	for i, item := range r.Drifts {
		items[i] = driftItemResponse{
			Type:        string(item.Type),
			Field:       item.Field,
			ColumnRef:   item.ColumnRef,
			OldValue:    item.OldValue,
			NewValue:    item.NewValue,
			Description: item.Description,
		}
	}
	return driftReportResponse{
		ID:           r.ID,
		ModelID:      r.ModelID,
		DatasourceID: r.DatasourceID,
		SyncEventID:  r.SyncEventID,
		Severity:     r.Severity,
		Drifts:       items,
		Resolved:     r.Resolved,
		ResolvedBy:   r.ResolvedBy,
		ResolvedAt:   r.ResolvedAt,
		DetectedAt:   r.DetectedAt,
	}
}
