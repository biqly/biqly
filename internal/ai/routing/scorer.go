package routing

import (
	"strings"
	"unicode"

	"github.com/biqly/biqly/internal/metadata"
)

func scoreTable(table metadata.Table, columns []metadata.Column, tokens map[string]struct{}) float64 {
	w := activeRoutingWeights()
	lex := activeRoutingLexicon()
	score := weightedTokenScore(tokens, table.SchemaName+" "+table.TableName, w.TableName)
	if table.Description != nil {
		score += weightedTokenScore(tokens, *table.Description, w.TableDescription)
	}
	score = w.ApplyTableBoosts(table.TableName, tokens, score, lex)
	revenueQ := lex.HasAnyToken(tokens, lex.RevenueTokens)
	for _, col := range columns {
		score += weightedTokenScore(tokens, col.ColumnName, w.ColumnName)
		score += weightedTokenScore(tokens, col.DataType, w.ColumnDataType)
		if col.Description != nil {
			score += weightedTokenScore(tokens, *col.Description, w.ColumnDescription)
		}
		if revenueQ && isRevenueLikeColumn(col, lex) {
			score += w.RevenueColumnBoost
		}
	}
	if wantsReadableLabelsQuestion(tokens) {
		for _, col := range columns {
			if isDisplayNameColumn(col.ColumnName) {
				score += w.ReadableLabelColumnBoost
				break
			}
		}
	}
	return score
}

func TokenSet(text string) map[string]struct{} {
	return tokenSet(text)
}

// WeightedTokenScore adds weight for every question token that also appears in
// text's token set. Exported so HTTP handlers reuse the exact same scoring as
// the table router.
func WeightedTokenScore(questionTokens map[string]struct{}, text string, weight float64) float64 {
	return weightedTokenScore(questionTokens, text, weight)
}

func weightedTokenScore(questionTokens map[string]struct{}, text string, weight float64) float64 {
	textTokens := tokenSet(text)
	score := 0.0
	for token := range questionTokens {
		if _, ok := textTokens[token]; ok {
			score += weight
		}
	}
	return score
}

func tokenSet(text string) map[string]struct{} {
	normalized := normalizeText(text)
	fields := strings.Fields(normalized)
	tokens := make(map[string]struct{}, len(fields))
	for _, token := range fields {
		for _, expanded := range expandToken(token) {
			tokens[expanded] = struct{}{}
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
	expanded = append(expanded, activeRoutingLexicon().ExpandTokenSynonyms(token)...)
	return expanded
}

// turkishLowerReplacer maps Turkish-specific upper/lowercase variants onto
// ASCII counterparts so downstream keyword and token matching is locale-safe.
// Allocated once at package init to avoid per-call NewReplacer cost.
var turkishLowerReplacer = strings.NewReplacer(
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

func normalizeText(text string) string {
	text = strings.ToLower(turkishLowerReplacer.Replace(text))
	needsNormalization := false
	for _, r := range text {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			needsNormalization = true
			break
		}
	}
	if !needsNormalization {
		return text
	}

	var sb strings.Builder
	sb.Grow(len(text))
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune(' ')
		}
	}
	return sb.String()
}

func isRevenueLikeQuestion(tokens map[string]struct{}) bool {
	lex := activeRoutingLexicon()
	return lex.HasAnyToken(tokens, lex.RevenueTokens)
}

func isRevenueLikeColumn(col metadata.Column, lex *Lexicon) bool {
	return lex.HasAnyToken(tokenSet(col.ColumnName), lex.RevenueColumnTokens)
}

func tableNameMatchesSubstrings(tableName string, substrings []string) bool {
	tn := strings.ToLower(tableName)
	for _, sub := range substrings {
		if strings.Contains(tn, sub) {
			return true
		}
	}
	return false
}
