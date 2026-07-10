-- AI-generated (or hand-written) natural-language description of what a join
-- semantically means, shown on the modeling canvas relationship tooltip.
ALTER TABLE semantic_joins ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
