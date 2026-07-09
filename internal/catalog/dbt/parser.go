package dbt

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
)

// ParseManifest unmarshals a dbt manifest.json payload.
func ParseManifest(data []byte) (*Manifest, error) {
	if len(data) == 0 {
		return nil, errors.New("manifest is empty")
	}
	var m Manifest
	if err := sonic.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Nodes == nil {
		m.Nodes = map[string]Node{}
	}
	if m.Sources == nil {
		m.Sources = map[string]Source{}
	}
	return &m, nil
}

// ParseCatalog unmarshals a dbt catalog.json payload. An empty catalog is allowed
// (column types then fall back to manifest data_type / text).
func ParseCatalog(data []byte) (*Catalog, error) {
	if len(data) == 0 {
		return &Catalog{Nodes: map[string]CatalogNode{}, Sources: map[string]CatalogNode{}}, nil
	}
	var c Catalog
	if err := sonic.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	if c.Nodes == nil {
		c.Nodes = map[string]CatalogNode{}
	}
	if c.Sources == nil {
		c.Sources = map[string]CatalogNode{}
	}
	return &c, nil
}

// ParseProject combines manifest + catalog into a normalized ParsedProject.
func ParseProject(manifestData, catalogData []byte) (*ParsedProject, error) {
	manifest, err := ParseManifest(manifestData)
	if err != nil {
		return nil, err
	}
	catalog, err := ParseCatalog(catalogData)
	if err != nil {
		return nil, err
	}
	return BuildParsedProject(manifest, catalog), nil
}

// BuildParsedProject extracts models, sources, and tests from a parsed manifest,
// enriching column types from the catalog when available.
func BuildParsedProject(manifest *Manifest, catalog *Catalog) *ParsedProject {
	out := &ParsedProject{
		Models:         make([]Node, 0),
		Sources:        make([]Source, 0),
		Relationships:  make([]RelationshipTest, 0),
		AcceptedValues: make([]AcceptedValuesTest, 0),
		NotNullColumns: map[string]map[string]bool{},
		UniqueColumns:  map[string]map[string]bool{},
		Warnings:       make([]string, 0),
		DbtVersion:     manifest.Metadata.DbtVersion,
		ProjectName:    manifest.Metadata.ProjectName,
	}

	for id, node := range manifest.Nodes {
		node.UniqueID = firstNonEmpty(node.UniqueID, id)
		switch strings.ToLower(node.ResourceType) {
		case "model":
			if node.Config.Enabled != nil && !*node.Config.Enabled {
				out.Warnings = append(out.Warnings, fmt.Sprintf("skipped disabled model %q", node.UniqueID))
				continue
			}
			enrichNodeColumns(&node, catalog)
			out.Models = append(out.Models, node)
		case "test":
			collectTest(out, node)
		}
	}

	for id, src := range manifest.Sources {
		src.UniqueID = firstNonEmpty(src.UniqueID, id)
		enrichSourceColumns(&src, catalog)
		out.Sources = append(out.Sources, src)
	}

	return out
}

func enrichNodeColumns(node *Node, catalog *Catalog) {
	if catalog == nil || node.Columns == nil {
		return
	}
	cat, ok := catalog.Nodes[node.UniqueID]
	if !ok {
		return
	}
	for name, col := range node.Columns {
		if catCol, ok := cat.Columns[name]; ok && col.DataType == "" {
			col.DataType = catCol.Type
			node.Columns[name] = col
		}
	}
	// Add catalog-only columns that lack documentation in the manifest.
	for name, catCol := range cat.Columns {
		if _, exists := node.Columns[name]; exists {
			continue
		}
		if node.Columns == nil {
			node.Columns = map[string]Column{}
		}
		node.Columns[name] = Column{
			Name:     name,
			DataType: catCol.Type,
		}
	}
}

func enrichSourceColumns(src *Source, catalog *Catalog) {
	if catalog == nil {
		return
	}
	cat, ok := catalog.Sources[src.UniqueID]
	if !ok {
		return
	}
	for name, col := range src.Columns {
		if catCol, ok := cat.Columns[name]; ok && col.DataType == "" {
			col.DataType = catCol.Type
			src.Columns[name] = col
		}
	}
	for name, catCol := range cat.Columns {
		if _, exists := src.Columns[name]; exists {
			continue
		}
		if src.Columns == nil {
			src.Columns = map[string]Column{}
		}
		src.Columns[name] = Column{
			Name:     name,
			DataType: catCol.Type,
		}
	}
}

func collectTest(out *ParsedProject, node Node) {
	if node.TestMetadata == nil {
		return
	}
	name := strings.ToLower(node.TestMetadata.Name)
	kwargs := node.TestMetadata.Kwargs
	if kwargs == nil {
		kwargs = map[string]any{}
	}

	modelID := firstNonEmpty(node.AttachedNode, stringArg(kwargs, "model"))
	column := firstNonEmpty(node.ColumnName, stringArg(kwargs, "column_name"))

	switch name {
	case "not_null":
		if modelID == "" || column == "" {
			return
		}
		ensureNested(out.NotNullColumns, modelID)[column] = true
	case "unique":
		if modelID == "" || column == "" {
			return
		}
		ensureNested(out.UniqueColumns, modelID)[column] = true
	case "accepted_values":
		if modelID == "" || column == "" {
			return
		}
		vals := stringSliceArg(kwargs, "values")
		if len(vals) == 0 {
			return
		}
		out.AcceptedValues = append(out.AcceptedValues, AcceptedValuesTest{
			ModelUniqueID: modelID,
			Column:        column,
			Values:        vals,
		})
	case "relationships":
		toRef := stringArg(kwargs, "to")
		toCol := stringArg(kwargs, "field")
		fromCol := firstNonEmpty(column, stringArg(kwargs, "column_name"))
		fromID := modelID
		toID := resolveRefUniqueID(toRef)
		if fromID == "" || toID == "" || fromCol == "" || toCol == "" {
			out.Warnings = append(out.Warnings, fmt.Sprintf("skipped incomplete relationships test %q", node.UniqueID))
			return
		}
		out.Relationships = append(out.Relationships, RelationshipTest{
			FromUniqueID: fromID,
			FromColumn:   fromCol,
			ToUniqueID:   toID,
			ToColumn:     toCol,
		})
	}
}

func resolveRefUniqueID(toRef string) string {
	toRef = strings.TrimSpace(toRef)
	if toRef == "" {
		return ""
	}
	// Common forms: "ref('orders')" / "ref(\"orders\")" / already a unique_id.
	if strings.HasPrefix(toRef, "model.") || strings.HasPrefix(toRef, "source.") {
		return toRef
	}
	lower := strings.ToLower(toRef)
	if strings.HasPrefix(lower, "ref(") {
		inner := toRef[len("ref("):]
		inner = strings.TrimSuffix(strings.TrimSpace(inner), ")")
		inner = strings.Trim(inner, `'"`)
		if inner == "" {
			return ""
		}
		// Package-qualified unique_id is unknown without project name; leave as bare name
		// and let the converter match by model name.
		return "name:" + inner
	}
	return toRef
}

func ensureNested(m map[string]map[string]bool, key string) map[string]bool {
	inner, ok := m[key]
	if !ok {
		inner = map[string]bool{}
		m[key] = inner
	}
	return inner
}

func stringArg(kwargs map[string]any, key string) string {
	v, ok := kwargs[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func stringSliceArg(kwargs map[string]any, key string) []string {
	v, ok := kwargs[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case []string:
		return t
	default:
		return nil
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
