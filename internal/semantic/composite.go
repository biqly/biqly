package semantic

import (
	"errors"
	"fmt"
	"strings"
)

// CompositeResolver flattens a CompositeModel and its component SemanticModels
// into a single merged SemanticModel that the existing compiler, validator and
// planner can consume unchanged. Component column references are fully
// qualified to schema.table.column so that secondary components living in a
// different base schema than the primary still resolve correctly.
type CompositeResolver struct{}

// NewCompositeResolver constructs a CompositeResolver.
func NewCompositeResolver() *CompositeResolver {
	return &CompositeResolver{}
}

// Resolve merges the composite model with its loaded component models (keyed by
// component alias) into a single SemanticModel. The returned model's base
// table is the primary component's base table; dimensions and metrics from all
// components are merged with conflict resolution applied; intra-model joins and
// cross-model joins are flattened into physical Join entries.
func (r *CompositeResolver) Resolve(composite *CompositeModel, components map[string]*SemanticModel) (*SemanticModel, error) {
	if composite == nil {
		return nil, errors.New("composite model is nil")
	}
	if len(composite.Components) == 0 {
		return nil, fmt.Errorf("composite model %q has no components", composite.Name)
	}

	primary, err := r.findPrimary(composite, components)
	if err != nil {
		return nil, err
	}

	merged := &SemanticModel{
		ID:           composite.ID,
		DatasourceID: composite.DatasourceID,
		Name:         composite.Name,
		Label:        composite.Label,
		Description:  composite.Description,
		BaseSchema:   primary.BaseSchema,
		BaseTable:    primary.BaseTable,
		IsActive:     composite.IsActive,
		Status:       composite.Status,
		Version:      composite.Version,
	}

	resolutions := indexResolutions(composite.ConflictResolutions)

	dims, err := r.mergeDimensions(composite, components, primary, resolutions)
	if err != nil {
		return nil, err
	}
	merged.Dimensions = dims

	metrics, err := r.mergeMetrics(composite, components)
	if err != nil {
		return nil, err
	}
	merged.Metrics = metrics

	joins, err := r.mergeJoins(composite, components)
	if err != nil {
		return nil, err
	}
	merged.Joins = joins

	r.applyCanonicalDate(composite, merged)

	return merged, nil
}

// findPrimary locates the primary component and returns its loaded model.
func (r *CompositeResolver) findPrimary(composite *CompositeModel, components map[string]*SemanticModel) (*SemanticModel, error) {
	var primaryAlias string
	for _, c := range composite.Components {
		if c.Role == ComponentRolePrimary {
			if primaryAlias != "" {
				return nil, fmt.Errorf("composite model %q has multiple primary components", composite.Name)
			}
			primaryAlias = c.Alias
		}
	}
	if primaryAlias == "" {
		// Fall back to the first declared component as primary.
		primaryAlias = composite.Components[0].Alias
	}
	model, ok := components[primaryAlias]
	if !ok || model == nil {
		return nil, fmt.Errorf("primary component %q not loaded for composite %q", primaryAlias, composite.Name)
	}
	return model, nil
}

// mergeDimensions unions all component dimensions, qualifying column refs and
// resolving duplicate names. Unresolved duplicates are deterministically
// disambiguated by prefixing the component alias so runtime never breaks; the
// publish validator surfaces them as warnings.
func (r *CompositeResolver) mergeDimensions(
	composite *CompositeModel,
	components map[string]*SemanticModel,
	primary *SemanticModel,
	resolutions map[string]DimensionConflictResolution,
) ([]Dimension, error) {
	var out []Dimension
	seen := make(map[string]string) // dimension name -> owning alias

	for _, comp := range composite.Components {
		model, ok := components[comp.Alias]
		if !ok || model == nil {
			return nil, fmt.Errorf("component %q not loaded for composite %q", comp.Alias, composite.Name)
		}
		isPrimary := model.BaseTable == primary.BaseTable && model.BaseSchema == primary.BaseSchema

		for i := range model.Dimensions {
			dim := model.Dimensions[i]
			dim.ColumnRef = qualifyRef(dim.ColumnRef, model.BaseSchema, model.BaseTable)

			if owner, dup := seen[dim.Name]; dup {
				name, keep := resolveDimensionConflict(dim.Name, comp.Alias, owner, isPrimary, resolutions[dim.Name])
				if !keep {
					continue
				}
				dim.Name = name
			}
			seen[dim.Name] = comp.Alias
			out = append(out, dim)
		}
	}
	return out, nil
}

