package handlers

import (
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/auth"
)

func (h *AuthHandler) handleMeExport(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	if h.gdpr == nil {
		h.respondError(w, http.StatusServiceUnavailable, "export not configured")
		return
	}
	data, err := h.gdpr.Export(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.audit != nil {
		if err := h.audit.Log(r.Context(), &userID, auth.AuditGDPRDataDump, new("user"), &userID,
			map[string]any{"sessions": len(data.Sessions), "audit_entries": len(data.AuditEntries)}, nil); err != nil {
			slog.WarnContext(r.Context(), "auth audit log failed", "action", auth.AuditGDPRDataDump, "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=biqly_user_data.json")
	w.WriteHeader(http.StatusOK)
	if err := sonic.ConfigStd.NewEncoder(w).Encode(data); err != nil {
		return
	}
}
