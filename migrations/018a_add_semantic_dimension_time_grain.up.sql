ALTER TABLE semantic_dimensions
    ADD COLUMN IF NOT EXISTS time_grain TEXT;

ALTER TABLE semantic_dimensions
    ADD CONSTRAINT semantic_dimensions_time_grain_check
    CHECK (time_grain IS NULL OR time_grain IN ('day', 'week', 'month', 'quarter', 'year'));
