package semanticgen

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

type GenerateModelOptions struct {
	DatasourceID   string
	DatasourceName string
	BaseSchema     string
	BaseTable      string
	ExistingNames  []string
	MaxDimensions  int
	MaxMetrics     int
	MaxRelatedDims int
}

type GeneratedModel struct {
	Model    *semantic.SemanticModel `json:"model"`
	Warnings []string                `json:"warnings,omitempty"`
}

func GenerateModelFromMetadata(tables []metadata.Table, columns []metadata.Column, relations []metadata.Relation, opts GenerateModelOptions) (*GeneratedModel, error) {
	if opts.DatasourceID == "" {
		return nil, errors.New("datasource_id is required")
	}
	if len(tables) == 0 {
		return nil, errors.New("metadata has no tables; sync metadata first")
	}
	opts = normalizeGenerateModelOptions(opts)

	base, ok := chooseBaseTable(tables, relations, opts.BaseSchema, opts.BaseTable)
	if !ok {
		return nil, errors.New("base table not found")
	}

	model, dimNames, metricNames := newGeneratedSemanticModel(base, opts)
	selectedTables := selectedTableKeys(tables)
	warnings := make([]string, 0, 4)
	baseCols := filterColumns(columns, base.SchemaName, base.TableName)

	warnings = appendBaseDimensions(model, dimNames, baseCols, base.SchemaName, opts, warnings)
	warnings = appendRelatedDimensions(model, dimNames, columns, base, selectedTables, opts, warnings)
	model.Metrics = appendGeneratedMetrics(model.Metrics, metricNames, baseCols, base.SchemaName, model.ID, opts, &warnings)
	model.Joins = joinsFromRelations(model.ID, relations, selectedTables)

	if len(model.Dimensions) == 0 {
		warnings = append(warnings, "no dimensions could be inferred from metadata")
	}
	return &GeneratedModel{Model: model, Warnings: warnings}, nil
}

func normalizeGenerateModelOptions(opts GenerateModelOptions) GenerateModelOptions {
	if opts.MaxDimensions <= 0 {
		opts.MaxDimensions = 512
	}
	if opts.MaxMetrics <= 0 {
		opts.MaxMetrics = 128
	}
	if opts.MaxRelatedDims <= 0 {
		opts.MaxRelatedDims = 512
	}
	return opts
}

func newGeneratedSemanticModel(base metadata.Table, opts GenerateModelOptions) (*semantic.SemanticModel, map[string]bool, map[string]bool) {
	modelID := uuid.New().String()
	baseNameForModel := strings.TrimSpace(opts.DatasourceName)
	if baseNameForModel == "" {
		baseNameForModel = base.TableName
	}
	modelName := uniqueModelName(normalizeName(baseNameForModel), opts.ExistingNames)
	label := humanLabel(baseNameForModel)
	return &semantic.SemanticModel{
		ID:           modelID,
		DatasourceID: opts.DatasourceID,
		Name:         modelName,
		Label:        &label,
		Description:  base.Description,
		BaseSchema:   base.SchemaName,
		BaseTable:    base.TableName,
		Synonyms:     synonyms(baseNameForModel),
		IsActive:     true,
		Status:       semantic.ModelStatusDraft,
		Dimensions:   make([]semantic.Dimension, 0, opts.MaxDimensions),
		Metrics:      make([]semantic.Metric, 0, opts.MaxMetrics),
		Joins:        make([]semantic.Join, 0),
	}, map[string]bool{}, map[string]bool{}
}

func selectedTableKeys(tables []metadata.Table) map[string]bool {
	selectedTables := make(map[string]bool, len(tables))
	for _, t := range tables {
		selectedTables[tableKey(t.SchemaName, t.TableName)] = true
	}
	return selectedTables
}

func appendBaseDimensions(model *semantic.SemanticModel, dimNames map[string]bool, baseCols []metadata.Column, baseSchema string, opts GenerateModelOptions, warnings []string) []string {
	for _, col := range baseCols {
		if len(model.Dimensions) >= opts.MaxDimensions {
			return append(warnings, "dimension limit reached; some base columns were skipped")
		}
		if shouldSkipDimension(col) {
			continue
		}
		dim := dimensionFromColumn(model.ID, col, baseSchema, dimNames, false)
		model.Dimensions = append(model.Dimensions, dim)
		model.Dimensions = appendDateGrainDimensions(model.Dimensions, model.ID, col, dim, dimNames, opts.MaxDimensions)
	}
	return warnings
}

