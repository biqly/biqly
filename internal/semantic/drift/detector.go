// Package drift detects semantic model schema drift against datasource metadata and notifies workspace owners.
package drift

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

// Detector compares semantic models with actual physical table schemas.
type Detector struct{}

// expressionParser parses a calculated-expression / metric string into an AST.
type expressionParser = func(string) (pkgsemantic.ExprNode, error)

// NewDetector creates a new Detector.
func NewDetector() *Detector {
	return &Detector{}
}

// Compare checks a model against the current state of tables and columns, returning a report.
func (d *Detector) Compare(_ context.Context, model semantic.SemanticModel, columns []metadata.Column, tables []metadata.Table) (*DriftReport, error) {
	parser := semantic.CurrentExpressionParser()
	colMap := d.buildColumnMap(columns)
	tableMap := d.buildTableMap(tables)

	drifts := d.checkBaseSchemaTable(model, tables, tableMap)
	drifts = append(drifts, d.checkDimensions(model, colMap, parser)...)
	drifts = append(drifts, d.checkMetrics(model, colMap, parser)...)
	drifts = append(drifts, d.checkJoins(model, colMap, tableMap)...)
	drifts = append(drifts, d.checkAddedColumns(model, columns)...)

	return d.buildReport(model, drifts), nil
}

func (d *Detector) buildColumnMap(columns []metadata.Column) map[string]metadata.Column {
	colMap := make(map[string]metadata.Column, len(columns))
	for _, col := range columns {
		colMap[d.normalizeKey(col.SchemaName, col.TableName, col.ColumnName)] = col
	}
	return colMap
}

func (d *Detector) buildTableMap(tables []metadata.Table) map[string]metadata.Table {
	tableMap := make(map[string]metadata.Table, len(tables))
	for _, tbl := range tables {
		tableMap[d.normalizeKey(tbl.SchemaName, tbl.TableName, "")] = tbl
	}
	return tableMap
}

// checkBaseSchemaTable verifies the model's base schema and table still exist.
func (d *Detector) checkBaseSchemaTable(model semantic.SemanticModel, tables []metadata.Table, tableMap map[string]metadata.Table) []DriftItem {
	schemaExists := false
	for _, tbl := range tables {
		if strings.EqualFold(tbl.SchemaName, model.BaseSchema) {
			schemaExists = true
			break
		}
	}

	if !schemaExists {
		return []DriftItem{{
			Type:        DriftTypeSchemaDropped,
			Field:       "",
			ColumnRef:   model.BaseSchema,
			NewValue:    "dropped",
			Description: fmt.Sprintf("Base schema %q no longer exists", model.BaseSchema),
		}}
	}

	baseTableKey := d.normalizeKey(model.BaseSchema, model.BaseTable, "")
	if _, exists := tableMap[baseTableKey]; !exists {
		return []DriftItem{{
			Type:        DriftTypeTableDropped,
			Field:       "",
			ColumnRef:   fmt.Sprintf("%s.%s", model.BaseSchema, model.BaseTable),
			NewValue:    "dropped",
			Description: fmt.Sprintf("Base table %q no longer exists in schema %q", model.BaseTable, model.BaseSchema),
		}}
	}
	return nil
}

// checkDimensions validates each active dimension's column (or calculated
// expression dependencies) against the physical schema.
func (d *Detector) checkDimensions(model semantic.SemanticModel, colMap map[string]metadata.Column, parser expressionParser) []DriftItem {
	var drifts []DriftItem
	for _, dim := range model.Dimensions {
		if !dim.IsActive {
			continue
		}
		if strings.TrimSpace(dim.CalculatedExpression) != "" {
			drifts = append(drifts, d.calculatedDimensionDrifts(model, dim, colMap, parser)...)
			continue
		}
		drifts = append(drifts, d.regularDimensionDrifts(model, dim, colMap)...)
	}
	return drifts
}

// calculatedDimensionDrifts reports columns referenced by a calculated
// dimension expression that no longer exist.
func (d *Detector) calculatedDimensionDrifts(model semantic.SemanticModel, dim semantic.Dimension, colMap map[string]metadata.Column, parser expressionParser) []DriftItem {
	expr := dim.CalculatedExpr
	if expr == nil && parser != nil {
		if parsed, err := parser(dim.CalculatedExpression); err == nil {
			expr = parsed
		}
	}
	if expr == nil {
		return nil
	}

	var drifts []DriftItem
	depCols, _, _ := pkgsemantic.ExprDependencies(expr)
	for _, depCol := range depCols {
		refSchema, refTable, refCol := d.parseColumnRef(exprDepRef(depCol), model.BaseSchema, model.BaseTable)
		if _, exists := colMap[d.normalizeKey(refSchema, refTable, refCol)]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeColumnDropped,
				Field:       dim.Name,
				ColumnRef:   fmt.Sprintf("%s.%s.%s", refSchema, refTable, refCol),
				NewValue:    "dropped",
				Description: fmt.Sprintf("Dimension calculated expression references dropped column %q", depCol.Column),
			})
		}
	}
	return drifts
}