// mergeMetrics unions all component metrics, qualifying expressions and
// disambiguating duplicate names by alias prefix.
func (r *CompositeResolver) mergeMetrics(
	composite *CompositeModel,
	components map[string]*SemanticModel,
) ([]Metric, error) {
	var out []Metric
	seen := make(map[string]struct{})

	for _, comp := range composite.Components {
		model, ok := components[comp.Alias]
		if !ok || model == nil {
			return nil, fmt.Errorf("component %q not loaded for composite %q", comp.Alias, composite.Name)
		}
		for i := range model.Metrics {
			metric := model.Metrics[i]
			metric.Expression = qualifyRef(metric.Expression, model.BaseSchema, model.BaseTable)
			if _, dup := seen[metric.Name]; dup {
				metric.Name = comp.Alias + "_" + metric.Name
			}
			seen[metric.Name] = struct{}{}
			out = append(out, metric)
		}
	}
	return out, nil
}

// mergeJoins flattens intra-model joins from every component plus the
// composite-level cross-model joins into physical Join entries with unique
// names.
func (r *CompositeResolver) mergeJoins(
	composite *CompositeModel,
	components map[string]*SemanticModel,
) ([]Join, error) {
	var out []Join
	names := make(map[string]struct{})

	addJoin := func(j Join) {
		name := j.Name
		for {
			if _, dup := names[name]; !dup {
				break
			}
			name += "_x"
		}
		j.Name = name
		names[name] = struct{}{}
		out = append(out, j)
	}

	for _, comp := range composite.Components {
		model := components[comp.Alias]
		if model == nil {
			continue
		}
		for _, j := range model.Joins {
			if j.FromSchema == "" {
				j.FromSchema = model.BaseSchema
			}
			if j.ToSchema == "" {
				j.ToSchema = model.BaseSchema
			}
			j.Name = comp.Alias + "_" + j.Name
			addJoin(j)
		}
	}

	for _, cj := range composite.CrossModelJoins {
		if !cj.IsActive {
			continue
		}
		physical, err := r.resolveCrossJoin(composite, components, cj)
		if err != nil {
			return nil, err
		}
		addJoin(physical)
	}

	return out, nil
}

// resolveCrossJoin translates a logical cross-model join (referencing component
// aliases and dimension names) into a physical Join.
func (r *CompositeResolver) resolveCrossJoin(
	composite *CompositeModel,
	components map[string]*SemanticModel,
	cj CrossModelJoin,
) (Join, error) {
	fromSchema, fromTable, fromColumn, err := r.dimensionEndpoint(components, cj.FromModel, cj.FromDimension)
	if err != nil {
		return Join{}, fmt.Errorf("composite %q cross join %q: %w", composite.Name, cj.Name, err)
	}
	toSchema, toTable, toColumn, err := r.dimensionEndpoint(components, cj.ToModel, cj.ToDimension)
	if err != nil {
		return Join{}, fmt.Errorf("composite %q cross join %q: %w", composite.Name, cj.Name, err)
	}

	joinType := cj.JoinType
	if joinType == "" {
		joinType = DefaultJoinType
	}
	relationship := cj.Relationship
	if relationship == "" {
		relationship = DefaultRelationshipType
	}

	return Join{
		Name:         "x_" + cj.Name,
		FromSchema:   fromSchema,
		FromTable:    fromTable,
		FromColumn:   fromColumn,
		ToSchema:     toSchema,
		ToTable:      toTable,
		ToColumn:     toColumn,
		JoinType:     joinType,
		Relationship: relationship,
		IsActive:     true,
	}, nil
}

