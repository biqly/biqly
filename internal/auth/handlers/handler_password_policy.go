package handlers

import (
	"github.com/bytedance/sonic"
	"net/http"
)

// PasswordPolicyResponse is the public-facing shape of the password policy
// returned to the SPA so it can mirror server validation client-side.
type PasswordPolicyResponse struct {
	MinLength              int  `json:"min_length"`
	MaxLength              int  `json:"max_length"`
	RequireUpper           bool `json:"require_upper"`
	RequireLower           bool `json:"require_lower"`
	RequireDigit           bool `json:"require_digit"`
	RequireSpecial         bool `json:"require_special"`
	MinScore               int  `json:"min_score"`
	SelfSignupEnabled      bool `json:"self_signup_enabled"`
	FirstUserSetupRequired bool `json:"first_user_setup_required"`
	LDAPEnabled            bool `json:"ldap_enabled"`
}

func (h *AuthHandler) handlePasswordPolicy(w http.ResponseWriter, r *http.Request) {
	policy := h.config.PasswordPolicy
	selfSignup := true
	if enabled, err := h.service.SelfSignupEnabled(r.Context()); err == nil {
		selfSignup = enabled
	}
	firstUserSetup := false
	if required, err := h.service.FirstUserSetupRequired(r.Context()); err == nil {
		firstUserSetup = required
	}
	resp := PasswordPolicyResponse{
		MinLength:              policy.MinLength,
		MaxLength:              policy.MaxLength,
		RequireUpper:           policy.RequireUpper,
		RequireLower:           policy.RequireLower,
		RequireDigit:           policy.RequireDigit,
		RequireSpecial:         policy.RequireSpecial,
		MinScore:               policy.MinScore,
		SelfSignupEnabled:      selfSignup,
		FirstUserSetupRequired: firstUserSetup,
		LDAPEnabled:            h.service.LDAPEnabled(r.Context()),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := sonic.ConfigStd.NewEncoder(w).Encode(resp); err != nil {
		return
	}
}
