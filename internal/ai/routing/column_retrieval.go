package routing

import (
	"sort"

	"github.com/biqly/biqly/internal/metadata"
)

// rankColumnsForSemanticModel narrows wide tables to columns relevant to the
// question using embedding similarity (when available) plus keyword overlap.
func rankColumnsForSemanticModel(
	selected []tableBundle,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
	questionTokens map[string]bool,
	columnScores map[string]float64,
	maxColsPerTable int,
) map[string][]metadata.Column {
	if maxColsPerTable <= 0 {
		maxColsPerTable = DefaultRoutingLimits().MaxColumnsPerTable
	}
	tokens := questionTokens
	selectedKeys := make(map[string]bool, len(selected))
	for _, bundle := range selected {
		selectedKeys[tableKey(bundle.table.SchemaName, bundle.table.TableName)] = true
	}
	relationCols := relationColumnsForSelectedTables(relations, selectedKeys)

	out := make(map[string][]metadata.Column, len(columnsByTable))
	for _, bundle := range selected {
		key := tableKey(bundle.table.SchemaName, bundle.table.TableName)
		cols := columnsByTable[key]
		if len(cols) <= minColumnsBeforeRanking {
			out[key] = cols
			continue
		}
		out[key] = rankColumnsForTable(cols, columnScores, relationCols[key], tokens, maxColsPerTable)
	}
	return out
}

func rankColumnsForTable(
	cols []metadata.Column,
	columnScores map[string]float64,
	relationCols map[string]bool,
	tokens map[string]bool,
	maxCols int,
) []metadata.Column {
	if maxCols <= 0 {
		maxCols = maxRankedColumnsPerTable
	}
	type scoredColumn struct {
		col      metadata.Column
		score    float64
		priority int
	}
	kept := make(map[string]bool)
	out := make([]metadata.Column, 0, min(len(cols), maxCols))
	add := func(col metadata.Column) {
		key := columnKey(col.SchemaName, col.TableName, col.ColumnName)
		if kept[key] {
			return
		}
		kept[key] = true
		out = append(out, col)
	}

	var candidates []scoredColumn
	for _, col := range cols {
		if isMandatorySemanticColumn(col, relationCols) {
			add(col)
			continue
		}
		key := columnKey(col.SchemaName, col.TableName, col.ColumnName)
		score := scoreColumnForQuestion(col, tokens, columnScores[key])
		candidates = append(candidates, scoredColumn{
			col:      col,
			score:    score,
			priority: columnPriority(col),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		return candidates[i].col.ColumnName < candidates[j].col.ColumnName
	})
	for _, cand := range candidates {
		if len(out) >= maxCols {
			break
		}
		add(cand.col)
	}
	return out
}

func scoreColumnForQuestion(col metadata.Column, tokens map[string]bool, embeddingScore float64) float64 {
	keyword := scoreColumnKeywords(col, tokens)
	w := activeRoutingWeights()
	if embeddingScore > 0 {
		return embeddingScore + keyword*w.ColumnKeywordMatch
	}
	return keyword
}

func scoreColumnKeywords(col metadata.Column, tokens map[string]bool) float64 {
	w := activeRoutingWeights()
	score := weightedTokenScore(tokens, col.ColumnName, w.ColumnKeywordName)
	if col.Description != nil {
		score += weightedTokenScore(tokens, *col.Description, w.ColumnKeywordDescription)
	}
	if isRevenueLikeQuestion(tokens) && isRevenueLikeColumn(col) {
		score += w.ColumnRevenueBoost
	}
	if isDisplayNameColumn(col.ColumnName) && wantsReadableLabelsQuestion(tokens) {
		score += w.ColumnDisplayNameBoost
	}
	return score
}
