-- Add 'agent' purpose for the web agent planner/finalizer (multi-step tool loop).
-- Applies to both bi_metadata and bi_auth databases.

-- bi_metadata.ai_models: add 'agent' to the purpose CHECK constraint.
ALTER TABLE ai_models DROP CONSTRAINT IF EXISTS ai_models_purpose_check;
ALTER TABLE ai_models ADD CONSTRAINT ai_models_purpose_check CHECK (purpose IN (
    'query',
    'describe',
    'embedding',
    'translation',
    'judge',
    'agent'
));
