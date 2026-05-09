package ai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	autoModelPrefix       = "auto:"
	maxAutoSelectedTables = 3
	minRouteConfidence    = 0.35
	// Limits for auto-generated semantic models (avoids multi-hundred-column explosion in LLM prompts).
	maxAutoModelDimensions = 150
	maxAutoModelMetrics    = 120
)

type bundleColumn struct {
	bundle tableBundle
	col    metadata.Column
}

func columnPriority(c metadata.Column) int {
	switch {
	case c.IsPrimaryKey:
		return 0
	case c.IsForeignKey:
		return 1
	case isDisplayNameColumn(c.ColumnName):
		return 2
	default:
		return 3
	}
}

// sortedBundleColumns returns columns across selected tables in a stable, business-relevant order.
func sortedBundleColumns(selected []tableBundle, columnsByTable map[string][]metadata.Column) []bundleColumn {
	var out []bundleColumn
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
var ErrTableScopeInvalid = errors.New("table scope invalid")

// MetadataReader provides datasource metadata needed by TableRouter.
type MetadataReader interface {
	ListTables(ctx context.Context, datasourceID, schemaName string) ([]metadata.Table, error)
	ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error)
	ListRelations(ctx context.Context, datasourceID string) ([]metadata.Relation, error)
}

// TableRouter selects relevant tables and builds a synthetic semantic model.
type TableRouter struct {
	reader MetadataReader
}

// NewTableRouter creates a metadata-backed table router.
func NewTableRouter(reader MetadataReader) *TableRouter {
	return &TableRouter{reader: reader}
}

// TableCandidate is a scored table candidate returned by automatic routing.
type TableCandidate struct {
	Table       string  `json:"table"`
	Score       float64 `json:"score"`
	Description string  `json:"description,omitempty"`
}

// TableRoutingResult describes the table-routing decision for an AI query.
type TableRoutingResult struct {
	SelectedTables     []string         `json:"selected_tables,omitempty"`
	JoinPaths          []string         `json:"join_paths,omitempty"`
	Candidates         []TableCandidate `json:"candidates,omitempty"`
	Confidence         float64          `json:"confidence"`
	NeedsClarification bool             `json:"needs_clarification"`
	Manual             bool             `json:"manual"`
}

type tableBundle struct {
	table metadata.Table
	score float64
}

// Route selects tables for a question and returns a semantic model over them.
func (r *TableRouter) Route(
	ctx context.Context,
	datasourceID string,
	question string,
	tableScope []string,
) (*semantic.SemanticModel, *TableRoutingResult, error) {
	tables, err := r.reader.ListTables(ctx, datasourceID, "")
	if err != nil {
		return nil, nil, fmt.Errorf("list tables: %w", err)
	}
	columns, err := r.reader.ListColumns(ctx, datasourceID, "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("list columns: %w", err)
	}
	relations, err := r.reader.ListRelations(ctx, datasourceID)
	if err != nil {
		return nil, nil, fmt.Errorf("list relations: %w", err)
	}

	columnsByTable := groupColumnsByTable(columns)
	selected, result, err := r.selectTables(tables, columnsByTable, question, tableScope)
	if err != nil {
		return nil, result, err
	}
	if result.NeedsClarification {
		return nil, result, nil
	}

	connected, joinPaths := connectSelectedTables(selected, relations)
	if result.Manual && len(connected) != len(selected) {
		result.NeedsClarification = true
		return nil, result, fmt.Errorf("%w: selected tables must have direct metadata relations", ErrTableScopeInvalid)
	}
	if len(connected) == 0 {
		result.NeedsClarification = true
		return nil, result, nil
	}
	result.SelectedTables = bundleLabels(connected)
	result.JoinPaths = joinPaths

	model := buildSemanticModel(datasourceID, connected, columnsByTable, relations)
	return model, result, nil
}

func (r *TableRouter) selectTables(
	tables []metadata.Table,
	columnsByTable map[string][]metadata.Column,
	question string,
	tableScope []string,
) ([]tableBundle, *TableRoutingResult, error) {
	if len(tables) == 0 {
		return nil, &TableRoutingResult{
			NeedsClarification: true,
			Confidence:         0,
		}, nil
	}

	if len(nonEmptyScope(tableScope)) > 0 {
		return selectManualTables(tables, tableScope)
	}

	return selectAutomaticTables(tables, columnsByTable, question)
}

