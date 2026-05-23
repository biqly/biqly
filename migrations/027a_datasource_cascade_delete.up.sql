-- Add ON DELETE CASCADE to every FK that references datasources(id) so a
-- DeleteDatasource() call no longer leaves orphaned history / permission /
-- example rows behind (and no longer fails with a FK constraint violation
-- when children exist).
--
-- Tables that already had CASCADE (schemas, tables, columns, relations,
-- semantic_models, business_glossary_terms) are unchanged.

ALTER TABLE query_history
    DROP CONSTRAINT IF EXISTS query_history_datasource_id_fkey,
    ADD  CONSTRAINT query_history_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id) ON DELETE CASCADE;

ALTER TABLE query_saved
    DROP CONSTRAINT IF EXISTS query_saved_datasource_id_fkey,
    ADD  CONSTRAINT query_saved_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id) ON DELETE CASCADE;

ALTER TABLE ai_query_history
    DROP CONSTRAINT IF EXISTS ai_query_history_datasource_id_fkey,
    ADD  CONSTRAINT ai_query_history_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id) ON DELETE CASCADE;

ALTER TABLE permissions
    DROP CONSTRAINT IF EXISTS permissions_datasource_id_fkey,
    ADD  CONSTRAINT permissions_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id) ON DELETE CASCADE;

ALTER TABLE few_shot_examples
    DROP CONSTRAINT IF EXISTS few_shot_examples_datasource_id_fkey,
    ADD  CONSTRAINT few_shot_examples_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id) ON DELETE CASCADE;

ALTER TABLE ai_feedback
    DROP CONSTRAINT IF EXISTS ai_feedback_datasource_id_fkey,
    ADD  CONSTRAINT ai_feedback_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id) ON DELETE CASCADE;
