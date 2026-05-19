package query

import (
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

// Planner analyzes a LogicalQuery and determines the optimal execution plan.
type Planner struct{}

// NewPlanner creates a new query planner.
func NewPlanner() *Planner {
	return &Planner{}
}

// PlanResult holds the planning output.
type PlanResult struct {
	RequiredJoins []string  `json:"required_joins"`
	Warnings      []string  `json:"warnings"`
	TableGraph    []TableNode `json:"table_graph"`
}

// TableNode represents a table in the join graph.
type TableNode struct {
	Name         string   `json:"name"`
	Columns      []string `json:"columns"`
	IsBaseTable  bool     `json:"is_base_table"`
}

// Plan analyzes a LogicalQuery and returns a plan.
//nolint:gocyclo // step-by-step planning process with independent phases
func (p *Planner) Plan(lq LogicalQuery, model *semantic.SemanticModel) (*PlanResult, error) {
	warnings := make([]string, 0, 4)
	var requiredJoins []string

	// Build dimension map
	dimMap := make(map[string]semantic.Dimension)
	for _, d := range model.Dimensions {
		dimMap[d.Name] = d
	}

	// Build metric map
	metricMap := make(map[string]semantic.Metric)
	for _, m := range model.Metrics {
		metricMap[m.Name] = m
	}

	// Determine which tables are needed
	tables := make(map[string]bool)
	tables[model.BaseTable] = true // Always include base table

	// Check select items
	for _, item := range lq.Select {
		switch item.Type {
		case SelectTypeDimension:
			if dim, ok := dimMap[item.Name]; ok {
				table := extractTable(dim.ColumnRef, model.BaseSchema)
				if table != "" && table != model.BaseTable {
					tables[table] = true
				}
			}
		case SelectTypeMetric:
			if metric, ok := metricMap[item.Name]; ok {
				table := extractTable(metric.Expression, model.BaseSchema)
				if table != "" && table != model.BaseTable {
					tables[table] = true
				}
			}
		}
	}

	// Check filters
	for _, f := range lq.Filters {
		if dim, ok := dimMap[f.Field]; ok {
			table := extractTable(dim.ColumnRef, model.BaseSchema)
			if table != "" && table != model.BaseTable {
				tables[table] = true
			}
		}
	}

	// Check group by
	for _, gb := range lq.GroupBy {
		if dim, ok := dimMap[gb.Field]; ok {
			table := extractTable(dim.ColumnRef, model.BaseSchema)
			if table != "" && table != model.BaseTable {
				tables[table] = true
			}
		}
	}

	// Determine required joins
	for _, join := range model.Joins {
		if tables[join.FromTable] || tables[join.ToTable] {
			requiredJoins = append(requiredJoins, join.Name)
		}
	}

	// Validate cardinality and detect fanout risks
	fanoutWarnings := p.checkFanout(model, tables)
	warnings = append(warnings, fanoutWarnings...)

	// Check for invalid metric/dimension combinations
	aggWarnings := p.checkAggregations(lq)
	warnings = append(warnings, aggWarnings...)

	// Build table graph
	var tableGraph []TableNode
	tableGraph = append(tableGraph, TableNode{
		Name:        model.BaseTable,
		IsBaseTable: true,
	})
	for table := range tables {
		if table != model.BaseTable {
			tableGraph = append(tableGraph, TableNode{
				Name:        table,
				IsBaseTable: false,
			})
		}
	}

	return &PlanResult{
		RequiredJoins: requiredJoins,
		Warnings:      warnings,
		TableGraph:    tableGraph,
	}, nil
}

// checkFanout detects potential fanout issues from many-to-many or multiple many-to-one joins.
func (p *Planner) checkFanout(model *semantic.SemanticModel, tables map[string]bool) []string {
	var warnings []string

	manyToManyCount := 0
	manyToOneCount := 0
	oneToManyCount := 0
	var riskyJoins []string

	for _, join := range model.Joins {
		if !tables[join.FromTable] && !tables[join.ToTable] {
			continue
		}
		switch join.Relationship {
		case semantic.RelationshipManyToMany:
			manyToManyCount++
			riskyJoins = append(riskyJoins, join.Name)
		case semantic.RelationshipOneToMany:
			oneToManyCount++
			riskyJoins = append(riskyJoins, join.Name)
		case semantic.RelationshipManyToOne, semantic.RelationshipOneToOne:
			manyToOneCount++
		}
	}

	if manyToManyCount > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"fanout risk: query involves %d many-to-many join(s) [%s] — aggregation results may be inflated",
			manyToManyCount, strings.Join(riskyJoins, ", "),
		))
	}

	if oneToManyCount > 0 && manyToManyCount == 0 {
		warnings = append(warnings, fmt.Sprintf(
			"fanout risk: query involves %d one-to-many join(s) [%s] — verify aggregation accuracy when grouping by base table columns",
			oneToManyCount, strings.Join(riskyJoins, ", "),
		))
	}

	// Multiple many-to-one joins from the same table can cause fanout
	if manyToOneCount > 2 {
		warnings = append(warnings, "multiple many-to-one joins detected - verify aggregation accuracy")
	}

	return warnings
}

// checkAggregations validates that metrics and dimensions can be safely combined.
func (p *Planner) checkAggregations(lq LogicalQuery) []string {
	var warnings []string

	hasMetrics := false
	for _, item := range lq.Select {
		if item.Type == SelectTypeMetric {
			hasMetrics = true
		}
	}

	// If metrics are selected without group_by, warn
	if hasMetrics && len(lq.GroupBy) == 0 {
		warnings = append(warnings, "metrics selected without GROUP BY - result will be a single aggregated row")
	}

	return warnings
}

// extractTable gets the table name from a column reference like "table.column" or "schema.table.column".
func extractTable(colRef, defaultSchema string) string {
	if p, ok := ParseColumnRef(colRef, defaultSchema); ok {
		return p.Table
	}
	return ""
}
