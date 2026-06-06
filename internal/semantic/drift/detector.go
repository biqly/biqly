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

// NewDetector creates a new Detector.
func NewDetector() *Detector {
	return &Detector{}
}

// Compare checks a model against the current state of tables and columns, returning a report.
//
//nolint:gocyclo,gocognit,funlen // compares dimensions, metrics, joins, and tables with multiple drift types
func (d *Detector) Compare(_ context.Context, model semantic.SemanticModel, columns []metadata.Column, tables []metadata.Table) (*DriftReport, error) {
	expressionParser := semantic.CurrentExpressionParser()
	colMap := make(map[string]metadata.Column, len(columns))
	for _, col := range columns {
		key := d.normalizeKey(col.SchemaName, col.TableName, col.ColumnName)
		colMap[key] = col
	}

	tableMap := make(map[string]metadata.Table, len(tables))
	for _, tbl := range tables {
		key := d.normalizeKey(tbl.SchemaName, tbl.TableName, "")
		tableMap[key] = tbl
	}

	var drifts []DriftItem

	// 1. Check base schema and table existence
	baseTableKey := d.normalizeKey(model.BaseSchema, model.BaseTable, "")

	schemaExists := false
	for _, tbl := range tables {
		if strings.EqualFold(tbl.SchemaName, model.BaseSchema) {
			schemaExists = true
			break
		}
	}

	if !schemaExists {
		drifts = append(drifts, DriftItem{
			Type:        DriftTypeSchemaDropped,
			Field:       "",
			ColumnRef:   model.BaseSchema,
			NewValue:    "dropped",
			Description: fmt.Sprintf("Base schema %q no longer exists", model.BaseSchema),
		})
	} else {
		if _, exists := tableMap[baseTableKey]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeTableDropped,
				Field:       "",
				ColumnRef:   fmt.Sprintf("%s.%s", model.BaseSchema, model.BaseTable),
				NewValue:    "dropped",
				Description: fmt.Sprintf("Base table %q no longer exists in schema %q", model.BaseTable, model.BaseSchema),
			})
		}
	}

	// 2. Validate Dimensions
	for _, dim := range model.Dimensions {
		if !dim.IsActive {
			continue
		}

		// Handle calculated expression dependencies
		if strings.TrimSpace(dim.CalculatedExpression) != "" { //nolint:nestif
			expr := dim.CalculatedExpr
			if expr == nil && expressionParser != nil {
				if parsed, err := expressionParser(dim.CalculatedExpression); err == nil {
					expr = parsed
				}
			}
			if expr != nil {
				depCols, _, _ := pkgsemantic.ExprDependencies(expr)
				for _, depCol := range depCols {
					var ref string
					if depCol.Table != "" {
						ref = depCol.Table + "." + depCol.Column
					} else {
						ref = depCol.Column
					}
					refSchema, refTable, refCol := d.parseColumnRef(ref, model.BaseSchema, model.BaseTable)
					colKey := d.normalizeKey(refSchema, refTable, refCol)
					if _, exists := colMap[colKey]; !exists {
						drifts = append(drifts, DriftItem{
							Type:        DriftTypeColumnDropped,
							Field:       dim.Name,
							ColumnRef:   fmt.Sprintf("%s.%s.%s", refSchema, refTable, refCol),
							NewValue:    "dropped",
							Description: fmt.Sprintf("Dimension calculated expression references dropped column %q", depCol.Column),
						})
					}
				}
			}
			continue
		}

		// Regular column reference
		refSchema, refTable, refCol := d.parseColumnRef(dim.ColumnRef, model.BaseSchema, model.BaseTable)
		colKey := d.normalizeKey(refSchema, refTable, refCol)

		dbCol, exists := colMap[colKey]
		if !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeColumnDropped,
				Field:       dim.Name,
				ColumnRef:   fmt.Sprintf("%s.%s.%s", refSchema, refTable, refCol),
				NewValue:    "dropped",
				Description: fmt.Sprintf("Column %q no longer exists for dimension %q", refCol, dim.Name),
			})
		} else if !d.isTypeCompatible(dbCol.DataType, dim.Type) {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeTypeChanged,
				Field:       dim.Name,
				ColumnRef:   fmt.Sprintf("%s.%s.%s", refSchema, refTable, refCol),
				OldValue:    dim.Type,
				NewValue:    dbCol.DataType,
				Description: fmt.Sprintf("Column %q type changed from %q to incompatible physical type %q for dimension %q", refCol, dim.Type, dbCol.DataType, dim.Name),
			})
		}
	}

	// 3. Validate Metrics
	for _, met := range model.Metrics {
		if !met.IsActive {
			continue
		}

		expr := met.Expr
		if expr == nil && expressionParser != nil {
			if parsed, err := expressionParser(met.Expression); err == nil {
				expr = parsed
			}
		}

		if expr != nil {
			depCols, _, _ := pkgsemantic.ExprDependencies(expr)
			for _, depCol := range depCols {
				var ref string
				if depCol.Table != "" {
					ref = depCol.Table + "." + depCol.Column
				} else {
					ref = depCol.Column
				}
				refSchema, refTable, refCol := d.parseColumnRef(ref, model.BaseSchema, model.BaseTable)
				colKey := d.normalizeKey(refSchema, refTable, refCol)
				if _, exists := colMap[colKey]; !exists {
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
	}

	// 4. Validate Joins
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

		fromTableKey := d.normalizeKey(fromSchema, join.FromTable, "")
		toTableKey := d.normalizeKey(toSchema, join.ToTable, "")

		// Check if source/target tables exist
		if _, exists := tableMap[fromTableKey]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s", fromSchema, join.FromTable),
				Description: fmt.Sprintf("Join %q references missing source table %s.%s", join.Name, fromSchema, join.FromTable),
			})
			continue
		}
		if _, exists := tableMap[toTableKey]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s", toSchema, join.ToTable),
				Description: fmt.Sprintf("Join %q references missing target table %s.%s", join.Name, toSchema, join.ToTable),
			})
			continue
		}

		// Check columns
		fromColKey := d.normalizeKey(fromSchema, join.FromTable, join.FromColumn)
		toColKey := d.normalizeKey(toSchema, join.ToTable, join.ToColumn)

		if _, exists := colMap[fromColKey]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s.%s", fromSchema, join.FromTable, join.FromColumn),
				Description: fmt.Sprintf("Join %q references missing source column %q", join.Name, join.FromColumn),
			})
		}
		if _, exists := colMap[toColKey]; !exists {
			drifts = append(drifts, DriftItem{
				Type:        DriftTypeJoinBroken,
				Field:       join.Name,
				ColumnRef:   fmt.Sprintf("%s.%s.%s", toSchema, join.ToTable, join.ToColumn),
				Description: fmt.Sprintf("Join %q references missing target column %q", join.Name, join.ToColumn),
			})
		}
	}

	// 5. Check ColumnAdded (Informational)
	// Find columns in the base table that are NOT mapped in model dimensions or joins
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

	for _, col := range columns {
		if strings.EqualFold(col.SchemaName, model.BaseSchema) && strings.EqualFold(col.TableName, model.BaseTable) {
			colLower := strings.ToLower(col.ColumnName)
			if !mappedColumns[colLower] {
				drifts = append(drifts, DriftItem{
					Type:        DriftTypeColumnAdded,
					Field:       "",
					ColumnRef:   fmt.Sprintf("%s.%s.%s", col.SchemaName, col.TableName, col.ColumnName),
					NewValue:    col.DataType,
					Description: fmt.Sprintf("New column %q found in table %q (not yet modeled)", col.ColumnName, model.BaseTable),
				})
			}
		}
	}

	if len(drifts) == 0 {
		return &DriftReport{
			ModelID:      model.ID,
			DatasourceID: model.DatasourceID,
			DetectedAt:   time.Now(),
			Severity:     SeverityInfo,
			Resolved:     true,
		}, nil
	}

	// Determine worst severity
	worstSeverity := SeverityInfo
	for _, item := range drifts {
		itemSev := GetDriftSeverity(item.Type)
		if itemSev == SeverityCritical {
			worstSeverity = SeverityCritical
		} else if itemSev == SeverityWarning && worstSeverity != SeverityCritical {
			worstSeverity = SeverityWarning
		}
	}

	return &DriftReport{
		ModelID:      model.ID,
		DatasourceID: model.DatasourceID,
		DetectedAt:   time.Now(),
		Drifts:       drifts,
		Severity:     worstSeverity,
		Resolved:     false,
	}, nil
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
