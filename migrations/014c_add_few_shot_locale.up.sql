-- Few-shot examples can be tagged with the language of the natural-language
-- question (e.g. "tr", "en"). The prompt builder prefers examples matching the
-- requester's locale; locale-agnostic rows (NULL) are always eligible. Older
-- rows stay NULL so existing data continues to act as a global fallback.

ALTER TABLE few_shot_examples
    ADD COLUMN IF NOT EXISTS locale TEXT;

CREATE INDEX IF NOT EXISTS idx_few_shot_examples_locale
    ON few_shot_examples (locale);
