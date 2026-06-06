package routing

import (
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

type bundleColumn struct {
	bundle tableBundle
	col    metadata.Column
}

func columnPriority(c metadata.Column) int {
	// List human-readable dimensions before raw identifiers so prompts and models
	// default to names/titles rather than PK/FK columns.
	switch {
	case isDisplayNameColumn(c.ColumnName):
		return 0
	case c.IsPrimaryKey:
		return 3
	case c.IsForeignKey:
		return 2
	default:
		return 1
	}
}

// sortedBundleColumns returns columns across selected tables in a stable, business-relevant order.
func sortedBundleColumns(selected []tableBundle, columnsByTable map[string][]metadata.Column) []bundleColumn {
	totalCols := 0
	for _, bundle := range selected {
		key := tableKey(bundle.table.SchemaName, bundle.table.TableName)
		totalCols += len(columnsByTable[key])
	}
	out := make([]bundleColumn, 0, totalCols)
	for _, bundle := range selected {
		key := tableKey(bundle.table.SchemaName, bundle.table.TableName)
		for _, col := range columnsByTable[key] {
			out = append(out, bundleColumn{bundle: bundle, col: col})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := columnPriority(out[i].col), columnPriority(out[j].col)
		if pi != pj {
			return pi < pj
		}
		li := tableLabel(out[i].bundle.table) + "." + out[i].col.ColumnName
		lj := tableLabel(out[j].bundle.table) + "." + out[j].col.ColumnName
		return li < lj
	})
	return out
}

// ErrTableScopeInvalid indicates that a manually provided table scope is invalid.
func buildSemanticModel(
	datasourceID string,
	selected []tableBundle,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
	limits Limits,
	timeGrains []metadata.TimeGrain,
) *semantic.SemanticModel {
	limits = limits.withDefaults()
	base := selected[0].table
	model := &semantic.SemanticModel{
		ID:           autoModelPrefix + strings.Join(bundleLabels(selected), ","),
		DatasourceID: datasourceID,
		Name:         autoModelPrefix + strings.Join(bundleLabels(selected), ","),
		Label:        new("Auto-detected tables"),
		Description:  new("Generated from datasource metadata for an AI question."),
		BaseSchema:   base.SchemaName,
		BaseTable:    base.TableName,
		IsActive:     true,
	}

	model.Dimensions = buildDimensions(selected, columnsByTable, limits, timeGrains)
	model.Metrics = buildMetrics(selected, columnsByTable, limits)
	model.Joins = buildJoins(selected, relations)
	return model
}

func isMandatorySemanticColumn(col metadata.Column, relationCols map[string]bool) bool {
	if col.IsPrimaryKey || col.IsForeignKey || isDateOrTimeType(col.DataType) || isDisplayNameColumn(col.ColumnName) {
		return true
	}
	if relationCols != nil && relationCols[col.ColumnName] {
		return true
	}
	return false
}

func relationColumnsForSelectedTables(relations []metadata.Relation, selectedKeys map[string]bool) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	add := func(tableKey, columnName string) {
		if out[tableKey] == nil {
			out[tableKey] = make(map[string]bool)
		}
		out[tableKey][columnName] = true
	}
	for _, rel := range relations {
		fromKey := tableKey(rel.FromSchema, rel.FromTable)
		toKey := tableKey(rel.ToSchema, rel.ToTable)
		if !selectedKeys[fromKey] || !selectedKeys[toKey] {
			continue
		}
		add(fromKey, rel.FromColumn)
		add(toKey, rel.ToColumn)
	}
	return out
}

