package metadata

const (
	datasourceSelectColumns = `id, name, type, dsn_encrypted, config, is_active, last_sync_at, created_at, updated_at,
		host, port, username, password_encrypted, database_name, ssl_mode, connection_params, dsn_mode`
	tableSelectColumns    = `id, datasource_id, schema_id, schema_name, table_name, table_type, row_estimate, description, label, display_expression, created_at, updated_at`
	columnSelectColumns   = `id, datasource_id, table_id, schema_name, table_name, column_name, data_type, nullable, ordinal_position, character_maximum_length, numeric_precision, numeric_scale, column_default, description, is_primary_key, is_foreign_key, referenced_schema, referenced_table, referenced_column, created_at, pii_type, pii_confidence, pii_detected_at, pii_reviewed_by, pii_masking_strategy`
	relationSelectColumns = `id, datasource_id, constraint_name, from_schema, from_table, from_column, to_schema, to_table, to_column, relationship_type, created_at`
)
