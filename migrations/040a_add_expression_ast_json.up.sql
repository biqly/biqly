ALTER TABLE semantic_dimensions
    ADD COLUMN IF NOT EXISTS calculated_expression TEXT,
    ADD COLUMN IF NOT EXISTS calculated_expr_json JSONB;

ALTER TABLE semantic_metrics
    ADD COLUMN IF NOT EXISTS expr_json JSONB;
