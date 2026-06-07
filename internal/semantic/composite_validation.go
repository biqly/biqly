package semantic

import (
	"strings"
)

func validateCompositeMetadata(composite *CompositeModel, addError func(string, ...any)) {
	if strings.TrimSpace(composite.Name) == "" {
		addError("composite name is required")
	}
	if len(composite.Components) < 2 {
		addError("composite model requires at least two component models")
	}
}

func validateCompositeComponentLayout(composite *CompositeModel, addError func(string, ...any)) map[string]bool {
	primaryCount := 0
	aliases := make(map[string]bool, len(composite.Components))
	for _, comp := range composite.Components {
		if aliases[comp.Alias] {
			addError("duplicate component alias %q", comp.Alias)
		}
		aliases[comp.Alias] = true
		if comp.Role == ComponentRolePrimary {
			primaryCount++
		}
	}
	if primaryCount == 0 {
		addError("composite model requires exactly one primary component; none found")
	}
	if primaryCount > 1 {
		addError("composite model requires exactly one primary component; found %d", primaryCount)
	}
	return aliases
}

func validateCompositeCrossJoins(
	composite *CompositeModel,
	aliases map[string]bool,
	components map[string]*SemanticModel,
	addError func(string, ...any),
) {
	for _, j := range composite.CrossModelJoins {
		if !j.IsActive {
			continue
		}
		if !aliases[j.FromModel] {
			addError("cross join %q references unknown alias %q", j.Name, j.FromModel)
		}
		if !aliases[j.ToModel] {
			addError("cross join %q references unknown alias %q", j.Name, j.ToModel)
		}
		if from, ok := components[j.FromModel]; ok && !dimensionExists(from, j.FromDimension) {
			addError("cross join %q references unknown dimension %q on %q", j.Name, j.FromDimension, j.FromModel)
		}
		if to, ok := components[j.ToModel]; ok && !dimensionExists(to, j.ToDimension) {
			addError("cross join %q references unknown dimension %q on %q", j.Name, j.ToDimension, j.ToModel)
		}
	}
}

func validateCompositeCanonicalDate(
	composite *CompositeModel,
	aliases map[string]bool,
	components map[string]*SemanticModel,
	addError, addWarning func(string, ...any),
) {
	if composite.CanonicalDate == nil {
		addWarning("no canonical date defined; cross-domain time filtering may be ambiguous")
		return
	}
	ref := composite.CanonicalDate
	if !aliases[ref.ModelAlias] {
		addError("canonical date references unknown alias %q", ref.ModelAlias)
		return
	}
	if model, ok := components[ref.ModelAlias]; ok && !dimensionExists(model, ref.DimensionName) {
		addError("canonical date references unknown dimension %q on %q", ref.DimensionName, ref.ModelAlias)
	}
}

func validateCompositeResolution(
	composite *CompositeModel,
	components map[string]*SemanticModel,
	sink *validationSink,
) (*SemanticModel, bool) {
	resolver := NewCompositeResolver()
	resolved, err := resolver.Resolve(composite, components)
	if err != nil {
		sink.addError("resolve composite: %s", err)
		return nil, false
	}

	graph := BuildMetricGraph(composite, resolved)
	if err := DetectCircularDependencies(graph); err != nil {
		sink.addError("metric dependency: %s", err)
	}
	appendCompositeFanoutWarnings(composite, sink.addWarning)
	sink.result.EstimatedPromptSize = estimatePromptSize(*resolved)
	return resolved, true
}

func appendCompositeFanoutWarnings(composite *CompositeModel, addWarning func(string, ...any)) {
	for _, j := range activeCrossJoins(composite.CrossModelJoins) {
		switch j.Relationship {
		case RelationshipManyToMany:
			addWarning("cross join %q is many_to_many; aggregated metrics may fan out and double-count", j.Name)
		case RelationshipOneToMany:
			addWarning("cross join %q is one_to_many; verify metric grain to avoid fanout", j.Name)
		}
	}
}