func appendRelatedDimensions(model *semantic.SemanticModel, dimNames map[string]bool, columns []metadata.Column, base metadata.Table, selectedTables map[string]bool, opts GenerateModelOptions, warnings []string) []string {
	relatedAdded := 0
	for _, col := range columns {
		if relatedAdded >= opts.MaxRelatedDims || len(model.Dimensions) >= opts.MaxDimensions {
			break
		}
		if col.SchemaName == base.SchemaName && col.TableName == base.TableName {
			continue
		}
		if !selectedTables[tableKey(col.SchemaName, col.TableName)] || shouldSkipRelatedDimension(col) {
			continue
		}
		dim := dimensionFromColumn(model.ID, col, base.SchemaName, dimNames, true)
		model.Dimensions = append(model.Dimensions, dim)
		model.Dimensions = appendDateGrainDimensions(model.Dimensions, model.ID, col, dim, dimNames, opts.MaxDimensions)
		relatedAdded++
	}
	return warnings
}

// AppendMissingDimensions returns dimensions that belong to tables already in
// the model (base table + join endpoints) but are not yet defined. It lets an
// existing, hand-edited model pick up dimensions for tables added after the
// initial generation — e.g. a table wired in via a manual relationship — without
// recreating the model and losing manual edits. Existing dimensions are matched
// by (column_ref, time_grain) so re-runs are idempotent.
func AppendMissingDimensions(model *semantic.SemanticModel, columns []metadata.Column, opts GenerateModelOptions) []semantic.Dimension {
	opts = normalizeGenerateModelOptions(opts)
	names := make(map[string]bool, len(model.Dimensions))
	existing := make(map[string]bool, len(model.Dimensions))
	for i := range model.Dimensions {
		d := model.Dimensions[i]
		names[d.Name] = true
		existing[dimDedupeKey(d.ColumnRef, d.TimeGrain)] = true
	}
	modelTables := modelTableKeys(model)
	added := make([]semantic.Dimension, 0)
	for _, col := range columns {
		if len(model.Dimensions)+len(added) >= opts.MaxDimensions {
			break
		}
		if !modelTables[tableKey(col.SchemaName, col.TableName)] {
			continue
		}
		isBase := col.SchemaName == model.BaseSchema && col.TableName == model.BaseTable
		if isBase && shouldSkipDimension(col) {
			continue
		}
		if !isBase && shouldSkipRelatedDimension(col) {
			continue
		}
		dim := dimensionFromColumn(model.ID, col, model.BaseSchema, names, !isBase)
		candidates := appendDateGrainDimensions([]semantic.Dimension{dim}, model.ID, col, dim, names, opts.MaxDimensions)
		for _, c := range candidates {
			key := dimDedupeKey(c.ColumnRef, c.TimeGrain)
			if existing[key] {
				continue
			}
			existing[key] = true
			added = append(added, c)
		}
	}
	return added
}

func dimDedupeKey(columnRef, timeGrain string) string {
	return columnRef + "|" + timeGrain
}

func modelTableKeys(model *semantic.SemanticModel) map[string]bool {
	keys := map[string]bool{tableKey(model.BaseSchema, model.BaseTable): true}
	for _, j := range model.Joins {
		keys[tableKey(j.FromSchema, j.FromTable)] = true
		keys[tableKey(j.ToSchema, j.ToTable)] = true
	}
	return keys
}

func appendGeneratedMetrics(metrics []semantic.Metric, metricNames map[string]bool, baseCols []metadata.Column, baseSchema, modelID string, opts GenerateModelOptions, warnings *[]string) []semantic.Metric {
	metrics = append(metrics, countMetric(modelID, metricNames))
	for _, col := range baseCols {
		if len(metrics) >= opts.MaxMetrics {
			*warnings = append(*warnings, "metric limit reached; some numeric columns were skipped")
			break
		}
		if !isNumericType(col.DataType) || col.IsPrimaryKey || col.IsForeignKey {
			continue
		}
		metrics = append(metrics, metricsFromNumericColumn(modelID, col, baseSchema, metricNames)...)
	}
	return metrics
}

func joinsFromRelations(modelID string, relations []metadata.Relation, selectedTables map[string]bool) []semantic.Join {
	joinNames := map[string]bool{}
	joins := make([]semantic.Join, 0, len(relations))
	for _, rel := range relations {
		if selectedTables[tableKey(rel.FromSchema, rel.FromTable)] && selectedTables[tableKey(rel.ToSchema, rel.ToTable)] {
			joins = append(joins, joinFromRelation(modelID, rel, joinNames))
		}
	}
	return joins
}

