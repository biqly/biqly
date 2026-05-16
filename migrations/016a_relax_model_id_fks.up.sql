-- 016a_relax_model_id_fks.up.sql
-- Allow semantic_models to be deleted even when referenced by historical rows
-- (query_history, ai_query_history, query_saved, few_shot_examples). Without
-- this, the FK constraint blocks DELETE on semantic_models once any history
-- exists, and the user has no way to clean up unused models.

ALTER TABLE query_history
    DROP CONSTRAINT IF EXISTS query_history_model_id_fkey,
    ADD CONSTRAINT query_history_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id) ON DELETE SET NULL;

ALTER TABLE query_saved
    DROP CONSTRAINT IF EXISTS query_saved_model_id_fkey,
    ADD CONSTRAINT query_saved_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id) ON DELETE SET NULL;

ALTER TABLE ai_query_history
    DROP CONSTRAINT IF EXISTS ai_query_history_model_id_fkey,
    ADD CONSTRAINT ai_query_history_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id) ON DELETE SET NULL;

ALTER TABLE few_shot_examples
    DROP CONSTRAINT IF EXISTS few_shot_examples_model_id_fkey,
    ADD CONSTRAINT few_shot_examples_model_id_fkey
        FOREIGN KEY (model_id) REFERENCES semantic_models(id) ON DELETE SET NULL;
