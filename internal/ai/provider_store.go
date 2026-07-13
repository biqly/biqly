package ai

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/platform/db/pgarray"
	"github.com/biqly/biqly/internal/security"
)

// Purpose identifies which AI workload a model serves. Each purpose can have at
// most one default model (enforced by a partial unique index), and the
// ProviderStore resolves the active provider/model per purpose at runtime.
type Purpose string

const (
	// PurposeQuery — NL → LogicalQuery generation.
	PurposeQuery Purpose = "query"

	// minQueryGenerationMaxTokens avoids truncated JSON when reasoning models fill
	// the completion budget before emitting the LogicalQuery payload.
	minQueryGenerationMaxTokens = 8192
	// PurposeDescribe — table/column AI description generation.
	PurposeDescribe Purpose = "describe"
	// PurposeEmbedding — table/column embedding generation.
	PurposeEmbedding Purpose = "embedding"
	// PurposeTranslation — metadata description translation.
	PurposeTranslation Purpose = "translation"
	// PurposeJudge — LLM-assisted evaluation judging.
	PurposeJudge Purpose = "judge"
	// PurposeAgent — web agent planner/finalizer (multi-step tool loop).
	// Falls back to PurposeQuery when no agent model is configured.
	PurposeAgent Purpose = "agent"
)

// AllPurposes lists every valid purpose in display order.
var AllPurposes = []Purpose{PurposeQuery, PurposeDescribe, PurposeEmbedding, PurposeTranslation, PurposeJudge, PurposeAgent}

// ValidPurpose reports whether p is a recognized purpose.
func ValidPurpose(p string) bool {
	switch Purpose(p) {
	case PurposeQuery, PurposeDescribe, PurposeEmbedding, PurposeTranslation, PurposeJudge, PurposeAgent:
		return true
	default:
		return false
	}
}

// UserSelectablePurposes is the subset of purposes a user may set a personal
// model preference for. Only purposes that are (a) actually resolved per-user
// at runtime and (b) safe to vary per user are exposed. embedding/translation
// write shared metadata/vectors (must stay consistent) and judge is an
// admin-only evaluation concern — those remain admin/global-controlled.
var UserSelectablePurposes = []Purpose{PurposeQuery, PurposeDescribe}

// UserSelectablePurpose reports whether a user may set a personal preference
// for purpose p.
func UserSelectablePurpose(p string) bool {
	switch Purpose(p) {
	case PurposeQuery, PurposeDescribe:
		return true
	case PurposeEmbedding, PurposeTranslation, PurposeJudge, PurposeAgent:
		return false
	default:
		return false
	}
}

// ValidProviderType reports whether t is a supported provider backend.
func ValidProviderType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "openai", "openai-compatible", "anthropic":
		return true
	default:
		return false
	}
}

// DefaultBaseURLForType returns the canonical base URL for a provider type, or
// "" for openai-compatible (Ollama, llama-server) where the operator must
// supply the host.
func DefaultBaseURLForType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	default:
		return ""
	}
}

// resolvedModel is the flattened provider+model configuration cached per purpose.
type resolvedModel struct {
	ProviderType        string
	BaseURL             string
	APIKey              string
	ModelID             string
	DisplayName         string
	MaxTokens           int
	Temperature         float64
	TopP                float64
	NumCtx              int
	HTTPTimeoutSeconds  int
	MaxPromptInputRunes int
}

// ProviderStore is the runtime source of truth for AI provider/model
// selection. It reads the ai_providers / ai_models tables, decrypts API keys,
// and caches the default model per purpose in memory. Admin mutations call
// RefreshCache so changes take effect without a restart. When the database has
// no default for a purpose, callers fall back to the env-backed AIConfig.
type ProviderStore struct {
	db        *sql.DB
	encryptor *security.Encryption
	fallback  config.AIConfig

	mu       sync.RWMutex
	resolved map[Purpose]*resolvedModel
	version  atomic.Int64
}

// NewProviderStore builds a store over the metadata pool. encryptor may be nil
// (API keys are then stored/read as plaintext, matching the datasource DSN
// fallback). fallback supplies the env-backed configuration used to seed an
// empty database and to resolve purposes with no configured default.
func NewProviderStore(db *sql.DB, encryptor *security.Encryption, fallback *config.AIConfig) *ProviderStore {
	var fb config.AIConfig
	if fallback != nil {
		fb = *fallback
	}
	return &ProviderStore{
		db:        db,
		encryptor: encryptor,
		fallback:  fb,
		resolved:  map[Purpose]*resolvedModel{},
	}
}

// Fallback returns the env-backed configuration the store was built with.
func (s *ProviderStore) Fallback() config.AIConfig { return s.fallback }