func buildDimensions(selected []tableBundle, columnsByTable map[string][]metadata.Column, limits Limits, timeGrains []metadata.TimeGrain) []semantic.Dimension {
	limits = limits.withDefaults()
	maxDims := limits.MaxDimensions
	maxDateGrains := limits.MaxDateGrainExtras
	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)
	if len(pairs) > maxDims {
		pairs = pairs[:maxDims]
	}
	dimensions := make([]semantic.Dimension, 0, len(pairs))
	dateGrainAdded := 0
	for _, p := range pairs {
		if len(dimensions) >= maxDims {
			break
		}
		name := p.col.ColumnName
		if nameCounts[p.col.ColumnName] > 1 {
			name = p.bundle.table.TableName + "_" + p.col.ColumnName
		}
		colRef := p.bundle.table.TableName + "." + p.col.ColumnName
		syn := displayNameSynonyms(p.bundle.table.TableName, p.col.ColumnName)
		if sx := softDeleteColumnSynonyms(p.col.ColumnName, p.col.DataType); len(sx) > 0 {
			syn = append(syn, sx...)
		}
		dimensions = append(dimensions, semantic.Dimension{
			Name:        name,
			ColumnRef:   colRef,
			Type:        dimensionType(p.col.DataType),
			Description: p.col.Description,
			Synonyms:    syn,
			IsActive:    true,
		})
		if !isDateOrTimeType(p.col.DataType) || dateGrainAdded >= maxDateGrains {
			continue
		}
		hasTime := hasTimeComponent(p.col.DataType)
		for _, g := range timeGrains {
			if g.RequiresTime && !hasTime {
				continue
			}
			if len(dimensions) >= maxDims || dateGrainAdded >= maxDateGrains {
				break
			}
			dimensions = append(dimensions, semantic.Dimension{
				Name:        name + g.Suffix,
				ColumnRef:   colRef,
				Type:        string(semantic.DimensionTypeDate),
				TimeGrain:   g.Grain,
				Synonyms:    g.Synonyms,
				Description: p.col.Description,
				IsActive:    true,
			})
			dateGrainAdded++
		}
	}
	return dimensions
}

// softDeleteColumnSynonyms adds NL phrases so questions like "silinen tweet"
// map to deletion-indicator dimensions (deleted_at, is_deleted, delete_flag, …).
func softDeleteColumnSynonyms(columnName, dataType string) []string {
	n := strings.ToLower(strings.TrimSpace(columnName))
	t := strings.ToLower(strings.TrimSpace(dataType))
	isTimeish := strings.Contains(t, "timestamp") || strings.Contains(t, "timestamptz") ||
		t == "date"
	isBool := strings.Contains(t, "bool")
	isNum := isNumericType(t)

	tsDeleted := n == "deleted_at" || strings.HasSuffix(n, "_deleted_at") ||
		n == "removed_at" || strings.HasSuffix(n, "_removed_at")
	tsArchived := n == "archived_at" || strings.HasSuffix(n, "_archived_at")

	switch {
	case isTimeish && tsDeleted:
		return []string{
			"deleted", "removed", "trashed", "erased", "soft delete", "soft-delete",
			"silinen", "silinmiş", "silindi", "silinmis", "kaldırılan", "kaldirilan",
		}
	case isTimeish && tsArchived:
		return []string{
			"archived", "arşiv", "arsiv", "arşivlenmiş", "arsivlenmis",
			"deleted", "silinen", "kaldırılan", "kaldirilan",
		}
	case isBool && (n == "is_deleted" || n == "is_removed" || n == "is_archived" || n == "deleted"):
		return []string{
			"deleted", "removed", "archived", "silinen", "silinmiş", "silinmis", "silindi", "kaldırılan", "kaldirilan",
		}
	case isNum && (n == "delete_flag" || n == "deleted_flag" || n == "is_delete"):
		return []string{
			"deleted", "delete flag", "silinen", "silme bayrağı", "silme bayragi",
		}
	default:
		return nil
	}
}

// displayNameSynonyms tags human-readable label columns (name, title, label, ...)
// with the parent table's name and its known translations, so a question that
// refers to the entity generically ("customer" / "müşteri") routes to the
// display column instead of the primary key.
func displayNameSynonyms(tableName, columnName string) []string {
	if !isDisplayNameColumn(columnName) {
		return nil
	}
	base := singularize(strings.ToLower(tableName))
	seen := map[string]bool{}
	add := func(s string) []string {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || seen[s] {
			return nil
		}
		seen[s] = true
		return []string{s}
	}
	var out []string
	out = append(out, add(tableName)...)
	out = append(out, add(base)...)
	for _, syn := range activeRoutingLexicon().ExpandTokenSynonyms(base) {
		out = append(out, add(syn)...)
	}
	return out
}

func isDisplayNameColumn(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "title", "label", "username", "email":
		return true
	}
	// Matches name, firstname, lastname, middlename, surname, full_name,
	// display_name, store_name, *Name, etc.
	return strings.HasSuffix(n, "name")
}