func selectManualTables(
	tables []metadata.Table,
	tableScope []string,
) ([]tableBundle, *TableRoutingResult, error) {
	tableIndex := indexTables(tables)
	var selected []tableBundle
	seen := make(map[string]bool)

	for _, ref := range nonEmptyScope(tableScope) {
		table, err := resolveTableRef(tableIndex, ref)
		if err != nil {
			result := &TableRoutingResult{Manual: true, NeedsClarification: true}
			return nil, result, fmt.Errorf("%w: %w", ErrTableScopeInvalid, err)
		}
		key := tableKey(table.SchemaName, table.TableName)
		if seen[key] {
			continue
		}
		seen[key] = true
		selected = append(selected, tableBundle{
			table: table,
			score: 1,
		})
	}

	result := &TableRoutingResult{
		SelectedTables: bundleLabels(selected),
		Confidence:     1,
		Manual:         true,
	}
	return selected, result, nil
}

func selectAutomaticTables(
	tables []metadata.Table,
	columnsByTable map[string][]metadata.Column,
	question string,
) ([]tableBundle, *TableRoutingResult, error) {
	tokens := tokenSet(question)
	bundles := make([]tableBundle, 0, len(tables))
	for _, table := range tables {
		key := tableKey(table.SchemaName, table.TableName)
		score := scoreTable(table, columnsByTable[key], tokens)
		bundles = append(bundles, tableBundle{
			table: table,
			score: score,
		})
	}

	sort.SliceStable(bundles, func(i, j int) bool {
		if bundles[i].score == bundles[j].score {
			return tableLabel(bundles[i].table) < tableLabel(bundles[j].table)
		}
		return bundles[i].score > bundles[j].score
	})

	candidates := topCandidates(bundles, 5)
	confidence := routeConfidence(bundles)
	if len(bundles) == 0 || bundles[0].score == 0 || confidence < minRouteConfidence {
		return nil, &TableRoutingResult{
			Candidates:         candidates,
			Confidence:         confidence,
			NeedsClarification: true,
		}, nil
	}

	selected := []tableBundle{bundles[0]}
	for _, bundle := range bundles[1:] {
		if len(selected) >= maxAutoSelectedTables || bundle.score == 0 {
			break
		}
		if bundle.score >= bundles[0].score*0.30 || bundle.score >= 5 {
			selected = append(selected, bundle)
		}
	}

	result := &TableRoutingResult{
		SelectedTables: bundleLabels(selected),
		Candidates:     candidates,
		Confidence:     confidence,
	}
	return selected, result, nil
}

func scoreTable(table metadata.Table, columns []metadata.Column, tokens map[string]bool) float64 {
	score := weightedTokenScore(tokens, table.SchemaName+" "+table.TableName, 5)
	if table.Description != nil {
		score += weightedTokenScore(tokens, *table.Description, 1.5)
	}
	for _, col := range columns {
		score += weightedTokenScore(tokens, col.ColumnName, 2)
		score += weightedTokenScore(tokens, col.DataType, 0.2)
		if col.Description != nil {
			score += weightedTokenScore(tokens, *col.Description, 1)
		}
		if isRevenueLikeQuestion(tokens) && isRevenueLikeColumn(col) {
			score += 2
		}
	}
	return score
}

func buildSemanticModel(
	datasourceID string,
	selected []tableBundle,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
) *semantic.SemanticModel {
	base := selected[0].table
	label := "Auto-detected tables"
	description := "Generated from datasource metadata for an AI question."
	model := &semantic.SemanticModel{
		ID:           autoModelPrefix + strings.Join(bundleLabels(selected), ","),
		DatasourceID: datasourceID,
		Name:         autoModelPrefix + strings.Join(bundleLabels(selected), ","),
		Label:        &label,
		Description:  &description,
		BaseSchema:   base.SchemaName,
		BaseTable:    base.TableName,
		IsActive:     true,
	}

	model.Dimensions = buildDimensions(selected, columnsByTable)
	model.Metrics = buildMetrics(selected, columnsByTable)
	model.Joins = buildJoins(selected, relations)
	return model
}

