package dbt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

// ConvertOptions controls dbt → SemanticModel conversion.
type ConvertOptions struct {
	DatasourceID   string
	ExistingNames  []string // model names already present for this datasource
	IncludeSources bool     // when true, also convert dbt sources as models
}

// ConvertResult is the output of converting a ParsedProject.
type ConvertResult struct {
	Models   []*semantic.SemanticModel
	Skipped  []string
	Warnings []string
}

// ConvertProject maps dbt models (and optionally sources) to draft SemanticModels.
func ConvertProject(project *ParsedProject, opts ConvertOptions) *ConvertResult {
	result := &ConvertResult{
		Models:   make([]*semantic.SemanticModel, 0),
		Skipped:  make([]string, 0),
		Warnings: append([]string{}, project.Warnings...),
	}
	if opts.DatasourceID == "" {
		result.Warnings = append(result.Warnings, "datasource_id is required")
		return result
	}

	usedNames := map[string]bool{}
	for _, n := range opts.ExistingNames {
		usedNames[strings.ToLower(n)] = true
	}

	// Index models by unique_id and by bare name for relationship resolution.
	byID := map[string]Node{}
	byName := map[string]Node{}
	for _, m := range project.Models {
		byID[m.UniqueID] = m
		byName[strings.ToLower(m.Name)] = m
	}

	acceptedByModelCol := map[string]map[string][]string{}
	for _, av := range project.AcceptedValues {
		if acceptedByModelCol[av.ModelUniqueID] == nil {
			acceptedByModelCol[av.ModelUniqueID] = map[string][]string{}
		}
		acceptedByModelCol[av.ModelUniqueID][av.Column] = av.Values
	}

	for _, node := range project.Models {
		model, warn := convertNode(node, opts.DatasourceID, usedNames, acceptedByModelCol[node.UniqueID], project)
		if model == nil {
			result.Skipped = append(result.Skipped, node.UniqueID)
			result.Warnings = append(result.Warnings, warn...)
			continue
		}
		result.Models = append(result.Models, model)
		result.Warnings = append(result.Warnings, warn...)
	}

	if opts.IncludeSources {
		for _, src := range project.Sources {
			model, warn := convertSource(src, opts.DatasourceID, usedNames, acceptedByModelCol[src.UniqueID])
			if model == nil {
				result.Skipped = append(result.Skipped, src.UniqueID)
				result.Warnings = append(result.Warnings, warn...)
				continue
			}
			result.Models = append(result.Models, model)
			result.Warnings = append(result.Warnings, warn...)
		}
	}

	// Attach joins from relationships tests onto the from-model when both sides exist.
	attachJoins(result.Models, project.Relationships, byID, byName)

	return result
}

