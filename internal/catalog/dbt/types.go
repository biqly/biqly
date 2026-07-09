// Package dbt converts dbt project artifacts (manifest.json + catalog.json)
// into biqly draft semantic models.
package dbt

// Manifest is the subset of dbt manifest.json (v7+/dbt 1.x) needed for import.
type Manifest struct {
	Metadata ManifestMetadata    `json:"metadata"`
	Nodes    map[string]Node     `json:"nodes"`
	Sources  map[string]Source   `json:"sources"`
	ChildMap map[string][]string `json:"child_map"`
}

// ManifestMetadata carries dbt version info.
type ManifestMetadata struct {
	DbtVersion  string `json:"dbt_version"`
	ProjectName string `json:"project_name"`
}

// Node is a dbt model, seed, or snapshot entry under nodes.
type Node struct {
	UniqueID     string            `json:"unique_id"`
	ResourceType string            `json:"resource_type"`
	Name         string            `json:"name"`
	PackageName  string            `json:"package_name"`
	Database     string            `json:"database"`
	Schema       string            `json:"schema"`
	Alias        string            `json:"alias"`
	Description  string            `json:"description"`
	Columns      map[string]Column `json:"columns"`
	Config       NodeConfig        `json:"config"`
	DependsOn    DependsOn         `json:"depends_on"`
	Meta         map[string]any    `json:"meta"`
	Tags         []string          `json:"tags"`
	// Test-specific fields (resource_type == "test").
	TestMetadata *TestMetadata `json:"test_metadata,omitempty"`
	AttachedNode string        `json:"attached_node,omitempty"`
	ColumnName   string        `json:"column_name,omitempty"`
}

// NodeConfig holds materialization and alias overrides.
type NodeConfig struct {
	Alias        string `json:"alias"`
	Schema       string `json:"schema"`
	Enabled      *bool  `json:"enabled"`
	Materialized string `json:"materialized"`
}

// DependsOn lists upstream node unique_ids.
type DependsOn struct {
	Nodes []string `json:"nodes"`
}

// Column is a documented column on a model or source.
type Column struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Meta        map[string]any `json:"meta"`
	DataType    string         `json:"data_type"`
	Tags        []string       `json:"tags"`
}

// Source is a dbt source table entry.
type Source struct {
	UniqueID    string            `json:"unique_id"`
	Name        string            `json:"name"`
	SourceName  string            `json:"source_name"`
	Database    string            `json:"database"`
	Schema      string            `json:"schema"`
	Identifier  string            `json:"identifier"`
	Description string            `json:"description"`
	Columns     map[string]Column `json:"columns"`
	Meta        map[string]any    `json:"meta"`
	Tags        []string          `json:"tags"`
}

// TestMetadata describes a generic dbt test (not_null, unique, relationships, accepted_values).
type TestMetadata struct {
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Kwargs    map[string]any `json:"kwargs"`
}

// Catalog is the subset of dbt catalog.json used for physical types.
type Catalog struct {
	Nodes   map[string]CatalogNode `json:"nodes"`
	Sources map[string]CatalogNode `json:"sources"`
}

// CatalogNode holds introspected column types for a relation.
type CatalogNode struct {
	UniqueID string                   `json:"unique_id"`
	Metadata CatalogNodeMeta          `json:"metadata"`
	Columns  map[string]CatalogColumn `json:"columns"`
}

// CatalogNodeMeta identifies the physical relation.
type CatalogNodeMeta struct {
	Type     string `json:"type"`
	Schema   string `json:"schema"`
	Name     string `json:"name"`
	Database string `json:"database"`
}

// CatalogColumn is a physical column type from the warehouse.
type CatalogColumn struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// RelationshipTest is a resolved relationships test between two models.
type RelationshipTest struct {
	FromUniqueID string
	FromColumn   string
	ToUniqueID   string
	ToColumn     string
}

// AcceptedValuesTest captures an accepted_values test for enum mapping.
type AcceptedValuesTest struct {
	ModelUniqueID string
	Column        string
	Values        []string
}

// ParsedProject is the normalized view of a dbt project after parsing.
type ParsedProject struct {
	Models         []Node
	Sources        []Source
	Relationships  []RelationshipTest
	AcceptedValues []AcceptedValuesTest
	NotNullColumns map[string]map[string]bool // model unique_id → column → true
	UniqueColumns  map[string]map[string]bool
	Warnings       []string
	DbtVersion     string
	ProjectName    string
}
