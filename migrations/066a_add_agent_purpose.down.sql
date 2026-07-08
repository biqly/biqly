-- Reverse: remove 'agent' from the purpose CHECK constraint.
ALTER TABLE ai_models DROP CONSTRAINT IF EXISTS ai_models_purpose_check;
ALTER TABLE ai_models ADD CONSTRAINT ai_models_purpose_check CHECK (purpose IN (
    'query',
    'describe',
    'embedding',
    'translation',
    'judge'
));
