package metadata

import (
	"encoding/json"
	"time"
)

const (
	// DSNModeRaw uses the legacy single encrypted DSN string column.
	DSNModeRaw = "raw"
	// DSNModeStructured composes the DSN from structured fields at runtime.
	DSNModeStructured = "structured"
)

// Datasource represents a configured database connection.
//
// Helpers that need access to encrypted columns (RuntimeDSN, etc.) live as
// free functions in internal/metadata so this package stays dependency-free.
type Datasource struct {
	ID           string     `json:"id" db:"id"`
	Name         string     `json:"name" db:"name"`
	Type         string     `json:"type" db:"type"`
	DSNEncrypted string     `json:"dsn_encrypted,omitempty" db:"dsn_encrypted"`
	Config       string     `json:"-" db:"config"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	LastSyncAt   *time.Time `json:"last_sync_at" db:"last_sync_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`

	Host     *string `json:"host,omitempty"`
	Port     *int    `json:"port,omitempty"`
	Username *string `json:"username,omitempty"`

	PasswordEncrypted string          `json:"password_encrypted,omitempty" db:"password_encrypted"`
	DatabaseName      *string         `json:"database_name,omitempty"`
	SSLMode           *string         `json:"ssl_mode,omitempty"`
	ConnectionParams  json.RawMessage `json:"connection_params,omitempty"`
	DSNMode           string          `json:"dsn_mode,omitempty"`
}

