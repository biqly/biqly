package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/bytedance/sonic"
)

// ai_runtime_config row keys, one per admin-tunable config domain.
const (
	ambiguityRuntimeConfigKey = "ambiguity"
	piiRuntimeConfigKey       = "pii"
	memoryRuntimeConfigKey    = "memory"
	queueRuntimeConfigKey     = "queue"
)

// maxLLMTierPerQuestionLimit caps the admin-settable LLM tier round budget.
const maxLLMTierPerQuestionLimit = 10

// maxAmbiguityOptionsLimit caps the admin-settable clarification option count.
const maxAmbiguityOptionsLimit = 10

// maxMemoryRecallLimit caps the admin-settable recalled few-shot examples.
const maxMemoryRecallLimit = 10

// maxQueueConcurrencyLimit caps the admin-settable concurrent job limit.
const maxQueueConcurrencyLimit = 10

// Wire field sources reported per knob so the UI can badge where a value
// comes from. "environment" covers both explicit env vars and code defaults —
// the loader cannot distinguish them after startup.
const (
	configSourceDatabase    = "database"
	configSourceEnvironment = "environment"
)

// ambiguityOverrides is the DB-managed subset of config.AmbiguityConfig.
// Nil fields fall back to the environment-derived defaults.
type ambiguityOverrides struct {
	CheckEnabled          *bool    `json:"check_enabled,omitempty"`
	ConfidenceThreshold   *float64 `json:"confidence_threshold,omitempty"`
	MaxOptions            *int     `json:"max_options,omitempty"`
	TieredEnabled         *bool    `json:"tiered_enabled,omitempty"`
	MaxLLMTierPerQuestion *int     `json:"max_llm_tier_per_question,omitempty"`
}

// piiOverrides is the DB-managed subset of config.PIIConfig. The BI_PII_ENABLED
// master switch is deliberately env-only: PII protection cannot be disabled at
// runtime through the admin API.
type piiOverrides struct {
	DetectionThreshold *float64 `json:"detection_threshold,omitempty"`
}

// memoryOverrides is the DB-managed subset of config.AIMemoryConfig.
type memoryOverrides struct {
	RecallEnabled *bool `json:"recall_enabled,omitempty"`
	RecallLimit   *int  `json:"recall_limit,omitempty"`
}

// queueOverrides is the DB-managed subset of the NATS config's concurrency.
type queueOverrides struct {
	Concurrency *int `json:"concurrency,omitempty"`
}

func (h *AIHandler) loadAmbiguityOverrides(ctx context.Context) ambiguityOverrides {
	return h.ambiguityOverridesCache.load(ctx, h.metaRepo(), ambiguityRuntimeConfigKey)
}

func (h *AIHandler) loadMemoryOverrides(ctx context.Context) memoryOverrides {
	return h.memoryOverridesCache.load(ctx, h.metaRepo(), memoryRuntimeConfigKey)
}

func (h *AIHandler) loadQueueOverrides(ctx context.Context) queueOverrides {
	return h.queueOverridesCache.load(ctx, h.metaRepo(), queueRuntimeConfigKey)
}

func (h *AIHandler) EffectiveConcurrency(ctx context.Context) int {
	limit := 1
	if h.deps != nil && h.deps.Config != nil {
		limit = h.deps.Config.NATS.Concurrency
	}
	if limit <= 0 {
		limit = 1
	}
	ov := h.loadQueueOverrides(ctx)
	if ov.Concurrency != nil {
		return *ov.Concurrency
	}
	return limit
}

func (h *AIHandler) metaRepo() *metadata.Repository {
	if h.deps == nil {
		return nil
	}
	return h.deps.MetaRepo
}

