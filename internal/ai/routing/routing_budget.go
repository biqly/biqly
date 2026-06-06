package routing

import (
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

func pruneAutoSemanticModel(
	model *semantic.SemanticModel,
	question string,
	limits Limits,
	columnScores map[string]float64,
) {
	if model == nil {
		return
	}
	limits = limits.withDefaults()
	tokens := tokenSet(question)
	countQ := isCountLikeQuestion(question, tokens)

	model.Dimensions = pruneDimensions(model.Dimensions, tokens, columnScores, limits, countQ)
	model.Metrics = pruneMetrics(model.Metrics, tokens, limits, countQ)
}

func isCountLikeQuestion(question string, tokens map[string]struct{}) bool {
	_, okKac := tokens["kaç"]
	_, okAdet := tokens["adet"]
	_, okCount := tokens["count"]
	_, okQuantity := tokens["quantity"]
	if okKac || okAdet || okCount || okQuantity {
		return true
	}
	_, okHow := tokens["how"]
	_, okMany := tokens["many"]
	if okHow && okMany {
		return true
	}
	_, okNumber := tokens["number"]
	_, okOf := tokens["of"]
	if okNumber && okOf {
		return true
	}
	q := strings.ToLower(strings.TrimSpace(question))
	for _, syn := range activeRoutingLexicon().RowCountSynonyms {
		s := strings.ToLower(strings.TrimSpace(syn))
		if s != "" && strings.Contains(q, s) {
			return true
		}
	}
	return false
}

func pruneDimensions(
	dims []semantic.Dimension,
	tokens map[string]struct{},
	columnScores map[string]float64,
	limits Limits,
	countQ bool,
) []semantic.Dimension {
	if len(dims) <= limits.MaxDimensions {
		return dims
	}
	type scored struct {
		d     semantic.Dimension
		score float64
	}
	scoredDims := make([]scored, 0, len(dims))
	for _, d := range dims {
		score := scoreDimensionForPrune(d, tokens, columnScores)
		if dimensionMandatoryForPrune(d, tokens, countQ) {
			score += 1000
		}
		scoredDims = append(scoredDims, scored{d: d, score: score})
	}
	sort.SliceStable(scoredDims, func(i, j int) bool {
		if scoredDims[i].score != scoredDims[j].score {
			return scoredDims[i].score > scoredDims[j].score
		}
		return scoredDims[i].d.Name < scoredDims[j].d.Name
	})
	out := make([]semantic.Dimension, 0, limits.MaxDimensions)
	for _, s := range scoredDims {
		if len(out) >= limits.MaxDimensions {
			break
		}
		out = append(out, s.d)
	}
	return out
}

func dimensionMandatoryForPrune(d semantic.Dimension, tokens map[string]struct{}, countQ bool) bool {
	if strings.HasSuffix(d.Name, "_ts_day") || strings.HasSuffix(d.Name, "_ts_hour") {
		return questionMentionsTimeGrain(tokens)
	}
	if strings.HasSuffix(d.Name, "_year") || strings.HasSuffix(d.Name, "_month") {
		return questionMentionsTimeGrain(tokens)
	}
	colName := d.Name
	if i := strings.LastIndex(d.ColumnRef, "."); i >= 0 && i+1 < len(d.ColumnRef) {
		colName = d.ColumnRef[i+1:]
	}
	if isDisplayNameColumn(colName) && wantsReadableLabelsQuestion(tokens) {
		return true
	}
	if countQ && isDateOrTimeDimension(d) {
		return questionMentionsTimeGrain(tokens)
	}
	return false
}

func isDateOrTimeDimension(d semantic.Dimension) bool {
	return d.Type == "date" || strings.Contains(d.Name, "date") || strings.Contains(d.Name, "_ts_")
}

func questionMentionsTimeGrain(tokens map[string]struct{}) bool {
	for _, w := range []string{
		"dün", "yesterday", "today", "bugün", "week", "hafta", "month", "ay",
		"year", "yıl", "quarter", "daily", "günlük", "hourly", "saat",
	} {
		if _, ok := tokens[w]; ok {
			return true
		}
	}
	return false
}

func scoreDimensionForPrune(d semantic.Dimension, tokens map[string]struct{}, columnScores map[string]float64) float64 {
	score := weightedTokenScore(tokens, d.Name, 4)
	score += weightedTokenScore(tokens, d.ColumnRef, 3)
	for _, syn := range d.Synonyms {
		score += weightedTokenScore(tokens, syn, 2)
	}
	for _, ev := range d.EnumValues {
		score += weightedTokenScore(tokens, ev.Label, 2)
	}
	if d.IsDisplay {
		score += 1
	}
	if parts := strings.SplitN(d.ColumnRef, ".", 2); len(parts) == 2 {
		key := columnKey("", parts[0], parts[1])
		if columnScores != nil {
			score += columnScores[key]
		}
	}
	return score
}

func pruneMetrics(metrics []semantic.Metric, tokens map[string]struct{}, limits Limits, countQ bool) []semantic.Metric {
	if len(metrics) <= limits.MaxMetrics {
		return metrics
	}
	type scored struct {
		m     semantic.Metric
		score float64
	}
	scoredMetrics := make([]scored, 0, len(metrics))
	for _, m := range metrics {
		score := scoreMetricForPrune(m, tokens)
		if metricMandatoryForPrune(m, tokens, countQ) {
			score += 1000
		}
		scoredMetrics = append(scoredMetrics, scored{m: m, score: score})
	}
	sort.SliceStable(scoredMetrics, func(i, j int) bool {
		if scoredMetrics[i].score != scoredMetrics[j].score {
			return scoredMetrics[i].score > scoredMetrics[j].score
		}
		return scoredMetrics[i].m.Name < scoredMetrics[j].m.Name
	})
	out := make([]semantic.Metric, 0, limits.MaxMetrics)
	for _, s := range scoredMetrics {
		if len(out) >= limits.MaxMetrics {
			break
		}
		if countQ && !metricAllowedForCountQuestion(s.m) {
			continue
		}
		out = append(out, s.m)
	}
	if len(out) == 0 {
		for _, m := range metrics {
			if m.Name == "row_count" {
				return []semantic.Metric{m}
			}
		}
	}
	return out
}

func metricMandatoryForPrune(m semantic.Metric, tokens map[string]struct{}, countQ bool) bool {
	if m.Name == "row_count" {
		return true
	}
	if countQ {
		return strings.HasPrefix(m.Name, "min_") || strings.HasPrefix(m.Name, "max_")
	}
	if isRevenueLikeQuestion(tokens) && strings.HasPrefix(m.Name, "sum_") {
		return true
	}
	return false
}

func metricAllowedForCountQuestion(m semantic.Metric) bool {
	if m.Name == "row_count" {
		return true
	}
	if strings.HasPrefix(m.Name, "min_") || strings.HasPrefix(m.Name, "max_") {
		return true
	}
	return false
}

func scoreMetricForPrune(m semantic.Metric, tokens map[string]struct{}) float64 {
	score := weightedTokenScore(tokens, m.Name, 4)
	score += weightedTokenScore(tokens, m.Expression, 3)
	for _, syn := range m.Synonyms {
		score += weightedTokenScore(tokens, syn, 2)
	}
	switch {
	case strings.HasPrefix(m.Name, "sum_"):
		score += weightedTokenScore(tokens, "total", 1)
		score += weightedTokenScore(tokens, "toplam", 1)
	case strings.HasPrefix(m.Name, "avg_"):
		score += weightedTokenScore(tokens, "average", 1)
		score += weightedTokenScore(tokens, "ortalama", 1)
	}
	return score
}
