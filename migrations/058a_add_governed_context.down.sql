-- Reverses 058a. Drops only the new unified/instructions tables; the legacy
-- ai_confirmed_queries and ai_skills tables are left untouched.
DROP TABLE IF EXISTS ai_instructions;
DROP TABLE IF EXISTS ai_saved_queries;
