ALTER TABLE semantic_metrics
    DROP COLUMN IF EXISTS expr_json;

ALTER TABLE semantic_dimensions
    DROP COLUMN IF EXISTS calculated_expr_json,
    DROP COLUMN IF EXISTS calculated_expression;