// CacheVersion increments whenever the resolved cache is refreshed, letting
// derived providers know they should rebuild.
func (s *ProviderStore) CacheVersion() int64 { return s.version.Load() }

// RefreshCache reloads the default+active model for every purpose from the
// database. It is safe to call concurrently; readers see either the old or new
// snapshot atomically.
func (s *ProviderStore) RefreshCache(ctx context.Context) error {
	const q = `
		SELECT m.purpose, p.provider_type, p.base_url, COALESCE(p.api_key_encrypted, ''),
		       m.model_id, COALESCE(NULLIF(TRIM(m.display_name), ''), m.model_id),
		       m.max_tokens, m.temperature, m.top_p, m.num_ctx,
		       m.max_prompt_input_runes, p.http_timeout_seconds
		FROM ai_models m
		JOIN ai_providers p ON p.id = m.provider_id
		WHERE m.is_default = true AND m.is_active = true AND p.is_active = true`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("load default ai models: %w", err)
	}
	defer func() { _ = rows.Close() }()

	next := make(map[Purpose]*resolvedModel, len(AllPurposes))
	for rows.Next() {
		var (
			purpose, providerType, baseURL, encKey string
			rm                                     resolvedModel
		)
		if err := rows.Scan(&purpose, &providerType, &baseURL, &encKey,
			&rm.ModelID, &rm.DisplayName, &rm.MaxTokens, &rm.Temperature, &rm.TopP, &rm.NumCtx,
			&rm.MaxPromptInputRunes, &rm.HTTPTimeoutSeconds); err != nil {
			return fmt.Errorf("scan default ai model: %w", err)
		}
		rm.ProviderType = providerType
		rm.BaseURL = baseURL
		rm.APIKey = s.decrypt(encKey)
		next[Purpose(purpose)] = &rm
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate default ai models: %w", err)
	}

	s.mu.Lock()
	s.resolved = next
	s.mu.Unlock()
	s.version.Add(1)
	return nil
}

func (s *ProviderStore) resolvedFor(p Purpose) (*resolvedModel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rm, ok := s.resolved[p]
	return rm, ok
}

// HasResolved reports whether a default model is configured for the purpose.
func (s *ProviderStore) HasResolved(p Purpose) bool {
	_, ok := s.resolvedFor(p)
	return ok
}

// ModelLabelForPurpose returns the human-facing model label for telemetry and UI.
// Prefers the DB display name, then model_id, then env fallback for the purpose.
func (s *ProviderStore) ModelLabelForPurpose(p Purpose) string {
	if rm, ok := s.resolvedFor(p); ok {
		if dn := strings.TrimSpace(rm.DisplayName); dn != "" {
			return dn
		}
		if id := strings.TrimSpace(rm.ModelID); id != "" {
			return id
		}
	}
	switch p {
	case PurposeQuery, PurposeAgent:
		return s.fallback.ResolvedQuery().Config.Connection.Model
	case PurposeDescribe, PurposeJudge:
		if m := strings.TrimSpace(s.fallback.Connection.Model); m != "" {
			return m
		}
	case PurposeEmbedding:
		if m := strings.TrimSpace(s.fallback.Embedding.Model); m != "" {
			return m
		}
	case PurposeTranslation:
		if m := strings.TrimSpace(s.fallback.Translation.Model); m != "" {
			return m
		}
	}
	return s.fallback.ResolvedQuery().Config.Connection.Model
}

// ChatConfigForModelUUID loads a single active model row by its metadata UUID.
func (s *ProviderStore) ChatConfigForModelUUID(ctx context.Context, modelUUID string) (config.AIConfig, bool) {
	modelUUID = strings.TrimSpace(modelUUID)
	if modelUUID == "" {
		return config.AIConfig{}, false
	}
	const q = `
		SELECT m.purpose, p.provider_type, p.base_url, COALESCE(p.api_key_encrypted, ''),
		       m.model_id, COALESCE(NULLIF(TRIM(m.display_name), ''), m.model_id),
		       m.max_tokens, m.temperature, m.top_p, m.num_ctx,
		       m.max_prompt_input_runes, p.http_timeout_seconds
		FROM ai_models m
		JOIN ai_providers p ON p.id = m.provider_id
		WHERE m.id = $1::uuid AND m.is_active = true AND p.is_active = true`
	var (
		purpose, providerType, baseURL, encKey string
		rm                                     resolvedModel
	)
	err := s.db.QueryRowContext(ctx, q, modelUUID).Scan(
		&purpose, &providerType, &baseURL, &encKey,
		&rm.ModelID, &rm.DisplayName, &rm.MaxTokens, &rm.Temperature, &rm.TopP, &rm.NumCtx,
		&rm.MaxPromptInputRunes, &rm.HTTPTimeoutSeconds,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return config.AIConfig{}, false
		}
		slog.Warn("load ai model by uuid failed", "model_id", modelUUID, "error", err)
		return config.AIConfig{}, false
	}
	rm.ProviderType = providerType
	rm.BaseURL = baseURL
	rm.APIKey = s.decrypt(encKey)
	cfg := s.chatConfigFromResolved(&rm)
	if Purpose(purpose) == PurposeQuery || Purpose(purpose) == PurposeAgent {
		cfg = ensureQueryMaxTokens(cfg)
	}
	return cfg, true
}

