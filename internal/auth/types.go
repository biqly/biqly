package auth

import "time"

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenResponse struct {
	AccessToken  string   `json:"access_token,omitempty"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	UserID       string   `json:"user_id,omitempty"`
	Email        string   `json:"email,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	MFARequired  bool     `json:"mfa_required,omitempty"`
	MFAToken     string   `json:"mfa_token,omitempty"`
}

type MFALoginRequest struct {
	MFAToken string `json:"mfa_token"`
	Code     string `json:"code"`
}

type MFAEnrollResponse struct {
	Secret        string   `json:"secret"`
	OTPAuthURL    string   `json:"otpauth_url"`
	RecoveryCodes []string `json:"recovery_codes"`
}

type MFAVerifyRequest struct {
	Code string `json:"code"`
}

type MFAStatusResponse struct {
	Enabled    bool       `json:"enabled"`
	Method     string     `json:"method,omitempty"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

type UserResponse struct {
	ID                string    `json:"id"`
	Email             string    `json:"email"`
	Username          *string   `json:"username,omitempty"`
	DisplayName       *string   `json:"display_name,omitempty"`
	AvatarURL         *string   `json:"avatar_url,omitempty"`
	IsActive          bool      `json:"is_active"`
	EmailVerified     bool      `json:"email_verified"`
	ActiveWorkspaceID string    `json:"active_workspace_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type SetActiveWorkspaceRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

type SetActiveWorkspaceResponse struct {
	AccessToken       string `json:"access_token"`
	ActiveWorkspaceID string `json:"active_workspace_id"`
}

type User struct {
	ID            string
	Email         string
	Username      *string
	DisplayName   *string
	AvatarURL     *string
	PasswordHash  *string
	IsActive      bool
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastLoginAt   *time.Time
}

type Session struct {
	ID           string
	UserID       string
	RefreshToken string
	UserAgent    *string
	IPAddress    *string
	CreatedAt     time.Time
	ExpiresAt    time.Time
	RevokedAt    *time.Time
}

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Permission struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	CreatedAt   time.Time `json:"created_at"`
}

type PasskeyInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type UserRoleInfo struct {
	RoleID    string `json:"role_id"`
	RoleName  string `json:"role_name"`
	ScopeType string `json:"scope_type"`
	ScopeID   string `json:"scope_id"`
}
