-- 053_add_columns_updated_at.down.sql
ALTER TABLE columns DROP COLUMN IF EXISTS updated_at;
