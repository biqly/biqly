-- Locale-specific AI prompt sections (system_rules, output_format, …).
-- Seeded from embedded defaults on startup when empty; editable without redeploy.

CREATE TABLE IF NOT EXISTS ai_prompt_templates (
    name        TEXT NOT NULL,
    locale      TEXT NOT NULL,
    content     TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (name, locale)
);

CREATE INDEX IF NOT EXISTS idx_ai_prompt_templates_locale
    ON ai_prompt_templates (locale);
