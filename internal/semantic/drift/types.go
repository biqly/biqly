package drift

import "time"

// DriftType defines the classification of a schema mismatch.
type DriftType string

const (
	DriftTypeColumnDropped DriftType = "column_dropped"
	DriftTypeColumnAdded   DriftType = "column_added"
	DriftTypeTypeChanged   DriftType = "type_changed"
	DriftTypeTableDropped  DriftType = "table_dropped"
	DriftTypeSchemaDropped DriftType = "schema_dropped"
	DriftTypeJoinBroken    DriftType = "join_broken"
	DriftTypeMetricBroken  DriftType = "metric_broken"
)

// DriftReport groups all detected drifts for a model.
type DriftReport struct {
	ID           string      `json:"id" db:"id"`
	ModelID      string      `json:"model_id" db:"model_id"`
	DatasourceID string      `json:"datasource_id" db:"datasource_id"`
	SyncEventID  *string     `json:"sync_event_id,omitempty" db:"sync_event_id"`
	Severity     string      `json:"severity" db:"severity"` // "critical" | "warning" | "info"
	Drifts       []DriftItem `json:"drifts" db:"drifts"`
	Resolved     bool        `json:"resolved" db:"resolved"`
	ResolvedBy   *string     `json:"resolved_by,omitempty" db:"resolved_by"`
	ResolvedAt   *time.Time  `json:"resolved_at,omitempty" db:"resolved_at"`
	DetectedAt   time.Time   `json:"detected_at" db:"detected_at"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
}

// DriftItem details a single model-to-datasource mismatch.
type DriftItem struct {
	Type        DriftType `json:"type"`
	Field       string    `json:"field"`                  // Name of modeled dimension/metric
	ColumnRef   string    `json:"column_ref"`             // schema.table.column
	OldValue    string    `json:"old_value,omitempty"`    // E.g. old data type
	NewValue    string    `json:"new_value,omitempty"`    // E.g. new data type, or "dropped"
	Description string    `json:"description,omitempty"` // Human-readable explanation
}

// Severity levels.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// GetDriftSeverity maps a DriftType to its severity level.
func GetDriftSeverity(t DriftType) string {
	switch t {
	case DriftTypeColumnDropped, DriftTypeTableDropped, DriftTypeSchemaDropped:
		return SeverityCritical
	case DriftTypeTypeChanged, DriftTypeJoinBroken, DriftTypeMetricBroken:
		return SeverityWarning
	case DriftTypeColumnAdded:
		return SeverityInfo
	default:
		return SeverityInfo
	}
}
