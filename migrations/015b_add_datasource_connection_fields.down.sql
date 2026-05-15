DROP INDEX IF EXISTS idx_datasources_dsn_mode;

ALTER TABLE datasources
    DROP COLUMN IF EXISTS dsn_mode,
    DROP COLUMN IF EXISTS connection_params,
    DROP COLUMN IF EXISTS ssl_mode,
    DROP COLUMN IF EXISTS database_name,
    DROP COLUMN IF EXISTS password_encrypted,
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS port,
    DROP COLUMN IF EXISTS host;