func (s *ProviderStore) chatConfigFromResolved(rm *resolvedModel) config.AIConfig {
	cfg := s.fallback
	cfg.Connection.Provider = rm.ProviderType
	cfg.Connection.Model = rm.ModelID
	cfg.Connection.BaseURL = rm.BaseURL
	cfg.Connection.APIKey = rm.APIKey
	if rm.MaxTokens > 0 {
		cfg.Generation.MaxTokens = rm.MaxTokens
	}
	cfg.Generation.Temperature = rm.Temperature
	cfg.Generation.TopP = rm.TopP
	cfg.Generation.NumCtx = rm.NumCtx
	if rm.HTTPTimeoutSeconds > 0 {
		cfg.Connection.HTTPTimeoutSeconds = rm.HTTPTimeoutSeconds
	}
	if rm.MaxPromptInputRunes > 0 {
		cfg.Generation.MaxPromptInputRunes = rm.MaxPromptInputRunes
	}
	cfg.Query.Provider, cfg.Query.Model, cfg.Query.BaseURL, cfg.Query.APIKey = "", "", "", ""
	cfg.Query.HTTPTimeoutSeconds = 0
	return cfg
}

// ListAllActiveModels returns every active model on active providers.
func (s *ProviderStore) ListAllActiveModels(ctx context.Context) ([]ModelRow, error) {
	const q = `
		SELECT m.id::text, m.provider_id::text, p.name, p.provider_type,
		       m.model_id, m.display_name, m.purpose, m.max_tokens, m.temperature,
		       m.top_p, m.num_ctx, m.max_prompt_input_runes, m.is_default, m.is_active,
		       m.created_at, m.updated_at
		FROM ai_models m JOIN ai_providers p ON p.id = m.provider_id
		WHERE m.is_active = true AND p.is_active = true
		ORDER BY m.purpose, p.name, m.display_name`
	return platformdb.QuerySliceErr(ctx, s.db, "list all active ai models", q, nil, scanModel)
}