// effectiveAmbiguityConfig overlays DB-managed overrides onto the
// environment-derived ambiguity config.
func (h *AIHandler) effectiveAmbiguityConfig(ctx context.Context) config.AmbiguityConfig {
	cfg := h.deps.Config.AI.Ambiguity
	ov := h.loadAmbiguityOverrides(ctx)
	if ov.CheckEnabled != nil {
		cfg.CheckEnabled = *ov.CheckEnabled
	}
	if ov.ConfidenceThreshold != nil {
		cfg.ConfidenceThreshold = *ov.ConfidenceThreshold
	}
	if ov.MaxOptions != nil {
		cfg.MaxOptions = *ov.MaxOptions
	}
	if ov.TieredEnabled != nil {
		cfg.TieredEnabled = *ov.TieredEnabled
	}
	if ov.MaxLLMTierPerQuestion != nil {
		cfg.MaxLLMTierPerQuestion = *ov.MaxLLMTierPerQuestion
	}
	return cfg
}

// effectiveMemoryConfig overlays DB-managed overrides onto the
// environment-derived memory recall config. A missing Config (test fixtures)
// and a zero RecallLimit both fall back to the few-shot prompt cap.
func (h *AIHandler) effectiveMemoryConfig(ctx context.Context) config.AIMemoryConfig {
	cfg := config.AIMemoryConfig{RecallEnabled: true, RecallLimit: fewShotLimit}
	if h.deps != nil && h.deps.Config != nil {
		cfg = h.deps.Config.AI.Memory
		if cfg.RecallLimit <= 0 {
			cfg.RecallLimit = fewShotLimit
		}
	}
	ov := h.loadMemoryOverrides(ctx)
	if ov.RecallEnabled != nil {
		cfg.RecallEnabled = *ov.RecallEnabled
	}
	if ov.RecallLimit != nil {
		cfg.RecallLimit = *ov.RecallLimit
	}
	return cfg
}

// effectivePIIConfig overlays DB-managed overrides onto the environment-derived
// PII config. It reads through to the database on every call: PII scans are
// rare, admin-triggered operations, and the catalog service has no shared
// handler state with the AI admin API that writes the overrides.
func effectivePIIConfig(ctx context.Context, repo *metadata.Repository, base config.PIIConfig) config.PIIConfig {
	ov := fetchRuntimeOverrides[piiOverrides](ctx, repo, piiRuntimeConfigKey)
	if ov.DetectionThreshold != nil {
		base.DetectionThreshold = *ov.DetectionThreshold
	}
	return base
}

// adminAmbiguityConfig is the wire shape of the admin-tunable ambiguity knobs.
type adminAmbiguityConfig struct {
	CheckEnabled          bool    `json:"check_enabled"`
	ConfidenceThreshold   float64 `json:"confidence_threshold"`
	MaxOptions            int     `json:"max_options"`
	TieredEnabled         bool    `json:"tiered_enabled"`
	MaxLLMTierPerQuestion int     `json:"max_llm_tier_per_question"`
	// DBOverride reports whether any value comes from the database rather
	// than the environment defaults.
	DBOverride bool   `json:"db_override"`
	Source     string `json:"source"` // "environment" | "database"
	// Sources maps each knob to where its effective value comes from.
	Sources map[string]string `json:"sources"`
}

// adminPIIConfig is the wire shape of the admin-tunable PII knobs. Enabled is
// reported read-only so the UI can show the env-managed master switch.
type adminPIIConfig struct {
	Enabled            bool              `json:"enabled"`
	DetectionThreshold float64           `json:"detection_threshold"`
	DBOverride         bool              `json:"db_override"`
	Source             string            `json:"source"`
	Sources            map[string]string `json:"sources"`
}

// adminMemoryConfig is the wire shape of the admin-tunable memory recall knobs.
type adminMemoryConfig struct {
	RecallEnabled bool              `json:"recall_enabled"`
	RecallLimit   int               `json:"recall_limit"`
	DBOverride    bool              `json:"db_override"`
	Source        string            `json:"source"`
	Sources       map[string]string `json:"sources"`
}