func convertNode(node Node, datasourceID string, usedNames map[string]bool, accepted map[string][]string, project *ParsedProject) (*semantic.SemanticModel, []string) {
	var warnings []string
	baseTable := firstNonEmpty(node.Alias, node.Config.Alias, node.Name)
	baseSchema := firstNonEmpty(node.Schema, node.Config.Schema)
	if baseTable == "" {
		return nil, []string{fmt.Sprintf("skipped model %q: missing table name", node.UniqueID)}
	}

	name := uniqueName(sanitizeName(node.Name), usedNames)
	modelID := uuid.New().String()
	model := &semantic.SemanticModel{
		ID:           modelID,
		DatasourceID: datasourceID,
		Name:         name,
		BaseSchema:   baseSchema,
		BaseTable:    baseTable,
		Synonyms:     synonymsFor(node.Name, node.Tags),
		IsActive:     true,
		Status:       semantic.ModelStatusDraft,
	}
	if node.Description != "" {
		model.Description = new(node.Description)
	}
	label := humanize(node.Name)
	model.Label = new(label)

	cols := sortedColumns(node.Columns)
	if len(cols) == 0 {
		warnings = append(warnings, fmt.Sprintf("model %q has no columns; created header only", node.UniqueID))
	}

	notNull := project.NotNullColumns[node.UniqueID]
	unique := project.UniqueColumns[node.UniqueID]
	dimNames := map[string]bool{}

	for _, col := range cols {
		dimName := uniqueName(sanitizeName(col.Name), dimNames)
		dim := semantic.Dimension{
			ID:        uuid.New().String(),
			ModelID:   modelID,
			Name:      dimName,
			ColumnRef: columnRef(baseSchema, baseTable, col.Name),
			Type:      semanticType(col.DataType),
			Synonyms:  synonymsFor(col.Name, col.Tags),
			IsActive:  true,
			IsDisplay: isDisplayColumn(col.Name),
		}
		if col.Description != "" {
			dim.Description = new(col.Description)
		}
		label := humanize(col.Name)
		dim.Label = new(label)

		if vals := accepted[col.Name]; len(vals) > 0 {
			dim.EnumValues = make([]semantic.EnumMapping, 0, len(vals))
			for i, v := range vals {
				dim.EnumValues = append(dim.EnumValues, semantic.EnumMapping{
					ID:        uuid.New().String(),
					RawValue:  v,
					Label:     v,
					SortOrder: i,
				})
			}
		}

		// Surface uniqueness/not-null as prompt synonyms when both apply (PK-like).
		if unique[col.Name] && notNull[col.Name] {
			dim.Synonyms = append(dim.Synonyms, "primary_key", "id")
		}

		model.Dimensions = append(model.Dimensions, dim)

		if isNumericType(col.DataType) && !looksLikeID(col.Name) {
			for _, agg := range []struct {
				prefix string
				fn     string
			}{
				{"sum_", string(semantic.AggSum)},
				{"avg_", string(semantic.AggAvg)},
			} {
				metricName := uniqueName(agg.prefix+sanitizeName(col.Name), dimNames)
				m := semantic.Metric{
					ID:          uuid.New().String(),
					ModelID:     modelID,
					Name:        metricName,
					Expression:  columnRef(baseSchema, baseTable, col.Name),
					Aggregation: agg.fn,
					Synonyms:    synonymsFor(col.Name, nil),
					IsActive:    true,
				}
				mlabel := humanize(metricName)
				m.Label = new(mlabel)
				model.Metrics = append(model.Metrics, m)
			}
		}
	}

	// Always add a row-count metric.
	countName := uniqueName("count_rows", dimNames)
	model.Metrics = append(model.Metrics, semantic.Metric{
		ID:          uuid.New().String(),
		ModelID:     modelID,
		Name:        countName,
		Expression:  "*",
		Aggregation: string(semantic.AggCount),
		Synonyms:    []string{"count", "row_count"},
		IsActive:    true,
		Label:       new("Count Rows"),
	})

	return model, warnings
}

func convertSource(src Source, datasourceID string, usedNames map[string]bool, accepted map[string][]string) (*semantic.SemanticModel, []string) {
	baseTable := firstNonEmpty(src.Identifier, src.Name)
	if baseTable == "" {
		return nil, []string{fmt.Sprintf("skipped source %q: missing table name", src.UniqueID)}
	}
	name := uniqueName(sanitizeName(src.SourceName+"_"+src.Name), usedNames)
	modelID := uuid.New().String()
	model := &semantic.SemanticModel{
		ID:           modelID,
		DatasourceID: datasourceID,
		Name:         name,
		BaseSchema:   src.Schema,
		BaseTable:    baseTable,
		Synonyms:     synonymsFor(src.Name, src.Tags),
		IsActive:     true,
		Status:       semantic.ModelStatusDraft,
	}
	if src.Description != "" {
		model.Description = new(src.Description)
	}
	label := humanize(src.SourceName + " " + src.Name)
	model.Label = new(label)

	dimNames := map[string]bool{}
	for _, col := range sortedColumns(src.Columns) {
		dimName := uniqueName(sanitizeName(col.Name), dimNames)
		dim := semantic.Dimension{
			ID:        uuid.New().String(),
			ModelID:   modelID,
			Name:      dimName,
			ColumnRef: columnRef(src.Schema, baseTable, col.Name),
			Type:      semanticType(col.DataType),
			Synonyms:  synonymsFor(col.Name, col.Tags),
			IsActive:  true,
			IsDisplay: isDisplayColumn(col.Name),
		}
		if col.Description != "" {
			dim.Description = new(col.Description)
		}
		l := humanize(col.Name)
		dim.Label = new(l)
		if vals := accepted[col.Name]; len(vals) > 0 {
			for i, v := range vals {
				dim.EnumValues = append(dim.EnumValues, semantic.EnumMapping{
					ID:        uuid.New().String(),
					RawValue:  v,
					Label:     v,
					SortOrder: i,
				})
			}
		}
		model.Dimensions = append(model.Dimensions, dim)
	}
	return model, nil
}

