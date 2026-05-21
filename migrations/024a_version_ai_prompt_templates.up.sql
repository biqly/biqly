ALTER TABLE ai_prompt_templates
    ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE ai_prompt_templates
SET created_at = updated_at
WHERE created_at IS NULL;

ALTER TABLE ai_prompt_templates
    DROP CONSTRAINT IF EXISTS ai_prompt_templates_pkey;

ALTER TABLE ai_prompt_templates
    ADD CONSTRAINT ai_prompt_templates_pkey PRIMARY KEY (name, locale, version);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_prompt_templates_one_active
    ON ai_prompt_templates (name, locale)
    WHERE is_active;

CREATE INDEX IF NOT EXISTS idx_ai_prompt_templates_active_locale
    ON ai_prompt_templates (locale, is_active);

ALTER TABLE eval_runs
    ADD COLUMN IF NOT EXISTS prompt_template_versions JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS prompt_template_bundle_version INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_eval_runs_prompt_template_bundle_version
    ON eval_runs (prompt_template_bundle_version);