// regularDimensionDrifts reports a dropped or type-incompatible physical column
// backing a non-calculated dimension.
func (d *Detector) regularDimensionDrifts(model semantic.SemanticModel, dim semantic.Dimension, colMap map[string]metadata.Column) []DriftItem {
	refSchema, refTable, refCol := d.parseColumnRef(dim.ColumnRef, model.BaseSchema, model.BaseTable)
	dbCol, exists := colMap[d.normalizeKey(refSchema, refTable, refCol)]
	if !exists {
		return []DriftItem{{
			Type:        DriftTypeColumnDropped,
			Field:       dim.Name,
			ColumnRef:   fmt.Sprintf("%s.%s.%s", refSchema, refTable, refCol),
			NewValue:    "dropped",
			Description: fmt.Sprintf("Column %q no longer exists for dimension %q", refCol, dim.Name),
		}}
	}
	if !d.isTypeCompatible(dbCol.DataType, dim.Type) {
		return []DriftItem{{
			Type:        DriftTypeTypeChanged,
			Field:       dim.Name,
			ColumnRef:   fmt.Sprintf("%s.%s.%s", refSchema, refTable, refCol),
			OldValue:    dim.Type,
			NewValue:    dbCol.DataType,
			Description: fmt.Sprintf("Column %q type changed from %q to incompatible physical type %q for dimension %q", refCol, dim.Type, dbCol.DataType, dim.Name),
		}}
	}
	return nil
}

// checkMetrics reports metric expressions that reference dropped columns.
func (d *Detector) checkMetrics(model semantic.SemanticModel, colMap map[string]metadata.Column, parser expressionParser) []DriftItem {
	var drifts []DriftItem
	for _, met := range model.Metrics {
		if !met.IsActive {
			continue
		}
		expr := met.Expr
		if expr == nil && parser != nil {
			if parsed, err := parser(met.Expression); err == nil {
				expr = parsed
			}
		}
		if expr == nil {
			continue
		}
		depCols, _, _ := pkgsemantic.ExprDependencies(expr)
		for _, depCol := range depCols {
			refSchema, refTable, refCol := d.parseColumnRef(exprDepRef(depCol), model.BaseSchema, model.BaseTable)
			if _, exists := colMap[d.normalizeKey(refSchema, refTable, refCol)]; !exists {
				drifts = append(drifts, DriftItem{
					Type:        DriftTypeMetricBroken,
					Field:       met.Name,
					ColumnRef:   fmt.Sprintf("%s.%s.%s", refSchema, refTable, refCol),
					NewValue:    "dropped",
					Description: fmt.Sprintf("Metric %q references dropped column %q", met.Name, depCol.Column),
				})
			}
		}
	}
	return drifts
}

// checkJoins validates that each active join's source/target tables and columns
// still exist.
func (d *Detector) checkJoins(model semantic.SemanticModel, colMap map[string]metadata.Column, tableMap map[string]metadata.Table) []DriftItem {
	var drifts []DriftItem
	for _, join := range model.Joins {
		if !join.IsActive {
			continue
		}
		fromSchema := join.FromSchema
		if fromSchema == "" {
			fromSchema = model.BaseSchema
		}
		toSchema := join.ToSchema
		if toSchema == "" {
			toSchema = model.BaseSchema
		}

		if _, exists := tableMap[d.normalizeKey(fromSchema, join.FromTable, "")]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s", fromSchema, join.FromTable),
				Description: fmt.Sprintf("Join %q references missing source table %s.%s", join.Name, fromSchema, join.FromTable),
			})
			continue
		}
		if _, exists := tableMap[d.normalizeKey(toSchema, join.ToTable, "")]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s", toSchema, join.ToTable),
				Description: fmt.Sprintf("Join %q references missing target table %s.%s", join.Name, toSchema, join.ToTable),
			})
			continue
		}

		if _, exists := colMap[d.normalizeKey(fromSchema, join.FromTable, join.FromColumn)]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s.%s", fromSchema, join.FromTable, join.FromColumn),
				Description: fmt.Sprintf("Join %q references missing source column %q", join.Name, join.FromColumn),
			})
		}
		if _, exists := colMap[d.normalizeKey(toSchema, join.ToTable, join.ToColumn)]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s.%s", toSchema, join.ToTable, join.ToColumn),
				Description: fmt.Sprintf("Join %q references missing target column %q", join.Name, join.ToColumn),
			})
		}
	}
	return drifts
}

