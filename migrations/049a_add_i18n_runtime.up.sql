-- Dynamic locale registry + message-bundle overlay (ADR-0001 K8, DİL-3).
-- i18n_locales is seeded from the embedded EN/TR profiles on startup; new
-- languages become available by inserting a row here plus (optionally) a
-- bundle row — no backend release required. Embedded EN stays the terminal
-- fallback and cannot be disabled.
CREATE TABLE IF NOT EXISTS i18n_locales (
    locale                     TEXT        PRIMARY KEY,
    label                      TEXT        NOT NULL,
    short_label                TEXT        NOT NULL,
    question_letters           TEXT        NOT NULL DEFAULT '',
    question_signals           JSONB       NOT NULL DEFAULT '[]',
    uses_metadata_translations BOOLEAN     NOT NULL DEFAULT FALSE,
    enabled                    BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS i18n_bundles (
    locale     TEXT        PRIMARY KEY,
    bundle     JSONB       NOT NULL,
    version    INT         NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