// ActiveModelUUIDsByProviders maps each provider UUID to active model UUIDs.
func (s *ProviderStore) ActiveModelUUIDsByProviders(ctx context.Context, providerIDs []string) (map[string][]string, error) {
	out := make(map[string][]string)
	if len(providerIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT m.provider_id::text, m.id::text
		FROM ai_models m JOIN ai_providers p ON p.id = m.provider_id
		WHERE m.is_active = true AND p.is_active = true AND m.provider_id = ANY($1::uuid[])`
	rows, err := s.db.QueryContext(ctx, q, pgarray.Strings(providerIDs))
	if err != nil {
		return nil, fmt.Errorf("list active models by providers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var pid, mid string
		if err := rows.Scan(&pid, &mid); err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], mid)
	}
	return out, rows.Err()
}

// ChatConfigForPurpose returns the AIConfig for a chat-completion purpose
// (query, describe, judge, translation, agent), layering the resolved DB selection
// over the fallback env config so non-connection tuning knobs carry through.
// The bool reports whether the DB supplied the selection.
//
// PurposeAgent falls back to PurposeQuery when no agent-specific model is
// configured, so existing deployments keep working with zero config.
func (s *ProviderStore) ChatConfigForPurpose(p Purpose) (config.AIConfig, bool) {
	// Agent falls back to the query model when no agent model is configured.
	if p == PurposeAgent {
		if rm, ok := s.resolvedFor(PurposeAgent); ok {
			cfg := s.chatConfigFromResolved(rm)
			cfg = ensureQueryMaxTokens(cfg)
			return cfg, true
		}
		// No agent model configured — use query (DB-resolved or fallback).
		return s.ChatConfigForPurpose(PurposeQuery)
	}
	rm, ok := s.resolvedFor(p)
	if !ok {
		cfg := s.fallback
		if p == PurposeQuery {
			cfg = ensureQueryMaxTokens(cfg)
		}
		return cfg, false
	}
	cfg := s.chatConfigFromResolved(rm)
	if p == PurposeQuery {
		cfg = ensureQueryMaxTokens(cfg)
	}
	return cfg, true
}

func ensureQueryMaxTokens(cfg config.AIConfig) config.AIConfig {
	if cfg.Generation.MaxTokens < minQueryGenerationMaxTokens {
		cfg.Generation.MaxTokens = minQueryGenerationMaxTokens
	}
	return cfg
}

// EffectiveConfig returns the fallback config with embedding and translation
// selections overlaid from the database when present. Used at startup to build
// the embedder and translation service (which read the Embedding*/Translation*
// fields rather than the base connection fields).
func (s *ProviderStore) EffectiveConfig() config.AIConfig {
	cfg := s.fallback
	if rm, ok := s.resolvedFor(PurposeEmbedding); ok {
		cfg.Embedding.Model = rm.ModelID
		cfg.Embedding.BaseURL = rm.BaseURL
		cfg.Embedding.APIKey = rm.APIKey
		if rm.HTTPTimeoutSeconds > 0 {
			cfg.Embedding.HTTPTimeoutSeconds = rm.HTTPTimeoutSeconds
		}
	}
	if rm, ok := s.resolvedFor(PurposeTranslation); ok {
		cfg.Translation.Model = rm.ModelID
		cfg.Translation.BaseURL = rm.BaseURL
		cfg.Translation.APIKey = rm.APIKey
		if rm.HTTPTimeoutSeconds > 0 {
			cfg.Translation.HTTPTimeoutSeconds = rm.HTTPTimeoutSeconds
		}
	}
	return cfg
}

// EffectiveConfigForEmbeddings returns config used to decide whether embeddings
// are available. When the DB has no default embedding model, env embedding fields
// are cleared so stale BI_AI_EMBEDDING_* values do not enable embed actions.
func (s *ProviderStore) EffectiveConfigForEmbeddings() config.AIConfig {
	cfg := s.EffectiveConfig()
	if !s.HasResolved(PurposeEmbedding) {
		cfg.Embedding.Model = ""
		cfg.Embedding.BaseURL = ""
		cfg.Embedding.APIKey = ""
		cfg.Embedding.HTTPTimeoutSeconds = 0
	}
	return cfg
}

func (s *ProviderStore) decrypt(enc string) string {
	enc = strings.TrimSpace(enc)
	if enc == "" {
		return ""
	}
	if s.encryptor == nil {
		return enc
	}
	plain, err := s.encryptor.Decrypt(enc)
	if err != nil {
		// A value that does not decrypt is most likely legacy plaintext.
		if s.encryptor.IsEncrypted(enc) {
			slog.Warn("ai provider api key failed to decrypt", "error", err)
			return ""
		}
		return enc
	}
	return plain
}

func (s *ProviderStore) encrypt(plain string) (sql.NullString, error) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return sql.NullString{}, nil
	}
	if s.encryptor == nil {
		return sql.NullString{String: plain, Valid: true}, nil
	}
	enc, err := s.encryptor.Encrypt(plain)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("encrypt api key: %w", err)
	}
	return sql.NullString{String: enc, Valid: true}, nil
}

// maskSecret renders a decrypted API key as "…last4" without leaking it.
func maskSecret(plain string) string {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return ""
	}
	if len(plain) <= 4 {
		return "••••"
	}
	return "••••" + plain[len(plain)-4:]
}

// ----- Row / input types -----------------------------------------------------

// ProviderRow is the API shape for an ai_providers record. The API key is never
// returned; only its masked form and whether one is configured.
type ProviderRow struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	ProviderType       string    `json:"provider_type"`
	BaseURL            string    `json:"base_url"`
	APIKeyMasked       string    `json:"api_key_masked,omitempty"`
	HasAPIKey          bool      `json:"has_api_key"`
	IsActive           bool      `json:"is_active"`
	HTTPTimeoutSeconds int       `json:"http_timeout_seconds"`
	RateLimitPerMinute int       `json:"rate_limit_per_minute"`
	ModelCount         int       `json:"model_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ModelRow is the API shape for an ai_models record joined to its provider.
type ModelRow struct {
	ID                  string    `json:"id"`
	ProviderID          string    `json:"provider_id"`
	ProviderName        string    `json:"provider_name"`
	ProviderType        string    `json:"provider_type"`
	ModelID             string    `json:"model_id"`
	DisplayName         string    `json:"display_name"`
	Purpose             string    `json:"purpose"`
	MaxTokens           int       `json:"max_tokens"`
	Temperature         float64   `json:"temperature"`
	TopP                float64   `json:"top_p"`
	NumCtx              int       `json:"num_ctx"`
	MaxPromptInputRunes int       `json:"max_prompt_input_runes"`
	IsDefault           bool      `json:"is_default"`
	IsActive            bool      `json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// CreateProviderInput is the payload for creating a provider. APIKey is plaintext.
type CreateProviderInput struct {
	Name               string
	ProviderType       string
	BaseURL            string
	APIKey             string
	IsActive           *bool
	HTTPTimeoutSeconds int
	RateLimitPerMinute int
}

// UpdateProviderInput patches a provider. APIKey nil leaves the stored key
// unchanged; a non-nil empty string clears it (keyless provider).
type UpdateProviderInput struct {
	Name               string
	ProviderType       string
	BaseURL            string
	APIKey             *string
	IsActive           *bool
	HTTPTimeoutSeconds *int
	RateLimitPerMinute *int
}

// CreateModelInput is the payload for adding a model to a provider.
type CreateModelInput struct {
	ProviderID          string
	ModelID             string
	DisplayName         string
	Purpose             string
	MaxTokens           int
	Temperature         float64
	TopP                float64
	NumCtx              int
	MaxPromptInputRunes int
	IsDefault           bool
	IsActive            *bool
}

// UpdateModelInput patches a model.
type UpdateModelInput struct {
	ModelID             string
	DisplayName         string
	Purpose             string
	MaxTokens           *int
	Temperature         *float64
	TopP                *float64
	NumCtx              *int
	MaxPromptInputRunes *int
	IsActive            *bool
}

// ErrProviderNotFound / ErrModelNotFound are returned when a target row is absent.
var (
	ErrProviderNotFound = errors.New("ai provider not found")
	ErrModelNotFound    = errors.New("ai model not found")
)

// ----- Provider CRUD ---------------------------------------------------------

// ListProviders returns all providers with their model counts and masked keys.
func (s *ProviderStore) ListProviders(ctx context.Context) ([]ProviderRow, error) {
	const q = `
		SELECT p.id::text, p.name, p.provider_type, p.base_url, COALESCE(p.api_key_encrypted, ''),
		       p.is_active, p.http_timeout_seconds, p.rate_limit_per_minute,
		       p.created_at, p.updated_at,
		       (SELECT count(*) FROM ai_models m WHERE m.provider_id = p.id)
		FROM ai_providers p
		ORDER BY p.name`
	return platformdb.QuerySliceErr(ctx, s.db, "list ai providers", q, nil, s.scanProvider)
}

// GetProvider returns a single provider by id.
func (s *ProviderStore) GetProvider(ctx context.Context, id string) (ProviderRow, error) {
	const q = `
		SELECT p.id::text, p.name, p.provider_type, p.base_url, COALESCE(p.api_key_encrypted, ''),
		       p.is_active, p.http_timeout_seconds, p.rate_limit_per_minute,
		       p.created_at, p.updated_at,
		       (SELECT count(*) FROM ai_models m WHERE m.provider_id = p.id)
		FROM ai_providers p WHERE p.id = $1::uuid`
	row, err := s.scanProvider(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ProviderRow{}, ErrProviderNotFound
	}
	return row, err
}

func (s *ProviderStore) scanProvider(sc platformdb.Scanner) (ProviderRow, error) {
	var p ProviderRow
	var encKey string
	if err := sc.Scan(&p.ID, &p.Name, &p.ProviderType, &p.BaseURL, &encKey,
		&p.IsActive, &p.HTTPTimeoutSeconds, &p.RateLimitPerMinute,
		&p.CreatedAt, &p.UpdatedAt, &p.ModelCount); err != nil {
		return p, err
	}
	if plain := s.decrypt(encKey); plain != "" {
		p.HasAPIKey = true
		p.APIKeyMasked = maskSecret(plain)
	}
	return p, nil
}

// CreateProvider inserts a provider and returns its id.
func (s *ProviderStore) CreateProvider(ctx context.Context, in *CreateProviderInput) (string, error) {
	if strings.TrimSpace(in.Name) == "" {
		return "", errors.New("name is required")
	}
	if !ValidProviderType(in.ProviderType) {
		return "", fmt.Errorf("invalid provider_type %q", in.ProviderType)
	}
	encKey, err := s.encrypt(in.APIKey)
	if err != nil {
		return "", err
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	timeout := in.HTTPTimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURLForType(in.ProviderType)
	}
	if err := providerpkg.CheckProviderBaseURL(baseURL); err != nil {
		return "", fmt.Errorf("invalid provider base_url: %w", err)
	}
	var id string
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO ai_providers (name, provider_type, base_url, api_key_encrypted, is_active, http_timeout_seconds, rate_limit_per_minute)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id::text`,
		strings.TrimSpace(in.Name), strings.ToLower(strings.TrimSpace(in.ProviderType)), baseURL,
		encKey, active, timeout, in.RateLimitPerMinute,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert ai provider: %w", err)
	}
	return id, nil
}

// UpdateProvider patches a provider by id.
func (s *ProviderStore) UpdateProvider(ctx context.Context, id string, in *UpdateProviderInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	if !ValidProviderType(in.ProviderType) {
		return fmt.Errorf("invalid provider_type %q", in.ProviderType)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURLForType(in.ProviderType)
	}
	if err := providerpkg.CheckProviderBaseURL(baseURL); err != nil {
		return fmt.Errorf("invalid provider base_url: %w", err)
	}
	encryptedAPIKey, err := s.encryptedArg(in.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt ai provider api key: %w", err)
	}
	// COALESCE-style partial update: build SET clause for optional fields.
	res, err := s.db.ExecContext(ctx,
		`UPDATE ai_providers SET
		    name = $1,
		    provider_type = $2,
		    base_url = $3,
		    is_active = COALESCE($4, is_active),
		    http_timeout_seconds = COALESCE($5, http_timeout_seconds),
		    rate_limit_per_minute = COALESCE($6, rate_limit_per_minute),
		    api_key_encrypted = CASE WHEN $7 THEN $8 ELSE api_key_encrypted END,
		    updated_at = now()
		 WHERE id = $9::uuid`,
		strings.TrimSpace(in.Name), strings.ToLower(strings.TrimSpace(in.ProviderType)), baseURL,
		boolPtrArg(in.IsActive), intPtrArg(in.HTTPTimeoutSeconds), intPtrArg(in.RateLimitPerMinute),
		in.APIKey != nil, encryptedAPIKey, id,
	)
	if err != nil {
		return fmt.Errorf("update ai provider: %w", err)
	}
	return rowsAffectedExist(res, ErrProviderNotFound)
}

// DeleteProvider removes a provider and (via FK cascade) its models.
func (s *ProviderStore) DeleteProvider(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ai_providers WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete ai provider: %w", err)
	}
	return rowsAffectedExist(res, ErrProviderNotFound)
}

