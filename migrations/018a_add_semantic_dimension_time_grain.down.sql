ALTER TABLE semantic_dimensions
    DROP CONSTRAINT IF EXISTS semantic_dimensions_time_grain_check;

ALTER TABLE semantic_dimensions
    DROP COLUMN IF EXISTS time_grain;
