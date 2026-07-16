-- 070a: relation descriptions live on the metadata relations themselves so any
-- introspected FK can be documented (and AI-described) without requiring a
-- semantic-model join. Localized values overlay via entity_translations with
-- entity_type = 'relation'.
ALTER TABLE relations
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
