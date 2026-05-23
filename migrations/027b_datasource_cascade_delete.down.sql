-- Restore the original FK constraints without ON DELETE CASCADE.

ALTER TABLE query_history
    DROP CONSTRAINT IF EXISTS query_history_datasource_id_fkey,
    ADD  CONSTRAINT query_history_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id);

ALTER TABLE query_saved
    DROP CONSTRAINT IF EXISTS query_saved_datasource_id_fkey,
    ADD  CONSTRAINT query_saved_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id);

ALTER TABLE ai_query_history
    DROP CONSTRAINT IF EXISTS ai_query_history_datasource_id_fkey,
    ADD  CONSTRAINT ai_query_history_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id);

ALTER TABLE permissions
    DROP CONSTRAINT IF EXISTS permissions_datasource_id_fkey,
    ADD  CONSTRAINT permissions_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id);

ALTER TABLE few_shot_examples
    DROP CONSTRAINT IF EXISTS few_shot_examples_datasource_id_fkey,
    ADD  CONSTRAINT few_shot_examples_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id);

ALTER TABLE ai_feedback
    DROP CONSTRAINT IF EXISTS ai_feedback_datasource_id_fkey,
    ADD  CONSTRAINT ai_feedback_datasource_id_fkey
        FOREIGN KEY (datasource_id) REFERENCES datasources(id);