func buildDimensions(selected []tableBundle, columnsByTable map[string][]metadata.Column) []semantic.Dimension {
	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)
	if len(pairs) > maxAutoModelDimensions {
		pairs = pairs[:maxAutoModelDimensions]
	}
	dimensions := make([]semantic.Dimension, 0, len(pairs))
	for _, p := range pairs {
		name := p.col.ColumnName
		if nameCounts[p.col.ColumnName] > 1 {
			name = p.bundle.table.TableName + "_" + p.col.ColumnName
		}
		dimensions = append(dimensions, semantic.Dimension{
			Name:        name,
			ColumnRef:   p.bundle.table.TableName + "." + p.col.ColumnName,
			Type:        dimensionType(p.col.DataType),
			Description: p.col.Description,
			Synonyms:    displayNameSynonyms(p.bundle.table.TableName, p.col.ColumnName),
			IsActive:    true,
		})
	}
	return dimensions
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
	for _, syn := range tokenSynonyms[base] {
		out = append(out, add(syn)...)
	}
	return out
}

func isDisplayNameColumn(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "name", "full_name", "fullname", "display_name", "displayname", "title", "label", "username", "email":
		return true
	}
	return strings.HasSuffix(n, "_name")
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

func buildMetrics(selected []tableBundle, columnsByTable map[string][]metadata.Column) []semantic.Metric {
	metrics := []semantic.Metric{{
		Name:        "row_count",
		Expression:  "*",
		Aggregation: string(semantic.AggCount),
		Synonyms:    []string{"count", "rows", "total", "adet", "sayisi", "sayısı", "kaç", "kac"},
		IsActive:    true,
	}}

	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)

	appendMetric := func(m semantic.Metric) {
		if len(metrics) >= maxAutoModelMetrics {
			return
		}
		metrics = append(metrics, m)
	}

	for _, p := range pairs {
		if len(metrics) >= maxAutoModelMetrics {
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
			appendMetric(metric("avg_"+name, expression, semantic.AggAvg, col.Description, nil))
			appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, minNumericSynonyms))
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, maxNumericSynonyms))
		case isDateOrTimeType(col.DataType):
			appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, minDateSynonyms))
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, maxDateSynonyms))
		}
	}
	return metrics
}

var (
	minNumericSynonyms = []string{"min", "minimum", "lowest", "smallest", "en az", "en kucuk"}
	maxNumericSynonyms = []string{"max", "maximum", "highest", "largest", "en cok", "en buyuk"}
	minDateSynonyms    = []string{"earliest", "first", "oldest", "ilk", "en eski", "en erken"}
	maxDateSynonyms    = []string{"latest", "last", "most recent", "newest", "son", "en son", "en yeni", "son tarih"}
)

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

func buildJoins(selected []tableBundle, relations []metadata.Relation) []semantic.Join {
	if len(selected) < 2 {
		return nil
	}
	base := selected[0].table
	selectedKeys := make(map[string]bool)
	for _, bundle := range selected[1:] {
		selectedKeys[tableKey(bundle.table.SchemaName, bundle.table.TableName)] = true
	}

	var joins []semantic.Join
	for _, rel := range relations {
		switch {
		case rel.FromSchema == base.SchemaName && rel.FromTable == base.TableName && selectedKeys[tableKey(rel.ToSchema, rel.ToTable)]:
			joins = append(joins, semantic.Join{
				Name:         rel.ConstraintName,
				FromTable:    rel.FromTable,
				FromColumn:   rel.FromColumn,
				ToTable:      rel.ToTable,
				ToColumn:     rel.ToColumn,
				JoinType:     "LEFT",
				Relationship: rel.RelationshipType,
				IsActive:     true,
			})
		case rel.ToSchema == base.SchemaName && rel.ToTable == base.TableName && selectedKeys[tableKey(rel.FromSchema, rel.FromTable)]:
			joins = append(joins, semantic.Join{
				Name:         rel.ConstraintName,
				FromTable:    rel.ToTable,
				FromColumn:   rel.ToColumn,
				ToTable:      rel.FromTable,
				ToColumn:     rel.FromColumn,
				JoinType:     "LEFT",
				Relationship: rel.RelationshipType,
				IsActive:     true,
			})
		}
	}
	return joins
}

