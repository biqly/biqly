CREATE TABLE ai_provider_workspace_grants (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id  UUID NOT NULL,
    granted_by   UUID REFERENCES users(id),
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, provider_id)
);

CREATE TABLE ai_model_workspace_grants (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    model_id     UUID NOT NULL,
    granted_by   UUID REFERENCES users(id),
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, model_id)
);

CREATE TABLE ai_provider_role_grants (
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL,
    granted_by  UUID REFERENCES users(id),
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, provider_id)
);

CREATE TABLE ai_model_role_grants (
    role_id    UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    model_id   UUID NOT NULL,
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, model_id)
);

CREATE TABLE user_ai_model_preferences (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    TEXT NOT NULL CHECK (purpose IN ('query', 'describe', 'embedding', 'translation', 'judge')),
    model_id   UUID NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, purpose)
);

CREATE INDEX idx_ai_provider_workspace_grants_provider ON ai_provider_workspace_grants(provider_id);
CREATE INDEX idx_ai_model_workspace_grants_model ON ai_model_workspace_grants(model_id);
CREATE INDEX idx_ai_provider_role_grants_provider ON ai_provider_role_grants(provider_id);
CREATE INDEX idx_ai_model_role_grants_model ON ai_model_role_grants(model_id);