func chooseBaseTable(tables []metadata.Table, relations []metadata.Relation, schemaName, tableName string) (metadata.Table, bool) {
	if tableName != "" {
		for _, t := range tables {
			if t.TableName == tableName && (schemaName == "" || t.SchemaName == schemaName) {
				return t, true
			}
		}
		return metadata.Table{}, false
	}
	scores := map[string]int{}
	for _, rel := range relations {
		scores[tableKey(rel.FromSchema, rel.FromTable)] += 2
		scores[tableKey(rel.ToSchema, rel.ToTable)]++
	}
	sorted := slices.Clone(tables)
	slices.SortFunc(sorted, func(a, b metadata.Table) int {
		as := scores[tableKey(a.SchemaName, a.TableName)]
		bs := scores[tableKey(b.SchemaName, b.TableName)]
		if as != bs {
			return bs - as
		}
		if rowEstimate(a) != rowEstimate(b) {
			if rowEstimate(b) > rowEstimate(a) {
				return 1
			}
			return -1
		}
		return strings.Compare(tableKey(a.SchemaName, a.TableName), tableKey(b.SchemaName, b.TableName))
	})
	return sorted[0], true
}

func dimensionFromColumn(modelID string, col metadata.Column, baseSchema string, names map[string]bool, related bool) semantic.Dimension {
	name := normalizeName(col.ColumnName)
	if related {
		name = normalizeName(col.TableName + "_" + col.ColumnName)
	}
	name = uniqueName(name, names)
	label := humanLabel(col.ColumnName)
	return semantic.Dimension{
		ID:          uuid.New().String(),
		ModelID:     modelID,
		Name:        name,
		Label:       &label,
		ColumnRef:   columnRef(col, baseSchema),
		Type:        semanticType(col.DataType),
		Synonyms:    synonyms(col.ColumnName),
		Description: col.Description,
		IsActive:    true,
		IsDisplay:   isDisplayColumn(col.ColumnName),
	}
}

func appendDateGrainDimensions(dimensions []semantic.Dimension, modelID string, col metadata.Column, base semantic.Dimension, names map[string]bool, maxDimensions int) []semantic.Dimension {
	if !isDateType(col.DataType) {
		return dimensions
	}
	// Grain structure (name + suffix) is code; the language-bearing synonym
	// lists come from the NL lexicon (single source shared with routing).
	grains := []struct {
		name   string
		suffix string
	}{
		{"year", "_year"},
		{"quarter", "_quarter"},
		{"month", "_month"},
		{"day", "_day"},
	}
	for _, grain := range grains {
		if len(dimensions) >= maxDimensions {
			break
		}
		labelValue := humanLabel(base.Name + grain.suffix)
		dimensions = append(dimensions, semantic.Dimension{
			ID:          uuid.New().String(),
			ModelID:     modelID,
			Name:        uniqueName(base.Name+grain.suffix, names),
			Label:       &labelValue,
			ColumnRef:   base.ColumnRef,
			Type:        string(semantic.DimensionTypeDate),
			TimeGrain:   grain.name,
			Synonyms:    dedupeStrings(append(synonyms(col.ColumnName+grain.suffix), lexicon.Active().Terms(lexicon.DomainGrainSynonym, grain.name)...)),
			Description: col.Description,
			IsActive:    true,
		})
	}
	return dimensions
}

func countMetric(modelID string, names map[string]bool) semantic.Metric {
	label := "Count"
	return semantic.Metric{
		ID:          uuid.New().String(),
		ModelID:     modelID,
		Name:        uniqueName("count", names),
		Label:       &label,
		Expression:  "*",
		Aggregation: string(semantic.AggCount),
		Synonyms:    lexicon.Active().Terms(lexicon.DomainRowCount, "row_count"),
		IsActive:    true,
	}
}

func metricsFromNumericColumn(modelID string, col metadata.Column, baseSchema string, names map[string]bool) []semantic.Metric {
	base := normalizeName(col.ColumnName)
	label := humanLabel(col.ColumnName)
	expr := columnRef(col, baseSchema)
	format := metricFormat(col.ColumnName)
	sumLabel := "Sum " + label
	avgLabel := "Average " + label
	return []semantic.Metric{
		{
			ID:          uuid.New().String(),
			ModelID:     modelID,
			Name:        uniqueName("sum_"+base, names),
			Label:       &sumLabel,
			Expression:  expr,
			Aggregation: string(semantic.AggSum),
			Format:      format,
			Synonyms:    synonyms(col.ColumnName),
			Description: col.Description,
			IsActive:    true,
		},
		{
			ID:          uuid.New().String(),
			ModelID:     modelID,
			Name:        uniqueName("avg_"+base, names),
			Label:       &avgLabel,
			Expression:  expr,
			Aggregation: string(semantic.AggAvg),
			Format:      format,
			Synonyms:    append(synonyms(col.ColumnName), "average "+strings.ToLower(label), "ortalama "+strings.ToLower(label)),
			Description: col.Description,
			IsActive:    true,
		},
	}
}

