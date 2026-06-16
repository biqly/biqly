ALTER TABLE ai_query_history
    DROP COLUMN IF EXISTS ab_variant_id,
    DROP COLUMN IF EXISTS ab_experiment_id;

DROP TABLE IF EXISTS ab_variants;
DROP TABLE IF EXISTS ab_experiments;
