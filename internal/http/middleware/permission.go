package middleware

import (
	"bytes"
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// maxDatasourceProbeBodyBytes caps how much of a JSON body the middleware will
// buffer to extract `datasource_id`. AI/query payloads stay well under this; we
// just need enough to read the first top-level field reliably.
const (
	maxDatasourceProbeBodyBytes = 1 << 20 // 1 MiB
	maxAuthClientCacheEntries   = 4096
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

	wsDSMu    sync.RWMutex
	wsDSCache map[string]wsDSEntry

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

type wsDSEntry struct {
	ids []string
	at  time.Time
}

var (
	sharedAuthMu      sync.Mutex
	sharedAuthClients = make(map[string]*AuthClient)
)

func NewAuthClient(baseURL, internalToken string) *AuthClient {
	return &AuthClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		permCache:     make(map[string]permEntry),
		dsCache:       make(map[string]dsEntry),
		wsDSCache:     make(map[string]wsDSEntry),
		cacheTTL:      5 * time.Minute,
	}
}

// SharedAuthClient returns a process-wide AuthClient for the given base URL and
// internal token so callers do not allocate a new HTTP client per request.
func SharedAuthClient(baseURL, internalToken string) *AuthClient {
	key := strings.TrimRight(baseURL, "/") + "\x00" + internalToken
	sharedAuthMu.Lock()
	defer sharedAuthMu.Unlock()
	if c, ok := sharedAuthClients[key]; ok {
		return c
	}
	c := NewAuthClient(baseURL, internalToken)
	sharedAuthClients[key] = c
	return c
}

func (c *AuthClient) evictExpiredPermLocked(now time.Time) {
	for k, e := range c.permCache {
		if now.Sub(e.at) >= c.cacheTTL {
			delete(c.permCache, k)
		}
	}
	if len(c.permCache) > maxAuthClientCacheEntries {
		for k := range c.permCache {
			delete(c.permCache, k)
			if len(c.permCache) <= maxAuthClientCacheEntries/2 {
				break
			}
		}
	}
}

func (c *AuthClient) evictExpiredDSLocked(now time.Time) {
	for k, e := range c.dsCache {
		if now.Sub(e.at) >= c.cacheTTL {
			delete(c.dsCache, k)
		}
	}
	if len(c.dsCache) > maxAuthClientCacheEntries {
		for k := range c.dsCache {
			delete(c.dsCache, k)
			if len(c.dsCache) <= maxAuthClientCacheEntries/2 {
				break
			}
		}
	}
}

func (c *AuthClient) evictExpiredWSDSLocked(now time.Time) {
	for k, e := range c.wsDSCache {
		if now.Sub(e.at) >= c.cacheTTL {
			delete(c.wsDSCache, k)
		}
	}
	if len(c.wsDSCache) > maxAuthClientCacheEntries {
		for k := range c.wsDSCache {
			delete(c.wsDSCache, k)
			if len(c.wsDSCache) <= maxAuthClientCacheEntries/2 {
				break
			}
		}
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

	body, err := sonic.ConfigStd.Marshal(map[string]string{
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

	now := time.Now()
	c.permMu.Lock()
	c.permCache[cacheKey] = permEntry{allowed: allowed, at: now}
	c.evictExpiredPermLocked(now)
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

	body, err := sonic.ConfigStd.Marshal(map[string]string{
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

	now := time.Now()
	c.dsMu.Lock()
	c.dsCache[cacheKey] = dsEntry{allowed: allowed, at: now}
	c.evictExpiredDSLocked(now)
	c.dsMu.Unlock()

	return allowed, nil
}

func (c *AuthClient) ListUserDatasources(ctx context.Context, userID string) ([]string, error) {
	return c.fetchDatasourceIDs(ctx, fmt.Sprintf("/internal/auth/user/%s/datasources", userID))
}

// UserAIAccess mirrors rbac.UserAIAccess for internal auth resolution.
type UserAIAccess struct {
	Restricted  bool              `json:"restricted"`
	ModelIDs    []string          `json:"model_ids"`
	ProviderIDs []string          `json:"provider_ids"`
	Preferences map[string]string `json:"preferences"`
}

func (c *AuthClient) UserAIAccess(ctx context.Context, userID string) (*UserAIAccess, error) {
	if userID == "" {
		return nil, nil //nolint:nilnil // empty user id means no AI access override
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf("/internal/auth/user/%s/ai-access", userID), http.NoBody)
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
		return nil, fmt.Errorf("user ai access: status %d", resp.StatusCode)
	}
	var out UserAIAccess
	if err := sonic.ConfigStd.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Preferences == nil {
		out.Preferences = map[string]string{}
	}
	return &out, nil
}

// UserAIPreference is a single per-purpose model choice stored in auth DB.
type UserAIPreference struct {
	Purpose string `json:"purpose"`
	ModelID string `json:"model_id"`
}

func (c *AuthClient) ListUserAIPreferences(ctx context.Context, userID string) ([]UserAIPreference, error) {
	if userID == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf("/internal/auth/user/%s/ai-preferences", userID), http.NoBody)
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
		return nil, fmt.Errorf("list user ai preferences: status %d", resp.StatusCode)
	}
	var out struct {
		Preferences []UserAIPreference `json:"preferences"`
	}
	if err := sonic.ConfigStd.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Preferences, nil
}

func (c *AuthClient) SetUserAIPreference(ctx context.Context, userID, purpose, modelID string) error {
	body, err := sonic.ConfigStd.Marshal(map[string]string{
		"purpose":  purpose,
		"model_id": modelID,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+fmt.Sprintf("/internal/auth/user/%s/ai-preferences", userID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("set user ai preference: status %d", resp.StatusCode)
	}
	return nil
}

func (c *AuthClient) DeleteUserAIPreference(ctx context.Context, userID, purpose string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+fmt.Sprintf("/internal/auth/user/%s/ai-preferences/%s", userID, purpose), http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete user ai preference: status %d", resp.StatusCode)
	}
	return nil
}

// ListWorkspaceDatasources returns the datasource IDs attached to a workspace.
// Cached in-memory per workspace ID with the same TTL as other auth checks.
func (c *AuthClient) ListWorkspaceDatasources(ctx context.Context, workspaceID string) ([]string, error) {
	if workspaceID == "" {
		return nil, nil
	}

	c.wsDSMu.RLock()
	if e, ok := c.wsDSCache[workspaceID]; ok && time.Since(e.at) < c.cacheTTL {
		ids := append([]string(nil), e.ids...)
		c.wsDSMu.RUnlock()
		return ids, nil
	}
	c.wsDSMu.RUnlock()

	ids, err := c.fetchDatasourceIDs(ctx, fmt.Sprintf("/internal/auth/workspaces/%s/datasources", workspaceID))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	c.wsDSMu.Lock()
	c.wsDSCache[workspaceID] = wsDSEntry{ids: append([]string(nil), ids...), at: now}
	c.evictExpiredWSDSLocked(now)
	c.wsDSMu.Unlock()
	return ids, nil
}

// InvalidateWorkspaceDatasourceCache drops the cached datasource list for the
// given workspace. Called when workspace_datasources changes propagate.
func (c *AuthClient) InvalidateWorkspaceDatasourceCache(workspaceID string) {
	c.wsDSMu.Lock()
	delete(c.wsDSCache, workspaceID)
	c.wsDSMu.Unlock()
}

func (c *AuthClient) GetUserEmail(ctx context.Context, userID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/auth/user/"+userID, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Internal-Token", c.internalToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get user email: status %d", resp.StatusCode)
	}

	var body struct {
		Email string `json:"email"`
	}
	if err := sonic.ConfigStd.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.Email, nil
}

func (c *AuthClient) fetchDatasourceIDs(ctx context.Context, path string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, http.NoBody)
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
	if err := sonic.ConfigStd.NewDecoder(resp.Body).Decode(&body); err != nil {
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
	if err := sonic.ConfigStd.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.Allowed, nil
}

// RequirePermission checks if the user has the given permission.
// super_admin role bypasses the check.
//
// When client is nil (auth feature flag disabled), the middleware is a
// pass-through. This lets routers wire permission checks unconditionally and
// rely on the BI_AUTH_ENABLED flag to gate enforcement.
func RequirePermission(client *AuthClient, permission string) func(http.Handler) http.Handler {
	if client == nil {
		return passThrough
	}
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

// passThrough returns a no-op middleware. Used when permission checks are
// disabled (auth feature flag off) so callers can wire middleware
// unconditionally.
func passThrough(next http.Handler) http.Handler { return next }

// RequireDatasourceAccess checks if the user has the given access level on the
// datasource identified by URL param `datasourceID` (or `id`), `datasource_id`
// query string, or a top-level `datasource_id` field in the JSON request body.
// super_admin bypasses. Returns a pass-through when client is nil.
//
// NOTE: For routes carrying table/column IDs instead of direct datasource IDs
// (e.g. /metadata/tables/{id}), permission checks are deferred to the handler level
// (using MetadataHandler.ResolveDatasourceID) since this middleware does not have
// access to the catalog's database repository to resolve those entities.
func RequireDatasourceAccess(client *AuthClient, requiredLevel string) func(http.Handler) http.Handler {
	if client == nil {
		return passThrough
	}
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
	if id := r.URL.Query().Get("datasource_id"); id != "" {
		return id
	}
	return extractDatasourceIDFromBody(r)
}

// extractDatasourceIDFromBody peeks at a JSON request body for a top-level
// `datasource_id` field and restores the body so the downstream handler can
// still decode it. Returns empty string for non-JSON, non-write methods, or
// when the field is absent.
func extractDatasourceIDFromBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return ""
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		return ""
	}
	buf, err := io.ReadAll(io.LimitReader(r.Body, maxDatasourceProbeBodyBytes))
	if err != nil {
		return ""
	}
	// Always restore the body — even when we found nothing — so the handler
	// can still decode it.
	r.Body = io.NopCloser(bytes.NewReader(buf))
	var probe struct {
		DatasourceID string `json:"datasource_id"`
	}
	if err := sonic.ConfigStd.Unmarshal(buf, &probe); err != nil {
		return ""
	}
	return probe.DatasourceID
}