// checkAddedColumns reports physical base-table columns not yet modeled (informational).
func (d *Detector) checkAddedColumns(model semantic.SemanticModel, columns []metadata.Column) []DriftItem {
	mappedColumns := make(map[string]bool)
	for _, dim := range model.Dimensions {
		if !dim.IsActive || strings.TrimSpace(dim.CalculatedExpression) != "" {
			continue
		}
		refSchema, refTable, refCol := d.parseColumnRef(dim.ColumnRef, model.BaseSchema, model.BaseTable)
		if strings.EqualFold(refSchema, model.BaseSchema) && strings.EqualFold(refTable, model.BaseTable) {
			mappedColumns[strings.ToLower(refCol)] = true
		}
	}

	var drifts []DriftItem
	for _, col := range columns {
		if !strings.EqualFold(col.SchemaName, model.BaseSchema) || !strings.EqualFold(col.TableName, model.BaseTable) {
			continue
		}
		if !mappedColumns[strings.ToLower(col.ColumnName)] {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeColumnAdded,
				Field:       "",
				ColumnRef:   fmt.Sprintf("%s.%s.%s", col.SchemaName, col.TableName, col.ColumnName),
				NewValue:    col.DataType,
				Description: fmt.Sprintf("New column %q found in table %q (not yet modeled)", col.ColumnName, model.BaseTable),
			})
		}
	}
	return drifts
}

// buildReport assembles a DriftReport from the collected drift items, computing
// the worst severity and resolved flag.
func (*Detector) buildReport(model semantic.SemanticModel, drifts []DriftItem) *DriftReport {
	if len(drifts) == 0 {
		return &DriftReport{
			ModelID:      model.ID,
			DatasourceID: model.DatasourceID,
			DetectedAt:   time.Now(),
			Severity:     SeverityInfo,
			Resolved:     true,
		}
	}

	worstSeverity := SeverityInfo
	for _, item := range drifts {
		switch GetDriftSeverity(item.Type) {
		case SeverityCritical:
			worstSeverity = SeverityCritical
		case SeverityWarning:
			if worstSeverity != SeverityCritical {
				worstSeverity = SeverityWarning
			}
		case SeverityInfo:
		}
	}

	return &DriftReport{
		ModelID:      model.ID,
		DatasourceID: model.DatasourceID,
		DetectedAt:   time.Now(),
		Drifts:       drifts,
		Severity:     worstSeverity,
		Resolved:     false,
	}
}

// exprDepRef renders an expression column dependency as a "table.column" (or
// bare "column") reference string.
func exprDepRef(dep pkgsemantic.ColumnRefExpr) string {
	if dep.Table != "" {
		return dep.Table + "." + dep.Column
	}
	return dep.Column
}

func (*Detector) normalizeKey(schema, table, column string) string {
	return strings.ToLower(
		strings.ReplaceAll(schema, "\"", "") + "." +
			strings.ReplaceAll(table, "\"", "") + "." +
			strings.ReplaceAll(column, "\"", ""),
	)
}

func (*Detector) parseColumnRef(ref string, defaultSchema, defaultTable string) (schema, table, column string) {
	ref = strings.ReplaceAll(ref, "\"", "")
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return defaultSchema, parts[0], parts[1]
	default:
		return defaultSchema, defaultTable, ref
	}
}

// isTypeCompatible checks if the database column's physical type is compatible with semantic dimension type.
func (*Detector) isTypeCompatible(physicalType, semanticType string) bool {
	p := strings.ToLower(physicalType)
	s := strings.ToLower(semanticType)

	switch s {
	case "number":
		// Matches int, float, decimal, numeric, double, real, serial, etc.
		return strings.Contains(p, "int") || strings.Contains(p, "num") || strings.Contains(p, "float") ||
			strings.Contains(p, "dec") || strings.Contains(p, "double") || strings.Contains(p, "real") ||
			strings.Contains(p, "serial") || strings.Contains(p, "precision")
	case "date":
		// Matches date, time, timestamp, interval, datetime, etc.
		return strings.Contains(p, "date") || strings.Contains(p, "time") || strings.Contains(p, "interval")
	case "boolean":
		return strings.Contains(p, "bool") || p == "bit" || p == "tinyint"
	case "geo":
		// Matches geometry, geography, point, polygon, etc.
		return strings.Contains(p, "geom") || strings.Contains(p, "geog") || strings.Contains(p, "point") ||
			strings.Contains(p, "polygon") || strings.Contains(p, "line")
	case "text":
		// Matches varchar, char, text, uuid, json, clob, xml, string, etc.
		return strings.Contains(p, "char") || strings.Contains(p, "text") || strings.Contains(p, "uuid") ||
			strings.Contains(p, "json") || strings.Contains(p, "xml") || strings.Contains(p, "clob") ||
			strings.Contains(p, "string")
	default:
		return true
	}
}
