package routing

import (
	"strings"
	"unicode"

	"github.com/biqly/biqly/internal/metadata"
)

func scoreTable(table metadata.Table, columns []metadata.Column, tokens map[string]bool) float64 {
	w := activeRoutingWeights()
	lex := activeRoutingLexicon()
	score := weightedTokenScore(tokens, table.SchemaName+" "+table.TableName, w.TableName)
	if table.Description != nil {
		score += weightedTokenScore(tokens, *table.Description, w.TableDescription)
	}
	score = w.ApplyTableBoosts(table.TableName, tokens, score, lex)
	for _, col := range columns {
		score += weightedTokenScore(tokens, col.ColumnName, w.ColumnName)
		score += weightedTokenScore(tokens, col.DataType, w.ColumnDataType)
		if col.Description != nil {
			score += weightedTokenScore(tokens, *col.Description, w.ColumnDescription)
		}
		if isRevenueLikeQuestion(tokens) && isRevenueLikeColumn(col) {
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

func TokenSet(text string) map[string]bool {
	return tokenSet(text)
}

// WeightedTokenScore adds weight for every question token that also appears in
// text's token set. Exported so HTTP handlers reuse the exact same scoring as
// the table router.
func WeightedTokenScore(questionTokens map[string]bool, text string, weight float64) float64 {
	return weightedTokenScore(questionTokens, text, weight)
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
	fields := strings.Fields(normalized)
	tokens := make(map[string]bool, len(fields))
	for _, token := range fields {
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

func isRevenueLikeQuestion(tokens map[string]bool) bool {
	return activeRoutingLexicon().HasAnyToken(tokens, activeRoutingLexicon().RevenueTokens)
}

func isRevenueLikeColumn(col metadata.Column) bool {
	return activeRoutingLexicon().HasAnyToken(tokenSet(col.ColumnName), activeRoutingLexicon().RevenueColumnTokens)
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