// ----- Model CRUD ------------------------------------------------------------

// ListModels returns models, optionally filtered by provider and/or purpose.
func (s *ProviderStore) ListModels(ctx context.Context, providerID, purpose string) ([]ModelRow, error) {
	q := `
		SELECT m.id::text, m.provider_id::text, p.name, p.provider_type,
		       m.model_id, m.display_name, m.purpose, m.max_tokens, m.temperature,
		       m.top_p, m.num_ctx, m.max_prompt_input_runes, m.is_default, m.is_active,
		       m.created_at, m.updated_at
		FROM ai_models m JOIN ai_providers p ON p.id = m.provider_id WHERE 1=1`
	var args []any
	if strings.TrimSpace(providerID) != "" {
		args = append(args, providerID)
		q += fmt.Sprintf(" AND m.provider_id = $%d::uuid", len(args))
	}
	if strings.TrimSpace(purpose) != "" {
		args = append(args, purpose)
		q += fmt.Sprintf(" AND m.purpose = $%d", len(args))
	}
	q += " ORDER BY m.purpose, p.name, m.display_name"
	return platformdb.QuerySliceErr(ctx, s.db, "list ai models", q, args, scanModel)
}

// ActiveModels returns the current default+active model for each purpose,
// joined to its provider. Used to render the "active models by purpose" view.
func (s *ProviderStore) ActiveModels(ctx context.Context) ([]ModelRow, error) {
	const q = `
		SELECT m.id::text, m.provider_id::text, p.name, p.provider_type,
		       m.model_id, m.display_name, m.purpose, m.max_tokens, m.temperature,
		       m.top_p, m.num_ctx, m.max_prompt_input_runes, m.is_default, m.is_active,
		       m.created_at, m.updated_at
		FROM ai_models m JOIN ai_providers p ON p.id = m.provider_id
		WHERE m.is_default = true AND m.is_active = true AND p.is_active = true
		ORDER BY m.purpose`
	return platformdb.QuerySliceErr(ctx, s.db, "list active ai models", q, nil, scanModel)
}

