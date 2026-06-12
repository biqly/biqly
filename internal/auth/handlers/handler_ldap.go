package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/auth"
)

// ldapConfigRequest is the admin payload for the LDAP configuration. The bind
// password is write-only: send a non-empty value to set it, or leave it empty
// to keep the stored one.
type ldapConfigRequest struct {
	Enabled         bool   `json:"enabled"`
	AutoCreateUsers bool   `json:"auto_create_users"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Security        string `json:"security"`
	SkipTLSVerify   bool   `json:"skip_tls_verify"`
	BindDN          string `json:"bind_dn"`
	BindPassword    string `json:"bind_password"`
	BaseDN          string `json:"base_dn"`
	UserFilter      string `json:"user_filter"`
	EmailAttr       string `json:"email_attr"`
	DisplayNameAttr string `json:"display_name_attr"`
}

func (req ldapConfigRequest) toConfig() auth.LDAPConfig {
	return auth.LDAPConfig{
		Enabled:         req.Enabled,
		AutoCreateUsers: req.AutoCreateUsers,
		Host:            req.Host,
		Port:            req.Port,
		Security:        req.Security,
		SkipTLSVerify:   req.SkipTLSVerify,
		BindDN:          req.BindDN,
		BindPassword:    req.BindPassword,
		BaseDN:          req.BaseDN,
		UserFilter:      req.UserFilter,
		EmailAttr:       req.EmailAttr,
		DisplayNameAttr: req.DisplayNameAttr,
	}
}

// requireSuperAdmin returns the caller's user id when they are a super admin,
// or writes the appropriate error response and returns ("", false).
func (h *AuthHandler) requireSuperAdmin(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return "", false
	}
	isSuper, err := h.service.IsSuperAdmin(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return "", false
	}
	if !isSuper {
		h.respondError(w, http.StatusForbidden, auth.ErrNotSuperAdmin.Error())
		return "", false
	}
	return userID, true
}

func (h *AuthHandler) handleAdminGetLDAPConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireSuperAdmin(w, r); !ok {
		return
	}
	cfg, err := h.service.GetLDAPConfig(r.Context())
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, cfg)
}

func (h *AuthHandler) handleAdminUpdateLDAPConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireSuperAdmin(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[ldapConfigRequest](w, r)
	if !ok {
		return
	}
	cfg, err := h.service.UpdateLDAPConfig(r.Context(), req.toConfig(), userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, cfg)
}

func (h *AuthHandler) handleAdminTestLDAP(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireSuperAdmin(w, r); !ok {
		return
	}
	req, ok := decodeJSON[ldapConfigRequest](w, r)
	if !ok {
		return
	}
	if err := h.service.TestLDAPConnection(r.Context(), req.toConfig()); err != nil {
		h.respondJSON(w, http.StatusOK, map[string]any{"status": "error", "message": err.Error()})
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
