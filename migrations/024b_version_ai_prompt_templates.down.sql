DROP INDEX IF EXISTS idx_eval_runs_prompt_template_bundle_version;

ALTER TABLE eval_runs
    DROP COLUMN IF EXISTS prompt_template_bundle_version,
    DROP COLUMN IF EXISTS prompt_template_versions;

DROP INDEX IF EXISTS idx_ai_prompt_templates_active_locale;
DROP INDEX IF EXISTS idx_ai_prompt_templates_one_active;

DELETE FROM ai_prompt_templates
WHERE is_active = FALSE;

ALTER TABLE ai_prompt_templates
    DROP CONSTRAINT IF EXISTS ai_prompt_templates_pkey;

ALTER TABLE ai_prompt_templates
    ADD CONSTRAINT ai_prompt_templates_pkey PRIMARY KEY (name, locale);

ALTER TABLE ai_prompt_templates
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS version;