// dimensionEndpoint resolves a (component alias, dimension name) pair to its
// physical schema/table/column.
func (r *CompositeResolver) dimensionEndpoint(
	components map[string]*SemanticModel,
	alias, dimensionName string,
) (schema, table, column string, err error) {
	model, ok := components[alias]
	if !ok || model == nil {
		return "", "", "", fmt.Errorf("component alias %q not found", alias)
	}
	for i := range model.Dimensions {
		if model.Dimensions[i].Name == dimensionName {
			ref := qualifyRef(model.Dimensions[i].ColumnRef, model.BaseSchema, model.BaseTable)
			s, t, c, ok := splitQualifiedRef(ref)
			if !ok {
				return "", "", "", fmt.Errorf("dimension %q has unparsable column ref %q", dimensionName, ref)
			}
			return s, t, c, nil
		}
	}
	return "", "", "", fmt.Errorf("dimension %q not found in component %q", dimensionName, alias)
}

// applyCanonicalDate marks the configured canonical date dimension. When the
// referenced dimension exists in the merged model it is left as-is; the
// canonical reference primarily guides prompt building and time-grain
// alignment downstream.
func (r *CompositeResolver) applyCanonicalDate(composite *CompositeModel, merged *SemanticModel) {
	if composite.CanonicalDate == nil {
		return
	}
	// No structural change required on the merged model today; the canonical
	// date is surfaced via the composite model itself. This hook exists so
	// future time-grain alignment can rewrite secondary date dimensions.
	_ = merged
}

// resolveDimensionConflict decides the final name (and whether to keep) for a
// duplicate dimension. owner is the alias that already claimed the name.
func resolveDimensionConflict(
	name, alias, owner string,
	isPrimary bool,
	rule DimensionConflictResolution,
) (string, bool) {
	switch rule.Resolution {
	case ConflictResolutionUsePrimary, ConflictResolutionMerge:
		// Keep whichever copy belongs to the primary; drop the rest.
		if isPrimary {
			return name, true
		}
		return "", false
	case ConflictResolutionRename:
		if rule.SourceAlias == alias && rule.TargetAlias != "" {
			return rule.TargetAlias, true
		}
		return alias + "_" + name, true
	default:
		// No explicit rule: deterministically disambiguate by alias prefix.
		_ = owner
		return alias + "_" + name, true
	}
}

func indexResolutions(rs []DimensionConflictResolution) map[string]DimensionConflictResolution {
	out := make(map[string]DimensionConflictResolution, len(rs))
	for _, r := range rs {
		out[r.DimensionName] = r
	}
	return out
}

// qualifyRef rewrites a column reference to fully qualified schema.table.column
// form using the component model's base schema and table for the missing parts.
// Expressions containing characters other than identifier/dot (e.g. calculated
// expressions, function calls) are returned unchanged.
func qualifyRef(ref, baseSchema, baseTable string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || !isSimpleColumnRef(ref) {
		return ref
	}
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 1:
		return joinRef(baseSchema, baseTable, parts[0])
	case 2:
		return joinRef(baseSchema, parts[0], parts[1])
	default:
		return ref
	}
}

func joinRef(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			cleaned = append(cleaned, p)
		}
	}
	return strings.Join(cleaned, ".")
}

// splitQualifiedRef splits a schema.table.column reference.
func splitQualifiedRef(ref string) (schema, table, column string, ok bool) {
	parts := strings.Split(strings.TrimSpace(ref), ".")
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2], true
	case 2:
		return "", parts[0], parts[1], true
	default:
		return "", "", "", false
	}
}

// isSimpleColumnRef reports whether ref is a plain dotted identifier path with
// no whitespace, operators or function calls.
func isSimpleColumnRef(ref string) bool {
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}
