package dashboard

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

// PublicShare is an anonymous, unguessable share link for one dashboard.
// Only the SHA-256 of the URL token is persisted.
type PublicShare struct {
	ID          string     `json:"id"`
	DashboardID string     `json:"dashboard_id"`
	WorkspaceID string     `json:"workspace_id"`
	TokenHash   string     `json:"-"`
	CreatedBy   string     `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// GenerateShareToken returns a new URL-safe share token. The plaintext is
// only ever shown once, at creation time.
func GenerateShareToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashShareToken mirrors the PAT posture (internal/auth/session.go HashToken):
// sha256 hex, duplicated here because catalog/query never import internal/auth.
func HashShareToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