// adminQueueConfig is the wire shape of the admin-tunable queue knobs.
type adminQueueConfig struct {
	Concurrency int               `json:"concurrency"`
	DBOverride  bool              `json:"db_override"`
	Source      string            `json:"source"`
	Sources     map[string]string `json:"sources"`
}

type adminRuntimeConfigResponse struct {
	Ambiguity adminAmbiguityConfig `json:"ambiguity"`
	PII       adminPIIConfig       `json:"pii"`
	Memory    adminMemoryConfig    `json:"memory"`
	Queue     adminQueueConfig     `json:"queue"`
}

func fieldSource(overridden bool) string {
	if overridden {
		return configSourceDatabase
	}
	return configSourceEnvironment
}

func domainSource(sources map[string]string) (string, bool) {
	for _, src := range sources {
		if src == configSourceDatabase {
			return configSourceDatabase, true
		}
	}
	return configSourceEnvironment, false
}

// effectiveAmbiguitySettings is the ambiguity domain of the admin wire shape;
// also embedded in the user-facing /ai/settings response.
func (h *AIHandler) effectiveAmbiguitySettings(ctx context.Context) adminAmbiguityConfig {
	ov := h.loadAmbiguityOverrides(ctx)
	eff := h.effectiveAmbiguityConfig(ctx)
	sources := map[string]string{
		"check_enabled":             fieldSource(ov.CheckEnabled != nil),
		"confidence_threshold":      fieldSource(ov.ConfidenceThreshold != nil),
		"max_options":               fieldSource(ov.MaxOptions != nil),
		"tiered_enabled":            fieldSource(ov.TieredEnabled != nil),
		"max_llm_tier_per_question": fieldSource(ov.MaxLLMTierPerQuestion != nil),
	}
	source, dbOverride := domainSource(sources)
	return adminAmbiguityConfig{
		CheckEnabled:          eff.CheckEnabled,
		ConfidenceThreshold:   eff.ConfidenceThreshold,
		MaxOptions:            eff.MaxOptions,
		TieredEnabled:         eff.TieredEnabled,
		MaxLLMTierPerQuestion: eff.MaxLLMTierPerQuestion,
		DBOverride:            dbOverride,
		Source:                source,
		Sources:               sources,
	}
}

func (h *AIHandler) adminRuntimeConfigResponse(ctx context.Context) adminRuntimeConfigResponse {
	piiBase := h.deps.Config.PII
	piiOv := fetchRuntimeOverrides[piiOverrides](ctx, h.metaRepo(), piiRuntimeConfigKey)
	piiEff := effectivePIIConfig(ctx, h.metaRepo(), piiBase)
	piiSources := map[string]string{
		"detection_threshold": fieldSource(piiOv.DetectionThreshold != nil),
	}
	piiSource, piiOverride := domainSource(piiSources)

	memOv := h.loadMemoryOverrides(ctx)
	memEff := h.effectiveMemoryConfig(ctx)
	memSources := map[string]string{
		"recall_enabled": fieldSource(memOv.RecallEnabled != nil),
		"recall_limit":   fieldSource(memOv.RecallLimit != nil),
	}
	memSource, memOverride := domainSource(memSources)

	queueOv := h.loadQueueOverrides(ctx)
	queueEff := h.EffectiveConcurrency(ctx)
	queueSources := map[string]string{
		"concurrency": fieldSource(queueOv.Concurrency != nil),
	}
	queueSource, queueOverride := domainSource(queueSources)

	return adminRuntimeConfigResponse{
		Ambiguity: h.effectiveAmbiguitySettings(ctx),
		PII: adminPIIConfig{
			Enabled:            piiBase.Enabled,
			DetectionThreshold: piiEff.DetectionThreshold,
			DBOverride:         piiOverride,
			Source:             piiSource,
			Sources:            piiSources,
		},
		Memory: adminMemoryConfig{
			RecallEnabled: memEff.RecallEnabled,
			RecallLimit:   memEff.RecallLimit,
			DBOverride:    memOverride,
			Source:        memSource,
			Sources:       memSources,
		},
		Queue: adminQueueConfig{
			Concurrency: queueEff,
			DBOverride:  queueOverride,
			Source:      queueSource,
			Sources:     queueSources,
		},
	}
}