func connectSelectedTables(selected []tableBundle, relations []metadata.Relation) ([]tableBundle, []string) {
	if len(selected) < 2 {
		return selected, nil
	}

	base := selected[0]
	connected := []tableBundle{base}
	var joinPaths []string
	for _, candidate := range selected[1:] {
		rel, ok := directRelation(base.table, candidate.table, relations)
		if !ok {
			continue
		}
		connected = append(connected, candidate)
		joinPaths = append(joinPaths, relationPath(rel))
	}
	return connected, joinPaths
}

func directRelation(
	left metadata.Table,
	right metadata.Table,
	relations []metadata.Relation,
) (metadata.Relation, bool) {
	leftKey := tableKey(left.SchemaName, left.TableName)
	rightKey := tableKey(right.SchemaName, right.TableName)
	for _, rel := range relations {
		fromKey := tableKey(rel.FromSchema, rel.FromTable)
		toKey := tableKey(rel.ToSchema, rel.ToTable)
		if (fromKey == leftKey && toKey == rightKey) || (fromKey == rightKey && toKey == leftKey) {
			return rel, true
		}
	}
	return metadata.Relation{}, false
}

func relationPath(rel metadata.Relation) string {
	return fmt.Sprintf(
		"%s.%s.%s = %s.%s.%s",
		rel.FromSchema,
		rel.FromTable,
		rel.FromColumn,
		rel.ToSchema,
		rel.ToTable,
		rel.ToColumn,
	)
}

type tableIndex struct {
	byFullName map[string]metadata.Table
	byName     map[string][]metadata.Table
}

func indexTables(tables []metadata.Table) tableIndex {
	idx := tableIndex{
		byFullName: make(map[string]metadata.Table, len(tables)),
		byName:     make(map[string][]metadata.Table),
	}
	for _, table := range tables {
		fullName := tableKey(table.SchemaName, table.TableName)
		idx.byFullName[fullName] = table
		idx.byName[table.TableName] = append(idx.byName[table.TableName], table)
	}
	return idx
}

func resolveTableRef(idx tableIndex, ref string) (metadata.Table, error) {
	ref = strings.TrimSpace(ref)
	if table, ok := idx.byFullName[ref]; ok {
		return table, nil
	}
	if matches := idx.byName[ref]; len(matches) == 1 {
		return matches[0], nil
	}
	if matches := idx.byName[ref]; len(matches) > 1 {
		return metadata.Table{}, fmt.Errorf("table %q is ambiguous; use schema.table", ref)
	}
	return metadata.Table{}, fmt.Errorf("table %q not found", ref)
}

func groupColumnsByTable(columns []metadata.Column) map[string][]metadata.Column {
	grouped := make(map[string][]metadata.Column)
	for _, col := range columns {
		key := tableKey(col.SchemaName, col.TableName)
		grouped[key] = append(grouped[key], col)
	}
	return grouped
}

func columnNameCounts(selected []tableBundle, columnsByTable map[string][]metadata.Column) map[string]int {
	counts := make(map[string]int)
	for _, bundle := range selected {
		for _, col := range columnsByTable[tableKey(bundle.table.SchemaName, bundle.table.TableName)] {
			counts[col.ColumnName]++
		}
	}
	return counts
}

