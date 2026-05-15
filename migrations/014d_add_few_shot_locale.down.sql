DROP INDEX IF EXISTS idx_few_shot_examples_locale;
ALTER TABLE few_shot_examples
    DROP COLUMN IF EXISTS locale;
