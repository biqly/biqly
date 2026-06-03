DROP INDEX IF EXISTS idx_columns_pii_type;
ALTER TABLE columns
    DROP CONSTRAINT IF EXISTS chk_columns_pii_confidence,
    DROP CONSTRAINT IF EXISTS chk_columns_pii_type,
    DROP COLUMN IF EXISTS pii_masking_strategy,
    DROP COLUMN IF EXISTS pii_reviewed_by,
    DROP COLUMN IF EXISTS pii_detected_at,
    DROP COLUMN IF EXISTS pii_confidence,
    DROP COLUMN IF EXISTS pii_type;
