DROP INDEX IF EXISTS idx_few_shot_examples_favorite;
ALTER TABLE few_shot_examples DROP COLUMN IF EXISTS is_favorite;
