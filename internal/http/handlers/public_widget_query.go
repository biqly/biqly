package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/biqly/biqly/internal/dashboard"
)

// PublicWidgetQueryHandler executes the stored logical query of one widget of
// a publicly shared dashboard. No query input is accepted from the visitor.
type PublicWidgetQueryHandler struct {
	resolver   *dashboard.PublicResolver
	runner     internalQueryRunner
	killSwitch workspaceSharingChecker
	cache      *redis.Client
	cacheTTL   time.Duration
}

// NewPublicWidgetQueryHandler creates a PublicWidgetQueryHandler.
func NewPublicWidgetQueryHandler(resolver *dashboard.PublicResolver, runner internalQueryRunner, killSwitch workspaceSharingChecker, cache *redis.Client, cacheTTL time.Duration) *PublicWidgetQueryHandler {
	return &PublicWidgetQueryHandler{resolver: resolver, runner: runner, killSwitch: killSwitch, cache: cache, cacheTTL: cacheTTL}
}

func (*PublicWidgetQueryHandler) cacheKey(token, widgetID string) string {
	return "pubshare:wq:" + dashboard.HashShareToken(token)[:32] + ":" + widgetID
}

// Run handles POST /api/public/widget-query/{token}/{widgetID}. Every failure
// mode that could reveal whether a token exists returns the same 404, so the
// endpoint leaks nothing about token validity or sharing policy. No part of the
// request body is read: the executed query is the one stored on the dashboard.
func (h *PublicWidgetQueryHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token, ok := requireURLParam(w, r, "token")
	if !ok {
		return
	}
	widgetID, ok := requireURLParam(w, r, "widgetID")
	if !ok {
		return
	}

	wq, err := h.resolver.ResolveWidgetQuery(ctx, token, widgetID)
	if err != nil {
		if errors.Is(err, dashboard.ErrShareNotFound) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to resolve widget", err)
		return
	}
	enabled, err := h.killSwitch.WorkspacePublicSharingEnabled(ctx, wq.WorkspaceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to check sharing policy", err)
		return
	}
	if !enabled {
		writeEntityNotFound(w, "dashboard")
		return
	}

	// Short-TTL cache shields the customer datasource from anonymous traffic.
	if h.cache != nil {
		if raw, err := h.cache.Get(ctx, h.cacheKey(token, widgetID)).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// raw is a JSON result payload we serialized ourselves before caching
			// (sonic.Marshal of *query.Result); it is not visitor-controlled, and
			// the Content-Type header plus the nosniff security header prevent
			// content sniffing.
			_, _ = w.Write(raw) //nolint:gosec // trusted cached JSON, served as application/json
			return
		}
	}

	result, se := h.runner.RunWithModel(ctx, wq.LogicalQuery, nil)
	if se != nil {
		writeServiceError(ctx, w, se)
		return
	}
	if h.cache != nil && result != nil && result.Result != nil {
		if payload, err := sonic.Marshal(result.Result); err == nil {
			_ = h.cache.Set(ctx, h.cacheKey(token, widgetID), payload, h.cacheTTL).Err()
		}
	}
	writeJSON(w, http.StatusOK, result.Result)
}
