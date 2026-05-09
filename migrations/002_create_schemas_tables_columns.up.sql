-- 002_create_schemas_tables_columns.up.sql
CREATE TABLE IF NOT EXISTS schemas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    schema_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(datasource_id, schema_name)
);

CREATE TABLE IF NOT EXISTS tables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    schema_id UUID NOT NULL REFERENCES schemas(id) ON DELETE CASCADE,
    schema_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    table_type TEXT NOT NULL DEFAULT 'BASE TABLE',
    row_estimate BIGINT,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(datasource_id, schema_name, table_name)
);

CREATE INDEX idx_tables_datasource ON tables(datasource_id);
CREATE INDEX idx_tables_schema ON tables(schema_id);

CREATE TABLE IF NOT EXISTS columns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    table_id UUID NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
    schema_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    column_name TEXT NOT NULL,
    data_type TEXT NOT NULL,
    nullable BOOLEAN NOT NULL DEFAULT true,
    ordinal_position INT,
    character_maximum_length INT,
    numeric_precision INT,
    numeric_scale INT,
    column_default TEXT,
    description TEXT,
    is_primary_key BOOLEAN NOT NULL DEFAULT false,
    is_foreign_key BOOLEAN NOT NULL DEFAULT false,
    referenced_schema TEXT,
    referenced_table TEXT,
    referenced_column TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(datasource_id, schema_name, table_name, column_name)
);

CREATE INDEX idx_columns_datasource ON columns(datasource_id);
CREATE INDEX idx_columns_table ON columns(table_id);
CREATE INDEX idx_columns_data_type ON columns(data_type);

CREATE TABLE IF NOT EXISTS relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    datasource_id UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
    constraint_name TEXT NOT NULL,
    from_schema TEXT NOT NULL,
    from_table TEXT NOT NULL,
    from_column TEXT NOT NULL,
    to_schema TEXT NOT NULL,
    to_table TEXT NOT NULL,
    to_column TEXT NOT NULL,
    relationship_type TEXT NOT NULL DEFAULT 'many_to_one',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(datasource_id, constraint_name)
);

CREATE INDEX idx_relations_datasource ON relations(datasource_id);
CREATE INDEX idx_relations_from ON relations(from_schema, from_table, from_column);
CREATE INDEX idx_relations_to ON relations(to_schema, to_table, to_column);
