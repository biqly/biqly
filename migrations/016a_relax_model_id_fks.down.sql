-- 016b_relax_model_id_fks.down.sql

ALTER TABLE query_history
    DROP CONSTRAINT IF EXISTS query_history_model_id_fkey,
    ADD CONSTRAINT query_history_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id);

ALTER TABLE query_saved
    DROP CONSTRAINT IF EXISTS query_saved_model_id_fkey,
    ADD CONSTRAINT query_saved_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id);

ALTER TABLE ai_query_history
    DROP CONSTRAINT IF EXISTS ai_query_history_model_id_fkey,
    ADD CONSTRAINT ai_query_history_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id);

ALTER TABLE few_shot_examples
    DROP CONSTRAINT IF EXISTS few_shot_examples_model_id_fkey,
    ADD CONSTRAINT few_shot_examples_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id);
