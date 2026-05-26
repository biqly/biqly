package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

// SetMFA attaches the MFA service to the handler. Routes registered later
// will return 503 if this is nil.
func (h *AuthHandler) SetMFA(svc *MFAService) { h.mfa = svc }

func (h *AuthHandler) requireMFA(w http.ResponseWriter) bool {
	if h.mfa == nil {
		h.respondError(w, http.StatusServiceUnavailable, "mfa not configured")
		return false
	}
	return true
}

func (h *AuthHandler) handleMFAEnroll(w http.ResponseWriter, r *http.Request) {
	if !h.requireMFA(w) {
		return
	}
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	email, _ := r.Context().Value(emailKey).(string)
	if email == "" {
		email = userID
	}
	result, err := h.mfa.Enroll(r.Context(), userID, email)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, MFAEnrollResponse{
		Secret:        result.Secret,
		OTPAuthURL:    result.OTPAuthURL,
		RecoveryCodes: result.RecoveryCodes,
	})
}

func (h *AuthHandler) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	if !h.requireMFA(w) {
		return
	}
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req MFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.mfa.Verify(r.Context(), userID, req.Code); err != nil {
		if errors.Is(err, ErrMFACodeInvalid) || errors.Is(err, ErrMFANotEnrolled) {
			h.respondError(w, http.StatusUnauthorized, ErrMFACodeInvalid.Error())
			return
		}
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	if !h.requireMFA(w) {
		return
	}
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req MFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Disable still allowed without code? Require code to prevent
		// hijacked session from silently dropping MFA.
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.mfa.VerifyCode(r.Context(), userID, req.Code); err != nil {
		h.respondError(w, http.StatusUnauthorized, ErrMFACodeInvalid.Error())
		return
	}
	if err := h.mfa.Disable(r.Context(), userID); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireMFA(w) {
		return
	}
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	enrol, err := h.mfa.Status(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := MFAStatusResponse{}
	if enrol != nil {
		resp.Enabled = enrol.Enabled
		resp.Method = enrol.Method
		resp.VerifiedAt = enrol.VerifiedAt
	}
	h.respondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) handleMFARegenerateRecovery(w http.ResponseWriter, r *http.Request) {
	if !h.requireMFA(w) {
		return
	}
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok || userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req MFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.mfa.VerifyCode(r.Context(), userID, req.Code); err != nil {
		h.respondError(w, http.StatusUnauthorized, ErrMFACodeInvalid.Error())
		return
	}
	codes, err := h.mfa.RegenerateRecoveryCodes(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string][]string{"recovery_codes": codes})
}

func (h *AuthHandler) handleMFALogin(w http.ResponseWriter, r *http.Request) {
	var req MFALoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ip := r.RemoteAddr
	ua := r.UserAgent()
	resp, err := h.service.CompleteMFALogin(r.Context(), req, &ua, &ip)
	switch {
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrMFACodeInvalid), errors.Is(err, ErrMFANotEnrolled):
		h.respondError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		return
	case errors.Is(err, ErrInactiveUser):
		h.respondError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		return
	case err != nil:
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, resp)
}
