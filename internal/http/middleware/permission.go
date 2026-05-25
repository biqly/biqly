package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

const RoleSuperAdmin = "super_admin"

// AuthClient calls the auth service /internal/auth endpoints.
type AuthClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client

	permMu    sync.RWMutex
	permCache map[string]permEntry

	dsMu    sync.RWMutex
	dsCache map[string]dsEntry

	cacheTTL time.Duration
}

type permEntry struct {
	allowed bool
	at      time.Time
}

type dsEntry struct {
	allowed bool
	at      time.Time
}

func NewAuthClient(baseURL, internalToken string) *AuthClient {
	return &AuthClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		permCache:     make(map[string]permEntry),
		dsCache:       make(map[string]dsEntry),
		cacheTTL:      5 * time.Minute,
	}
}

func (c *AuthClient) CheckPermission(ctx context.Context, userID, permission, scopeType, scopeID string) (bool, error) {
	cacheKey := fmt.Sprintf("%s:%s:%s:%s", userID, permission, scopeType, scopeID)

	c.permMu.RLock()
	if e, ok := c.permCache[cacheKey]; ok && time.Since(e.at) < c.cacheTTL {
		c.permMu.RUnlock()
		return e.allowed, nil
	}
	c.permMu.RUnlock()

	body, err := json.Marshal(map[string]string{
		"user_id":    userID,
		"permission": permission,
		"scope_type": scopeType,
		"scope_id":   scopeID,
	})
	if err != nil {
		return false, err
	}

	allowed, err := c.postBool(ctx, "/internal/auth/check-permission", body)
	if err != nil {
		return false, err
	}

	c.permMu.Lock()
	c.permCache[cacheKey] = permEntry{allowed: allowed, at: time.Now()}
	c.permMu.Unlock()

	return allowed, nil
}

func (c *AuthClient) CheckDatasourceAccess(ctx context.Context, userID, datasourceID, level string) (bool, error) {
	cacheKey := fmt.Sprintf("%s:%s:%s", userID, datasourceID, level)

	c.dsMu.RLock()
	if e, ok := c.dsCache[cacheKey]; ok && time.Since(e.at) < c.cacheTTL {
		c.dsMu.RUnlock()
		return e.allowed, nil
	}
	c.dsMu.RUnlock()

	body, err := json.Marshal(map[string]string{
		"user_id":       userID,
		"datasource_id": datasourceID,
		"level":         level,
	})
	if err != nil {
		return false, err
	}

	allowed, err := c.postBool(ctx, "/internal/auth/check-datasource-access", body)
	if err != nil {
		return false, err
	}

	c.dsMu.Lock()
	c.dsCache[cacheKey] = dsEntry{allowed: allowed, at: time.Now()}
	c.dsMu.Unlock()

	return allowed, nil
}

func (c *AuthClient) ListUserDatasources(ctx context.Context, userID string) ([]string, error) {
	url := fmt.Sprintf("%s/internal/auth/user/%s/datasources", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list datasources: status %d", resp.StatusCode)
	}

	var body struct {
		DatasourceIDs []string `json:"datasource_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.DatasourceIDs, nil
}

func (c *AuthClient) postBool(ctx context.Context, path string, body []byte) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("auth service returned %d", resp.StatusCode)
	}

	var result struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.Allowed, nil
}

// RequirePermission checks if the user has the given permission.
// super_admin role bypasses the check.
func RequirePermission(client *AuthClient, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if HasRole(r.Context(), RoleSuperAdmin) {
				next.ServeHTTP(w, r)
				return
			}

			userID := UserID(r.Context())
			if userID == "" {
				writeAuthError(w, http.StatusUnauthorized, "no user in context")
				return
			}

			workspaceID := WorkspaceID(r.Context())
			allowed, err := client.CheckPermission(r.Context(), userID, permission, "workspace", workspaceID)
			if err != nil {
				writeAuthError(w, http.StatusServiceUnavailable, "permission check failed")
				return
			}
			if !allowed {
				writeAuthError(w, http.StatusForbidden, "permission denied: "+permission)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireDatasourceAccess checks if the user has the given access level on the
// datasource identified by URL param `datasourceID` (or `id`) or "datasource_id"
// in JSON body. super_admin bypasses.
func RequireDatasourceAccess(client *AuthClient, requiredLevel string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if HasRole(r.Context(), RoleSuperAdmin) {
				next.ServeHTTP(w, r)
				return
			}

			userID := UserID(r.Context())
			if userID == "" {
				writeAuthError(w, http.StatusUnauthorized, "no user in context")
				return
			}

			datasourceID := extractDatasourceID(r)
			if datasourceID == "" {
				writeAuthError(w, http.StatusBadRequest, "datasource id required")
				return
			}

			allowed, err := client.CheckDatasourceAccess(r.Context(), userID, datasourceID, requiredLevel)
			if err != nil {
				writeAuthError(w, http.StatusServiceUnavailable, "datasource access check failed")
				return
			}
			if !allowed {
				writeAuthError(w, http.StatusForbidden, "datasource access denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractDatasourceID(r *http.Request) string {
	if id := chi.URLParam(r, "datasourceID"); id != "" {
		return id
	}
	if id := chi.URLParam(r, "id"); id != "" {
		return id
	}
	return r.URL.Query().Get("datasource_id")
}
