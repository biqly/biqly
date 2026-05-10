-- 006_add_ai_examples_feedback.up.sql

-- Curated few-shot examples for AI text-to-SQL prompt injection.
CREATE TABLE IF NOT EXISTS few_shot_examples (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID REFERENCES datasources(id),
    model_id UUID REFERENCES semantic_models(id),
    question TEXT NOT NULL,
    logical_query JSONB NOT NULL,
    tags TEXT[] DEFAULT '{}',
    dialect TEXT DEFAULT 'postgresql',
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_few_shot_examples_datasource ON few_shot_examples(datasource_id);
CREATE INDEX idx_few_shot_examples_model ON few_shot_examples(model_id);
CREATE INDEX idx_few_shot_examples_tags ON few_shot_examples USING GIN(tags);

-- User feedback on AI query results.
CREATE TABLE IF NOT EXISTS ai_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_query_history_id UUID REFERENCES ai_query_history(id),
    question TEXT NOT NULL,
    datasource_id UUID REFERENCES datasources(id),
    rating TEXT NOT NULL CHECK (rating IN ('positive', 'negative')),
    categories TEXT[] DEFAULT '{}',
    feedback_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_feedback_query ON ai_feedback(ai_query_history_id);
CREATE INDEX idx_ai_feedback_datasource ON ai_feedback(datasource_id);
CREATE INDEX idx_ai_feedback_rating ON ai_feedback(rating);

-- Extend ai_query_history with user feedback and execution metadata.
ALTER TABLE ai_query_history
    ADD COLUMN IF NOT EXISTS user_rating TEXT CHECK (user_rating IN ('positive', 'negative')),
    ADD COLUMN IF NOT EXISTS model_used TEXT,
    ADD COLUMN IF NOT EXISTS token_count INT,
    ADD COLUMN IF NOT EXISTS cost_usd NUMERIC(10, 6),
    ADD COLUMN IF NOT EXISTS latency_ms INT;

-- AI usage aggregation view (for dashboard analytics).
CREATE OR REPLACE VIEW v_ai_usage_daily AS
SELECT
    DATE(created_at) AS usage_date,
    COUNT(*) AS total_queries,
    COUNT(*) FILTER (WHERE user_rating = 'positive') AS positive_feedback,
    COUNT(*) FILTER (WHERE user_rating = 'negative') AS negative_feedback,
    COALESCE(AVG(latency_ms), 0) AS avg_latency_ms,
    COALESCE(SUM(cost_usd), 0) AS total_cost,
    COALESCE(SUM(token_count), 0) AS total_tokens
FROM ai_query_history
GROUP BY DATE(created_at)
ORDER BY usage_date DESC;