func joinFromRelation(modelID string, rel metadata.Relation, names map[string]bool) semantic.Join {
	relationship := rel.RelationshipType
	if relationship == "" {
		relationship = semantic.DefaultRelationshipType
	}
	return semantic.Join{
		ID:           uuid.New().String(),
		ModelID:      modelID,
		Name:         uniqueName(normalizeName(rel.FromTable+"_"+rel.FromColumn+"_to_"+rel.ToTable+"_"+rel.ToColumn), names),
		FromSchema:   rel.FromSchema,
		FromTable:    rel.FromTable,
		FromColumn:   rel.FromColumn,
		ToSchema:     rel.ToSchema,
		ToTable:      rel.ToTable,
		ToColumn:     rel.ToColumn,
		JoinType:     semantic.DefaultJoinType,
		Relationship: relationship,
		IsActive:     true,
	}
}

func filterColumns(columns []metadata.Column, schemaName, tableName string) []metadata.Column {
	out := make([]metadata.Column, 0)
	for _, c := range columns {
		if c.SchemaName == schemaName && c.TableName == tableName {
			out = append(out, c)
		}
	}
	return out
}

func shouldSkipDimension(col metadata.Column) bool {
	name := strings.ToLower(col.ColumnName)
	return strings.Contains(name, "password") || strings.Contains(name, "secret") || strings.Contains(name, "token")
}

func shouldSkipRelatedDimension(col metadata.Column) bool {
	if shouldSkipDimension(col) || col.IsForeignKey {
		return true
	}
	if col.IsPrimaryKey && !isDisplayColumn(col.ColumnName) {
		return true
	}
	return !isTextType(col.DataType) && !isDateType(col.DataType) && !isBooleanType(col.DataType) && !isDisplayColumn(col.ColumnName)
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

func isTextType(dataType string) bool {
	t := strings.ToLower(dataType)
	return strings.Contains(t, "char") || strings.Contains(t, "text") || strings.Contains(t, "uuid") ||
		strings.Contains(t, "json") || strings.Contains(t, "enum")
}

func isDisplayColumn(name string) bool {
	n := strings.ToLower(name)
	return n == "name" || n == "title" || n == "label" || n == "code" || n == "number" ||
		strings.HasSuffix(n, "_name") || strings.HasSuffix(n, "_title") || strings.HasSuffix(n, "_code") ||
		strings.Contains(n, "description")
}

func metricFormat(name string) *string {
	n := strings.ToLower(name)
	if strings.Contains(n, "amount") || strings.Contains(n, "total") || strings.Contains(n, "price") ||
		strings.Contains(n, "cost") || strings.Contains(n, "revenue") {
		return new("#,##0.00")
	}
	return nil
}

func columnRef(col metadata.Column, baseSchema string) string {
	if col.SchemaName != "" && col.SchemaName != baseSchema {
		return col.SchemaName + "." + col.TableName + "." + col.ColumnName
	}
	return col.TableName + "." + col.ColumnName
}

func rowEstimate(t metadata.Table) int64 {
	if t.RowEstimate == nil {
		return 0
	}
	return *t.RowEstimate
}

func tableKey(schemaName, tableName string) string {
	if schemaName == "" {
		return tableName
	}
	return schemaName + "." + tableName
}

var nonNameRunes = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	name = nonNameRunes.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return "field"
	}
	return name
}

func humanLabel(value string) string {
	parts := strings.Fields(strings.ReplaceAll(normalizeName(value), "_", " "))
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func synonyms(name string) []string {
	return dedupeStrings([]string{name, humanLabel(name), strings.ReplaceAll(name, "_", " ")})
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

func uniqueModelName(base string, existing []string) string {
	seen := map[string]bool{}
	for _, name := range existing {
		seen[name] = true
	}
	if !seen[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", base, i)
		if !seen[candidate] {
			return candidate
		}
	}
}

func uniqueName(base string, seen map[string]bool) string {
	if base == "" {
		base = "field"
	}
	if !seen[base] {
		seen[base] = true
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", base, i)
		if !seen[candidate] {
			seen[candidate] = true
			return candidate
		}
	}
}
