ALTER TABLE semantic_metrics
    DROP CONSTRAINT IF EXISTS semantic_metrics_rate_behavior_check;

ALTER TABLE semantic_metrics
    DROP COLUMN IF EXISTS rate_behavior;