func topCandidates(bundles []tableBundle, limit int) []TableCandidate {
	if len(bundles) < limit {
		limit = len(bundles)
	}
	candidates := make([]TableCandidate, 0, limit)
	for _, bundle := range bundles[:limit] {
		candidate := TableCandidate{
			Table: tableLabel(bundle.table),
			Score: math.Round(bundle.score*100) / 100,
		}
		if bundle.table.Description != nil {
			candidate.Description = *bundle.table.Description
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func routeConfidence(bundles []tableBundle) float64 {
	if len(bundles) == 0 || bundles[0].score == 0 {
		return 0
	}
	second := 0.0
	if len(bundles) > 1 {
		second = bundles[1].score
	}
	confidence := bundles[0].score / (bundles[0].score + second + 2)
	return math.Round(confidence*100) / 100
}

func bundleLabels(bundles []tableBundle) []string {
	labels := make([]string, 0, len(bundles))
	for _, bundle := range bundles {
		labels = append(labels, tableLabel(bundle.table))
	}
	return labels
}

func nonEmptyScope(scope []string) []string {
	filtered := make([]string, 0, len(scope))
	for _, ref := range scope {
		if strings.TrimSpace(ref) != "" {
			filtered = append(filtered, strings.TrimSpace(ref))
		}
	}
	return filtered
}

func tableLabel(table metadata.Table) string {
	return tableKey(table.SchemaName, table.TableName)
}

func tableKey(schemaName, tableName string) string {
	return schemaName + "." + tableName
}

func weightedTokenScore(questionTokens map[string]bool, text string, weight float64) float64 {
	textTokens := tokenSet(text)
	score := 0.0
	for token := range questionTokens {
		if textTokens[token] {
			score += weight
		}
	}
	return score
}

func tokenSet(text string) map[string]bool {
	normalized := normalizeText(text)
	tokens := make(map[string]bool)
	for _, token := range strings.Fields(normalized) {
		for _, expanded := range expandToken(token) {
			tokens[expanded] = true
		}
	}
	return tokens
}

func expandToken(token string) []string {
	if token == "" {
		return nil
	}
	expanded := []string{token}
	if strings.HasSuffix(token, "ies") && len(token) > 3 {
		expanded = append(expanded, strings.TrimSuffix(token, "ies")+"y")
	}
	if strings.HasSuffix(token, "s") && len(token) > 3 {
		expanded = append(expanded, strings.TrimSuffix(token, "s"))
	}
	expanded = append(expanded, tokenSynonyms[token]...)
	return expanded
}

func normalizeText(text string) string {
	replacer := strings.NewReplacer(
		"İ", "i",
		"I", "i",
		"ı", "i",
		"Ş", "s",
		"ş", "s",
		"Ğ", "g",
		"ğ", "g",
		"Ü", "u",
		"ü", "u",
		"Ö", "o",
		"ö", "o",
		"Ç", "c",
		"ç", "c",
	)
	text = strings.ToLower(replacer.Replace(text))
	var sb strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			continue
		}
		sb.WriteRune(' ')
	}
	return sb.String()
}

var tokenSynonyms = map[string][]string{
	"adet":     {"count", "row", "rows"},
	"amount":   {"total", "revenue", "sales"},
	"avg":      {"average", "mean", "ortalama"},
	"average":  {"avg", "ortalama"},
	"customer": {"client", "musteri"},
	"gelir":    {"revenue", "sales", "amount", "total"},
	"kac":      {"count", "row", "rows"},
	"musteri":  {"customer", "client"},
	"order":    {"purchase", "sale", "siparis"},
	"ortalama": {"avg", "average"},
	"price":    {"amount", "total", "revenue", "sales"},
	"purchase": {"order", "sale"},
	"revenue":  {"amount", "total", "sales", "price"},
	"sale":     {"order", "revenue", "amount", "total"},
	"sales":    {"order", "revenue", "amount", "total"},
	"satis":    {"sales", "sale", "revenue", "amount", "total"},
	"sayisi":   {"count", "row", "rows"},
	"siparis":  {"order", "sale", "purchase"},
	"total":    {"amount", "revenue", "sales"},
	"urun":     {"product", "item"},
}

func isRevenueLikeQuestion(tokens map[string]bool) bool {
	for _, token := range []string{"revenue", "sales", "sale", "amount", "total", "gelir", "satis"} {
		if tokens[token] {
			return true
		}
	}
	return false
}

func isRevenueLikeColumn(col metadata.Column) bool {
	tokens := tokenSet(col.ColumnName)
	for _, token := range []string{"amount", "total", "price", "revenue", "sales"} {
		if tokens[token] {
			return true
		}
	}
	return false
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

func isNumericType(dataType string) bool {
	t := strings.ToLower(dataType)
	for _, marker := range []string{
		"int",
		"numeric",
		"decimal",
		"double",
		"float",
		"real",
		"money",
		"number",
	} {
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
