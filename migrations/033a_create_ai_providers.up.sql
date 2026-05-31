-- AI provider and model configuration moved from BI_AI_* environment variables
-- into the database so providers/models can be managed at runtime (no restart).
-- API keys are stored AES-256-GCM encrypted (security.Encryption); keyless
-- providers (Ollama, local llama-server) leave api_key_encrypted NULL.
CREATE TABLE IF NOT EXISTS ai_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('openai', 'openai-compatible', 'anthropic')),
    base_url TEXT NOT NULL DEFAULT '',
    api_key_encrypted TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    http_timeout_seconds INT NOT NULL DEFAULT 120,
    rate_limit_per_minute INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name)
);

CREATE INDEX idx_ai_providers_type ON ai_providers(provider_type);
CREATE INDEX idx_ai_providers_active ON ai_providers(is_active);

-- One row per (provider, model, purpose). A purpose selects which AI workload
-- the model serves; at most one default per purpose is enforced below.
CREATE TABLE IF NOT EXISTS ai_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES ai_providers(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN (
        'query',
        'describe',
        'embedding',
        'translation',
        'judge'
    )),
    max_tokens INT NOT NULL DEFAULT 4096,
    temperature DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    top_p DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    num_ctx INT NOT NULL DEFAULT 0,
    max_prompt_input_runes INT NOT NULL DEFAULT 80000,
    is_default BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider_id, model_id, purpose)
);

CREATE INDEX idx_ai_models_purpose ON ai_models(purpose);

-- Enforce "at most one default per purpose" at the schema level so concurrent
-- writers cannot create two defaults; the store also clears prior defaults
-- inside the SetDefault transaction.
CREATE UNIQUE INDEX idx_ai_models_one_default_per_purpose
    ON ai_models(purpose) WHERE is_default = true;
