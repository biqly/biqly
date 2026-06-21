-- 053_add_columns_updated_at.up.sql
-- Adds updated_at column to the columns table, matching the pattern used
-- by the tables table. The column is backfilled with now() on existing rows.
ALTER TABLE columns ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