func attachJoins(models []*semantic.SemanticModel, rels []RelationshipTest, byID map[string]Node, byName map[string]Node) {
	modelByBase := map[string]*semantic.SemanticModel{}
	for _, m := range models {
		key := strings.ToLower(m.BaseSchema + "." + m.BaseTable)
		modelByBase[key] = m
		modelByBase[strings.ToLower(m.Name)] = m
	}

	resolveNode := func(ref string) (Node, bool) {
		if n, ok := byID[ref]; ok {
			return n, true
		}
		if strings.HasPrefix(ref, "name:") {
			n, ok := byName[strings.ToLower(strings.TrimPrefix(ref, "name:"))]
			return n, ok
		}
		n, ok := byName[strings.ToLower(ref)]
		return n, ok
	}

	for _, rel := range rels {
		fromNode, okFrom := resolveNode(rel.FromUniqueID)
		toNode, okTo := resolveNode(rel.ToUniqueID)
		if !okFrom || !okTo {
			continue
		}
		fromTable := firstNonEmpty(fromNode.Alias, fromNode.Config.Alias, fromNode.Name)
		toTable := firstNonEmpty(toNode.Alias, toNode.Config.Alias, toNode.Name)
		fromModel := modelByBase[strings.ToLower(fromNode.Schema+"."+fromTable)]
		if fromModel == nil {
			fromModel = modelByBase[strings.ToLower(fromNode.Name)]
		}
		if fromModel == nil {
			continue
		}
		joinName := sanitizeName(fromTable + "_to_" + toTable)
		fromModel.Joins = append(fromModel.Joins, semantic.Join{
			ID:           uuid.New().String(),
			ModelID:      fromModel.ID,
			Name:         joinName,
			FromSchema:   fromNode.Schema,
			FromTable:    fromTable,
			FromColumn:   rel.FromColumn,
			ToSchema:     toNode.Schema,
			ToTable:      toTable,
			ToColumn:     rel.ToColumn,
			JoinType:     semantic.DefaultJoinType,
			Relationship: semantic.RelationshipManyToOne,
			IsActive:     true,
		})
	}
}

func sortedColumns(cols map[string]Column) []Column {
	if len(cols) == 0 {
		return nil
	}
	out := make([]Column, 0, len(cols))
	for name, c := range cols {
		if c.Name == "" {
			c.Name = name
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func columnRef(schema, table, column string) string {
	if schema == "" {
		return table + "." + column
	}
	return schema + "." + table + "." + column
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	b.Grow(len(s))
	prevUnderscore := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unnamed"
	}
	return out
}

func uniqueName(base string, used map[string]bool) string {
	name := base
	for i := 2; used[strings.ToLower(name)]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	used[strings.ToLower(name)] = true
	return name
}

func synonymsFor(name string, tags []string) []string {
	out := []string{}
	if name != "" {
		out = append(out, name)
		h := humanize(name)
		if !strings.EqualFold(h, name) {
			out = append(out, h)
		}
	}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func humanize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func isDisplayColumn(name string) bool {
	n := strings.ToLower(name)
	return n == "name" || n == "title" || n == "label" ||
		strings.HasSuffix(n, "_name") || strings.HasSuffix(n, "_title")
}

func looksLikeID(name string) bool {
	n := strings.ToLower(name)
	return n == "id" || strings.HasSuffix(n, "_id") || strings.HasPrefix(n, "id_")
}

func semanticType(dataType string) string {
	switch {
	case isDateType(dataType):
		return string(semantic.DimensionTypeDate)
	case isBooleanType(dataType):
		return string(semantic.DimensionTypeBoolean)
	case isNumericType(dataType):
		return string(semantic.DimensionTypeNumber)
	default:
		return string(semantic.DimensionTypeText)
	}
}

func isNumericType(dataType string) bool {
	t := strings.ToLower(dataType)
	return strings.Contains(t, "int") || strings.Contains(t, "numeric") || strings.Contains(t, "decimal") ||
		strings.Contains(t, "double") || strings.Contains(t, "float") || strings.Contains(t, "real") || strings.Contains(t, "money")
}

func isDateType(dataType string) bool {
	t := strings.ToLower(dataType)
	return strings.Contains(t, "date") || strings.Contains(t, "time")
}

func isBooleanType(dataType string) bool {
	t := strings.ToLower(dataType)
	return t == "bool" || t == "boolean" || strings.Contains(t, "tinyint(1)")
}
