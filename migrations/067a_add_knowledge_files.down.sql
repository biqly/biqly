DROP INDEX IF EXISTS idx_saved_queries_knowledge_file;
DROP INDEX IF EXISTS idx_glossary_knowledge_file;
DROP INDEX IF EXISTS idx_ai_instructions_knowledge_file;
ALTER TABLE ai_saved_queries DROP COLUMN IF EXISTS knowledge_file_id;
ALTER TABLE business_glossary_terms DROP COLUMN IF EXISTS knowledge_file_id;
ALTER TABLE ai_instructions DROP COLUMN IF EXISTS knowledge_file_id;
DROP INDEX IF EXISTS idx_knowledge_files_datasource;
DROP TABLE IF EXISTS ai_knowledge_files;
