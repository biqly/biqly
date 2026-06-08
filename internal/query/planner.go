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
	RequiredJoins []string    `json:"required_joins"`
	Warnings      []string    `json:"warnings"`
	TableGraph    []TableNode `json:"table_graph"`
}

// TableNode represents a table in the join graph.
type TableNode struct {
	Name        string   `json:"name"`
	Columns     []string `json:"columns"`
	IsBaseTable bool     `json:"is_base_table"`
}

// Plan analyzes a LogicalQuery and returns a plan.
func (p *Planner) Plan(lq *LogicalQuery, model *semantic.SemanticModel) (*PlanResult, error) {
	dimMap, metricMap := buildSemanticFieldMaps(model)
	tables := collectRequiredTables(lq, model, dimMap, metricMap)
	requiredJoins := requiredJoinNames(model, tables)

	warnings := make([]string, 0, 4)
	warnings = append(warnings, p.checkFanout(model, tables)...)
	warnings = append(warnings, p.checkAggregations(lq)...)

	return &PlanResult{
		RequiredJoins: requiredJoins,
		Warnings:      warnings,
		TableGraph:    buildTableGraph(model.BaseTable, tables),
	}, nil
}

func buildSemanticFieldMaps(model *semantic.SemanticModel) (map[string]*semantic.Dimension, map[string]*semantic.Metric) {
	dimMap := make(map[string]*semantic.Dimension, len(model.Dimensions))
	for i := range model.Dimensions {
		dimMap[model.Dimensions[i].Name] = &model.Dimensions[i]
	}
	metricMap := make(map[string]*semantic.Metric, len(model.Metrics))
	for i := range model.Metrics {
		metricMap[model.Metrics[i].Name] = &model.Metrics[i]
	}
	return dimMap, metricMap
}

func collectRequiredTables(lq *LogicalQuery, model *semantic.SemanticModel, dimMap map[string]*semantic.Dimension, metricMap map[string]*semantic.Metric) map[string]bool {
	tables := make(map[string]bool, len(model.Joins)+1)
	tables[model.BaseTable] = true
	markTableFromDimension := func(field string) {
		if dim, ok := dimMap[field]; ok {
			markNonBaseTable(tables, extractTable(dim.ColumnRef, model.BaseSchema), model.BaseTable)
		}
	}
	for _, item := range lq.Select {
		switch item.Type {
		case SelectTypeDimension:
			markTableFromDimension(item.Name)
		case SelectTypeMetric:
			if metric, ok := metricMap[item.Name]; ok {
				markNonBaseTable(tables, extractTable(metric.Expression, model.BaseSchema), model.BaseTable)
			}
		}
	}
	for _, f := range lq.Filters {
		markTableFromDimension(f.Field)
	}
	for _, gb := range lq.GroupBy {
		markTableFromDimension(gb.Field)
	}
	return tables
}

func markNonBaseTable(tables map[string]bool, table, baseTable string) {
	if table != "" && table != baseTable {
		tables[table] = true
	}
}

func requiredJoinNames(model *semantic.SemanticModel, tables map[string]bool) []string {
	requiredJoins := make([]string, 0, len(model.Joins))
	for _, join := range model.Joins {
		if tables[join.FromTable] || tables[join.ToTable] {
			requiredJoins = append(requiredJoins, join.Name)
		}
	}
	return requiredJoins
}

func buildTableGraph(baseTable string, tables map[string]bool) []TableNode {
	tableGraph := []TableNode{{Name: baseTable, IsBaseTable: true}}
	for table := range tables {
		if table != baseTable {
			tableGraph = append(tableGraph, TableNode{Name: table, IsBaseTable: false})
		}
	}
	return tableGraph
}

// checkFanout detects potential fanout issues from many-to-many or multiple many-to-one joins.
func (*Planner) checkFanout(model *semantic.SemanticModel, tables map[string]bool) []string {
	warnings := make([]string, 0, 4)

	manyToManyCount := 0
	manyToOneCount := 0
	oneToManyCount := 0
	riskyJoins := make([]string, 0, 4)

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
func (*Planner) checkAggregations(lq *LogicalQuery) []string {
	warnings := make([]string, 0, 4)

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
