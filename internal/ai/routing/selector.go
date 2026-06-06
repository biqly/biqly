package routing

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func selectManualTables(
	tables []metadata.Table,
	tableScope []string,
) ([]tableBundle, *TableRoutingResult, error) {
	tableIndex := indexTables(tables)
	var selected []tableBundle
	seen := make(map[string]struct{})

	for _, ref := range nonEmptyScope(tableScope) {
		table, err := resolveTableRef(tableIndex, ref)
		if err != nil {
			result := &TableRoutingResult{Manual: true, NeedsClarification: true}
			return nil, result, fmt.Errorf("%w: %w", ErrTableScopeInvalid, err)
		}
		key := tableKey(table.SchemaName, table.TableName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, tableBundle{
			table:        table,
			score:        1,
			keywordScore: 1,
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
	embedBoost map[string]float64,
) ([]tableBundle, *TableRoutingResult, error) {
	return selectAutomaticTablesWithTokens(tables, columnsByTable, tokenSet(question), embedBoost)
}

// selectAutomaticTablesWithTokens is the same logic as selectAutomaticTables
// but accepts pre-computed question tokens so callers that already have them
// (e.g. the outer routing loop) don't re-tokenize the same question twice.
func selectAutomaticTablesWithTokens(
	tables []metadata.Table,
	columnsByTable map[string][]metadata.Column,
	tokens map[string]struct{},
	embedBoost map[string]float64,
) ([]tableBundle, *TableRoutingResult, error) {
	bundles := make([]tableBundle, 0, len(tables))
	for _, table := range tables {
		key := tableKey(table.SchemaName, table.TableName)
		keywordScore := scoreTable(table, columnsByTable[key], tokens)
		embeddingScore := embedBoost[key]
		score := keywordScore + embeddingScore
		bundles = append(bundles, tableBundle{
			table:          table,
			score:          score,
			keywordScore:   keywordScore,
			embeddingScore: embeddingScore,
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
		if bundle.score >= bundles[0].score*activeRoutingWeights().SelectionRelativeThreshold {
			selected = append(selected, bundle)
		}
	}
	lex := activeRoutingLexicon()
	selected = appendTableIfMissing(selected, bundles, tokens, maxAutoSelectedTables, lex.CategoryTableSubstrings)
	selected = appendTableIfMissing(selected, bundles, tokens, maxAutoSelectedTables, lex.ProductCatalogSubstrings)

	result := &TableRoutingResult{
		SelectedTables: bundleLabels(selected),
		Candidates:     candidates,
		Confidence:     confidence,
	}
	return selected, result, nil
}

// appendTableIfMissing ensures category/product breakdown questions pull in a matching
// catalog table even when it ranked just below the score threshold.
func appendTableIfMissing(
	selected []tableBundle,
	bundles []tableBundle,
	tokens map[string]struct{},
	maxN int,
	nameSubstrings []string,
) []tableBundle {
	if !isCategoryOrProductQuestion(tokens) {
		return selected
	}
	for _, b := range selected {
		if tableNameMatchesSubstrings(b.table.TableName, nameSubstrings) {
			return selected
		}
	}
	pick := func() (tableBundle, bool) {
		for _, b := range bundles {
			if b.score == 0 {
				continue
			}
			if !tableNameMatchesSubstrings(b.table.TableName, nameSubstrings) {
				continue
			}
			k := tableKey(b.table.SchemaName, b.table.TableName)
			if bundleSliceContains(selected, k) {
				continue
			}
			return b, true
		}
		return tableBundle{}, false
	}
	if len(selected) < maxN {
		if b, ok := pick(); ok {
			selected = append(selected, b)
		}
		return selected
	}
	if len(selected) < 2 {
		return selected
	}
	if b, ok := pick(); ok {
		selected[len(selected)-1] = b
	}
	return selected
}

// appendQuestionEntityTables pulls in unselected tables whose name (after token
// expansion) matches a question token, following the shortest FK path from the
// already-selected set. This covers "show total sales by customer" — the user
// names an entity but does not say "customer name", so the readable-labels
// resolver does not fire, yet the customers table still needs to be in the
// context for a "by customer" group-by to be sensible.
//
// Conservative: only adds a table when there is an FK path to one of the
// already-selected tables within maxHops, so unrelated entity collisions
// ("show sales" → don't pull in employees) are filtered out.
func isCategoryOrProductQuestion(tokens map[string]struct{}) bool {
	return activeRoutingLexicon().HasAnyToken(tokens, activeRoutingLexicon().CategoryProductTokens)
}

type tableIndex struct {
	byFullName map[string]metadata.Table
	byName     map[string][]metadata.Table
}

func indexTables(tables []metadata.Table) tableIndex {
	idx := tableIndex{
		byFullName: make(map[string]metadata.Table, len(tables)),
		byName:     make(map[string][]metadata.Table, len(tables)),
	}
	for _, table := range tables {
		fullName := tableKey(table.SchemaName, table.TableName)
		idx.byFullName[fullName] = table
		idx.byName[table.TableName] = append(idx.byName[table.TableName], table)
	}
	return idx
}

// validateManualScopeAgainstTypeScope checks manual table refs against the full datasource
// catalog so we return a clear error when the user picks a base table while "views only" (or vice versa),
// even if type filtering would remove all tables.
func validateManualScopeAgainstTypeScope(
	idx tableIndex,
	tableScope []string,
	includeBaseTables bool,
	includeViews bool,
) error {
	for _, ref := range nonEmptyScope(tableScope) {
		table, err := resolveTableRef(idx, ref)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrTableScopeInvalid, err)
		}
		if !tableMatchesTypeScope(table, includeBaseTables, includeViews) {
			return fmt.Errorf("%w: %q is excluded by type scope (base tables vs views)", ErrTableScopeInvalid, ref)
		}
	}
	return nil
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

func groupColumnsByTable(columns []metadata.Column, tableCount int) map[string][]metadata.Column {
	grouped := make(map[string][]metadata.Column, tableCount)
	for _, col := range columns {
		key := tableKey(col.SchemaName, col.TableName)
		grouped[key] = append(grouped[key], col)
	}
	return grouped
}

func columnNameCounts(selected []tableBundle, columnsByTable map[string][]metadata.Column) map[string]int {
	totalCols := 0
	for _, bundle := range selected {
		totalCols += len(columnsByTable[tableKey(bundle.table.SchemaName, bundle.table.TableName)])
	}
	counts := make(map[string]int, totalCols)
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
			Table:          tableLabel(bundle.table),
			Score:          roundScore(bundle.score),
			TotalScore:     roundScore(bundle.score),
			KeywordScore:   roundScore(bundle.keywordScore),
			EmbeddingScore: roundScore(bundle.embeddingScore),
		}
		if bundle.table.Description != nil {
			candidate.Description = *bundle.table.Description
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func roundScore(score float64) float64 {
	return math.Round(score*100) / 100
}

func markSelectedCandidates(candidates []TableCandidate, selectedTables []string) {
	selected := make(map[string]struct{}, len(selectedTables))
	for _, table := range selectedTables {
		selected[table] = struct{}{}
	}
	for i := range candidates {
		if _, ok := selected[candidates[i].Table]; ok {
			candidates[i].Selected = true
			continue
		}
		candidates[i].RejectedReason = "not selected for final connected context"
	}
}

func eliminatedCandidateLabels(candidates []TableCandidate) []string {
	var out []string
	for _, candidate := range candidates {
		if !candidate.Selected {
			out = append(out, candidate.Table)
		}
	}
	return out
}

func applyModelContextToRouting(
	result *TableRoutingResult,
	model *semantic.SemanticModel,
	contextSource string,
	updatedAt *time.Time,
) {
	if result == nil || model == nil {
		return
	}
	result.ContextSource = contextSource
	result.ContextKey = model.ID
	if result.ContextKey == "" {
		result.ContextKey = model.Name
	}
	if model.Name != "" {
		result.SelectedModels = []string{model.Name}
	}
	if updatedAt != nil {
		result.ContextUpdatedAt = updatedAt
	} else if !model.UpdatedAt.IsZero() {
		result.ContextUpdatedAt = new(model.UpdatedAt)
	}
	result.SelectedDimensions = dimensionNames(model.Dimensions)
	result.SelectedMetrics = metricNames(model.Metrics)
}

func dimensionNames(dimensions []semantic.Dimension) []string {
	names := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		names = append(names, dimension.Name)
	}
	return names
}

func metricNames(metrics []semantic.Metric) []string {
	names := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		names = append(names, metric.Name)
	}
	return names
}

func bundleKeySet(bundles []tableBundle) map[string]struct{} {
	keys := make(map[string]struct{}, len(bundles))
	for _, bundle := range bundles {
		keys[tableKey(bundle.table.SchemaName, bundle.table.TableName)] = struct{}{}
	}
	return keys
}

func addedBundleLabels(before map[string]struct{}, bundles []tableBundle) []string {
	var added []string
	for _, bundle := range bundles {
		label := tableLabel(bundle.table)
		if _, ok := before[label]; ok {
			continue
		}
		added = append(added, label)
	}
	return added
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

func columnKey(schemaName, tableName, columnName string) string {
	return schemaName + "." + tableName + "." + columnName
}

func tableMatchesTypeScope(table metadata.Table, includeBase, includeViews bool) bool {
	typ := strings.ToUpper(strings.TrimSpace(table.TableType))
	switch typ {
	case "VIEW":
		return includeViews
	case "BASE TABLE":
		return includeBase
	default:
		// Empty or unknown: treat like a physical table (older syncs).
		return includeBase
	}
}

func filterTablesByTypeScope(tables []metadata.Table, includeBase, includeViews bool) []metadata.Table {
	out := make([]metadata.Table, 0, len(tables))
	for _, t := range tables {
		if tableMatchesTypeScope(t, includeBase, includeViews) {
			out = append(out, t)
		}
	}
	return out
}

func filterColumnsForTables(columns []metadata.Column, tables []metadata.Table) []metadata.Column {
	allowed := make(map[string]struct{}, len(tables))
	for _, t := range tables {
		allowed[tableKey(t.SchemaName, t.TableName)] = struct{}{}
	}
	out := make([]metadata.Column, 0, len(columns))
	for _, c := range columns {
		if _, ok := allowed[tableKey(c.SchemaName, c.TableName)]; ok {
			out = append(out, c)
		}
	}
	return out
}

// TokenSet normalizes text into a set of search tokens (Turkish-aware, with
// light stemming and synonym expansion) used for keyword-based routing. It is
// the single source of truth shared by the table router and the HTTP handlers
// so routing accuracy never diverges between the two.
