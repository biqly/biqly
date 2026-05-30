ALTER TABLE few_shot_examples ADD COLUMN IF NOT EXISTS is_favorite BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_few_shot_examples_favorite
    ON few_shot_examples (is_favorite, created_at DESC)
    WHERE is_favorite;
