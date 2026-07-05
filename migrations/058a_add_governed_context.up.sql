-- Governed-context consolidation (SP1): a single unified store that can represent
-- an EXAMPLE (embedding-RAG few-shot grounding), a SKILL (executable parameterized
-- LogicalQuery), or both, plus a free-form business-rules store.
--
-- ADDITIVE + behavior-preserving: the legacy ai_confirmed_queries and ai_skills
-- tables are left intact and are backfilled into ai_saved_queries below. Consumers
-- (few-shot recall + skill execution) are repointed at ai_saved_queries in the same
-- change. The legacy tables are dropped only after prod verification (SP5).

CREATE TABLE IF NOT EXISTS ai_saved_queries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    model_id UUID REFERENCES semantic_models(id) ON DELETE SET NULL,
    name TEXT NOT NULL DEFAULT '',                 -- skill name; '' for pure examples
    description TEXT NOT NULL DEFAULT '',
    question TEXT NOT NULL DEFAULT '',              -- NL question (nl_query for examples, question for skills)
    question_hash TEXT NOT NULL DEFAULT '',
    sql_query TEXT NOT NULL DEFAULT '',             -- grounding payload from examples; '' for skills
    logical_query JSONB,                            -- executable payload from skills; NULL for examples
    parameters JSONB NOT NULL DEFAULT '[]',
    question_embedding JSONB,                       -- from examples; NULL for skills (RAG uses rows WHERE embedding IS NOT NULL)
    semantic_model_hash TEXT NOT NULL DEFAULT '',
    tags TEXT[] NOT NULL DEFAULT '{}',
    source TEXT NOT NULL DEFAULT 'example',         -- 'example' | 'skill'
    runnable BOOLEAN NOT NULL DEFAULT false,        -- true when logical_query present (skill)
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    last_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Mirrors idx_ai_skills_datasource: datasource-scoped listing, active first.
CREATE INDEX IF NOT EXISTS idx_ai_saved_queries_datasource
    ON ai_saved_queries (datasource_id, is_active, updated_at DESC);

-- Mirrors idx_ai_confirmed_queries_recall, restricted to embedding-bearing rows
-- since few-shot recall only ever ranks rows that carry an embedding.
CREATE INDEX IF NOT EXISTS idx_ai_saved_queries_recall
    ON ai_saved_queries (datasource_id, model_id, is_active, created_at DESC)
    WHERE is_active = true AND question_embedding IS NOT NULL;

-- Fast path for the runnable (skill) library listing.
CREATE INDEX IF NOT EXISTS idx_ai_saved_queries_runnable
    ON ai_saved_queries (datasource_id, is_active, updated_at DESC)
    WHERE runnable = true;

-- Preserves the ai_skills UNIQUE (datasource_id, name) constraint for skills only;
-- examples share the empty name and must not collide.
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_saved_queries_skill_name
    ON ai_saved_queries (datasource_id, name)
    WHERE source = 'skill';

-- Upsert key for grounding examples, mirroring the legacy ai_confirmed_queries
-- unique key, so positive-feedback dual-writes ON CONFLICT DO UPDATE instead of
-- inserting duplicate recall rows.
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_saved_queries_example_key
    ON ai_saved_queries (
        datasource_id, question_hash, semantic_model_hash,
        COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid)
    )
    WHERE source = 'example';

-- Backfill grounding examples from ai_confirmed_queries.
INSERT INTO ai_saved_queries (
    datasource_id, model_id, name, description, question, question_hash,
    sql_query, logical_query, parameters, question_embedding,
    semantic_model_hash, tags, source, runnable, is_active,
    created_by, version, last_verified_at, created_at, updated_at
)
SELECT
    datasource_id, model_id, '', '', nl_query, question_hash,
    sql_query, NULL, '[]'::jsonb, question_embedding,
    semantic_model_hash, '{}'::text[], 'example', false, is_active,
    COALESCE(user_id, ''), 1, NULL, confirmed_at, confirmed_at
FROM ai_confirmed_queries;

-- Backfill executable skills from ai_skills.
INSERT INTO ai_saved_queries (
    datasource_id, model_id, name, description, question, question_hash,
    sql_query, logical_query, parameters, question_embedding,
    semantic_model_hash, tags, source, runnable, is_active,
    created_by, version, last_verified_at, created_at, updated_at
)
SELECT
    datasource_id, model_id, name, description, question, '',
    '', logical_query, parameters, NULL,
    '', tags, 'skill', true, is_active,
    created_by, version, last_verified_at, created_at, updated_at
FROM ai_skills;

-- Free-form business rules (wren-style instructions), injected into the prompt as
-- a "## Business Rules" block. New store; no legacy source to backfill.
CREATE TABLE IF NOT EXISTS ai_instructions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    model_id UUID REFERENCES semantic_models(id) ON DELETE SET NULL,
    title TEXT NOT NULL DEFAULT '',
    body_md TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_instructions_datasource
    ON ai_instructions (datasource_id, is_active, updated_at DESC);
