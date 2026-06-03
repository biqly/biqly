-- PII column annotations: detection results and review state per column.
ALTER TABLE columns
    ADD COLUMN IF NOT EXISTS pii_type TEXT,
    ADD COLUMN IF NOT EXISTS pii_confidence DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS pii_detected_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS pii_reviewed_by TEXT,
    ADD COLUMN IF NOT EXISTS pii_masking_strategy TEXT;

ALTER TABLE columns
    ADD CONSTRAINT chk_columns_pii_type CHECK (
        pii_type IS NULL OR pii_type IN (
            'email', 'phone', 'iban', 'tc_kimlik_no', 'address', 'ip_address', 'credit_card_like'
        )
    );

ALTER TABLE columns
    ADD CONSTRAINT chk_columns_pii_confidence CHECK (
        pii_confidence IS NULL OR (pii_confidence >= 0.0 AND pii_confidence <= 1.0)
    );

CREATE INDEX IF NOT EXISTS idx_columns_pii_type
    ON columns (pii_type)
    WHERE pii_type IS NOT NULL;
