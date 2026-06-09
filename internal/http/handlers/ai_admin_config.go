package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/config"
	"github.com/bytedance/sonic"
)

// ambiguityRuntimeConfigKey is the ai_runtime_config row holding ambiguity overrides.
const ambiguityRuntimeConfigKey = "ambiguity"

// ambiguityOverridesTTL bounds cross-replica staleness after an admin PUT;
// the writing replica invalidates immediately, others converge within the TTL.
const ambiguityOverridesTTL = 30 * time.Second

// maxLLMTierPerQuestionLimit caps the admin-settable LLM tier round budget.
const maxLLMTierPerQuestionLimit = 10

// ambiguityOverrides is the DB-managed subset of config.AmbiguityConfig.
// Nil fields fall back to the environment-derived defaults.
type ambiguityOverrides struct {
	TieredEnabled         *bool `json:"tiered_enabled,omitempty"`
	MaxLLMTierPerQuestion *int  `json:"max_llm_tier_per_question,omitempty"`
}

type ambiguityOverridesCache struct {
	mu        sync.Mutex
	overrides ambiguityOverrides
	expires   time.Time
}

// loadAmbiguityOverrides returns the cached DB overrides, refreshing once per
// TTL window. Errors degrade to "no overrides" so query handling never fails
// on the config path.
func (h *AIHandler) loadAmbiguityOverrides(ctx context.Context) ambiguityOverrides {
	c := &h.ambiguityOverridesCache
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expires) {
		return c.overrides
	}
	ov := ambiguityOverrides{}
	if h.deps != nil && h.deps.MetaRepo != nil {
		raw, err := h.deps.MetaRepo.GetAIRuntimeConfig(ctx, ambiguityRuntimeConfigKey)
		switch {
		case err == nil:
			if jsonErr := sonic.Unmarshal(raw, &ov); jsonErr != nil {
				slog.WarnContext(ctx, "decode ambiguity runtime config", "error", jsonErr)
				ov = ambiguityOverrides{}
			}
		case errors.Is(err, sql.ErrNoRows):
			// Key unset — environment defaults apply.
		default:
			slog.WarnContext(ctx, "load ambiguity runtime config", "error", err)
		}
	}
	c.overrides = ov
	c.expires = time.Now().Add(ambiguityOverridesTTL)
	return ov
}

func (h *AIHandler) invalidateAmbiguityOverrides() {
	c := &h.ambiguityOverridesCache
	c.mu.Lock()
	c.expires = time.Time{}
	c.mu.Unlock()
}

// effectiveAmbiguityConfig overlays DB-managed overrides onto the
// environment-derived ambiguity config.
func (h *AIHandler) effectiveAmbiguityConfig(ctx context.Context) config.AmbiguityConfig {
	cfg := h.deps.Config.AI.Ambiguity
	ov := h.loadAmbiguityOverrides(ctx)
	if ov.TieredEnabled != nil {
		cfg.TieredEnabled = *ov.TieredEnabled
	}
	if ov.MaxLLMTierPerQuestion != nil {
		cfg.MaxLLMTierPerQuestion = *ov.MaxLLMTierPerQuestion
	}
	return cfg
}

// adminAmbiguityConfig is the wire shape of the admin-tunable ambiguity knobs.
type adminAmbiguityConfig struct {
	TieredEnabled         bool `json:"tiered_enabled"`
	MaxLLMTierPerQuestion int  `json:"max_llm_tier_per_question"`
	// DBOverride reports whether the values come from the database rather
	// than the environment defaults.
	DBOverride bool `json:"db_override"`
}

type adminRuntimeConfigResponse struct {
	Ambiguity adminAmbiguityConfig `json:"ambiguity"`
}

func (h *AIHandler) adminRuntimeConfigResponse(ctx context.Context) adminRuntimeConfigResponse {
	ov := h.loadAmbiguityOverrides(ctx)
	eff := h.effectiveAmbiguityConfig(ctx)
	return adminRuntimeConfigResponse{
		Ambiguity: adminAmbiguityConfig{
			TieredEnabled:         eff.TieredEnabled,
			MaxLLMTierPerQuestion: eff.MaxLLMTierPerQuestion,
			DBOverride:            ov.TieredEnabled != nil || ov.MaxLLMTierPerQuestion != nil,
		},
	}
}

// AdminRuntimeConfig returns the effective admin-tunable AI runtime config.
func (h *AIHandler) AdminRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.adminRuntimeConfigResponse(r.Context()))
}

type adminRuntimeConfigUpdateRequest struct {
	Ambiguity ambiguityOverrides `json:"ambiguity"`
}

// UpdateAdminRuntimeConfig persists ambiguity overrides and refreshes the cache.
func (h *AIHandler) UpdateAdminRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[adminRuntimeConfigUpdateRequest](w, r)
	if !ok {
		return
	}
	ov := input.Ambiguity
	if ov.TieredEnabled == nil || ov.MaxLLMTierPerQuestion == nil {
		writeError(w, http.StatusBadRequest, "ambiguity.tiered_enabled and ambiguity.max_llm_tier_per_question are required")
		return
	}
	if *ov.MaxLLMTierPerQuestion < 0 || *ov.MaxLLMTierPerQuestion > maxLLMTierPerQuestionLimit {
		writeError(w, http.StatusBadRequest, "ambiguity.max_llm_tier_per_question must be between 0 and 10")
		return
	}
	raw, err := sonic.Marshal(ov)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to encode runtime config", err)
		return
	}
	ctx := r.Context()
	if err := h.deps.MetaRepo.UpsertAIRuntimeConfig(ctx, ambiguityRuntimeConfigKey, raw); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save runtime config", err)
		return
	}
	h.invalidateAmbiguityOverrides()
	writeJSON(w, http.StatusOK, h.adminRuntimeConfigResponse(ctx))
}