func scanModel(sc platformdb.Scanner) (ModelRow, error) {
	var m ModelRow
	err := sc.Scan(&m.ID, &m.ProviderID, &m.ProviderName, &m.ProviderType,
		&m.ModelID, &m.DisplayName, &m.Purpose, &m.MaxTokens, &m.Temperature,
		&m.TopP, &m.NumCtx, &m.MaxPromptInputRunes, &m.IsDefault, &m.IsActive,
		&m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// CreateModel inserts a model. When IsDefault is set it atomically clears any
// existing default for the same purpose.
func (s *ProviderStore) CreateModel(ctx context.Context, in *CreateModelInput) (string, error) {
	if strings.TrimSpace(in.ProviderID) == "" {
		return "", errors.New("provider_id is required")
	}
	if strings.TrimSpace(in.ModelID) == "" {
		return "", errors.New("model_id is required")
	}
	if !ValidPurpose(in.Purpose) {
		return "", fmt.Errorf("invalid purpose %q", in.Purpose)
	}
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		display = strings.TrimSpace(in.ModelID)
	}
	maxTokens := in.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	maxRunes := in.MaxPromptInputRunes
	if maxRunes <= 0 {
		maxRunes = 80000
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	var id string
	err := platformdb.RunInTx(ctx, s.db, func(tx *sql.Tx) error {
		if in.IsDefault {
			if _, err := tx.ExecContext(ctx,
				`UPDATE ai_models SET is_default = false, updated_at = now() WHERE purpose = $1 AND is_default = true`,
				in.Purpose); err != nil {
				return err
			}
		}
		return tx.QueryRowContext(ctx,
			`INSERT INTO ai_models (provider_id, model_id, display_name, purpose, max_tokens,
			    temperature, top_p, num_ctx, max_prompt_input_runes, is_default, is_active)
			 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id::text`,
			in.ProviderID, strings.TrimSpace(in.ModelID), display, in.Purpose, maxTokens,
			in.Temperature, in.TopP, in.NumCtx, maxRunes, in.IsDefault, active,
		).Scan(&id)
	})
	if err != nil {
		return "", fmt.Errorf("insert ai model: %w", err)
	}
	return id, nil
}

// UpdateModel patches a model by id.
func (s *ProviderStore) UpdateModel(ctx context.Context, id string, in *UpdateModelInput) error {
	if strings.TrimSpace(in.ModelID) == "" {
		return errors.New("model_id is required")
	}
	if !ValidPurpose(in.Purpose) {
		return fmt.Errorf("invalid purpose %q", in.Purpose)
	}
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		display = strings.TrimSpace(in.ModelID)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE ai_models SET
		    model_id = $1,
		    display_name = $2,
		    purpose = $3,
		    max_tokens = COALESCE($4, max_tokens),
		    temperature = COALESCE($5, temperature),
		    top_p = COALESCE($6, top_p),
		    num_ctx = COALESCE($7, num_ctx),
		    max_prompt_input_runes = COALESCE($8, max_prompt_input_runes),
		    is_active = COALESCE($9, is_active),
		    updated_at = now()
		 WHERE id = $10::uuid`,
		strings.TrimSpace(in.ModelID), display, in.Purpose,
		intPtrArg(in.MaxTokens), floatPtrArg(in.Temperature), floatPtrArg(in.TopP),
		intPtrArg(in.NumCtx), intPtrArg(in.MaxPromptInputRunes), boolPtrArg(in.IsActive), id,
	)
	if err != nil {
		return fmt.Errorf("update ai model: %w", err)
	}
	return rowsAffectedExist(res, ErrModelNotFound)
}

// DeleteModel removes a model by id.
func (s *ProviderStore) DeleteModel(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ai_models WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete ai model: %w", err)
	}
	return rowsAffectedExist(res, ErrModelNotFound)
}

// SetDefaultModel makes the model the default for its purpose, clearing any
// prior default in the same transaction.
func (s *ProviderStore) SetDefaultModel(ctx context.Context, modelID string) error {
	return platformdb.RunInTx(ctx, s.db, func(tx *sql.Tx) error {
		var purpose string
		err := tx.QueryRowContext(ctx, `SELECT purpose FROM ai_models WHERE id = $1::uuid`, modelID).Scan(&purpose)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrModelNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE ai_models SET is_default = false, updated_at = now() WHERE purpose = $1 AND is_default = true`,
			purpose); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE ai_models SET is_default = true, is_active = true, updated_at = now() WHERE id = $1::uuid`,
			modelID)
		return err
	})
}

// ----- Connection test ------------------------------------------------------

// ConnectionTestResult reports the outcome of a provider connectivity probe.
type ConnectionTestResult struct {
	Status    string `json:"status"` // "connected" | "error"
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Message   string `json:"message,omitempty"`
	Model     string `json:"model,omitempty"`
}

// ListRemoteModels fetches model ids from the provider's upstream catalog API.
func (s *ProviderStore) ListRemoteModels(ctx context.Context, providerID string) ([]RemoteModelOption, error) {
	prov, err := s.GetProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}

	var encKey sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT api_key_encrypted FROM ai_providers WHERE id = $1::uuid`, providerID).Scan(&encKey); err != nil {
		return nil, fmt.Errorf("load provider api key: %w", err)
	}
	apiKey := ""
	if encKey.Valid {
		apiKey = s.decrypt(encKey.String)
	}

	timeout := time.Duration(prov.HTTPTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultRemoteModelsTimeout
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	return ListRemoteModelsFromEndpoint(ctx, prov.ProviderType, prov.BaseURL, apiKey, timeout)
}

// TestConnection sends a tiny prompt to the provider using modelID (or the
// provider's first configured model) and reports latency or the error.
func (s *ProviderStore) TestConnection(ctx context.Context, providerID, modelID string) (ConnectionTestResult, error) {
	prov, err := s.GetProvider(ctx, providerID)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	// Pick the provider's default model. When a specific modelID is given we
	// look up its purpose; otherwise we take the default model of any purpose so
	// embedding-only providers can be tested too.
	var purpose string
	if strings.TrimSpace(modelID) == "" {
		if err := s.db.QueryRowContext(ctx,
			`SELECT model_id, purpose FROM ai_models WHERE provider_id = $1::uuid ORDER BY is_default DESC, created_at LIMIT 1`,
			providerID).Scan(&modelID, &purpose); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ConnectionTestResult{}, fmt.Errorf("lookup default provider model: %w", err)
		}
	} else {
		if err := s.db.QueryRowContext(ctx,
			`SELECT purpose FROM ai_models WHERE provider_id = $1::uuid AND model_id = $2 LIMIT 1`,
			providerID, strings.TrimSpace(modelID)).Scan(&purpose); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ConnectionTestResult{}, fmt.Errorf("lookup provider model purpose: %w", err)
		}
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ConnectionTestResult{Status: "error", Message: "no model configured for this provider"}, nil
	}

	var apiKey string
	var encKey sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT api_key_encrypted FROM ai_providers WHERE id = $1::uuid`, providerID).Scan(&encKey); err == nil && encKey.Valid {
		apiKey = s.decrypt(encKey.String)
	}

	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()

	// Embedding providers expose /embeddings, not /chat/completions — probe the
	// right endpoint so the test reflects how the model is actually used.
	if Purpose(purpose) == PurposeEmbedding {
		cfg := s.fallback
		cfg.Embedding.Model = modelID
		cfg.Embedding.BaseURL = prov.BaseURL
		cfg.Embedding.APIKey = apiKey
		cfg.Embedding.HTTPTimeoutSeconds = 30
		embedder := providerpkg.NewOpenAIEmbedder(cfg)
		defer func() { _ = embedder.Close() }()
		if _, err := embedder.Embed(testCtx, []string{"connectivity check"}); err != nil {
			return ConnectionTestResult{Status: "error", Message: err.Error(), Model: modelID}, nil
		}
		return ConnectionTestResult{Status: "connected", LatencyMS: time.Since(start).Milliseconds(), Model: modelID}, nil
	}

	cfg := s.fallback
	cfg.Connection.Provider = prov.ProviderType
	cfg.Connection.BaseURL = prov.BaseURL
	cfg.Connection.APIKey = apiKey
	cfg.Connection.Model = modelID
	cfg.Generation.MaxTokens = 16
	cfg.Connection.HTTPTimeoutSeconds = 30
	cfg.Query.Provider, cfg.Query.Model, cfg.Query.BaseURL, cfg.Query.APIKey = "", "", "", ""

	p, err := providerpkg.NewProvider(cfg)
	if err != nil {
		return ConnectionTestResult{Status: "error", Message: err.Error(), Model: modelID}, nil
	}
	defer closeProvider(p)

	if _, err := p.Generate(testCtx, "Respond with the single word: OK"); err != nil {
		return ConnectionTestResult{Status: "error", Message: err.Error(), Model: modelID}, nil
	}
	return ConnectionTestResult{
		Status:    "connected",
		LatencyMS: time.Since(start).Milliseconds(),
		Model:     modelID,
	}, nil
}

// ----- small arg helpers -----------------------------------------------------

func (s *ProviderStore) encryptedArg(apiKey *string) (sql.NullString, error) {
	if apiKey == nil {
		return sql.NullString{}, nil
	}
	enc, err := s.encrypt(*apiKey)
	if err != nil {
		return sql.NullString{}, err
	}
	return enc, nil
}

func boolPtrArg(v *bool) any {
	if v == nil {
		return nil
	}
	return *v
}

func intPtrArg(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func floatPtrArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func rowsAffectedExist(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return nil
}

func closeProvider(v any) {
	if c, ok := v.(interface{ Close() error }); ok && c != nil {
		_ = c.Close()
	}
}
