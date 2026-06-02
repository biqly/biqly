package semantic

import "time"

// Component model roles within a composite model.
const (
	// ComponentRolePrimary marks the component whose base table becomes the
	// FROM clause root of the merged model. Exactly one component must be primary.
	ComponentRolePrimary = "primary"
	// ComponentRoleSecondary marks a component reachable only through cross-model joins.
	ComponentRoleSecondary = "secondary"
)

// Dimension conflict resolution strategies for composite models.
const (
	// ConflictResolutionUsePrimary keeps the dimension from the primary component
	// and drops the duplicate from secondary components.
	ConflictResolutionUsePrimary = "use_primary"
	// ConflictResolutionRename keeps both dimensions, renaming the conflicting one
	// from the source component to TargetAlias.
	ConflictResolutionRename = "rename"
	// ConflictResolutionMerge treats the duplicate dimensions as a single logical
	// field (only valid when they reference equivalent columns).
	ConflictResolutionMerge = "merge"
)

// CompositeModel combines multiple SemanticModels into a single cross-domain
// model so users can ask questions spanning several business domains (e.g.
// sales + campaign + customer). All component models must currently belong to
// the same datasource.
type CompositeModel struct {
	ID           string  `json:"id" db:"id"`
	DatasourceID string  `json:"datasource_id" db:"datasource_id"`
	Name         string  `json:"name" db:"name"`
	Label        *string `json:"label" db:"label"`
	Description  *string `json:"description" db:"description"`

	Components          []ComponentModelRef           `json:"components,omitempty"`
	CrossModelJoins     []CrossModelJoin              `json:"cross_model_joins,omitempty"`
	CanonicalDate       *CanonicalDateRef             `json:"canonical_date,omitempty"`
	ConflictResolutions []DimensionConflictResolution `json:"conflict_resolutions,omitempty"`

	IsActive       bool       `json:"is_active" db:"is_active"`
	Status         string     `json:"status" db:"status"`
	Version        int        `json:"version" db:"version"`
	PublishedAt    *time.Time `json:"published_at,omitempty" db:"published_at"`
	PublishedBy    *string    `json:"published_by,omitempty" db:"published_by"`
	DraftUpdatedAt time.Time  `json:"draft_updated_at" db:"draft_updated_at"`
	CreatedBy      *string    `json:"created_by" db:"created_by"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// ComponentModelRef references a SemanticModel participating in a composite model.
type ComponentModelRef struct {
	ID          string `json:"id" db:"id"`
	CompositeID string `json:"composite_id" db:"composite_id"`
	ModelID     string `json:"model_id" db:"model_id"`
	// Alias is a short, composite-unique handle for the component (e.g. "sales").
	Alias string `json:"alias" db:"alias"`
	// Role is ComponentRolePrimary or ComponentRoleSecondary.
	Role      string    `json:"role" db:"role"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CrossModelJoin defines how two component models are joined through their
// dimensions. It mirrors Join but operates on logical model aliases rather than
// physical tables.
type CrossModelJoin struct {
	ID          string `json:"id" db:"id"`
	CompositeID string `json:"composite_id" db:"composite_id"`
	Name        string `json:"name" db:"name"`
	// FromModel and ToModel reference ComponentModelRef.Alias values.
	FromModel     string    `json:"from_model" db:"from_alias"`
	FromDimension string    `json:"from_dimension" db:"from_dimension"`
	ToModel       string    `json:"to_model" db:"to_alias"`
	ToDimension   string    `json:"to_dimension" db:"to_dimension"`
	JoinType      string    `json:"join_type" db:"join_type"`       // LEFT, INNER, RIGHT
	Relationship  string    `json:"relationship" db:"relationship"` // many_to_one, one_to_many, one_to_one, many_to_many
	IsActive      bool      `json:"is_active" db:"is_active"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// CanonicalDateRef designates the shared date dimension used to align time
// grains across component models.
type CanonicalDateRef struct {
	ModelAlias    string `json:"model_alias"`
	DimensionName string `json:"dimension_name"`
}

// DimensionConflictResolution describes how a duplicate dimension name across
// component models is resolved during merge.
type DimensionConflictResolution struct {
	ID            string `json:"id" db:"id"`
	CompositeID   string `json:"composite_id" db:"composite_id"`
	DimensionName string `json:"dimension_name" db:"dimension_name"`
	// Resolution is one of ConflictResolutionUsePrimary, ConflictResolutionRename, ConflictResolutionMerge.
	Resolution  string `json:"resolution" db:"resolution"`
	SourceAlias string `json:"source_alias" db:"source_alias"`
	TargetAlias string `json:"target_alias" db:"target_alias"`
}
