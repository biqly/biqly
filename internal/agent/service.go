package agent

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/http/response"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/queue"
)

// Queue is what the agent service needs from the message bus: publish and
// subscribe against the explicit subjects in config.AgentConfig
// (JobSubject/StepSubject/ResultSubject/ErrorSubject).
type Queue interface {
	queue.Publisher
	queue.Consumer
}

// AgentDependencies is the agentic query runner's full dependency graph —
// deliberately smaller than app.Dependencies (the monolith's), exposing
// only what the runtime loop and its internal HTTP server need.
type AgentDependencies struct {
	Config   *config.Config
	MetaRepo *metadata.Repository
	Queue    Queue
	Planner  Planner
	Policy   *PolicyEngine
	Tools    *Registry
	Runtime  *Runtime
	Shadow   *ShadowEvaluator
	Metrics  *observability.Metrics
	Close    func() error

	// Ready flips true once the job consumer has successfully subscribed;
	// /readyz reports 503 until then, so a load balancer/orchestrator never
	// routes traffic (or counts the pod healthy) before the service can
	// actually process a job.
	Ready *atomic.Bool
	// Runs tracks cancel funcs for runs currently executing on this
	// process, so POST /internal/agent/runs/{id}/cancel can stop one. A
	// run absent from Runs may already be terminal, running on a
	// different replica, or unknown — all reported the same way (404).
	Runs *RunRegistry
}

// RunRegistry tracks cancel funcs for in-flight runs on this process.
type RunRegistry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewRunRegistry builds an empty RunRegistry.
func NewRunRegistry() *RunRegistry {
	return &RunRegistry{cancels: make(map[string]context.CancelFunc)}
}

// Register records cancel as the way to stop runID. Callers must Unregister
// once the run finishes, whatever the outcome.
func (r *RunRegistry) Register(runID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[runID] = cancel
}

// Unregister removes runID's tracked cancel func.
func (r *RunRegistry) Unregister(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, runID)
}

// Cancel cancels runID's context if it is currently tracked on this
// process. Returns false when it is not (already terminal, elsewhere, or
// unknown) — the caller reports that as "not found", not as an error.
func (r *RunRegistry) Cancel(runID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cancel, ok := r.cancels[runID]
	if !ok {
		return false
	}
	cancel()
	return true
}

// NewServer builds the agent service's internal HTTP handler: public
// healthz/readyz/metrics, and internal-token-protected run diagnostics.
func NewServer(deps *AgentDependencies) http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if deps.Ready == nil || !deps.Ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Route("/internal/agent", func(r chi.Router) {
		r.Use(internalTokenMiddleware(deps.Config.Security.InternalAPIToken))
		r.Get("/runs/{id}", getRunHandler(deps))
		r.Post("/runs/{id}/cancel", cancelRunHandler(deps))
	})

	return r
}

// internalTokenMiddleware gates diagnostics/cancel behind the shared
// internal API token. Duplicated (not imported) from
// internal/http/handlers.InternalTokenMiddleware — internal/agent must stay
// free of that import: internal/http/handlers already depends on
// internal/app, which wires AgentDependencies, so importing it here would
// create an import cycle.
func internalTokenMiddleware(token string) func(http.Handler) http.Handler {
	token = strings.TrimSpace(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				response.WriteJSON(w, http.StatusForbidden, map[string]string{"error": "internal API token is not configured"})
				return
			}
			got := internalTokenFromRequest(r)
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				response.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or missing internal API token"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func internalTokenFromRequest(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-Internal-Token")); token != "" {
		return token
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "bearer "
	if len(authHeader) > len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	return ""
}

type agentRunView struct {
	Run   metadata.AgentRunRow    `json:"run"`
	Steps []metadata.AgentStepRow `json:"steps"`
}

func getRunHandler(deps *AgentDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		run, steps, err := deps.MetaRepo.GetAgentRun(r.Context(), id)
		if err != nil {
			if errors.Is(err, metadata.ErrAgentRunNotFound) {
				response.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
				return
			}
			response.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		response.WriteJSON(w, http.StatusOK, agentRunView{Run: run, Steps: steps})
	}
}

func cancelRunHandler(deps *AgentDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if deps.Runs == nil || !deps.Runs.Cancel(id) {
			response.WriteJSON(w, http.StatusNotFound, map[string]string{
				"error": "run not found or not running on this instance",
			})
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}