// AdminRuntimeConfig returns the effective admin-tunable AI runtime config.
func (h *AIHandler) AdminRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.adminRuntimeConfigResponse(r.Context()))
}

// adminRuntimeConfigUpdateRequest carries per-domain override documents.
// Each provided domain REPLACES that domain's stored override row: fields
// omitted within a provided domain fall back to environment defaults, and an
// empty object clears every override for the domain.
type adminRuntimeConfigUpdateRequest struct {
	Ambiguity json.RawMessage `json:"ambiguity,omitempty"`
	PII       json.RawMessage `json:"pii,omitempty"`
	Memory    json.RawMessage `json:"memory,omitempty"`
	Queue     json.RawMessage `json:"queue,omitempty"`
}

// strictUnmarshalJSON decodes data into v, rejecting unknown fields so typos
// in admin payloads surface as 400s instead of silently doing nothing.
func strictUnmarshalJSON(data []byte, v any) error {
	dec := sonic.ConfigStd.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func validateAmbiguityOverrides(ov ambiguityOverrides) string {
	if ov.ConfidenceThreshold != nil && (*ov.ConfidenceThreshold < 0 || *ov.ConfidenceThreshold > 1) {
		return "ambiguity.confidence_threshold must be between 0.0 and 1.0"
	}
	if ov.MaxOptions != nil && (*ov.MaxOptions < 1 || *ov.MaxOptions > maxAmbiguityOptionsLimit) {
		return "ambiguity.max_options must be between 1 and 10"
	}
	if ov.MaxLLMTierPerQuestion != nil && (*ov.MaxLLMTierPerQuestion < 0 || *ov.MaxLLMTierPerQuestion > maxLLMTierPerQuestionLimit) {
		return "ambiguity.max_llm_tier_per_question must be between 0 and 10"
	}
	return ""
}

func validatePIIOverrides(ov piiOverrides) string {
	if ov.DetectionThreshold != nil && (*ov.DetectionThreshold <= 0 || *ov.DetectionThreshold > 1) {
		return "pii.detection_threshold must be greater than 0.0 and at most 1.0"
	}
	return ""
}

func validateMemoryOverrides(ov memoryOverrides) string {
	if ov.RecallLimit != nil && (*ov.RecallLimit < 1 || *ov.RecallLimit > maxMemoryRecallLimit) {
		return "memory.recall_limit must be between 1 and 10"
	}
	return ""
}

func validateQueueOverrides(ov queueOverrides) string {
	if ov.Concurrency != nil && (*ov.Concurrency < 1 || *ov.Concurrency > maxQueueConcurrencyLimit) {
		return "queue.concurrency must be between 1 and 10"
	}
	return ""
}

// decodeDomainOverrides strict-decodes one domain document and validates its
// ranges, returning a field-specific error message on failure.
func decodeDomainOverrides[T any](domain string, raw json.RawMessage, validate func(T) string) (T, string) {
	var ov T
	if err := strictUnmarshalJSON(raw, &ov); err != nil {
		return ov, domain + " contains an unknown or malformed field"
	}
	if msg := validate(ov); msg != "" {
		return ov, msg
	}
	return ov, ""
}

// upsertRuntimeConfigDomain persists one normalized domain row and returns the
// previous raw value for audit logging ("" when the row did not exist).
func (h *AIHandler) upsertRuntimeConfigDomain(ctx context.Context, key string, ov any) (oldValue, newValue string, err error) {
	raw, err := sonic.Marshal(ov)
	if err != nil {
		return "", "", err
	}
	prev, prevErr := h.deps.MetaRepo.GetAIRuntimeConfig(ctx, key)
	switch {
	case prevErr == nil:
		oldValue = string(prev)
	case !errors.Is(prevErr, sql.ErrNoRows):
		slog.WarnContext(ctx, "load previous runtime config for audit", "key", key, "error", prevErr)
	}
	if upErr := h.deps.MetaRepo.UpsertAIRuntimeConfig(ctx, key, raw); upErr != nil {
		return "", "", upErr
	}
	return oldValue, string(raw), nil
}

// UpdateAdminRuntimeConfig persists the provided domain overrides, refreshes
// the caches, audit-logs the change, and echoes the new effective config.
func (h *AIHandler) UpdateAdminRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	body, ok := readRequestBody(w, r)
	if !ok {
		return
	}
	var input adminRuntimeConfigUpdateRequest
	if err := strictUnmarshalJSON(body, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: unknown or malformed field")
		return
	}
	if input.Ambiguity == nil && input.PII == nil && input.Memory == nil && input.Queue == nil {
		writeError(w, http.StatusBadRequest, "at least one config domain (ambiguity, pii, memory, queue) is required")
		return
	}

	type domainUpdate struct {
		key string
		ov  any
	}
	updates := make([]domainUpdate, 0, 4)
	if input.Ambiguity != nil {
		ov, msg := decodeDomainOverrides("ambiguity", input.Ambiguity, validateAmbiguityOverrides)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		updates = append(updates, domainUpdate{key: ambiguityRuntimeConfigKey, ov: ov})
	}
	if input.PII != nil {
		ov, msg := decodeDomainOverrides("pii", input.PII, validatePIIOverrides)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		updates = append(updates, domainUpdate{key: piiRuntimeConfigKey, ov: ov})
	}
	if input.Memory != nil {
		ov, msg := decodeDomainOverrides("memory", input.Memory, validateMemoryOverrides)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		updates = append(updates, domainUpdate{key: memoryRuntimeConfigKey, ov: ov})
	}
	if input.Queue != nil {
		ov, msg := decodeDomainOverrides("queue", input.Queue, validateQueueOverrides)
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		updates = append(updates, domainUpdate{key: queueRuntimeConfigKey, ov: ov})
	}

	ctx := r.Context()
	changes := make(map[string]any, len(updates))
	for _, u := range updates {
		oldValue, newValue, err := h.upsertRuntimeConfigDomain(ctx, u.key, u.ov)
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save runtime config", err)
			return
		}
		changes[u.key] = map[string]any{"old": oldValue, "new": newValue}
	}
	h.ambiguityOverridesCache.invalidate()
	h.memoryOverridesCache.invalidate()
	h.queueOverridesCache.invalidate()

	if h.deps.AuditLogger != nil {
		h.deps.AuditLogger.Log(ctx, audit.Event{
			UserID:    bimw.UserID(ctx),
			EventType: audit.EventAIConfigUpdated,
			Details:   changes,
		})
	}
	writeJSON(w, http.StatusOK, h.adminRuntimeConfigResponse(ctx))
}

// effectivePIIScanSettings resolves the PII scan threshold and sample limit
// from package defaults, environment config, and DB runtime overrides.
func effectivePIIScanSettings(ctx context.Context, repo *metadata.Repository, cfg *config.Config) (threshold float64, sampleLimit int) {
	threshold = pii.DefaultThreshold
	sampleLimit = pii.DefaultSampleLimit
	if cfg == nil {
		return threshold, sampleLimit
	}
	eff := effectivePIIConfig(ctx, repo, cfg.PII)
	if eff.DetectionThreshold > 0 {
		threshold = eff.DetectionThreshold
	}
	if eff.SampleDataLimit > 0 {
		sampleLimit = eff.SampleDataLimit
	}
	return threshold, sampleLimit
}