// Schema represents a database schema within a datasource.
type Schema struct {
	ID           string    `json:"id" db:"id"`
	DatasourceID string    `json:"datasource_id" db:"datasource_id"`
	SchemaName   string    `json:"schema_name" db:"schema_name"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Table represents a database table.
type Table struct {
	ID           string    `json:"id" db:"id"`
	DatasourceID string    `json:"datasource_id" db:"datasource_id"`
	SchemaID     string    `json:"schema_id" db:"schema_id"`
	SchemaName   string    `json:"schema_name" db:"schema_name"`
	TableName    string    `json:"table_name" db:"table_name"`
	TableType    string    `json:"table_type" db:"table_type"`
	RowEstimate  *int64    `json:"row_estimate" db:"row_estimate"`
	Description  *string   `json:"description" db:"description"`
	Label        *string   `json:"label" db:"label"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Column represents a column within a table.
type Column struct {
	ID               string    `json:"id" db:"id"`
	DatasourceID     string    `json:"datasource_id" db:"datasource_id"`
	TableID          string    `json:"table_id" db:"table_id"`
	SchemaName       string    `json:"schema_name" db:"schema_name"`
	TableName        string    `json:"table_name" db:"table_name"`
	ColumnName       string    `json:"column_name" db:"column_name"`
	DataType         string    `json:"data_type" db:"data_type"`
	Nullable         bool      `json:"nullable" db:"nullable"`
	OrdinalPosition  *int      `json:"ordinal_position" db:"ordinal_position"`
	CharMaxLength    *int      `json:"character_maximum_length" db:"character_maximum_length"`
	NumericPrecision *int      `json:"numeric_precision" db:"numeric_precision"`
	NumericScale     *int      `json:"numeric_scale" db:"numeric_scale"`
	ColumnDefault    *string   `json:"column_default" db:"column_default"`
	Description      *string   `json:"description" db:"description"`
	IsPrimaryKey     bool      `json:"is_primary_key" db:"is_primary_key"`
	IsForeignKey     bool      `json:"is_foreign_key" db:"is_foreign_key"`
	ReferencedSchema *string   `json:"referenced_schema" db:"referenced_schema"`
	ReferencedTable  *string   `json:"referenced_table" db:"referenced_table"`
	ReferencedColumn *string   `json:"referenced_column" db:"referenced_column"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`

	PIIType            *string    `json:"pii_type,omitempty" db:"pii_type"`
	PIIConfidence      *float64   `json:"pii_confidence,omitempty" db:"pii_confidence"`
	PIIDetectedAt      *time.Time `json:"pii_detected_at,omitempty" db:"pii_detected_at"`
	PIIReviewedBy      *string    `json:"pii_reviewed_by,omitempty" db:"pii_reviewed_by"`
	PIIMaskingStrategy *string    `json:"pii_masking_strategy,omitempty" db:"pii_masking_strategy"`
}

// ColumnEmbedding pairs a fully-qualified column with its stored embedding.
// Columns that have never been embedded are excluded from ListColumnEmbeddings.
type ColumnEmbedding struct {
	SchemaName string
	TableName  string
	ColumnName string
	Model      string
	Embedding  []float32
}

// Relation represents a foreign key relationship.
type Relation struct {
	ID               string    `json:"id" db:"id"`
	DatasourceID     string    `json:"datasource_id" db:"datasource_id"`
	ConstraintName   string    `json:"constraint_name" db:"constraint_name"`
	FromSchema       string    `json:"from_schema" db:"from_schema"`
	FromTable        string    `json:"from_table" db:"from_table"`
	FromColumn       string    `json:"from_column" db:"from_column"`
	ToSchema         string    `json:"to_schema" db:"to_schema"`
	ToTable          string    `json:"to_table" db:"to_table"`
	ToColumn         string    `json:"to_column" db:"to_column"`
	RelationshipType string    `json:"relationship_type" db:"relationship_type"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// AIQueryHistoryEntry represents one natural-language AI query attempt.
type AIQueryHistoryEntry struct {
	ID                 string    `json:"id" db:"id"`
	DatasourceID       string    `json:"datasource_id" db:"datasource_id"`
	ModelID            *string   `json:"model_id" db:"model_id"`
	UserID             *string   `json:"user_id" db:"user_id"`
	Question           string    `json:"question" db:"question"`
	PromptContext      any       `json:"prompt_context" db:"prompt_context"`
	AIResponse         any       `json:"ai_response" db:"ai_response"`
	LogicalQuery       any       `json:"logical_query" db:"logical_query"`
	ConfidenceScore    *float64  `json:"confidence_score" db:"confidence_score"`
	Warnings           []string  `json:"warnings" db:"warnings"`
	OutcomeStatus      string    `json:"outcome_status" db:"outcome_status"`
	RetryCount         int       `json:"retry_count" db:"retry_count"`
	NeedsClarification bool      `json:"needs_clarification" db:"needs_clarification"`
	ModelUsed          *string   `json:"model_used" db:"model_used"`
	PromptTokens       *int      `json:"prompt_tokens" db:"prompt_tokens"`
	CompletionTokens   *int      `json:"completion_tokens" db:"completion_tokens"`
	TokenCount         *int      `json:"token_count" db:"token_count"`
	CostUSD            *float64  `json:"cost_usd" db:"cost_usd"`
	LatencyMs          *int      `json:"latency_ms" db:"latency_ms"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

// PermissionPolicyRecord captures a stored access policy as it lives in the
// metadata database. Internal-API consumers receive this shape when querying
// per-user permissions.
type PermissionPolicyRecord struct {
	DeniedFields []string
	RowFilters   []PermissionRowFilter
}

// PermissionRowFilter is the persisted form of a row-level filter used by
// permission enforcement.
type PermissionRowFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator,omitempty"`
	Value    any    `json:"value,omitempty"`
}

// PIIColumnAccess describes how a user may access a PII-annotated column.
type PIIColumnAccess struct {
	Access string `json:"access"` // "raw", "masked", "hidden"
}

// SecurityPolicy represents a row-level and column-level access control policy in the database.
type SecurityPolicy struct {
	ID            string                     `json:"id" db:"id"`
	UserID        string                     `json:"user_id" db:"user_id"`
	DatasourceID  string                     `json:"datasource_id" db:"datasource_id"`
	AllowedModels []string                   `json:"allowed_models" db:"allowed_models"`
	DeniedFields  []string                   `json:"denied_fields" db:"denied_fields"`
	RowFilters    []PermissionRowFilter      `json:"row_filters" db:"row_filters"`
	PIIPolicy     map[string]PIIColumnAccess `json:"pii_policy,omitempty" db:"pii_policy"`
	CreatedAt     time.Time                  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time                  `json:"updated_at" db:"updated_at"`
}
