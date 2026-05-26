package auth

import (
	"encoding/json"
	"net/http"
)

// PasswordPolicyResponse is the public-facing shape of the password policy
// returned to the SPA so it can mirror server validation client-side.
type PasswordPolicyResponse struct {
	MinLength      int  `json:"min_length"`
	MaxLength      int  `json:"max_length"`
	RequireUpper   bool `json:"require_upper"`
	RequireLower   bool `json:"require_lower"`
	RequireDigit   bool `json:"require_digit"`
	RequireSpecial bool `json:"require_special"`
	MinScore       int  `json:"min_score"`
}

func (h *AuthHandler) handlePasswordPolicy(w http.ResponseWriter, r *http.Request) {
	policy := h.config.PasswordPolicy
	resp := PasswordPolicyResponse{
		MinLength:      policy.MinLength,
		MaxLength:      policy.MaxLength,
		RequireUpper:   policy.RequireUpper,
		RequireLower:   policy.RequireLower,
		RequireDigit:   policy.RequireDigit,
		RequireSpecial: policy.RequireSpecial,
		MinScore:       policy.MinScore,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(resp)
}
