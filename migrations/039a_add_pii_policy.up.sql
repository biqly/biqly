-- Per-user PII access policy keyed by qualified column name.
-- Shape: { "<schema.table.column>": { "access": "raw" | "masked" | "hidden" } }
ALTER TABLE permissions
    ADD COLUMN IF NOT EXISTS pii_policy JSONB NOT NULL DEFAULT '{}';
