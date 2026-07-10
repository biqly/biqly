-- Markdown knowledge base: datasource-scoped .md files (with YAML frontmatter)
-- arranged in virtual folders (glossary/, instructions/, metrics/, sql-pairs/).
-- Publishing a file extracts structured records into the existing stores
-- (ai_instructions, business_glossary_terms, ai_saved_queries) so the prompt
-- pipeline keeps consuming the same loaders; the link back is knowledge_file_id.
CREATE TABLE IF NOT EXISTS ai_knowledge_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    folder TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    content_md TEXT NOT NULL DEFAULT '',
    frontmatter JSONB,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_knowledge_files_path UNIQUE (datasource_id, path)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_files_datasource
    ON ai_knowledge_files (datasource_id, folder, path);

-- Idempotent publish-extraction upserts key on the source file.
ALTER TABLE ai_instructions ADD COLUMN IF NOT EXISTS knowledge_file_id UUID REFERENCES ai_knowledge_files(id) ON DELETE SET NULL;
ALTER TABLE business_glossary_terms ADD COLUMN IF NOT EXISTS knowledge_file_id UUID REFERENCES ai_knowledge_files(id) ON DELETE SET NULL;
ALTER TABLE ai_saved_queries ADD COLUMN IF NOT EXISTS knowledge_file_id UUID REFERENCES ai_knowledge_files(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_ai_instructions_knowledge_file
    ON ai_instructions (knowledge_file_id) WHERE knowledge_file_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_glossary_knowledge_file
    ON business_glossary_terms (knowledge_file_id) WHERE knowledge_file_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_saved_queries_knowledge_file
    ON ai_saved_queries (knowledge_file_id) WHERE knowledge_file_id IS NOT NULL;
