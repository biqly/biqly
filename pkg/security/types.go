package security

// PermissionPolicy defines what a user can access.
type PermissionPolicy struct {
	UserID        string      `json:"user_id"`
	DatasourceID  string      `json:"datasource_id"`
	AllowedModels []string    `json:"allowed_models,omitempty"`
	DeniedFields  []string    `json:"denied_fields,omitempty"`
	RowFilters    []RowFilter `json:"row_filters,omitempty"`
}

// RowFilter defines a mandatory filter to inject into queries.
type RowFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}
