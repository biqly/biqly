ALTER TABLE business_glossary_terms
    ADD COLUMN IF NOT EXISTS ai_context JSONB;
