// Package semantic provides business-friendly semantic models over physical database tables.
package semantic

//revive:disable:exported // alias shim — canonical docs live in pkg/semantic

import (
	"strings"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

// SemanticModel and the sibling aliases below re-export pkg/semantic data
// structures so existing callers continue to import the legacy
// "internal/semantic" path. The behavioural types (MetricRegistry,
// repositories, publisher) remain in this package.
type (
	SemanticModel   = pkgsemantic.SemanticModel
	Dimension       = pkgsemantic.Dimension
	EnumMapping     = pkgsemantic.EnumMapping
	Metric          = pkgsemantic.Metric
	Join            = pkgsemantic.Join
	ModelField      = pkgsemantic.ModelField
	DimensionType   = pkgsemantic.DimensionType
	AggregationType = pkgsemantic.AggregationType

	CompositeModel              = pkgsemantic.CompositeModel
	ComponentModelRef           = pkgsemantic.ComponentModelRef
	CrossModelJoin              = pkgsemantic.CrossModelJoin
	CanonicalDateRef            = pkgsemantic.CanonicalDateRef
	DimensionConflictResolution = pkgsemantic.DimensionConflictResolution
)

// Re-exported composite model role and conflict resolution constants.
const (
	ComponentRolePrimary         = pkgsemantic.ComponentRolePrimary
	ComponentRoleSecondary       = pkgsemantic.ComponentRoleSecondary
	ConflictResolutionUsePrimary = pkgsemantic.ConflictResolutionUsePrimary
	ConflictResolutionRename     = pkgsemantic.ConflictResolutionRename
	ConflictResolutionMerge      = pkgsemantic.ConflictResolutionMerge
)

// Re-exported dimension types.
const (
	DimensionTypeText    = pkgsemantic.DimensionTypeText
	DimensionTypeNumber  = pkgsemantic.DimensionTypeNumber
	DimensionTypeDate    = pkgsemantic.DimensionTypeDate
	DimensionTypeBoolean = pkgsemantic.DimensionTypeBoolean
	DimensionTypeGeo     = pkgsemantic.DimensionTypeGeo
)

// Re-exported aggregation functions.
const (
	AggCount         = pkgsemantic.AggCount
	AggSum           = pkgsemantic.AggSum
	AggAvg           = pkgsemantic.AggAvg
	AggMin           = pkgsemantic.AggMin
	AggMax           = pkgsemantic.AggMax
	AggCountDistinct = pkgsemantic.AggCountDistinct
)

// Re-exported metric rate behaviors.
const (
	RateBehaviorRatioOfSums            = pkgsemantic.RateBehaviorRatioOfSums
	RateBehaviorAverageOfCustomerRates = pkgsemantic.RateBehaviorAverageOfCustomerRates
	RateBehaviorWeightedAverage        = pkgsemantic.RateBehaviorWeightedAverage
	RateBehaviorLatestValue            = pkgsemantic.RateBehaviorLatestValue
)

// Re-exported join defaults and relationship strings.
const (
	DefaultJoinType         = pkgsemantic.DefaultJoinType
	RelationshipManyToOne   = pkgsemantic.RelationshipManyToOne
	RelationshipOneToMany   = pkgsemantic.RelationshipOneToMany
	RelationshipOneToOne    = pkgsemantic.RelationshipOneToOne
	RelationshipManyToMany  = pkgsemantic.RelationshipManyToMany
	DefaultRelationshipType = pkgsemantic.DefaultRelationshipType
)

// MetricRegistry provides a single source of truth for metric definitions,
// used by the AI prompt builder, query validator, and SQL compiler.
type MetricRegistry struct {
	byName    map[string]*Metric
	bySynonym map[string]*Metric // maps synonyms and aliases to canonical metric
}

// NewMetricRegistry builds a registry from a semantic model's metrics.
func NewMetricRegistry(metrics []Metric) *MetricRegistry {
	r := &MetricRegistry{
		byName:    make(map[string]*Metric, len(metrics)),
		bySynonym: make(map[string]*Metric, len(metrics)*2),
	}
	for i := range metrics {
		m := &metrics[i]
		key := strings.ToLower(m.Name)
		r.byName[key] = m
		for _, syn := range m.Synonyms {
			r.bySynonym[strings.ToLower(syn)] = m
		}
		if m.Label != nil {
			r.bySynonym[strings.ToLower(*m.Label)] = m
		}
	}
	return r
}

// Lookup finds a metric by name, synonym, or label. Returns nil if not found.
func (r *MetricRegistry) Lookup(name string) *Metric {
	key := strings.ToLower(strings.TrimSpace(name))
	if m, ok := r.byName[key]; ok {
		return m
	}
	if m, ok := r.bySynonym[key]; ok {
		return m
	}
	return nil
}

// Has returns true if the metric exists by any known name.
func (r *MetricRegistry) Has(name string) bool {
	return r.Lookup(name) != nil
}
