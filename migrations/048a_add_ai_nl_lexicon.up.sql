-- Locale-dimensioned NL vocabulary (synonyms, phrases, intent tokens) used by
-- routing, ambiguity detection, and semantic model generation (ADR-0001).
-- Seeded from embedded defaults on startup when empty; editable without redeploy.
CREATE TABLE IF NOT EXISTS ai_nl_lexicon (
    locale      TEXT        NOT NULL,
    domain      TEXT        NOT NULL,
    key         TEXT        NOT NULL,
    value       JSONB       NOT NULL,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (locale, domain, key)
);

CREATE INDEX IF NOT EXISTS idx_ai_nl_lexicon_domain ON ai_nl_lexicon (domain, locale);