func singularize(name string) string {
	switch {
	case strings.HasSuffix(name, "ies") && len(name) > 3:
		return strings.TrimSuffix(name, "ies") + "y"
	case strings.HasSuffix(name, "ses") && len(name) > 3:
		return strings.TrimSuffix(name, "es")
	case strings.HasSuffix(name, "s") && len(name) > 3:
		return strings.TrimSuffix(name, "s")
	default:
		return name
	}
}

func buildMetrics(selected []tableBundle, columnsByTable map[string][]metadata.Column, limits Limits) []semantic.Metric {
	limits = limits.withDefaults()
	maxMetrics := limits.MaxMetrics
	lex := activeRoutingLexicon()
	metrics := []semantic.Metric{{
		Name:        "row_count",
		Expression:  "*",
		Aggregation: string(semantic.AggCount),
		Synonyms:    append([]string(nil), lex.RowCountSynonyms...),
		IsActive:    true,
	}}

	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)

	appendMetric := func(m semantic.Metric) {
		if len(metrics) >= maxMetrics {
			return
		}
		metrics = append(metrics, m)
	}

	for _, p := range pairs {
		if len(metrics) >= maxMetrics {
			break
		}
		col := p.col
		name := col.ColumnName
		if nameCounts[col.ColumnName] > 1 {
			name = p.bundle.table.TableName + "_" + col.ColumnName
		}
		expression := p.bundle.table.TableName + "." + col.ColumnName
		switch {
		case isNumericType(col.DataType):
			appendMetric(metric("sum_"+name, expression, semantic.AggSum, col.Description, nil))
			if !limits.SlimNumericMetrics {
				appendMetric(metric("avg_"+name, expression, semantic.AggAvg, col.Description, nil))
				appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, lex.MetricSynonymList("min_numeric")))
			}
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, lex.MetricSynonymList("max_numeric")))
		case isDateOrTimeType(col.DataType):
			appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, lex.MetricSynonymList("min_date")))
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, lex.MetricSynonymList("max_date")))
		}
	}
	return metrics
}

func metric(name string, expression string, aggregation semantic.AggregationType, description *string, synonyms []string) semantic.Metric {
	return semantic.Metric{
		Name:        name,
		Expression:  expression,
		Aggregation: string(aggregation),
		Description: description,
		Synonyms:    synonyms,
		IsActive:    true,
	}
}

func dimensionType(dataType string) string {
	t := strings.ToLower(dataType)
	switch {
	case strings.Contains(t, "date"), strings.Contains(t, "time"):
		return string(semantic.DimensionTypeDate)
	case strings.Contains(t, "bool"):
		return string(semantic.DimensionTypeBoolean)
	case isNumericType(t):
		return string(semantic.DimensionTypeNumber)
	default:
		return string(semantic.DimensionTypeText)
	}
}

// numericTypeMarkers holds substrings that, when present in a column's
// SQL type, mean the column is numeric. Package-level so the slice is not
// re-allocated on every isNumericType call (this is a hot path inside the
// table router for every column of every routed table).
var numericTypeMarkers = [...]string{
	"int",
	"numeric",
	"decimal",
	"double",
	"float",
	"real",
	"money",
	"number",
}

func isNumericType(dataType string) bool {
	t := strings.ToLower(dataType)
	for _, marker := range numericTypeMarkers {
		if strings.Contains(t, marker) {
			return true
		}
	}
	return false
}

func isDateOrTimeType(dataType string) bool {
	t := strings.ToLower(dataType)
	return strings.Contains(t, "date") || strings.Contains(t, "time")
}

// hasTimeComponent reports whether a date/time column carries clock time, so
// hour-grain bucketing is meaningful. Pure DATE columns get only y/q/m/d
// variants; TIMESTAMP / DATETIME / TIME-typed columns also get _hour.
func hasTimeComponent(dataType string) bool {
	t := strings.ToLower(dataType)
	if strings.Contains(t, "timestamp") || strings.Contains(t, "datetime") {
		return true
	}
	// "time without time zone" / "time with time zone" / "time" — has clock,
	// but no calendar; skip hour as a calendar bucket anyway. Only true
	// "date" rejects.
	if strings.Contains(t, "date") && !strings.Contains(t, "datetime") {
		return false
	}
	return strings.Contains(t, "time")
}
