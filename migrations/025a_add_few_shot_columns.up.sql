ALTER TABLE few_shot_examples ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE few_shot_examples ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE few_shot_examples ADD COLUMN IF NOT EXISTS is_few_shot BOOLEAN NOT NULL DEFAULT true;

-- Seed name from question for existing data
UPDATE few_shot_examples SET name = question WHERE name IS NULL OR name = '';
