ALTER TABLE semantic_metrics
    ADD COLUMN rate_behavior TEXT NOT NULL DEFAULT '';

ALTER TABLE semantic_metrics
    ADD CONSTRAINT semantic_metrics_rate_behavior_check
    CHECK (rate_behavior IN ('', 'ratio_of_sums', 'average_of_customer_rates', 'weighted_average', 'latest_value'));
