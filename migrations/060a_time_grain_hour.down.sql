DELETE FROM semantic_dimensions WHERE time_grain = 'hour';

ALTER TABLE semantic_dimensions
    DROP CONSTRAINT IF EXISTS semantic_dimensions_time_grain_check;

ALTER TABLE semantic_dimensions
    ADD CONSTRAINT semantic_dimensions_time_grain_check
    CHECK (time_grain IS NULL OR time_grain IN ('day', 'week', 'month', 'quarter', 'year'));
